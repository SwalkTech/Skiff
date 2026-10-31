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

func TestPartitionsAndButtons(t *testing.T) {
	model := &Model{Selected: -1, Status: "未连接"}
	driver := newDesk(t, model)
	snapshot := driver.Snapshot()

	host, ok := labelByValue(snapshot, "主机")
	if !ok {
		t.Fatal("没有主机栏")
	}
	terminal, ok := labelByValue(snapshot, "终端")
	if !ok {
		t.Fatal("没有终端栏")
	}
	if host.Bounds.X >= terminal.Bounds.X {
		t.Fatalf("主机栏没有在终端栏左边: 主机 x=%v 终端 x=%v", host.Bounds.X, terminal.Bounds.X)
	}
	if host.Bounds.Width <= 0 || terminal.Bounds.Width <= 0 || host.Bounds.Height <= 0 || terminal.Bounds.Height <= 0 {
		t.Fatal("分区标题没有可显示的区域")
	}
	for _, label := range []string{"添加", "连接", "删除", "还没有主机", "未连接"} {
		if !hasExactLabel(snapshot, label) {
			t.Fatalf("界面缺少 %s", label)
		}
	}
	for _, id := range []vivaui.ID{addID, connectID, deleteID} {
		bounds, ok := hitBounds(snapshot, id)
		if !ok || bounds.Width < 24 || bounds.Height < 16 {
			t.Fatalf("按钮 %v 不可点击: %+v", id, bounds)
		}
	}
}

func TestClickHostSelectsIt(t *testing.T) {
	model := &Model{
		Hosts: []store.Host{
			{Address: "a.example", User: "ann", Port: 22},
			{Address: "b.example", User: "bob", Port: 22},
		},
		Selected: 0,
		Status:   "未连接",
	}
	driver := newDesk(t, model)
	clickID(t, driver, hostRows.ID(vivaui.Index(1)))

	snapshot := driver.Snapshot()
	if !hasLabel(snapshot, "bob@b.example") {
		t.Fatal("点击主机后没有显示选中的地址")
	}
	if hasExactLabel(snapshot, "bob@b.example") {
		t.Fatal("会话区又用大号文字重复了主机")
	}
	if !hasExactLabel(snapshot, "当前") || !hasExactLabel(snapshot, "bob") {
		t.Fatal("被点击的主机没有标成当前")
	}
	if model.Selected != 1 {
		t.Fatalf("选中下标 = %d", model.Selected)
	}
}

func TestRememberedPasswordConnectsWithoutRetyping(t *testing.T) {
	passwords := &memoryPasswords{saved: map[string]string{
		store.PasswordTarget("example.com", "alice", 22): "secret-password",
	}}
	model := &Model{
		Hosts:     []store.Host{{Address: "example.com", User: "alice", Port: 22}},
		Selected:  0,
		Status:    "未连接",
		passwords: passwords,
	}
	model.recallPassword(model.Hosts[0])
	driver := newDesk(t, model)
	if !hasLabel(driver.Snapshot(), "口令已记住，可直接连接") {
		t.Fatal("已记住口令时仍要求重新输入")
	}
	model.password.SetText("")
	clickID(t, driver, connectID)
	if !hasLabel(driver.Snapshot(), "正在连接") {
		t.Fatal("记住口令后没有直接开始连接")
	}
	if hasLabel(driver.Snapshot(), "secret-password") {
		t.Fatal("直接连接时界面显示了口令")
	}
}

func TestConnectResponds(t *testing.T) {
	empty := newDesk(t, &Model{Selected: -1})
	clickID(t, empty, connectID)
	if !hasLabel(empty.Snapshot(), "请先选择一台主机") {
		t.Fatal("没有主机时连接没有响应")
	}

	model := &Model{
		Hosts:    []store.Host{{Address: "example.com", User: "alice", Port: 22}},
		Selected: 0,
		Status:   "未连接",
	}
	driver := newDesk(t, model)
	clickID(t, driver, connectID)
	snapshot := driver.Snapshot()
	if !hasLabel(snapshot, "请输入口令") {
		t.Fatal("没有口令时连接没有要求输入")
	}
	if !hasLabel(snapshot, "alice@example.com:22") {
		t.Fatal("终端栏没有显示要连接的主机")
	}
	if hasLabel(snapshot, "secret-password") {
		t.Fatal("界面显示了口令")
	}
}

func TestAddAndDeleteHost(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hosts.json")
	model, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	driver := newDesk(t, model)
	clickID(t, driver, addID)
	model.address.SetText("example.com")
	model.user.SetText("alice")
	model.port.SetText("2222")
	clickID(t, driver, saveID)

	snapshot := driver.Snapshot()
	if !hasExactLabel(snapshot, "alice") || !hasExactLabel(snapshot, "example.com") || !hasExactLabel(snapshot, "2222") {
		t.Fatal("保存后界面没有显示新主机")
	}
	if !hasLabel(snapshot, "已添加 example.com") {
		t.Fatal("保存后没有状态响应")
	}
	hosts, err := store.Open(path).List()
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 1 || hosts[0].User != "alice" || hosts[0].Port != 2222 {
		t.Fatalf("清单 = %#v", hosts)
	}

	clickID(t, driver, deleteID)
	if !hasLabel(driver.Snapshot(), "已删除 example.com") {
		t.Fatal("删除后没有状态响应")
	}
	hosts, err = store.Open(path).List()
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 0 {
		t.Fatalf("删除后清单 = %#v", hosts)
	}
}

func TestTerminalShowsGrid(t *testing.T) {
	grid := term.New(20, 3)
	if _, err := grid.Write([]byte("hello\r\n世界")); err != nil {
		t.Fatal(err)
	}
	driver := newDesk(t, &Model{Grid: grid, Selected: -1})
	snapshot := driver.Snapshot()
	if !hasLabel(snapshot, "hello") || !hasLabel(snapshot, "世界") {
		t.Fatal("终端区没有显示网格中的字符")
	}
}

func TestLoadAndFailureStayVisible(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hosts.json")
	if err := store.Open(path).Add(store.Host{Address: "bastion.example", User: "root", Port: 2222}); err != nil {
		t.Fatal(err)
	}
	model, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	driver := newDesk(t, model)
	snapshot := driver.Snapshot()
	if !hasExactLabel(snapshot, "root") || !hasExactLabel(snapshot, "bastion.example") || !hasExactLabel(snapshot, "2222") {
		t.Fatal("没有显示清单中的主机")
	}
	if !strings.Contains(snapshot.Title, "bastion.example") || !strings.Contains(snapshot.Title, "未连接") {
		t.Fatalf("窗口标题 = %s", snapshot.Title)
	}

	model.Fail("认证失败")
	if err := driver.Resize(960, 640); err != nil {
		t.Fatal(err)
	}
	if !hasLabel(driver.Snapshot(), "认证失败") {
		t.Fatal("失败原因没有留在界面上")
	}
	if hasLabel(driver.Snapshot(), "secret-password") {
		t.Fatal("界面显示了口令")
	}
}

func newDesk(t *testing.T, model *Model) *app.Driver {
	t.Helper()
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	t.Cleanup(func() { driver.Close() })
	return driver
}

func clickID(t *testing.T, driver *app.Driver, id vivaui.ID) {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == id {
			bounds := hit.Bounds
			if err := driver.Click(bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("没有可点击的控件 %v", id)
}

func hitBounds(snapshot app.Snapshot, id vivaui.ID) (vivaui.Rect, bool) {
	for _, hit := range snapshot.Hits {
		if hit.ID == id {
			return hit.Bounds, true
		}
	}
	return vivaui.Rect{}, false
}

func hasExactLabel(snapshot app.Snapshot, want string) bool {
	_, ok := labelByValue(snapshot, want)
	return ok
}

func labelByValue(snapshot app.Snapshot, want string) (app.Label, bool) {
	for _, label := range snapshot.Labels {
		if label.Value == want {
			return label, true
		}
	}
	return app.Label{}, false
}

func hasLabel(snapshot app.Snapshot, want string) bool {
	for _, label := range snapshot.Labels {
		if strings.Contains(label.Value, want) {
			return true
		}
	}
	return false
}
