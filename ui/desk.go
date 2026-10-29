package ui

import (
	"strconv"
	"strings"

	"github.com/SwalkTech/Skiff/store"
	"github.com/SwalkTech/Skiff/term"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	vivaui "github.com/vivaui/viva/ui"
)

const (
	terminalLineHeight float32 = 20
	terminalCellWidth  float32 = 8
)

var (
	tree       = vivaui.Tree{}
	page       = tree.Feature("desk")
	hostListID = tree.ID(page, "hosts")
	hostRows   = tree.Sub(page, tree.Name("host-rows"))
)

// Model 是主窗口要画的内容。它不保存连接，也不发起 SSH 握手。
type Model struct {
	Hosts []store.Host
	// Selected 是当前主机在 Hosts 中的下标。没有可选主机时为 -1。
	Selected int
	Screen   string
	Status   string
	Grid     *term.Grid
	// Fingerprint 是等待确认的主机密钥指纹。为空表示没有确认请求。
	Fingerprint string
	// Filter 按用户或地址筛选主机。为空表示显示全部。
	Filter string
	// Scrollback 是回看位置。为空表示没有可回看的历史。
	Scrollback string
	// Selection 是终端选区说明。为空表示没有选区。
	Selection string
	// Tab 是当前会话标签。为空表示没有会话。
	Tab string
	// ShowNewHost 为真时显示新增主机表单。
	ShowNewHost bool
	// EditHost 为真时显示正在编辑的主机。下标由 Selected 指定。
	EditHost bool
	// ConfirmDelete 为真时显示删除确认。下标由 Selected 指定。
	ConfirmDelete bool
	// Focus 是当前焦点说明。为空时未连接界面显示主机列表。
	Focus string
	view  flex.View

	paintedTerminal    string
	terminalPaints     int
	lastTerminalHeight float32
}

// NewConfig 描述主机列表和一块静态等宽文本。测试可以驱动同一份配置而不打开窗口。
func NewConfig(m *Model) app.Config {
	theme := vivaui.LightTheme()
	config := app.Config{Tree: &tree, Title: windowTitle(m), Width: 800, Height: 480, OnScroll: m.view.Wheel}
	config.Paint = func(canvas *vivaui.Canvas, size vivaui.Size) {
		bounds := vivaui.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.Background)
		frame := m.view.Begin(canvas, theme, bounds)
		var terminal flex.Box
		frame.Column().Fill().Pad(16).Gap(8).Children(func() {
			if tab := sessionTabText(m); tab != "" {
				frame.Row().Gap(8).Children(func() {
					frame.Text(tab)
					if closeLabel := tabCloseText(m); closeLabel != "" {
						frame.Text(closeLabel)
					}
				})
			}
			frame.Text("主机").Bold()
			if form := newHostFormText(m); form != "" {
				frame.Row().Gap(12).Children(func() {
					for _, field := range strings.Split(form, "|") {
						frame.Text(field)
					}
				})
			}
			if fields := editHostFields(m); len(fields) > 0 {
				frame.Row().Gap(12).Children(func() {
					for _, field := range fields {
						frame.Text(field)
					}
				})
			}
			if fields := deleteConfirmFields(m); len(fields) > 0 {
				frame.Row().Gap(12).Children(func() {
					for _, field := range fields {
						frame.Text(field)
					}
				})
			}
			frame.ScrollArea(hostListID).MaxHeight(120).Children(func() {
				visible := visibleHosts(m)
				if len(m.Hosts) == 0 {
					frame.Text("还没有主机").Muted()
				} else if len(visible) == 0 {
					frame.Text("没有匹配的主机").Muted()
				}
				for _, item := range visible {
					i, host := item.index, item.host
					selected := i == m.Selected
					row := frame.Row().Gap(16).HoverBackground(theme.Muted)
					row.Hit(hostRows.ID(vivaui.Index(i)), host.Address)
					row.Children(func() {
						if selected {
							frame.Text("当前").Bold()
						}
						user := frame.Text(host.User)
						address := frame.Text(host.Address).MaxWidth(120).Ellipsis()
						port := frame.Text(strconv.Itoa(host.Port))
						if selected {
							user.Bold()
							address.Bold()
							port.Bold()
						}
					})
				}
			})
			frame.Divider()
			frame.Text("终端").Bold()
			terminal = frame.Column().FillHeight().MinHeight(terminalLineHeight)
			lines := terminalLines(m)
			m.noteTerminal(lines)
			terminal.Children(func() {
				for _, line := range lines {
					frame.Text(line).Mono()
				}
			})
			frame.Text(statusText(m.Status)).Muted()
			if focus := focusText(m); focus != "" {
				frame.Text(focus).Muted()
			}
			if place := scrollbackText(m); place != "" {
				frame.Text(place).Muted()
			}
			if selection := selectionText(m); selection != "" {
				frame.Text(selection).Muted()
			}
			if prompt := fingerprintPrompt(m); prompt != "" {
				frame.Text(prompt)
			}
		})
		frame.End()
		area := terminal.Rect()
		m.lastTerminalHeight = area.Height
		m.fitGrid(area.Width, area.Height)
	}
	return config
}

// Load 从主机清单文件构造窗口模型。文件不存在时列表为空。
// 读到的地址、用户和端口会原样显示，口令不在清单中。
func Load(path string) (*Model, error) {
	hosts, err := store.Open(path).List()
	if err != nil {
		return nil, err
	}
	selected := -1
	if len(hosts) > 0 {
		selected = 0
	}
	return &Model{Hosts: hosts, Selected: selected, Status: "未连接"}, nil
}

// Connecting 把状态栏设为正在连接。窗口保持打开，状态中不写入口令。
func (m *Model) Connecting() {
	m.Status = "正在连接"
}

// Fail 把失败原因写到状态栏。窗口保持打开，原因中不应包含口令。
func (m *Model) Fail(reason string) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "连接失败"
	}
	m.Status = reason
}

// Run 打开主窗口。调用方负责准备要显示的主机和文本。
func Run(m *Model) error {
	var canvas vivaui.Canvas
	return app.RunCanvas(NewConfig(m), &canvas)
}

type indexedHost struct {
	index int
	host  store.Host
}

func visibleHosts(m *Model) []indexedHost {
	if m == nil {
		return nil
	}
	filter := strings.ToLower(strings.TrimSpace(m.Filter))
	out := make([]indexedHost, 0, len(m.Hosts))
	for i, host := range m.Hosts {
		if filter != "" && !strings.Contains(strings.ToLower(host.User), filter) && !strings.Contains(strings.ToLower(host.Address), filter) {
			continue
		}
		out = append(out, indexedHost{index: i, host: host})
	}
	return out
}

func screenLines(screen string) []string {
	if strings.TrimSpace(screen) == "" {
		return []string{"等待连接"}
	}
	return strings.Split(screen, "\n")
}

// noteTerminal 只在可见终端文本变化时记一次重绘。
func (m *Model) noteTerminal(lines []string) {
	current := strings.Join(lines, "\n")
	if current == m.paintedTerminal {
		return
	}
	m.paintedTerminal = current
	m.terminalPaints++
}

// terminalLines 优先显示网格里已经出现的字符。没有网格或网格仍为空时，退回静态文本。
// 光标所在行会插入标记，位置与 Grid.Cursor 的列一致。
func terminalLines(m *Model) []string {
	if m.Grid == nil {
		return screenLines(m.Screen)
	}
	_, rows := m.Grid.Size()
	x, cursorY := m.Grid.Cursor()
	lines := make([]string, 0, rows)
	for y := 0; y < rows; y++ {
		line := m.Grid.Row(y)
		if y == cursorY {
			line = withCursor(line, x)
		}
		if line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		return screenLines(m.Screen)
	}
	return lines
}

// withCursor 在第 x 个字符前放入光标标记。x 超出文本时先补空格。
func withCursor(line string, x int) string {
	if x < 0 {
		x = 0
	}
	chars := []rune(line)
	for len(chars) < x {
		chars = append(chars, ' ')
	}
	marked := make([]rune, 0, len(chars)+1)
	marked = append(marked, chars[:x]...)
	marked = append(marked, '▍')
	marked = append(marked, chars[x:]...)
	return string(marked)
}

func focusText(m *Model) string {
	if m == nil || !statusBeforeConnected(m.Status) {
		return ""
	}
	if strings.TrimSpace(m.Focus) == "" {
		return "焦点 主机列表"
	}
	return "焦点 " + strings.TrimSpace(m.Focus)
}

func newHostFormText(m *Model) string {
	if m == nil || !m.ShowNewHost || !statusBeforeConnected(m.Status) {
		return ""
	}
	return "地址|用户|端口"
}

func editHostFields(m *Model) []string {
	return selectedHostFields(m, m != nil && m.EditHost)
}

func deleteConfirmFields(m *Model) []string {
	fields := selectedHostFields(m, m != nil && m.ConfirmDelete)
	if len(fields) == 0 {
		return nil
	}
	return append([]string{"删除"}, fields...)
}

func selectedHostFields(m *Model, enabled bool) []string {
	if m == nil || !enabled || !statusBeforeConnected(m.Status) {
		return nil
	}
	if m.Selected < 0 || m.Selected >= len(m.Hosts) {
		return nil
	}
	host := m.Hosts[m.Selected]
	return []string{host.Address, host.User, strconv.Itoa(host.Port)}
}

func tabCloseText(m *Model) string {
	if m == nil || !statusBeforeConnected(m.Status) || strings.TrimSpace(m.Tab) == "" {
		return ""
	}
	return "关闭"
}

func sessionTabText(m *Model) string {
	if m == nil || !statusBeforeConnected(m.Status) {
		return ""
	}
	if strings.TrimSpace(m.Tab) == "" {
		return "无会话"
	}
	return strings.TrimSpace(m.Tab)
}

func windowTitle(m *Model) string {
	if m == nil {
		return "Skiff"
	}
	switch statusText(m.Status) {
	case "未连接":
		return "Skiff — 未连接"
	case "正在连接":
		return "Skiff — 正在连接"
	default:
		return "Skiff — " + statusText(m.Status)
	}
}

func selectionText(m *Model) string {
	if m == nil || !statusBeforeConnected(m.Status) {
		return ""
	}
	if strings.TrimSpace(m.Selection) == "" {
		return "无选区"
	}
	return strings.TrimSpace(m.Selection)
}

func scrollbackText(m *Model) string {
	if m == nil || !statusBeforeConnected(m.Status) {
		return ""
	}
	if strings.TrimSpace(m.Scrollback) == "" {
		return "无回看"
	}
	return strings.TrimSpace(m.Scrollback)
}

func statusBeforeConnected(status string) bool {
	switch statusText(status) {
	case "未连接", "正在连接":
		return true
	default:
		return false
	}
}

func statusText(status string) string {
	if strings.TrimSpace(status) == "" {
		return "未连接"
	}
	return status
}

// fingerprintPrompt 在尚未连上且有待确认指纹时返回完整提示。提示中不包含口令。
func fingerprintPrompt(m *Model) string {
	if m == nil || strings.TrimSpace(m.Fingerprint) == "" {
		return ""
	}
	switch statusText(m.Status) {
	case "未连接", "正在连接":
		return "确认主机密钥 " + strings.TrimSpace(m.Fingerprint)
	default:
		return ""
	}
}

// rowsForHeight 按终端区域高度计算网格行数，至少一行。
func rowsForHeight(height float32) int {
	rows := int(height / terminalLineHeight)
	if rows < 1 {
		return 1
	}
	return rows
}

// colsForWidth 按终端区域宽度计算网格列数，至少一列。
func colsForWidth(width float32) int {
	cols := int(width / terminalCellWidth)
	if cols < 1 {
		return 1
	}
	return cols
}

// fitGrid 让网格行列跟随终端区域。
func (m *Model) fitGrid(width, height float32) {
	if m == nil || m.Grid == nil {
		return
	}
	m.Grid.Resize(colsForWidth(width), rowsForHeight(height))
}
