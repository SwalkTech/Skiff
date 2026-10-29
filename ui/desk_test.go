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

func TestDisconnectedFingerprintPromptIsComplete(t *testing.T) {
	const fingerprint = "SHA256:abcd1234"
	model := &Model{Status: "未连接", Fingerprint: fingerprint}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	want := "确认主机密钥 " + fingerprint
	found := false
	for _, label := range driver.Snapshot().Labels {
		if strings.Contains(label.Value, "secret-password") {
			t.Fatal("指纹确认界面显示了口令")
		}
		if label.Value != want {
			continue
		}
		found = true
		if label.Bounds.Width <= 0 || label.Bounds.Height <= 0 {
			t.Fatal("指纹确认没有可显示的区域")
		}
	}
	if !found {
		t.Fatalf("未完整显示指纹确认: %s", want)
	}
}

func TestDisconnectedStatusIsComplete(t *testing.T) {
	model := &Model{Status: "未连接"}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	found := false
	for _, label := range driver.Snapshot().Labels {
		if strings.Contains(label.Value, "secret-password") {
			t.Fatal("未连接界面显示了口令")
		}
		if label.Value != "未连接" {
			continue
		}
		found = true
		if label.Bounds.Width <= 0 || label.Bounds.Height <= 0 {
			t.Fatal("未连接状态没有可显示的区域")
		}
	}
	if !found {
		t.Fatal("状态栏没有完整显示未连接")
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

	cols, tall := model.Grid.Size()
	if err := driver.Resize(800, 240); err != nil {
		t.Fatalf("缩小窗口失败: %v", err)
	}
	shortCols, shortRows := model.Grid.Size()
	if shortRows >= tall {
		t.Fatalf("区域变矮后行数 = %d，原来 %d", shortRows, tall)
	}
	if shortCols != cols {
		t.Fatalf("只改高度时列数从 %d 变成了 %d", cols, shortCols)
	}
	if err := driver.Resize(800, 720); err != nil {
		t.Fatalf("放大窗口失败: %v", err)
	}
	grownCols, grown := model.Grid.Size()
	if grown <= shortRows {
		t.Fatalf("区域变高后行数 = %d，较矮时 %d", grown, shortRows)
	}
	if grownCols != cols {
		t.Fatalf("只改高度时列数从 %d 变成了 %d", cols, grownCols)
	}
}

func TestBackspaceRemovesErasedCharacter(t *testing.T) {
	grid := term.New(20, 2)
	if _, err := grid.Write([]byte("ab")); err != nil {
		t.Fatalf("写入网格失败: %v", err)
	}
	model := &Model{Grid: grid}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	if _, err := grid.Write([]byte("\b")); err != nil {
		t.Fatalf("退格失败: %v", err)
	}
	if err := driver.Resize(800, 480); err != nil {
		t.Fatalf("退格后刷新失败: %v", err)
	}
	snapshot := driver.Snapshot()
	if hasLabel(snapshot, "b") {
		t.Fatal("退格后界面仍显示已被擦除的字符")
	}
	if !hasLabel(snapshot, "a") {
		t.Fatal("退格后界面没有保留剩余字符")
	}
}

func TestClearScreenRemovesOldText(t *testing.T) {
	grid := term.New(20, 3)
	if _, err := grid.Write([]byte("hello")); err != nil {
		t.Fatalf("写入网格失败: %v", err)
	}
	model := &Model{Grid: grid}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()
	if !hasLabel(driver.Snapshot(), "hello") {
		t.Fatal("清屏前没有显示 hello")
	}

	if _, err := grid.Write([]byte("\x1b[2J")); err != nil {
		t.Fatalf("清屏失败: %v", err)
	}
	if err := driver.Resize(800, 480); err != nil {
		t.Fatalf("清屏后刷新失败: %v", err)
	}
	if hasLabel(driver.Snapshot(), "hello") {
		t.Fatal("清屏后界面仍显示旧字符")
	}
}

func TestCursorMatchesGridCell(t *testing.T) {
	grid := term.New(10, 2)
	if _, err := grid.Write([]byte("ab\r")); err != nil {
		t.Fatalf("写入网格失败: %v", err)
	}
	x, y := grid.Cursor()
	model := &Model{Grid: grid}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	want := withCursor(grid.Row(y), x)
	if !hasLabel(driver.Snapshot(), want) {
		t.Fatalf("界面没有在光标单元格显示 %q", want)
	}
	if []rune(want)[x] != '▍' {
		t.Fatalf("光标标记不在第 %d 列: %q", x, want)
	}
}

func TestTerminalRedrawsOnlyWhenGridChanges(t *testing.T) {
	grid := term.New(20, 3)
	model := &Model{Grid: grid, Screen: "等待连接"}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	first := model.terminalPaints
	if first == 0 {
		t.Fatal("首次绘制没有记录终端重绘")
	}
	if err := driver.Resize(800, 480); err != nil {
		t.Fatalf("相同内容再次绘制失败: %v", err)
	}
	if model.terminalPaints != first {
		t.Fatalf("相同网格再次绘制，重绘次数从 %d 变成 %d", first, model.terminalPaints)
	}
	if _, err := grid.Write([]byte("hello")); err != nil {
		t.Fatalf("写入网格失败: %v", err)
	}
	if err := driver.Resize(800, 480); err != nil {
		t.Fatalf("内容变化后绘制失败: %v", err)
	}
	if model.terminalPaints == first {
		t.Fatal("网格变化后没有重绘终端")
	}
}

func TestGridColsFollowTerminalWidth(t *testing.T) {
	grid := term.New(8, 2)
	model := &Model{Grid: grid}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	wide, rows := model.Grid.Size()
	if err := driver.Resize(320, 480); err != nil {
		t.Fatalf("收窄窗口失败: %v", err)
	}
	narrow, _ := model.Grid.Size()
	if narrow >= wide {
		t.Fatalf("区域变窄后列数 = %d，原来 %d", narrow, wide)
	}
	if err := driver.Resize(960, 480); err != nil {
		t.Fatalf("加宽窗口失败: %v", err)
	}
	grown, grownRows := model.Grid.Size()
	if grown <= narrow {
		t.Fatalf("区域变宽后列数 = %d，较窄时 %d", grown, narrow)
	}
	if grownRows != rows {
		t.Fatalf("只改宽度时行数从 %d 变成了 %d", rows, grownRows)
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
