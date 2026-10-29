package store

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
)

// ErrInvalidHost 表示主机缺少地址、用户或有效端口。
var ErrInvalidHost = errors.New("主机缺少地址、用户或 1 到 65535 之间的端口")

// ErrExists 表示相同地址、用户和端口的主机已经存在。
var ErrExists = errors.New("主机已存在")

// Host 是一台可连接的 SSH 主机。口令不属于这里。
type Host struct {
	Address string `json:"address"`
	User    string `json:"user"`
	Port    int    `json:"port"`
}

// File 把主机清单保存为 JSON 文件。文件内容不是机密。
type File struct {
	path string
}

// Open 打开路径上的主机清单。文件尚不存在时视为空清单。
func Open(path string) *File {
	return &File{path: path}
}

// List 返回当前保存的主机。调用方得到的是副本。
func (f *File) List() ([]Host, error) {
	hosts, err := f.load()
	if err != nil {
		return nil, err
	}
	out := make([]Host, len(hosts))
	copy(out, hosts)
	return out, nil
}

// Add 校验并追加一台主机，然后立即写回文件。
func (f *File) Add(host Host) error {
	host.Address = strings.TrimSpace(host.Address)
	host.User = strings.TrimSpace(host.User)
	if host.Address == "" || host.User == "" || host.Port < 1 || host.Port > 65535 {
		return ErrInvalidHost
	}
	hosts, err := f.load()
	if err != nil {
		return err
	}
	for _, existing := range hosts {
		if sameHost(existing, host) {
			return ErrExists
		}
	}
	return f.save(append(hosts, host))
}

// Delete 删除第一台地址、用户和端口都相同的主机。没有匹配项时不报错。
func (f *File) Delete(address, user string, port int) error {
	hosts, err := f.load()
	if err != nil {
		return err
	}
	target := Host{Address: strings.TrimSpace(address), User: strings.TrimSpace(user), Port: port}
	kept := hosts[:0]
	for _, host := range hosts {
		if sameHost(host, target) {
			continue
		}
		kept = append(kept, host)
	}
	if len(kept) == len(hosts) {
		return nil
	}
	return f.save(kept)
}

func (f *File) load() ([]Host, error) {
	data, err := os.ReadFile(f.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, nil
	}
	var hosts []Host
	if err := json.Unmarshal(data, &hosts); err != nil {
		return nil, err
	}
	return hosts, nil
}

func (f *File) save(hosts []Host) error {
	if hosts == nil {
		hosts = []Host{}
	}
	data, err := json.MarshalIndent(hosts, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(f.path, data, 0o600)
}

func sameHost(a, b Host) bool {
	return a.Address == b.Address && a.User == b.User && a.Port == b.Port
}
