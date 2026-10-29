package ui

import (
	"fmt"
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
	if !hasExactLabel(snapshot, "alice") || !hasExactLabel(snapshot, "example.com") || !hasExactLabel(snapshot, "22") {
		t.Fatal("主机列表没有把用户、地址和端口分成三列")
	}
	if hasLabel(snapshot, "alice@example.com:22") {
		t.Fatal("主机字段仍挤在同一段文字里")
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
	if !hasExactLabel(driver.Snapshot(), "root") || !hasExactLabel(driver.Snapshot(), "bastion.example") || !hasExactLabel(driver.Snapshot(), "2222") {
		t.Fatal("界面没有按三列显示清单中的主机")
	}
	if !hasLabel(driver.Snapshot(), "未连接") {
		t.Fatal("没有会话时状态栏应显示未连接")
	}
}

func TestConnectingErrorIsComplete(t *testing.T) {
	model := &Model{Status: "正在连接"}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	model.Fail("认证失败")
	if err := driver.Resize(800, 480); err != nil {
		t.Fatalf("连接失败后刷新失败: %v", err)
	}
	const want = "认证失败"
	found := false
	for _, label := range driver.Snapshot().Labels {
		if strings.Contains(label.Value, "secret-password") {
			t.Fatal("正在连接时的错误提示显示了口令")
		}
		if label.Value != want {
			continue
		}
		found = true
		if label.Bounds.Width <= 0 || label.Bounds.Height <= 0 {
			t.Fatal("正在连接时的错误提示没有可显示的区域")
		}
	}
	if !found {
		t.Fatalf("未完整显示正在连接时的错误提示: %s", want)
	}
}

func TestConnectingFingerprintPromptIsComplete(t *testing.T) {
	const fingerprint = "SHA256:connect1234"
	model := &Model{Status: "正在连接", Fingerprint: fingerprint}
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
			t.Fatal("正在连接的指纹确认显示了口令")
		}
		if label.Value != want {
			continue
		}
		found = true
		if label.Bounds.Width <= 0 || label.Bounds.Height <= 0 {
			t.Fatal("正在连接的指纹确认没有可显示的区域")
		}
	}
	if !found {
		t.Fatalf("未完整显示正在连接时的指纹确认: %s", want)
	}
}

func TestConnectingStatusIsComplete(t *testing.T) {
	model := &Model{Status: "未连接"}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	model.Connecting()
	if err := driver.Resize(800, 480); err != nil {
		t.Fatalf("进入正在连接后刷新失败: %v", err)
	}
	const want = "正在连接"
	found := false
	for _, label := range driver.Snapshot().Labels {
		if strings.Contains(label.Value, "secret-password") {
			t.Fatal("正在连接界面显示了口令")
		}
		if label.Value != want {
			continue
		}
		found = true
		if label.Bounds.Width <= 0 || label.Bounds.Height <= 0 {
			t.Fatal("正在连接状态没有可显示的区域")
		}
	}
	if !found {
		t.Fatal("状态栏没有完整显示正在连接")
	}
}

func TestDisconnectedHoverRowIsDistinct(t *testing.T) {
	model := &Model{
		Status: "未连接",
		Hosts: []store.Host{{
			User:    "alice",
			Address: "example.com",
			Port:    22,
		}},
		Selected: 0,
	}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	snapshot := driver.Snapshot()
	var hit *vivaui.Hit
	for i := range snapshot.Hits {
		if snapshot.Hits[i].Label == "example.com" {
			hit = &snapshot.Hits[i]
			break
		}
	}
	if hit == nil || hit.Bounds.Width <= 0 || hit.Bounds.Height <= 0 {
		t.Fatal("未连接时没有可悬停的主机行")
	}
	x := hit.Bounds.X + hit.Bounds.Width/2
	y := hit.Bounds.Y + hit.Bounds.Height/2
	if err := driver.Move(x, y); err != nil {
		t.Fatalf("悬停主机行失败: %v", err)
	}
	hovered := driver.Snapshot()
	if hovered.Hovered != hit.ID {
		t.Fatal("悬停没有落在主机行上")
	}
	user, ok := labelByValue(hovered, "alice")
	if !ok || user.Bounds.Width <= 0 {
		t.Fatal("悬停后用户列不完整")
	}
	address, ok := labelByValue(hovered, "example.com")
	if !ok || address.Bounds.Width <= 0 {
		t.Fatal("悬停后地址列不完整")
	}
	if overlaps(user.Bounds, address.Bounds) {
		t.Fatal("悬停后主机字段重叠")
	}
	for _, label := range hovered.Labels {
		if strings.Contains(label.Value, "secret-password") {
			t.Fatal("悬停行显示了口令")
		}
	}
}

func TestDisconnectedSelectedRowIsDistinct(t *testing.T) {
	model := &Model{
		Status: "未连接",
		Hosts: []store.Host{
			{User: "ann", Address: "a.example", Port: 22},
			{User: "bob", Address: "b.example", Port: 2222},
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
	current, ok := labelByValue(snapshot, "当前")
	if !ok || current.Bounds.Width <= 0 || current.Bounds.Height <= 0 {
		t.Fatal("未连接时选中行没有可见的当前标记")
	}
	bob, ok := labelByValue(snapshot, "bob")
	if !ok {
		t.Fatal("未连接时看不到选中用户")
	}
	ann, ok := labelByValue(snapshot, "ann")
	if !ok {
		t.Fatal("未连接时看不到未选中用户")
	}
	if bob.Weight <= ann.Weight {
		t.Fatal("未连接时选中行没有比其他行更强调")
	}
	if overlaps(current.Bounds, bob.Bounds) {
		t.Fatal("未连接时当前标记与用户重叠")
	}
	for _, label := range snapshot.Labels {
		if strings.Contains(label.Value, "secret-password") {
			t.Fatal("未连接的选中行显示了口令")
		}
	}
}

func TestDisconnectedHostRowAndColumns(t *testing.T) {
	model := &Model{
		Status: "未连接",
		Hosts: []store.Host{{
			User:    "alice",
			Address: "example.com",
			Port:    22,
		}},
		Selected: 0,
	}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	snapshot := driver.Snapshot()
	fields := []string{"alice", "example.com", "22"}
	var row float32 = -1
	for _, value := range fields {
		label, ok := labelByValue(snapshot, value)
		if !ok || label.Bounds.Width <= 0 || label.Bounds.Height <= 0 {
			t.Fatalf("未连接时字段 %s 不完整", value)
		}
		if row < 0 {
			row = label.Bounds.Y
			continue
		}
		if abs(label.Bounds.Y-row) > 1 {
			t.Fatalf("字段 %s 不在同一主机行", value)
		}
	}
	for _, label := range snapshot.Labels {
		if strings.Contains(label.Value, "secret-password") {
			t.Fatal("未连接的主机行显示了口令")
		}
	}
}

func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func TestDisconnectedHostListIsComplete(t *testing.T) {
	model := &Model{
		Status: "未连接",
		Hosts: []store.Host{{
			User:    "alice",
			Address: "example.com",
			Port:    22,
		}},
		Selected: 0,
	}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	snapshot := driver.Snapshot()
	user, ok := labelByValue(snapshot, "alice")
	if !ok || user.Bounds.Width <= 0 || user.Bounds.Height <= 0 {
		t.Fatal("未连接时用户列不完整")
	}
	address, ok := labelByValue(snapshot, "example.com")
	if !ok || address.Bounds.Width <= 0 || address.Bounds.Height <= 0 {
		t.Fatal("未连接时地址列不完整")
	}
	port, ok := labelByValue(snapshot, "22")
	if !ok || port.Bounds.Width <= 0 || port.Bounds.Height <= 0 {
		t.Fatal("未连接时端口列不完整")
	}
	if overlaps(user.Bounds, address.Bounds) || overlaps(address.Bounds, port.Bounds) || overlaps(user.Bounds, port.Bounds) {
		t.Fatal("未连接时主机列表字段重叠")
	}
	for _, label := range snapshot.Labels {
		if strings.Contains(label.Value, "secret-password") {
			t.Fatal("未连接的主机列表显示了口令")
		}
	}
}

func TestTrailingEmptyCellsAreNotDrawn(t *testing.T) {
	grid := term.New(40, 2)
	if _, err := grid.Write([]byte("a")); err != nil {
		t.Fatalf("写入网格失败: %v", err)
	}
	model := &Model{Grid: grid}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	found := false
	for _, label := range driver.Snapshot().Labels {
		if !strings.Contains(label.Value, "a") || !strings.Contains(label.Value, "▍") {
			continue
		}
		found = true
		for _, symbol := range []string{"·", "□", "▢", "."} {
			if strings.Contains(label.Value, symbol) {
				t.Fatalf("行尾空单元格被画成 %s: %q", symbol, label.Value)
			}
		}
		if len([]rune(label.Value)) > 4 {
			t.Fatalf("行尾空单元格被画出来了: %q", label.Value)
		}
	}
	if !found {
		t.Fatal("没有看到只包含文字和光标的短行")
	}
}

func TestLongAddressTruncatesButStaysAvailable(t *testing.T) {
	const full = "very-long-host-name.internal.example.test"
	model := &Model{
		Hosts: []store.Host{{
			User:    "alice",
			Address: full,
			Port:    22,
		}},
		Selected: 0,
	}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()
	if err := driver.Resize(320, 480); err != nil {
		t.Fatalf("收窄窗口失败: %v", err)
	}

	snapshot := driver.Snapshot()
	truncated := false
	for _, label := range snapshot.Labels {
		if label.Value == full {
			t.Fatal("窄窗口仍完整画出超长地址")
		}
		if strings.HasSuffix(label.Value, "…") && strings.HasPrefix(full, strings.TrimSuffix(label.Value, "…")) {
			truncated = true
		}
	}
	if !truncated {
		t.Fatal("超长地址没有被截断")
	}

	found := false
	for _, hit := range snapshot.Hits {
		if hit.Label != full {
			continue
		}
		found = true
		if hit.Bounds.Width <= 0 || hit.Bounds.Height <= 0 {
			t.Fatal("完整地址没有可查看的区域")
		}
	}
	if !found {
		t.Fatal("无法通过可访问名称查看完整地址")
	}

	user, ok := labelByValue(snapshot, "alice")
	if !ok {
		t.Fatal("窄窗口看不到用户")
	}
	port, ok := labelByValue(snapshot, "22")
	if !ok {
		t.Fatal("窄窗口看不到端口")
	}
	if overlaps(user.Bounds, port.Bounds) {
		t.Fatal("窄窗口里用户和端口重叠")
	}
}

func overlaps(a, b vivaui.Rect) bool {
	return a.X < b.X+b.Width && b.X < a.X+a.Width && a.Y < b.Y+b.Height && b.Y < a.Y+a.Height
}

func TestHostListScrollsToLastItem(t *testing.T) {
	hosts := make([]store.Host, 30)
	for i := range hosts {
		hosts[i] = store.Host{Address: fmt.Sprintf("h%02d.example", i), User: "u", Port: 22}
	}
	model := &Model{Hosts: hosts, Selected: 0}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	state := model.view.ScrollState(hostListID)
	if state.Content <= state.Viewport {
		t.Fatalf("主机内容 %v 没有超过可见高度 %v", state.Content, state.Viewport)
	}
	state.ScrollToEnd()
	if err := driver.Resize(800, 480); err != nil {
		t.Fatalf("滚到末尾后刷新失败: %v", err)
	}
	if !model.view.ScrollState(hostListID).AtEnd() {
		t.Fatal("主机列表没有滚到末尾")
	}
	if !hasExactLabel(driver.Snapshot(), "h29.example") {
		t.Fatal("滚到末尾后看不到最后一台主机")
	}
}

func TestHostListAndTerminalStayVisibleWhenResized(t *testing.T) {
	model := &Model{
		Status: "未连接",
		Hosts:  []store.Host{{User: "alice", Address: "example.com", Port: 22}},
	}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	var shortTerminal float32
	for _, height := range []int{280, 720} {
		if err := driver.Resize(800, height); err != nil {
			t.Fatalf("调整窗口高度 %d 失败: %v", height, err)
		}
		snapshot := driver.Snapshot()
		host, ok := labelByValue(snapshot, "example.com")
		if !ok || host.Bounds.Width <= 0 || host.Bounds.Height <= 0 {
			t.Fatalf("高度 %d 时主机列表不可见", height)
		}
		terminal, ok := labelByValue(snapshot, "等待连接")
		if !ok || terminal.Bounds.Width <= 0 || terminal.Bounds.Height <= 0 {
			t.Fatalf("高度 %d 时终端区不可见", height)
		}
		if host.Bounds.Y+host.Bounds.Height > terminal.Bounds.Y+1 {
			t.Fatalf("高度 %d 时主机列表与终端区重叠", height)
		}
		if height == 280 {
			shortTerminal = model.lastTerminalHeight
		}
		if height == 720 && model.lastTerminalHeight <= shortTerminal {
			t.Fatalf("窗口变高后终端区高度没有增加: %v -> %v", shortTerminal, model.lastTerminalHeight)
		}
	}
}

func TestNoSessionShowsTerminalPlaceholder(t *testing.T) {
	model := &Model{Status: "未连接"}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	const want = "等待连接"
	found := false
	for _, label := range driver.Snapshot().Labels {
		if label.Value != want {
			continue
		}
		found = true
		if label.Bounds.Width <= 0 || label.Bounds.Height <= 0 {
			t.Fatal("等待连接没有可显示的区域")
		}
	}
	if !found {
		t.Fatal("没有会话时终端区没有显示等待连接")
	}
}

func TestConnectingTerminalCellsAreComplete(t *testing.T) {
	grid := term.New(20, 2)
	if _, err := grid.Write([]byte("ab")); err != nil {
		t.Fatalf("写入网格失败: %v", err)
	}
	model := &Model{Status: "正在连接", Grid: grid}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	x, y := grid.Cursor()
	want := withCursor(grid.Row(y), x)
	snapshot := driver.Snapshot()
	label, ok := labelByValue(snapshot, want)
	if !ok || label.Bounds.Width <= 0 || label.Bounds.Height <= 0 {
		t.Fatalf("正在连接时终端单元格不完整: %q", want)
	}
	if !strings.Contains(label.Value, "a") || !strings.Contains(label.Value, "b") {
		t.Fatalf("正在连接时单元格被截断: %q", label.Value)
	}
	for _, item := range snapshot.Labels {
		if strings.Contains(item.Value, "secret-password") {
			t.Fatal("正在连接的终端单元格显示了口令")
		}
	}
}

func TestConnectingTerminalAreaIsComplete(t *testing.T) {
	model := &Model{
		Status: "正在连接",
		Hosts: []store.Host{{
			User:    "alice",
			Address: "example.com",
			Port:    22,
		}},
	}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	snapshot := driver.Snapshot()
	terminal, ok := labelByValue(snapshot, "等待连接")
	if !ok || terminal.Bounds.Width <= 0 || terminal.Bounds.Height <= 0 {
		t.Fatal("正在连接时终端区没有完整显示等待连接")
	}
	host, ok := labelByValue(snapshot, "example.com")
	if !ok {
		t.Fatal("正在连接时看不到主机地址")
	}
	if overlaps(host.Bounds, terminal.Bounds) {
		t.Fatal("正在连接时终端区与主机列表重叠")
	}
	for _, label := range snapshot.Labels {
		if strings.Contains(label.Value, "secret-password") {
			t.Fatal("正在连接的终端区显示了口令")
		}
	}
}

func TestConnectingFilterResults(t *testing.T) {
	model := &Model{
		Status: "正在连接",
		Filter: "bastion",
		Hosts: []store.Host{
			{User: "alice", Address: "app.example", Port: 22},
			{User: "root", Address: "bastion.example", Port: 2222},
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
	user, ok := labelByValue(snapshot, "root")
	if !ok || user.Bounds.Width <= 0 || user.Bounds.Height <= 0 {
		t.Fatal("正在连接时过滤结果没有完整显示匹配用户")
	}
	address, ok := labelByValue(snapshot, "bastion.example")
	if !ok || address.Bounds.Width <= 0 || address.Bounds.Height <= 0 {
		t.Fatal("正在连接时过滤结果没有完整显示匹配地址")
	}
	port, ok := labelByValue(snapshot, "2222")
	if !ok || port.Bounds.Width <= 0 || port.Bounds.Height <= 0 {
		t.Fatal("正在连接时过滤结果没有完整显示匹配端口")
	}
	if overlaps(user.Bounds, address.Bounds) || overlaps(address.Bounds, port.Bounds) {
		t.Fatal("正在连接时过滤结果中的字段重叠")
	}
	if hasExactLabel(snapshot, "alice") || hasExactLabel(snapshot, "app.example") {
		t.Fatal("正在连接时过滤结果包含未匹配的主机")
	}
	for _, label := range snapshot.Labels {
		if strings.Contains(label.Value, "secret-password") {
			t.Fatal("正在连接的过滤结果界面显示了口令")
		}
	}
}

func TestConnectingEmptyListPrompt(t *testing.T) {
	model := &Model{Status: "正在连接", Hosts: nil, Selected: -1}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	snapshot := driver.Snapshot()
	prompt, ok := labelByValue(snapshot, "还没有主机")
	if !ok || prompt.Bounds.Width <= 0 || prompt.Bounds.Height <= 0 {
		t.Fatal("正在连接时没有完整显示还没有主机")
	}
	placeholder, ok := labelByValue(snapshot, "等待连接")
	if ok && overlaps(prompt.Bounds, placeholder.Bounds) {
		t.Fatal("正在连接时的空列表提示与终端占位说明重叠")
	}
	for _, label := range snapshot.Labels {
		if strings.Contains(label.Value, "secret-password") {
			t.Fatal("正在连接的空列表提示显示了口令")
		}
	}
}

func TestConnectingHoverRowIsDistinct(t *testing.T) {
	model := &Model{
		Status: "正在连接",
		Hosts: []store.Host{{
			User:    "alice",
			Address: "example.com",
			Port:    22,
		}},
		Selected: 0,
	}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	snapshot := driver.Snapshot()
	var hit *vivaui.Hit
	for i := range snapshot.Hits {
		if snapshot.Hits[i].Label == "example.com" {
			hit = &snapshot.Hits[i]
			break
		}
	}
	if hit == nil || hit.Bounds.Width <= 0 || hit.Bounds.Height <= 0 {
		t.Fatal("正在连接时没有可悬停的主机行")
	}
	if err := driver.Move(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
		t.Fatalf("悬停主机行失败: %v", err)
	}
	hovered := driver.Snapshot()
	if hovered.Hovered != hit.ID {
		t.Fatal("正在连接时悬停没有落在主机行上")
	}
	user, ok := labelByValue(hovered, "alice")
	if !ok || user.Bounds.Width <= 0 {
		t.Fatal("正在连接时悬停后用户列不完整")
	}
	address, ok := labelByValue(hovered, "example.com")
	if !ok || address.Bounds.Width <= 0 {
		t.Fatal("正在连接时悬停后地址列不完整")
	}
	if overlaps(user.Bounds, address.Bounds) {
		t.Fatal("正在连接时悬停后主机字段重叠")
	}
	for _, label := range hovered.Labels {
		if strings.Contains(label.Value, "secret-password") {
			t.Fatal("正在连接的悬停行显示了口令")
		}
	}
}

func TestConnectingSelectedRowIsDistinct(t *testing.T) {
	model := &Model{
		Status: "正在连接",
		Hosts: []store.Host{
			{User: "ann", Address: "a.example", Port: 22},
			{User: "bob", Address: "b.example", Port: 2222},
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
	current, ok := labelByValue(snapshot, "当前")
	if !ok || current.Bounds.Width <= 0 || current.Bounds.Height <= 0 {
		t.Fatal("正在连接时选中行没有可见的当前标记")
	}
	bob, ok := labelByValue(snapshot, "bob")
	if !ok {
		t.Fatal("正在连接时看不到选中用户")
	}
	ann, ok := labelByValue(snapshot, "ann")
	if !ok {
		t.Fatal("正在连接时看不到未选中用户")
	}
	if bob.Weight <= ann.Weight {
		t.Fatal("正在连接时选中行没有比其他行更强调")
	}
	if overlaps(current.Bounds, bob.Bounds) {
		t.Fatal("正在连接时当前标记与用户重叠")
	}
	for _, label := range snapshot.Labels {
		if strings.Contains(label.Value, "secret-password") {
			t.Fatal("正在连接的选中行显示了口令")
		}
	}
}

func TestConnectingHostRowAndColumns(t *testing.T) {
	model := &Model{
		Status: "正在连接",
		Hosts: []store.Host{{
			User:    "alice",
			Address: "example.com",
			Port:    22,
		}},
		Selected: 0,
	}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	snapshot := driver.Snapshot()
	fields := []string{"alice", "example.com", "22"}
	var row float32 = -1
	for _, value := range fields {
		label, ok := labelByValue(snapshot, value)
		if !ok || label.Bounds.Width <= 0 || label.Bounds.Height <= 0 {
			t.Fatalf("正在连接时字段 %s 不完整", value)
		}
		if row < 0 {
			row = label.Bounds.Y
			continue
		}
		if abs(label.Bounds.Y-row) > 1 {
			t.Fatalf("字段 %s 不在同一主机行", value)
		}
	}
	for _, label := range snapshot.Labels {
		if strings.Contains(label.Value, "secret-password") {
			t.Fatal("正在连接的主机行显示了口令")
		}
	}
}

func TestConnectingHostListIsComplete(t *testing.T) {
	model := &Model{
		Status: "正在连接",
		Hosts: []store.Host{{
			User:    "alice",
			Address: "example.com",
			Port:    22,
		}},
		Selected: 0,
	}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	snapshot := driver.Snapshot()
	user, ok := labelByValue(snapshot, "alice")
	if !ok || user.Bounds.Width <= 0 || user.Bounds.Height <= 0 {
		t.Fatal("正在连接时用户列不完整")
	}
	address, ok := labelByValue(snapshot, "example.com")
	if !ok || address.Bounds.Width <= 0 || address.Bounds.Height <= 0 {
		t.Fatal("正在连接时地址列不完整")
	}
	port, ok := labelByValue(snapshot, "22")
	if !ok || port.Bounds.Width <= 0 || port.Bounds.Height <= 0 {
		t.Fatal("正在连接时端口列不完整")
	}
	if overlaps(user.Bounds, address.Bounds) || overlaps(address.Bounds, port.Bounds) || overlaps(user.Bounds, port.Bounds) {
		t.Fatal("正在连接时主机列表字段重叠")
	}
	for _, label := range snapshot.Labels {
		if strings.Contains(label.Value, "secret-password") {
			t.Fatal("正在连接的主机列表显示了口令")
		}
	}
}

func TestDisconnectedFocusIsComplete(t *testing.T) {
	model := &Model{Status: "未连接", Focus: "终端"}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	snapshot := driver.Snapshot()
	focus, ok := labelByValue(snapshot, "焦点 终端")
	if !ok || focus.Bounds.Width <= 0 || focus.Bounds.Height <= 0 {
		t.Fatal("未连接时没有完整显示焦点指示")
	}
	status, ok := labelByValue(snapshot, "未连接")
	if ok && overlaps(focus.Bounds, status.Bounds) {
		t.Fatal("焦点指示与状态栏重叠")
	}
	for _, label := range snapshot.Labels {
		if strings.Contains(label.Value, "secret-password") {
			t.Fatal("未连接的焦点指示显示了口令")
		}
	}

	empty := &Model{Status: "未连接"}
	emptyDriver, err := app.NewDriver(NewConfig(empty), &canvas)
	if err != nil {
		t.Fatalf("创建默认焦点界面失败: %v", err)
	}
	defer emptyDriver.Close()
	defaultFocus, ok := labelByValue(emptyDriver.Snapshot(), "焦点 主机列表")
	if !ok || defaultFocus.Bounds.Width <= 0 || defaultFocus.Bounds.Height <= 0 {
		t.Fatal("未连接时没有完整显示默认焦点")
	}
}

func TestDisconnectedDeleteConfirmIsComplete(t *testing.T) {
	model := &Model{
		Status:        "未连接",
		ConfirmDelete: true,
		Selected:      0,
		Hosts: []store.Host{{
			User:    "alice",
			Address: "example.com",
			Port:    22,
		}},
	}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	snapshot := driver.Snapshot()
	fields := []string{"删除", "example.com", "alice", "22"}
	var boxes []vivaui.Rect
	var row float32 = -1
	for _, value := range fields {
		label, ok := labelByValue(snapshot, value)
		if !ok || label.Bounds.Width <= 0 || label.Bounds.Height <= 0 {
			t.Fatalf("未连接时删除确认缺少完整字段 %s", value)
		}
		if row < 0 {
			row = label.Bounds.Y
		} else if abs(label.Bounds.Y-row) > 1 {
			t.Fatalf("字段 %s 不在删除确认同一行", value)
		}
		boxes = append(boxes, label.Bounds)
	}
	for i := 0; i < len(boxes); i++ {
		for j := i + 1; j < len(boxes); j++ {
			if overlaps(boxes[i], boxes[j]) {
				t.Fatal("删除确认字段重叠")
			}
		}
	}
	for _, label := range snapshot.Labels {
		if strings.Contains(label.Value, "secret-password") || label.Value == "口令" {
			t.Fatal("未连接的删除确认显示了口令")
		}
	}
}

func TestDisconnectedEditHostFormIsComplete(t *testing.T) {
	model := &Model{
		Status:   "未连接",
		EditHost: true,
		Selected: 0,
		Hosts: []store.Host{{
			User:    "alice",
			Address: "example.com",
			Port:    22,
		}},
	}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	snapshot := driver.Snapshot()
	fields := []string{"example.com", "alice", "22"}
	var boxes []vivaui.Rect
	var row float32 = -1
	for _, value := range fields {
		label, ok := labelByValue(snapshot, value)
		if !ok || label.Bounds.Width <= 0 || label.Bounds.Height <= 0 {
			t.Fatalf("未连接时编辑表单缺少完整字段 %s", value)
		}
		if row < 0 {
			row = label.Bounds.Y
		} else if abs(label.Bounds.Y-row) > 1 {
			t.Fatalf("字段 %s 不在编辑表单同一行", value)
		}
		boxes = append(boxes, label.Bounds)
	}
	for i := 0; i < len(boxes); i++ {
		for j := i + 1; j < len(boxes); j++ {
			if overlaps(boxes[i], boxes[j]) {
				t.Fatal("编辑主机表单字段重叠")
			}
		}
	}
	for _, label := range snapshot.Labels {
		if strings.Contains(label.Value, "secret-password") || label.Value == "口令" {
			t.Fatal("未连接的编辑主机表单显示了口令")
		}
	}
}

func TestDisconnectedNewHostFormIsComplete(t *testing.T) {
	model := &Model{Status: "未连接", ShowNewHost: true}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	snapshot := driver.Snapshot()
	fields := []string{"地址", "用户", "端口"}
	var row float32 = -1
	var boxes []vivaui.Rect
	for _, value := range fields {
		label, ok := labelByValue(snapshot, value)
		if !ok || label.Bounds.Width <= 0 || label.Bounds.Height <= 0 {
			t.Fatalf("未连接时新增表单缺少完整字段 %s", value)
		}
		if row < 0 {
			row = label.Bounds.Y
		} else if abs(label.Bounds.Y-row) > 1 {
			t.Fatalf("字段 %s 不在新增表单同一行", value)
		}
		boxes = append(boxes, label.Bounds)
	}
	for i := 0; i < len(boxes); i++ {
		for j := i + 1; j < len(boxes); j++ {
			if overlaps(boxes[i], boxes[j]) {
				t.Fatal("新增主机表单字段重叠")
			}
		}
	}
	for _, label := range snapshot.Labels {
		if strings.Contains(label.Value, "secret-password") || label.Value == "口令" {
			t.Fatal("未连接的新增主机表单显示了口令")
		}
	}
}

func TestDisconnectedDividerSeparatesRegions(t *testing.T) {
	model := &Model{
		Status: "未连接",
		Hosts: []store.Host{{
			User:    "alice",
			Address: "example.com",
			Port:    22,
		}},
	}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	snapshot := driver.Snapshot()
	host, ok := labelByValue(snapshot, "example.com")
	if !ok {
		t.Fatal("未连接时看不到主机地址")
	}
	terminal, ok := labelByValue(snapshot, "等待连接")
	if !ok {
		t.Fatal("未连接时看不到终端占位说明")
	}
	found := false
	for _, primitive := range snapshot.Primitives {
		if primitive.Bounds.Height > 2 || primitive.Bounds.Width <= 0 {
			continue
		}
		if primitive.Bounds.Y <= host.Bounds.Y || primitive.Bounds.Y+primitive.Bounds.Height > terminal.Bounds.Y+1 {
			continue
		}
		found = true
		if overlaps(primitive.Bounds, host.Bounds) || overlaps(primitive.Bounds, terminal.Bounds) {
			t.Fatal("分隔条与主机或终端文字重叠")
		}
	}
	if !found {
		t.Fatal("未连接时没有看到主机列表和终端区之间的分隔条")
	}
	for _, label := range snapshot.Labels {
		if strings.Contains(label.Value, "secret-password") {
			t.Fatal("未连接的分隔条界面显示了口令")
		}
	}
}

func TestDisconnectedTabCloseButtonIsComplete(t *testing.T) {
	model := &Model{Status: "未连接", Tab: "alice@example.com"}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	snapshot := driver.Snapshot()
	closeButton, ok := labelByValue(snapshot, "关闭")
	if !ok || closeButton.Bounds.Width <= 0 || closeButton.Bounds.Height <= 0 {
		t.Fatal("未连接时没有完整显示标签关闭按钮")
	}
	tab, ok := labelByValue(snapshot, "alice@example.com")
	if !ok {
		t.Fatal("未连接时看不到会话标签")
	}
	if overlaps(closeButton.Bounds, tab.Bounds) {
		t.Fatal("关闭按钮与会话标签重叠")
	}
	for _, label := range snapshot.Labels {
		if strings.Contains(label.Value, "secret-password") {
			t.Fatal("未连接的关闭按钮界面显示了口令")
		}
	}
}

func TestDisconnectedSessionTabIsComplete(t *testing.T) {
	model := &Model{Status: "未连接", Tab: "alice@example.com"}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	snapshot := driver.Snapshot()
	tab, ok := labelByValue(snapshot, "alice@example.com")
	if !ok || tab.Bounds.Width <= 0 || tab.Bounds.Height <= 0 {
		t.Fatal("未连接时没有完整显示会话标签")
	}
	status, ok := labelByValue(snapshot, "未连接")
	if ok && overlaps(tab.Bounds, status.Bounds) {
		t.Fatal("会话标签与状态栏重叠")
	}
	for _, label := range snapshot.Labels {
		if strings.Contains(label.Value, "secret-password") {
			t.Fatal("未连接的会话标签显示了口令")
		}
	}

	empty := &Model{Status: "未连接"}
	emptyDriver, err := app.NewDriver(NewConfig(empty), &canvas)
	if err != nil {
		t.Fatalf("创建无会话界面失败: %v", err)
	}
	defer emptyDriver.Close()
	none, ok := labelByValue(emptyDriver.Snapshot(), "无会话")
	if !ok || none.Bounds.Width <= 0 || none.Bounds.Height <= 0 {
		t.Fatal("没有会话时未完整显示无会话")
	}
}

func TestDisconnectedWindowTitleIsComplete(t *testing.T) {
	model := &Model{Status: "未连接"}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	title := driver.Snapshot().Title
	const want = "Skiff — 未连接"
	if title != want {
		t.Fatalf("未连接时窗口标题 = %q，希望 %q", title, want)
	}
	if strings.Contains(title, "secret-password") {
		t.Fatal("未连接的窗口标题显示了口令")
	}
}

func TestDisconnectedSelectionIsComplete(t *testing.T) {
	model := &Model{Status: "未连接", Selection: "第 1 行第 2 列起"}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	snapshot := driver.Snapshot()
	selection, ok := labelByValue(snapshot, "第 1 行第 2 列起")
	if !ok || selection.Bounds.Width <= 0 || selection.Bounds.Height <= 0 {
		t.Fatal("未连接时没有完整显示文本选区")
	}
	terminal, ok := labelByValue(snapshot, "等待连接")
	if ok && overlaps(selection.Bounds, terminal.Bounds) {
		t.Fatal("文本选区与终端占位说明重叠")
	}
	for _, label := range snapshot.Labels {
		if strings.Contains(label.Value, "secret-password") {
			t.Fatal("未连接的文本选区显示了口令")
		}
	}

	empty := &Model{Status: "未连接"}
	emptyDriver, err := app.NewDriver(NewConfig(empty), &canvas)
	if err != nil {
		t.Fatalf("创建无选区界面失败: %v", err)
	}
	defer emptyDriver.Close()
	none, ok := labelByValue(emptyDriver.Snapshot(), "无选区")
	if !ok || none.Bounds.Width <= 0 || none.Bounds.Height <= 0 {
		t.Fatal("没有选区时未完整显示无选区")
	}
}

func TestDisconnectedScrollbackIsComplete(t *testing.T) {
	model := &Model{Status: "未连接", Scrollback: "第 12 行"}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	snapshot := driver.Snapshot()
	place, ok := labelByValue(snapshot, "第 12 行")
	if !ok || place.Bounds.Width <= 0 || place.Bounds.Height <= 0 {
		t.Fatal("未连接时没有完整显示回看位置")
	}
	terminal, ok := labelByValue(snapshot, "等待连接")
	if ok && overlaps(place.Bounds, terminal.Bounds) {
		t.Fatal("回看位置与终端占位说明重叠")
	}
	for _, label := range snapshot.Labels {
		if strings.Contains(label.Value, "secret-password") {
			t.Fatal("未连接的回看位置显示了口令")
		}
	}

	empty := &Model{Status: "未连接"}
	emptyDriver, err := app.NewDriver(NewConfig(empty), &canvas)
	if err != nil {
		t.Fatalf("创建空回看界面失败: %v", err)
	}
	defer emptyDriver.Close()
	none, ok := labelByValue(emptyDriver.Snapshot(), "无回看")
	if !ok || none.Bounds.Width <= 0 || none.Bounds.Height <= 0 {
		t.Fatal("没有历史时未完整显示无回看")
	}
}

func TestDisconnectedCursorIsComplete(t *testing.T) {
	grid := term.New(10, 2)
	if _, err := grid.Write([]byte("ab\r")); err != nil {
		t.Fatalf("写入网格失败: %v", err)
	}
	x, y := grid.Cursor()
	want := withCursor(grid.Row(y), x)
	model := &Model{Status: "未连接", Grid: grid}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	snapshot := driver.Snapshot()
	label, ok := labelByValue(snapshot, want)
	if !ok || label.Bounds.Width <= 0 || label.Bounds.Height <= 0 {
		t.Fatalf("未连接时光标不完整: %q", want)
	}
	if []rune(label.Value)[x] != '▍' {
		t.Fatalf("未连接时光标不在第 %d 列: %q", x, label.Value)
	}
	for _, item := range snapshot.Labels {
		if strings.Contains(item.Value, "secret-password") {
			t.Fatal("未连接的光标界面显示了口令")
		}
	}
}

func TestDisconnectedTerminalCellsAreComplete(t *testing.T) {
	grid := term.New(20, 2)
	if _, err := grid.Write([]byte("ab")); err != nil {
		t.Fatalf("写入网格失败: %v", err)
	}
	model := &Model{Status: "未连接", Grid: grid}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	x, y := grid.Cursor()
	want := withCursor(grid.Row(y), x)
	snapshot := driver.Snapshot()
	label, ok := labelByValue(snapshot, want)
	if !ok || label.Bounds.Width <= 0 || label.Bounds.Height <= 0 {
		t.Fatalf("未连接时终端单元格不完整: %q", want)
	}
	if !strings.Contains(label.Value, "a") || !strings.Contains(label.Value, "b") {
		t.Fatalf("未连接时单元格被截断: %q", label.Value)
	}
	for _, item := range snapshot.Labels {
		if strings.Contains(item.Value, "secret-password") {
			t.Fatal("未连接的终端单元格显示了口令")
		}
	}
}

func TestDisconnectedTerminalAreaIsComplete(t *testing.T) {
	model := &Model{
		Status: "未连接",
		Hosts: []store.Host{{
			User:    "alice",
			Address: "example.com",
			Port:    22,
		}},
	}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	snapshot := driver.Snapshot()
	terminal, ok := labelByValue(snapshot, "等待连接")
	if !ok || terminal.Bounds.Width <= 0 || terminal.Bounds.Height <= 0 {
		t.Fatal("未连接时终端区没有完整显示等待连接")
	}
	host, ok := labelByValue(snapshot, "example.com")
	if !ok {
		t.Fatal("未连接时看不到主机地址")
	}
	if overlaps(host.Bounds, terminal.Bounds) {
		t.Fatal("未连接时终端区与主机列表重叠")
	}
	for _, label := range snapshot.Labels {
		if strings.Contains(label.Value, "secret-password") {
			t.Fatal("未连接的终端区显示了口令")
		}
	}
}

func TestDisconnectedFilterResults(t *testing.T) {
	model := &Model{
		Status: "未连接",
		Filter: "bastion",
		Hosts: []store.Host{
			{User: "alice", Address: "app.example", Port: 22},
			{User: "root", Address: "bastion.example", Port: 2222},
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
	user, ok := labelByValue(snapshot, "root")
	if !ok || user.Bounds.Width <= 0 || user.Bounds.Height <= 0 {
		t.Fatal("过滤结果没有完整显示匹配用户")
	}
	address, ok := labelByValue(snapshot, "bastion.example")
	if !ok || address.Bounds.Width <= 0 || address.Bounds.Height <= 0 {
		t.Fatal("过滤结果没有完整显示匹配地址")
	}
	port, ok := labelByValue(snapshot, "2222")
	if !ok || port.Bounds.Width <= 0 || port.Bounds.Height <= 0 {
		t.Fatal("过滤结果没有完整显示匹配端口")
	}
	if overlaps(user.Bounds, address.Bounds) || overlaps(address.Bounds, port.Bounds) {
		t.Fatal("过滤结果中的字段重叠")
	}
	if hasExactLabel(snapshot, "alice") || hasExactLabel(snapshot, "app.example") {
		t.Fatal("过滤结果包含未匹配的主机")
	}
	for _, label := range snapshot.Labels {
		if strings.Contains(label.Value, "secret-password") {
			t.Fatal("过滤结果界面显示了口令")
		}
	}
}

func TestDisconnectedEmptyListPrompt(t *testing.T) {
	model := &Model{Status: "未连接", Hosts: nil, Selected: -1}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	snapshot := driver.Snapshot()
	prompt, ok := labelByValue(snapshot, "还没有主机")
	if !ok || prompt.Bounds.Width <= 0 || prompt.Bounds.Height <= 0 {
		t.Fatal("未连接时没有完整显示还没有主机")
	}
	placeholder, ok := labelByValue(snapshot, "等待连接")
	if ok && overlaps(prompt.Bounds, placeholder.Bounds) {
		t.Fatal("空列表提示与终端占位说明重叠")
	}
	for _, label := range snapshot.Labels {
		if strings.Contains(label.Value, "secret-password") {
			t.Fatal("未连接的空列表提示显示了口令")
		}
	}
}

func TestEmptyHostListShowsPlaceholder(t *testing.T) {
	model := &Model{Hosts: nil, Selected: -1}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	const want = "还没有主机"
	found := false
	for _, label := range driver.Snapshot().Labels {
		if label.Value != want {
			continue
		}
		found = true
		if label.Bounds.Width <= 0 || label.Bounds.Height <= 0 {
			t.Fatal("空主机列表没有可显示的区域")
		}
	}
	if !found {
		t.Fatal("空主机列表没有显示还没有主机")
	}
}

func TestDisconnectedErrorIsComplete(t *testing.T) {
	model := &Model{Status: "未连接"}
	var canvas vivaui.Canvas
	driver, err := app.NewDriver(NewConfig(model), &canvas)
	if err != nil {
		t.Fatalf("创建界面驱动失败: %v", err)
	}
	defer driver.Close()

	model.Fail("主机密钥不匹配")
	if err := driver.Resize(800, 480); err != nil {
		t.Fatalf("显示错误后刷新失败: %v", err)
	}
	const want = "主机密钥不匹配"
	found := false
	for _, label := range driver.Snapshot().Labels {
		if strings.Contains(label.Value, "secret-password") {
			t.Fatal("错误提示界面显示了口令")
		}
		if label.Value != want {
			continue
		}
		found = true
		if label.Bounds.Width <= 0 || label.Bounds.Height <= 0 {
			t.Fatal("错误提示没有可显示的区域")
		}
	}
	if !found {
		t.Fatalf("未完整显示错误提示: %s", want)
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
	if !hasExactLabel(snapshot, "当前") || !hasExactLabel(snapshot, "bob") || !hasExactLabel(snapshot, "b.example") {
		t.Fatal("选中主机没有分成当前标记和独立字段")
	}
	if !hasExactLabel(snapshot, "ann") || !hasExactLabel(snapshot, "a.example") {
		t.Fatal("未选中主机没有显示独立字段")
	}
	bob, ok := labelByValue(snapshot, "bob")
	if !ok {
		t.Fatal("没有 bob 字段")
	}
	ann, ok := labelByValue(snapshot, "ann")
	if !ok {
		t.Fatal("没有 ann 字段")
	}
	if bob.Weight <= ann.Weight {
		t.Fatal("选中主机没有比未选中主机更强调")
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
