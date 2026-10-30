package ui

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pkg/sftp"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	vivaui "github.com/vivaui/viva/ui"
)

type remoteFile struct {
	Name string
	Dir  bool
	Size int64
}

type fileResult struct {
	dir   string
	files []remoteFile
	note  string
	err   string
}

func (m *Model) paintFiles(frame *flex.Frame, fg vivaui.Color) {
	frame.Row().Gap(8).Children(func() {
		if frame.Button(upID, "上级").Outline().Small().Clicked() {
			m.fileUp()
		}
		if frame.Button(refreshID, "刷新").Outline().Small().Clicked() {
			m.refreshFiles()
		}
		if frame.Button(uploadID, "上传").Small().Clicked() {
			m.uploadFiles()
		}
		if frame.Button(downloadID, "下载").Outline().Small().Clicked() {
			m.downloadFile()
		}
		if frame.Button(deleteFileID, "删除").DestructiveOutline().Small().Clicked() {
			m.deleteFile()
		}
	})
	dir := m.remoteDir
	if dir == "" {
		dir = "远程目录"
	}
	frame.Text(dir).Color(fg)
	if note := strings.TrimSpace(m.fileNote); note != "" {
		frame.Text(note).Color(vivaui.RGB(160, 168, 176))
	}
	frame.ScrollArea(fileListID).Grow(1).Gap(4).Children(func() {
		if !m.connected {
			frame.Text("请先连接").Color(vivaui.RGB(160, 168, 176))
			return
		}
		if len(m.files) == 0 {
			frame.Text("这个目录是空的").Color(vivaui.RGB(160, 168, 176))
			return
		}
		for i, file := range m.files {
			selected := i == m.filePick
			label := file.Name
			if file.Dir {
				label += "/"
			}
			row := frame.Row().FillWidth().Pad(8).Gap(12).Radius(8)
			if selected {
				row.Background(vivaui.RGB(28, 36, 48))
			}
			row.Hit(fileRows.At(i), file.Name)
			if row.Children(func() {
				name := frame.Text(label).Color(fg)
				if selected || file.Dir {
					name.Bold()
				}
				frame.Spacer()
				if !file.Dir {
					frame.Text(formatSize(file.Size)).Color(vivaui.RGB(140, 148, 156))
				}
			}).Clicked() {
				if file.Dir && m.filePick == i {
					m.enterDir(file.Name)
					return
				}
				m.filePick = i
			}
		}
	})
}

func (m *Model) openFiles() {
	if !m.connected {
		m.fileNote = "请先连接"
		return
	}
	if m.remoteDir == "" {
		m.remoteDir = "."
	}
	m.refreshFiles()
}

func (m *Model) fileUp() {
	if !m.connected {
		m.fileNote = "请先连接"
		return
	}
	dir := m.remoteDir
	if dir == "" || dir == "." || dir == "/" {
		m.remoteDir = "/"
	} else {
		m.remoteDir = path.Dir(dir)
	}
	m.refreshFiles()
}

func (m *Model) enterDir(name string) {
	if m.remoteDir == "" || m.remoteDir == "." {
		m.remoteDir = name
	} else {
		m.remoteDir = path.Join(m.remoteDir, name)
	}
	m.filePick = -1
	m.refreshFiles()
}

func (m *Model) refreshFiles() {
	if !m.connected {
		m.fileNote = "请先连接"
		return
	}
	dir := m.remoteDir
	if dir == "" {
		dir = "."
	}
	m.fileNote = "正在读取"
	go m.loadFiles(dir)
}

func (m *Model) loadFiles(dir string) {
	client, err := m.filesClient()
	if err != nil {
		m.publishFiles(fileResult{dir: dir, err: err.Error()})
		return
	}
	if dir == "." {
		if wd, err := client.Getwd(); err == nil && wd != "" {
			dir = wd
		}
	}
	infos, err := client.ReadDir(dir)
	if err != nil {
		m.publishFiles(fileResult{dir: dir, err: err.Error()})
		return
	}
	files := make([]remoteFile, 0, len(infos))
	for _, info := range infos {
		name := info.Name()
		if name == "." || name == ".." {
			continue
		}
		files = append(files, remoteFile{Name: name, Dir: info.IsDir(), Size: info.Size()})
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].Dir != files[j].Dir {
			return files[i].Dir
		}
		return files[i].Name < files[j].Name
	})
	m.publishFiles(fileResult{dir: dir, files: files, note: fmt.Sprintf("%d 项", len(files))})
}

func (m *Model) uploadFiles() {
	if !m.connected {
		m.fileNote = "请先连接"
		return
	}
	paths, err := app.OpenFiles(app.OpenFileOptions{Title: "上传到远程目录", AllowMultiple: true})
	if err != nil {
		if !errorsIsCanceled(err) {
			m.fileNote = err.Error()
		}
		return
	}
	dir := m.remoteDir
	go func() {
		client, err := m.filesClient()
		if err != nil {
			m.publishFiles(fileResult{dir: dir, err: err.Error()})
			return
		}
		var failed string
		for _, local := range paths {
			if err := uploadFile(client, dir, local); err != nil {
				failed = filepath.Base(local) + ": " + err.Error()
				break
			}
		}
		if failed != "" {
			m.publishFiles(fileResult{dir: dir, err: failed})
			return
		}
		m.loadFiles(dir)
	}()
}

func (m *Model) downloadFile() {
	file, ok := m.pickedFile()
	if !ok || file.Dir {
		m.fileNote = "请选择一个文件"
		return
	}
	localDir, err := app.OpenDirectory(app.OpenDirectoryOptions{Title: "下载到本地目录"})
	if err != nil {
		if !errorsIsCanceled(err) {
			m.fileNote = err.Error()
		}
		return
	}
	dir := m.remoteDir
	remote := path.Join(dir, file.Name)
	go func() {
		client, err := m.filesClient()
		if err != nil {
			m.fileNote = err.Error()
			m.wakeUp()
			return
		}
		dest := filepath.Join(localDir, file.Name)
		if err := downloadFile(client, remote, dest); err != nil {
			m.publishFiles(fileResult{dir: dir, err: err.Error()})
			return
		}
		m.loadFiles(dir)
	}()
}

func (m *Model) deleteFile() {
	file, ok := m.pickedFile()
	if !ok {
		m.fileNote = "请选择一个文件"
		return
	}
	dir := m.remoteDir
	target := path.Join(dir, file.Name)
	go func() {
		client, err := m.filesClient()
		if err != nil {
			m.publishFiles(fileResult{dir: dir, err: err.Error()})
			return
		}
		var removeErr error
		if file.Dir {
			removeErr = client.RemoveDirectory(target)
		} else {
			removeErr = client.Remove(target)
		}
		if removeErr != nil {
			m.publishFiles(fileResult{dir: dir, err: removeErr.Error()})
			return
		}
		m.loadFiles(dir)
	}()
}

func (m *Model) pickedFile() (remoteFile, bool) {
	if m.filePick < 0 || m.filePick >= len(m.files) {
		return remoteFile{}, false
	}
	return m.files[m.filePick], true
}

func (m *Model) filesClient() (*sftp.Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sftp != nil {
		return m.sftp, nil
	}
	if m.client == nil {
		return nil, errors.New("未连接")
	}
	raw, err := m.client.Conn()
	if err != nil {
		return nil, err
	}
	client, err := sftp.NewClient(raw)
	if err != nil {
		return nil, err
	}
	m.sftp = client
	return client, nil
}

func (m *Model) closeFiles() {
	m.mu.Lock()
	client := m.sftp
	m.sftp = nil
	m.mu.Unlock()
	if client != nil {
		client.Close()
	}
	m.showFiles = false
	m.files = nil
	m.filePick = -1
	m.remoteDir = ""
}

func (m *Model) publishFiles(result fileResult) {
	m.mu.Lock()
	m.pendingFiles = &result
	m.mu.Unlock()
	m.wakeUp()
}

func (m *Model) applyFiles() {
	m.mu.Lock()
	result := m.pendingFiles
	m.pendingFiles = nil
	m.mu.Unlock()
	if result == nil {
		return
	}
	if result.dir != "" {
		m.remoteDir = result.dir
	}
	if result.files != nil {
		m.files = result.files
		if m.filePick >= len(m.files) {
			m.filePick = -1
		}
	}
	if result.err != "" {
		m.fileNote = result.err
		return
	}
	m.fileNote = result.note
}

func uploadFile(client *sftp.Client, dir, local string) error {
	src, err := os.Open(local)
	if err != nil {
		return err
	}
	defer src.Close()
	remote := path.Join(dir, filepath.Base(local))
	dst, err := client.Create(remote)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}

func downloadFile(client *sftp.Client, remote, local string) error {
	src, err := client.Open(remote)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(local, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}

func formatSize(size int64) string {
	switch {
	case size < 1024:
		return fmt.Sprintf("%d B", size)
	case size < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
	}
}

func errorsIsCanceled(err error) bool {
	return errors.Is(err, app.ErrCanceled)
}
