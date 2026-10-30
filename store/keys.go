package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// KnownKey 是已接受的主机密钥。口令不属于这里。
type KnownKey struct {
	Address    string `json:"address"`
	Port       int    `json:"port"`
	Authorized string `json:"authorized"`
}

// Keys 把已接受的主机密钥保存为 JSON 文件。
type Keys struct {
	path string
}

// KeysPath 返回主机清单旁边的密钥文件路径。清单路径为空时返回空字符串。
func KeysPath(hostsPath string) string {
	if strings.TrimSpace(hostsPath) == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(hostsPath), "known_hosts.json")
}

// OpenKeys 打开路径上的主机密钥记录。文件不存在时视为空记录。
func OpenKeys(path string) *Keys {
	return &Keys{path: path}
}

// Trusted 返回已保存的授权密钥文本。没有匹配记录时 ok 为假。
func (k *Keys) Trusted(address string, port int) (string, bool) {
	if k == nil || k.path == "" {
		return "", false
	}
	keys, err := k.load()
	if err != nil {
		return "", false
	}
	address = strings.TrimSpace(address)
	for _, item := range keys {
		if item.Address == address && item.Port == port && item.Authorized != "" {
			return item.Authorized, true
		}
	}
	return "", false
}

// Remember 记下这台主机的授权密钥。路径为空时不写文件。
func (k *Keys) Remember(address string, port int, authorized string) error {
	if k == nil || k.path == "" {
		return nil
	}
	address = strings.TrimSpace(address)
	authorized = strings.TrimSpace(authorized)
	if address == "" || authorized == "" || port < 1 || port > 65535 {
		return ErrInvalidHost
	}
	keys, err := k.load()
	if err != nil {
		return err
	}
	next := KnownKey{Address: address, Port: port, Authorized: authorized}
	replaced := false
	for i, item := range keys {
		if item.Address == address && item.Port == port {
			keys[i] = next
			replaced = true
			break
		}
	}
	if !replaced {
		keys = append(keys, next)
	}
	return k.save(keys)
}

func (k *Keys) load() ([]KnownKey, error) {
	data, err := os.ReadFile(k.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, nil
	}
	var keys []KnownKey
	if err := json.Unmarshal(data, &keys); err != nil {
		return nil, err
	}
	return keys, nil
}

func (k *Keys) save(keys []KnownKey) error {
	if keys == nil {
		keys = []KnownKey{}
	}
	data, err := json.MarshalIndent(keys, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(k.path, data, 0o600)
}
