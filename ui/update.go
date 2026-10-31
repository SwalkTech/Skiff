package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Version 是当前程序版本。启动后用它和 GitHub 上的最新正式版比较。
const Version = "0.3.1"

const updateAsset = "Skiff-Setup.exe"

func updatePrompt(m *Model) string {
	if m == nil || m.updateDismissed || strings.TrimSpace(m.updateVersion) == "" {
		return ""
	}
	if note := strings.TrimSpace(m.updateNote); note != "" {
		return note
	}
	return "发现新版本 " + m.updateVersion + "，是否更新到最新？"
}

func (m *Model) checkUpdate() {
	version, rawURL, err := fetchLatest(context.Background())
	if err != nil || !newerVersion(Version, version) || !allowedSetupURL(rawURL) {
		return
	}
	m.mu.Lock()
	m.pendingUpdateVersion = version
	m.pendingUpdateURL = rawURL
	m.mu.Unlock()
	m.wakeUp()
}

func (m *Model) applyUpdateNotice() {
	m.mu.Lock()
	version := m.pendingUpdateVersion
	rawURL := m.pendingUpdateURL
	note := m.pendingUpdateNote
	m.pendingUpdateVersion = ""
	m.pendingUpdateURL = ""
	m.pendingUpdateNote = ""
	m.mu.Unlock()
	if version != "" {
		m.updateVersion = version
		m.updateURL = rawURL
	}
	if note != "" {
		m.updateBusy = false
		m.updateNote = note
	}
}

func (m *Model) beginUpdate() {
	if m == nil || m.updateBusy || !allowedSetupURL(m.updateURL) {
		return
	}
	m.updateBusy = true
	m.updateNote = "正在下载更新"
	rawURL := m.updateURL
	go func() {
		path, err := downloadUpdate(context.Background(), rawURL)
		if err != nil {
			m.publishUpdateNote("更新下载失败")
			return
		}
		if err := applyUpdate(path); err != nil {
			m.publishUpdateNote("无法启动安装")
			return
		}
		os.Exit(0)
	}()
}

func (m *Model) publishUpdateNote(note string) {
	m.mu.Lock()
	m.pendingUpdateNote = note
	m.mu.Unlock()
	m.wakeUp()
}

func fetchLatest(ctx context.Context) (string, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/SwalkTech/Skiff/releases/latest", nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", "Skiff/"+Version)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := updateClient().Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("更新检查失败: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", "", err
	}
	return parseLatestRelease(body)
}

func parseLatestRelease(body []byte) (string, string, error) {
	var release struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(body, &release); err != nil {
		return "", "", err
	}
	version := strings.TrimPrefix(strings.TrimSpace(release.TagName), "v")
	for _, asset := range release.Assets {
		if asset.Name == updateAsset && allowedSetupURL(asset.URL) {
			return version, asset.URL, nil
		}
	}
	return "", "", errors.New("发布里没有安装包")
}

func newerVersion(current, latest string) bool {
	cur, ok := parseVersion(current)
	if !ok {
		return false
	}
	next, ok := parseVersion(latest)
	if !ok {
		return false
	}
	for i := 0; i < 3; i++ {
		if next[i] > cur[i] {
			return true
		}
		if next[i] < cur[i] {
			return false
		}
	}
	return false
}

func parseVersion(raw string) ([3]int, bool) {
	raw = strings.TrimPrefix(strings.TrimSpace(raw), "v")
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return [3]int{}, false
	}
	var out [3]int
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return [3]int{}, false
		}
		out[i] = n
	}
	return out, true
}

func allowedSetupURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host != "github.com" || parsed.User != nil {
		return false
	}
	return strings.HasPrefix(parsed.Path, "/SwalkTech/Skiff/releases/download/") && strings.HasSuffix(parsed.Path, "/"+updateAsset)
}

func downloadUpdate(ctx context.Context, rawURL string) (string, error) {
	if !allowedSetupURL(rawURL) {
		return "", errors.New("更新地址不被允许")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Skiff/"+Version)
	resp, err := updateClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("更新下载失败: %s", resp.Status)
	}
	dest := filepath.Join(os.TempDir(), "Skiff-Setup-update.exe")
	file, err := os.Create(dest)
	if err != nil {
		return "", err
	}
	defer file.Close()
	n, err := io.Copy(file, io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return "", err
	}
	if n < 2 {
		return "", errors.New("更新文件不完整")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	header := make([]byte, 2)
	if _, err := io.ReadFull(file, header); err != nil || header[0] != 'M' || header[1] != 'Z' {
		return "", errors.New("更新文件不是程序")
	}
	return dest, nil
}

func updateClient() *http.Client {
	return &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("更新重定向过多")
			}
			if req.URL.Scheme != "https" || !allowedUpdateHost(req.URL.Host) {
				return errors.New("更新重定向不被允许")
			}
			return nil
		},
	}
}

func allowedUpdateHost(host string) bool {
	switch host {
	case "api.github.com", "github.com", "objects.githubusercontent.com", "release-assets.githubusercontent.com", "github-releases.githubusercontent.com":
		return true
	default:
		return false
	}
}
