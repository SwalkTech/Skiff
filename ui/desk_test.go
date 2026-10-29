package ui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/SwalkTech/Skiff/store"
	"github.com/vivaui/viva/app"
	vivaui "github.com/vivaui/viva/ui"
)

func TestDeskShowsHostAndMonoText(t *testing.T) {
	model := &Model{
		Hosts: []store.Host{{
			Address: "example.com",
			User:    "alice",
			Port:    22,
		}},
		Screen: "Skiff\n等待连接",
	}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	snapshot := driver.Snapshot()
	if !hasLabel(snapshot, "alice@example.com:22") {
		t.Fatal("主机列表没有显示已保存的主机")
	}
	if !hasLabel(snapshot, "Skiff") || !hasLabel(snapshot, "等待连接") {
		t.Fatal("等宽文本区没有显示静态内容")
	}
}

func TestLoadShowsStoredHosts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hosts.json")
	catalog := store.Open(path)
	host := store.Host{Address: "bastion.example", User: "root", Port: 2222}
	if err := catalog.Add(host); err != nil {
		t.Fatalf("保存主机失败: %v", err)
	}

	model, err := Load(path)
	if err != nil {
		t.Fatalf("读取主机清单失败: %v", err)
	}
	if len(model.Hosts) != 1 || model.Hosts[0] != host {
		t.Fatalf("模型中的主机 = %#v", model.Hosts)
	}

	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()
	if !hasLabel(driver.Snapshot(), "root@bastion.example:2222") {
		t.Fatal("界面没有显示清单中的主机")
	}
}

func hasLabel(snapshot app.Snapshot, want string) bool {
	for _, label := range snapshot.Labels {
		if strings.Contains(label.Value, want) {
			return true
		}
	}
	return false
}
