package ui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/SwalkTech/Skiff/store"
	"github.com/SwalkTech/Skiff/term"
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
	if !hasLabel(driver.Snapshot(), "未连接") {
		t.Fatal("没有会话时状态栏应显示未连接")
	}
}

func TestGridRowsFollowTerminalHeight(t *testing.T) {
	grid := term.New(8, 2)
	model := &Model{Grid: grid}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	_, tall := model.Grid.Size()
	if err := driver.Resize(800, 240); err != nil {
		t.Fatalf("缩小窗口失败: %v", err)
	}
	_, shortRows := model.Grid.Size()
	if shortRows >= tall {
		t.Fatalf("区域变矮后行数 = %d，原来 %d", shortRows, tall)
	}
	if err := driver.Resize(800, 720); err != nil {
		t.Fatalf("放大窗口失败: %v", err)
	}
	cols, grown := model.Grid.Size()
	if grown <= shortRows {
		t.Fatalf("区域变高后行数 = %d，较矮时 %d", grown, shortRows)
	}
	if cols != 8 {
		t.Fatalf("调整行数时列数变成了 %d", cols)
	}
}

func TestGridTextAppearsInTerminal(t *testing.T) {
	grid := term.New(20, 3)
	if _, err := grid.Write([]byte("hello\r\n世界")); err != nil {
		t.Fatalf("写入网格失败: %v", err)
	}
	model := &Model{Grid: grid}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	snapshot := driver.Snapshot()
	if !hasLabel(snapshot, "hello") || !hasLabel(snapshot, "世界") {
		t.Fatal("终端区没有显示网格中的字符")
	}
}

func TestSelectedHostIsDistinct(t *testing.T) {
	model := &Model{
		Hosts: []store.Host{
			{Address: "a.example", User: "ann", Port: 22},
			{Address: "b.example", User: "bob", Port: 22},
		},
		Selected: 1,
	}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	snapshot := driver.Snapshot()
	if !hasLabel(snapshot, "当前 bob@b.example:22") {
		t.Fatal("选中主机没有当前标记")
	}
	if hasLabel(snapshot, "当前 ann@a.example:22") {
		t.Fatal("未选中主机也被标成当前")
	}
}

func TestFailureStaysOnStatusBar(t *testing.T) {
	model := &Model{Status: "未连接"}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	model.Fail("认证失败")
	if err := driver.Resize(800, 480); err != nil {
		t.Fatalf("失败后窗口已不可刷新: %v", err)
	}
	snapshot := driver.Snapshot()
	if !hasLabel(snapshot, "认证失败") {
		t.Fatal("状态栏没有显示失败原因")
	}
	if !hasLabel(driver.Snapshot(), "认证失败") {
		t.Fatal("失败后窗口未能继续显示状态")
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
