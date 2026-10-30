package ui

import (
	"context"
	"strconv"
	"strings"
	"sync"

	"github.com/SwalkTech/Skiff/session"
	"github.com/SwalkTech/Skiff/store"
	"github.com/SwalkTech/Skiff/term"
	"github.com/pkg/sftp"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	vivaui "github.com/vivaui/viva/ui"
)

const (
	terminalLineHeight float32 = 20
	terminalCellWidth  float32 = 8
)

var (
	tree             = vivaui.Tree{}
	page             = tree.Feature("desk")
	hostListID       = tree.ID(page, "hosts")
	hostRows         = tree.Sub(page, tree.Name("host-rows"))
	addID            = tree.ID(page, "add")
	deleteID         = tree.ID(page, "delete")
	connectID        = tree.ID(page, "connect")
	saveID           = tree.ID(page, "save")
	cancelID         = tree.ID(page, "cancel")
	splitID          = tree.ID(page, "split")
	addressID        = tree.ID(page, "address")
	userID           = tree.ID(page, "user")
	portID           = tree.ID(page, "port")
	passwordID       = tree.ID(page, "password")
	changePasswordID = tree.ID(page, "change-password")
	acceptID         = tree.ID(page, "accept")
	rejectID         = tree.ID(page, "reject")
	terminalID       = tree.ID(page, "terminal")
	viewChoice       = tree.Sub(page, tree.Name("view"))
	fileRows         = tree.Sub(page, tree.Name("files"))
	fileListID       = tree.ID(page, "file-list")
	upID             = tree.ID(page, "up")
	refreshID        = tree.ID(page, "refresh")
	uploadID         = tree.ID(page, "upload")
	downloadID       = tree.ID(page, "download")
	deleteFileID     = tree.ID(page, "delete-file")
)

// Model 是主窗口的状态。连接在这个进程里完成，口令不写入清单。
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
	// ShowNewHost 为真时显示新增主机表单。
	ShowNewHost    bool
	view           flex.View
	split          flex.SplitterState
	path           string
	address        flex.Field
	user           flex.Field
	port           flex.Field
	password       flex.Field
	passwords      store.PasswordStore
	remembered     bool
	changePassword bool
	passwordCache  map[string]bool
	invalidator    app.Invalidator
	connected      bool
	dialing        bool
	sentCols       int
	sentRows       int
	showFiles      bool
	remoteDir      string
	files          []remoteFile
	filePick       int
	fileNote       string
	sftp           *sftp.Client
	pendingFiles   *fileResult

	mu             sync.Mutex
	writeMu        sync.Mutex
	client         *session.Client
	cancel         context.CancelFunc
	generation     int
	keyReply       chan error
	pending        []byte
	askFingerprint string
	askAuthorized  string
	dialErr        string
	dialOK         bool
	readErr        error

	paintedTerminal    string
	terminalPaints     int
	lastTerminalHeight float32
}

// NewConfig 描述主机栏、终端栏和可点击的操作。测试可以驱动同一份配置而不打开窗口。
func NewConfig(m *Model) app.Config {
	if m == nil {
		m = &Model{Selected: -1, Status: "未连接"}
	}
	if m.split.Pos == 0 {
		m.split.Pos = 300
	}
	m.password.Secure = true
	fields := flex.Fields(&m.address, &m.user, &m.port, &m.password)
	theme := vivaui.DarkTheme()
	terminalBG := vivaui.RGB(8, 10, 14)
	terminalFG := vivaui.RGB(226, 230, 233)
	config := app.Config{
		Tree:        &tree,
		Title:       windowTitle(m),
		Width:       1080,
		Height:      700,
		OnScroll:    m.view.Wheel,
		OnKey:       m.onKey,
		OnShortcut:  m.onShortcut,
		OnWake:      m.wake,
		OnClosed:    m.disconnect,
		Invalidator: &m.invalidator,
		TextInput: func(ctx app.TextInputContext) *app.TextInput {
			if ctx.Terminal && ctx.ID == terminalID && m.connected {
				return &app.TextInput{OnEvent: m.onTerminalText}
			}
			return fields(ctx)
		},
	}
	config.Paint = func(canvas *vivaui.Canvas, size vivaui.Size) {
		bounds := vivaui.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.Background)
		frame := m.view.Begin(canvas, theme, bounds)
		var terminal flex.Box
		target := "未选择主机"
		if host, ok := selectedHost(m); ok {
			target = host.User + "@" + host.Address + ":" + strconv.Itoa(host.Port)
		}
		frame.Column().Fill().Children(func() {
			frame.Row().FillWidth().Height(52).PadX(16).Items(flex.Center).Background(theme.Card).Border(1, theme.Border).Children(func() {
				frame.Text("Skiff").FontSize(18).Bold()
				frame.Text(target).Muted()
				frame.Spacer()
				frame.Row().PadX(12).PadY(6).Radius(999).Background(theme.Muted).Children(func() {
					frame.Text(statusText(m.Status)).FontSize(13)
				})
			})
			frame.Row().Fill().Children(func() {
				frame.Column().Width(m.split.Pos).FillHeight().Background(theme.Card).Border(1, theme.Border).Pad(16).Gap(12).Children(func() {
					visible := visibleHosts(m)
					frame.Row().Items(flex.Center).Children(func() {
						frame.Text("主机").FontSize(15).Bold()
						frame.Spacer()
						frame.Text(strconv.Itoa(len(visible))).Muted()
					})
					frame.Row().Gap(8).Children(func() {
						if frame.Button(addID, "添加").Outline().Small().Clicked() {
							m.toggleNewHost()
						}
						if frame.Button(deleteID, "删除").DestructiveOutline().Small().Clicked() {
							m.deleteSelected()
						}
					})
					if frame.Button(connectID, m.connectLabel()).Clicked() {
						m.connectSelected()
					}
					if !m.connected {
						if m.remembered && !m.changePassword {
							frame.Text("口令已记住，可直接连接").Muted()
							if frame.Button(changePasswordID, "更改口令").Ghost().Small().Clicked() {
								m.changePassword = true
								m.password.SetText("")
								m.remembered = false
							}
						} else if frame.Field(&m.password, passwordID, "口令", "仅首次输入").Submitted() {
							m.connectSelected()
						}
					}
					if m.ShowNewHost {
						frame.Field(&m.address, addressID, "地址", "example.com")
						frame.Field(&m.user, userID, "用户", "alice")
						frame.Field(&m.port, portID, "端口", "22")
						frame.Row().Gap(8).Children(func() {
							if frame.Button(saveID, "保存").Small().Clicked() {
								m.saveNewHost()
							}
							if frame.Button(cancelID, "取消").Outline().Small().Clicked() {
								m.ShowNewHost = false
							}
						})
					}
					frame.ScrollArea(hostListID).Grow(1).Gap(8).Children(func() {
						if len(m.Hosts) == 0 {
							frame.Text("还没有主机").Muted()
							frame.Text("点添加，把主机放进这一栏").Muted()
						} else if len(visible) == 0 {
							frame.Text("没有匹配的主机").Muted()
						}
						for _, item := range visible {
							i, host := item.index, item.host
							selected := i == m.Selected
							bg := theme.Background
							border := theme.Border
							if selected {
								bg = theme.Accent
								border = theme.Primary
							}
							row := frame.Column().FillWidth().Pad(12).Gap(4).Radius(10).Border(1, border).Background(bg).HoverBackground(theme.Hover)
							row.Hit(hostRows.ID(vivaui.Index(i)), host.Address)
							if row.Children(func() {
								frame.Row().Gap(8).Items(flex.Center).Children(func() {
									if selected {
										frame.Text("当前").Color(theme.Primary).Bold()
									}
									user := frame.Text(host.User)
									frame.Spacer()
									frame.Text(strconv.Itoa(host.Port)).Muted()
									if selected {
										user.Bold()
									}
								})
								frame.Row().Gap(8).Children(func() {
									frame.Text(host.Address).MaxWidth(180).Ellipsis().Muted()
									if m.hostHasPassword(host) {
										frame.Text("已记住").Color(theme.Primary)
									}
								})
							}).Clicked() {
								if selected && !m.connected && !m.dialing {
									m.connectSelected()
									return
								}
								m.Selected = i
								m.changePassword = false
								m.recallPassword(host)
								if m.remembered {
									m.Status = "可直接连接"
								} else {
									m.Status = "已选择 " + host.User + "@" + host.Address
								}
							}
						}
					})
				})
				frame.Splitter(splitID, &m.split).Mins(240, 420)
				frame.Column().Grow(1).FillHeight().Background(theme.Background).Children(func() {
					frame.Row().FillWidth().Height(44).PadX(16).Gap(12).Items(flex.Center).Background(theme.Card).Border(1, theme.Border).Children(func() {
						view := 0
						if m.showFiles {
							view = 1
						}
						if i, ok := frame.Choice(viewChoice, []string{"终端", "文件"}, view).ChosenIndex(); ok && i != view {
							m.showFiles = i == 1
							if m.showFiles {
								m.openFiles()
							}
						}
						frame.Spacer()
						frame.Text(target).Muted()
					})
					if prompt := fingerprintPrompt(m); prompt != "" {
						frame.Column().FillWidth().Pad(16).Gap(8).Background(theme.Card).Border(1, theme.Border).Children(func() {
							frame.Text(prompt)
							frame.Row().Gap(8).Children(func() {
								if frame.Button(acceptID, "接受").Small().Clicked() {
									m.acceptHostKey()
								}
								if frame.Button(rejectID, "拒绝").DestructiveOutline().Small().Clicked() {
									m.rejectHostKey()
								}
							})
						})
					}
					terminal = frame.Column().Grow(1).Fill().MinHeight(220).Background(terminalBG).Pad(16).Gap(8)
					if m.showFiles {
						terminal.Children(func() { m.paintFiles(frame, terminalFG) })
					} else {
						if m.connected {
							terminal.HitWith(terminalID, "终端输入", vivaui.HitOptions{Role: vivaui.RoleTerminal})
						}
						lines := terminalLines(m)
						m.noteTerminal(lines)
						terminal.Children(func() {
							empty := m.Grid == nil && strings.TrimSpace(m.Screen) == ""
							if empty && !m.connected {
								hint := "选择左边的主机，然后点连接"
								if _, ok := selectedHost(m); ok && m.remembered {
									hint = "口令已记住，点连接或再点一次主机"
								} else if _, ok := selectedHost(m); ok {
									hint = "输入口令后点连接，之后不用再填"
								}
								frame.Text(hint).Color(vivaui.RGB(150, 158, 166))
							}
							if m.dialing && empty {
								frame.Text("正在连接 " + target).Color(terminalFG)
							}
							if m.connected && empty {
								frame.Text("点击这里输入").Color(vivaui.RGB(150, 158, 166))
							}
							for _, line := range lines {
								if empty && !m.connected && line == statusText(m.Status) {
									continue
								}
								frame.Text(line).Mono().FontSize(14).Color(terminalFG)
							}
						})
					}
				})
			})
		})
		frame.End()
		area := terminal.Rect()
		m.lastTerminalHeight = area.Height
		m.fitGrid(area.Width, area.Height)
		m.noteResize()
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
	m := &Model{path: path, Hosts: hosts, Selected: selected, Status: "未连接", passwords: store.SystemPasswords()}
	if host, ok := selectedHost(m); ok {
		m.recallPassword(host)
	}
	return m, nil
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

func (m *Model) toggleNewHost() {
	m.ShowNewHost = !m.ShowNewHost
	if m.ShowNewHost && strings.TrimSpace(m.port.Text()) == "" {
		m.port.SetText("22")
	}
}

func (m *Model) saveNewHost() {
	portText := strings.TrimSpace(m.port.Text())
	if portText == "" {
		portText = "22"
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		m.Status = "端口必须是 1 到 65535"
		return
	}
	host := store.Host{
		Address: strings.TrimSpace(m.address.Text()),
		User:    strings.TrimSpace(m.user.Text()),
		Port:    port,
	}
	if host.Address == "" || host.User == "" {
		m.Status = store.ErrInvalidHost.Error()
		return
	}
	if m.path != "" {
		if err := store.Open(m.path).Add(host); err != nil {
			m.Status = err.Error()
			return
		}
		hosts, err := store.Open(m.path).List()
		if err != nil {
			m.Status = err.Error()
			return
		}
		m.Hosts = hosts
	} else {
		for _, existing := range m.Hosts {
			if existing.Address == host.Address && existing.User == host.User && existing.Port == host.Port {
				m.Status = store.ErrExists.Error()
				return
			}
		}
		m.Hosts = append(m.Hosts, host)
	}
	m.Selected = len(m.Hosts) - 1
	m.ShowNewHost = false
	m.address.SetText("")
	m.user.SetText("")
	m.port.SetText("")
	m.Status = "已添加 " + host.Address
}

func (m *Model) deleteSelected() {
	host, ok := selectedHost(m)
	if !ok {
		m.Status = "请先选择一台主机"
		return
	}
	if m.path != "" {
		if err := store.Open(m.path).Delete(host.Address, host.User, host.Port); err != nil {
			m.Status = err.Error()
			return
		}
	}
	m.forgetPassword(host)
	next := make([]store.Host, 0, len(m.Hosts)-1)
	for i, item := range m.Hosts {
		if i == m.Selected {
			continue
		}
		next = append(next, item)
	}
	m.Hosts = next
	if len(m.Hosts) == 0 {
		m.Selected = -1
		m.Status = "已删除 " + host.Address
		return
	}
	if m.Selected >= len(m.Hosts) {
		m.Selected = len(m.Hosts) - 1
	}
	m.Status = "已删除 " + host.Address
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

func screenLines(m *Model) []string {
	if m != nil && strings.TrimSpace(m.Screen) != "" {
		return strings.Split(m.Screen, "\n")
	}
	if m == nil {
		return []string{"未连接"}
	}
	return []string{statusText(m.Status)}
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
		return screenLines(m)
	}
	_, rows := m.Grid.Size()
	x, cursorY := m.Grid.Cursor()
	last := cursorY
	for y := rows - 1; y > last; y-- {
		if m.Grid.Row(y) != "" {
			last = y
			break
		}
	}
	lines := make([]string, 0, last+1)
	for y := 0; y <= last; y++ {
		line := m.Grid.Row(y)
		if y == cursorY {
			line = withCursor(line, x)
		}
		if line == "" {
			line = " "
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return screenLines(m)
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

func windowTitle(m *Model) string {
	if m == nil {
		return "Skiff"
	}
	title := "Skiff"
	if host, ok := selectedHost(m); ok && strings.TrimSpace(host.Address) != "" {
		title += " — " + strings.TrimSpace(host.Address)
	}
	return title + " — " + statusText(m.Status)
}

func selectedHost(m *Model) (store.Host, bool) {
	if m == nil || m.Selected < 0 || m.Selected >= len(m.Hosts) {
		return store.Host{}, false
	}
	return m.Hosts[m.Selected], true
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
