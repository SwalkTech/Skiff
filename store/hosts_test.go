package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddListAndReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hosts.json")
	catalog := Open(path)
	host := Host{Address: "example.com", User: "alice", Port: 22}
	if err := catalog.Add(host); err != nil {
		t.Fatalf("添加主机失败: %v", err)
	}

	reloaded := Open(path)
	hosts, err := reloaded.List()
	if err != nil {
		t.Fatalf("重新加载失败: %v", err)
	}
	if len(hosts) != 1 || hosts[0] != host {
		t.Fatalf("读回的主机 = %#v", hosts)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取清单文件失败: %v", err)
	}
	var decoded []map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("解析清单失败: %v", err)
	}
	if _, ok := decoded[0]["password"]; ok {
		t.Fatal("主机清单写入了口令字段")
	}
}

func TestRejectInvalidAndDuplicate(t *testing.T) {
	catalog := Open(filepath.Join(t.TempDir(), "hosts.json"))
	if err := catalog.Add(Host{Address: "example.com", User: "alice", Port: 0}); err != ErrInvalidHost {
		t.Fatalf("无效端口错误 = %v", err)
	}
	if err := catalog.Add(Host{Address: "example.com", User: "alice", Port: 22}); err != nil {
		t.Fatalf("首次添加失败: %v", err)
	}
	if err := catalog.Add(Host{Address: " example.com ", User: " alice ", Port: 22}); err != ErrExists {
		t.Fatalf("重复主机错误 = %v", err)
	}
}

func TestKeyFileStoresPathOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hosts.json")
	catalog := Open(path)
	if err := catalog.Add(Host{Address: "example.com", User: "alice", Port: 22, KeyFile: " C:\\keys\\id_rsa "}); err != nil {
		t.Fatalf("添加主机失败: %v", err)
	}
	hosts, err := catalog.List()
	if err != nil {
		t.Fatalf("列出主机失败: %v", err)
	}
	if hosts[0].KeyFile != "C:\\keys\\id_rsa" {
		t.Fatalf("私钥路径 = %q", hosts[0].KeyFile)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取清单失败: %v", err)
	}
	if strings.Contains(string(raw), "PRIVATE KEY") || strings.Contains(string(raw), "password") {
		t.Fatal("主机清单写入了私钥或口令")
	}
	if err := catalog.SetKeyFile("example.com", "alice", 22, ""); err != nil {
		t.Fatalf("清除私钥路径失败: %v", err)
	}
	hosts, err = catalog.List()
	if err != nil {
		t.Fatalf("重新列出失败: %v", err)
	}
	if hosts[0].KeyFile != "" {
		t.Fatalf("清除后路径 = %q", hosts[0].KeyFile)
	}
}

func TestDelete(t *testing.T) {
	catalog := Open(filepath.Join(t.TempDir(), "hosts.json"))
	if err := catalog.Add(Host{Address: "example.com", User: "alice", Port: 22}); err != nil {
		t.Fatalf("添加主机失败: %v", err)
	}
	if err := catalog.Delete("example.com", "alice", 22); err != nil {
		t.Fatalf("删除主机失败: %v", err)
	}
	hosts, err := catalog.List()
	if err != nil {
		t.Fatalf("列出主机失败: %v", err)
	}
	if len(hosts) != 0 {
		t.Fatalf("删除后仍有主机: %#v", hosts)
	}
}
