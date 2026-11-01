package ui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/SwalkTech/Skiff/session"
	"github.com/SwalkTech/Skiff/store"
	"github.com/SwalkTech/Skiff/term"
	"github.com/pkg/sftp"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	vivaui "github.com/vivaui/viva/ui"
)

const (
	terminalFontSize float32 = 14
	terminalPad      float32 = 16
	titlebarHeight   float32 = 52
	minWindowWidth   float32 = 800
	minSidebarWidth  float32 = 240
	minTerminalWidth float32 = 420
	// 统一间距规范
	gapTight float32 = 4
	gapSmall float32 = 8
	gapStd   float32 = 12
	gapLarge float32 = 16
	// 统一圆角规范
	radiusSmall float32 = 6
	radiusStd   float32 = 10
	radiusLarge float32 = 12
)

// skiffPalette 是窗口、侧栏和终端共用的一套颜色。
type skiffPalette struct {
	window, chrome, elevated, selected, terminal vivaui.Color
	ink, muted, accent, success, danger, line    vivaui.Color
}

func colors(dark bool) skiffPalette {
	if !dark {
		return skiffPalette{
			window:   vivaui.RGB(248, 249, 252),
			chrome:   vivaui.RGB(255, 255, 255),
			elevated: vivaui.RGB(236, 241, 250),
			selected: vivaui.RGB(230, 240, 255),
			terminal: vivaui.RGB(228, 233, 242),
			ink:      vivaui.RGB(30, 36, 51),
			muted:    vivaui.RGB(100, 115, 140),
			accent:   vivaui.RGB(52, 112, 224),
			success:  vivaui.RGB(40, 140, 75),
			danger:   vivaui.RGB(200, 50, 85),
			line:     vivaui.RGB(214, 220, 230),
		}
	}
	return skiffPalette{
		window:   vivaui.RGB(13, 15, 23),
		chrome:   vivaui.RGB(20, 23, 35),
		elevated: vivaui.RGB(35, 40, 60),
		selected: vivaui.RGB(42, 52, 82),
		terminal: vivaui.RGB(10, 12, 20),
		ink:      vivaui.RGB(200, 210, 245),
		muted:    vivaui.RGB(135, 145, 180),
		accent:   vivaui.RGB(120, 168, 255),
		success:  vivaui.RGB(165, 210, 100),
		danger:   vivaui.RGB(245, 115, 145),
		line:     vivaui.RGB(46, 52, 74),
	}
}

var (
	tree             = vivaui.Tree{}
	page             = tree.Feature("desk")
	hostListID       = tree.ID(page, "hosts")
	hostRows         = tree.Sub(page, tree.Name("host-rows"))
	editHosts        = tree.Sub(page, tree.Name("edit-host"))
	deleteHosts      = tree.Sub(page, tree.Name("delete-host"))
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
	keyPickID        = tree.ID(page, "key-pick")
	keyClearID       = tree.ID(page, "key-clear")
	changePasswordID = tree.ID(page, "change-password")
	acceptID         = tree.ID(page, "accept")
	rejectID         = tree.ID(page, "reject")
	terminalID       = tree.ID(page, "terminal")
	viewChoice       = tree.Sub(page, tree.Name("view"))
	themeID          = tree.ID(page, "theme")
	updateYesID      = tree.ID(page, "update-yes")
	updateNoID       = tree.ID(page, "update-no")
	addPaneID        = tree.ID(page, "add-pane")
	paneTerms        = tree.Sub(page, tree.Name("pane-term"))
	paneFocus        = tree.Sub(page, tree.Name("pane-focus"))
	closePane        = tree.Sub(page, tree.Name("close-pane"))
	panePick0        = tree.Sub(page, tree.Name("pane-pick-0"))
	panePick1        = tree.Sub(page, tree.Name("pane-pick-1"))
	panePick2        = tree.Sub(page, tree.Name("pane-pick-2"))
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
	ShowNewHost bool
	// ShowEditHost 为真时显示编辑主机表单。
	ShowEditHost bool
	// EditingIndex 是正在编辑的主机在 Hosts 中的下标。
	EditingIndex    int
	view            flex.View
	split           flex.SplitterState
	path            string
	address         flex.Field
	user            flex.Field
	port            flex.Field
	password        flex.Field
	passwords       store.PasswordStore
	remembered      bool
	changePassword  bool
	passwordCache   map[string]bool
	dark            bool
	invalidator     app.Invalidator
	connected       bool
	dialing         bool
	sentCols        int
	sentRows        int
	showFiles       bool
	remoteDir       string
	files           []remoteFile
	filePick        int
	fileNote        string
	sftp            *sftp.Client
	pendingFiles    *fileResult
	load            serverLoad
	loadOK          bool
	loadNote        string
	pendingLoad     *serverLoad
	pendingLoadNote string
	writeCh         chan []byte
	pendingEcho     []rune
	altScreen       bool
	wakePending     atomic.Bool
	stableCols      int
	stableRows      int

	mu                   sync.Mutex
	writeMu              sync.Mutex
	client               *session.Client
	cancel               context.CancelFunc
	generation           int
	keyReply             chan error
	pending              []byte
	askFingerprint       string
	askAuthorized        string
	dialErr              string
	dialOK               bool
	readErr              error
	updateVersion        string
	updateURL            string
	updateNote           string
	updateBusy           bool
	updateDismissed      bool
	pendingUpdateVersion string
	pendingUpdateURL     string
	pendingUpdateNote    string
	sessions             []*conn
	panes                []pane
	focus                int
	nextSession          int
	paneBox              flex.Box

	paintedTerminal    string
	terminalPaints     int
	lastTerminalHeight float32
}

// NewConfig 描述主机栏、终端栏和可点击的操作。测试可以驱动同一份配置而不打开窗口。
func NewConfig(m *Model) app.Config {
	if m == nil {
		m = &Model{Selected: -1, Status: "未连接", dark: true}
	}
	if !m.dark && !themeChosen(m.path) {
		m.dark = true
	}
	m.ensurePanes()
	if m.split.Pos == 0 {
		// 响应式初始化：通常侧栏宽度 300，小窗口可减小到 200
		m.split.Pos = 300
	}
	m.password.Secure = true
	fields := flex.Fields(&m.address, &m.user, &m.port, &m.password)
	config := app.Config{
		Tree:           &tree,
		Title:          windowTitle(m),
		Icon:           windowIcon(),
		Width:          1080,
		Height:         700,
		WindowMode:     app.WindowCustomTitlebar,
		TitlebarHeight: titlebarHeight,
		ClearColor:     app.Color{Red: 15 / 255.0, Green: 17 / 255.0, Blue: 26 / 255.0, Alpha: 1},
		WindowControls: func() app.WindowControlsStyle {
			ink := colors(m.dark)
			return app.WindowControlsStyle{
				Foreground: ink.ink,
				Hover:      ink.elevated,
				Close:      ink.danger,
				Minimize:   "最小化",
				Maximize:   "最大化",
				Restore:    "还原",
				CloseName:  "关闭",
			}
		},
		OnScroll:    m.view.Wheel,
		OnKey:       m.onKey,
		OnShortcut:  m.onShortcut,
		OnWake:      m.wake,
		OnClosed:    m.disconnectAll,
		Invalidator: &m.invalidator,
		TextInput: func(ctx app.TextInputContext) *app.TextInput {
			if ctx.Terminal && m.acceptsTerminal(ctx.ID) {
				bounds := ctx.Bounds
				if bounds.Width < 8 || bounds.Height < 8 {
					bounds = vivaui.Rect{X: 8, Y: 8, Width: 16, Height: 20}
				}
				font := ctx.FontSize
				if font < 1 {
					font = 14
				}
				return &app.TextInput{Bounds: bounds, FontSize: font, OnEvent: m.onTerminalText}
			}
			return fields(ctx)
		},
	}
	config.Paint = func(canvas *vivaui.Canvas, size vivaui.Size) {
		ink := colors(m.dark)
		theme := themed(ink, m.dark)
		bounds := vivaui.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, ink.window)
		frame := m.view.Begin(canvas, theme, bounds)
		cellW, cellH := canvas.MonoCellSize(terminalFontSize, vivaui.MonoSystem)
		var terminal flex.Box
		var logoBox flex.Box
		target := "未选择主机"
		if host, ok := activeHost(m); ok {
			target = host.User + "@" + host.Address + ":" + strconv.Itoa(host.Port)
		}
		frame.Column().Fill().Children(func() {
			_, trailing := app.TitlebarInsets()
			frame.Row().FillWidth().Height(titlebarHeight).PadX(16).PadSides(0, 16+trailing, 0, 16).Gap(gapSmall).Items(flex.Center).Background(ink.chrome).MovesWindow().Children(func() {
				logoBox = frame.Slot().Width(28).Height(28)
				frame.Text("Skiff").FontSize(18).Bold()
				frame.Text(target).Grow(1).MaxWidth(360).Ellipsis().Muted()
				frame.Spacer()
				if frame.Button(themeID, themeLabel(m.dark)).Ghost().Small().Clicked() {
					m.dark = !m.dark
					m.saveTheme()
				}
				frame.Row().PadX(12).PadY(6).Radius(999).Background(ink.elevated).Children(func() {
					frame.Text(statusPillText(m.Status)).FontSize(13).MaxWidth(160).Ellipsis().Color(statusColor(ink, m.Status))
				})
			})
			frame.Row().Fill().Children(func() {
				frame.Column().Width(m.split.Pos).FillHeight().Background(ink.chrome).Pad(gapLarge).Gap(gapStd).Children(func() {
					visible := visibleHosts(m)
					frame.Row().Items(flex.Center).Children(func() {
						frame.Text("主机").FontSize(15).Bold()
						frame.Text(strconv.Itoa(len(visible))).Muted()
						frame.Spacer()
						if frame.Button(addID, "添加").Outline().Small().Clicked() {
							m.toggleNewHost()
						}
					})
					frame.ScrollArea(hostListID).Grow(1).Gap(gapSmall).Children(func() {
						if len(m.Hosts) == 0 {
							frame.Text("还没有主机").Muted()
							frame.Text("点添加，把主机放进这一栏").Muted()
						} else if len(visible) == 0 {
							frame.Text("没有匹配的主机").Muted()
						}
						for _, item := range visible {
							i, host := item.index, item.host
							selected := i == m.Selected
							bg := hostCardStyle(ink, selected)
							row := frame.Column().FillWidth().Pad(gapStd).Gap(gapSmall).Radius(radiusLarge).Background(bg).HoverBackground(ink.elevated)
							row.Children(func() {
								summary := frame.Column().FillWidth().Gap(gapTight)
								summary.Hit(hostRows.ID(vivaui.Index(i)), host.Address)
								if summary.Children(func() {
									frame.Row().Gap(gapSmall).Items(flex.Center).Children(func() {
										if selected {
											frame.Text("当前").Color(ink.accent).Bold()
										}
										user := frame.Text(host.User).Grow(1).MaxWidth(150).Ellipsis()
										frame.Text(strconv.Itoa(host.Port)).Muted()
										if selected {
											user.Bold()
										}
									})
									frame.Row().Gap(gapSmall).Items(flex.Center).Children(func() {
										frame.Text(host.Address).Grow(1).MaxWidth(150).Ellipsis().Muted()
										if strings.TrimSpace(host.KeyFile) != "" {
											frame.Text("密钥").Color(ink.accent).FontSize(12)
										}
										if m.hostHasPassword(host) {
											frame.Text("已记住").Color(ink.accent).FontSize(12)
										}
										if live := m.sessionFor(host); live != nil && live.connected {
											frame.Text("在线").Color(ink.success).FontSize(12)
										}
									})
								}).Clicked() {
									if live := m.sessionFor(host); live != nil && (live.connected || live.dialing) {
										m.Selected = i
										m.changePassword = false
										m.recallPassword(host)
										m.assignFocused(live)
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
								frame.Row().Gap(gapSmall).Children(func() {
									if frame.Button(editHostID(i), "编辑").Outline().Small().Clicked() {
										m.startEditHost(i)
									}
									frame.Spacer()
									if frame.Button(deleteHostID(i), "删除").DestructiveOutline().Small().Clicked() {
										m.Selected = i
										m.deleteSelected()
									}
								})
							})
						}
					})
					frame.Column().FillWidth().Pad(gapStd).Gap(gapSmall).Radius(radiusLarge).Background(ink.elevated).Children(func() {
						frame.Text("连接").FontSize(13).Color(ink.muted)
						host, hasHost := selectedHost(m)
						selectedLive := false
						if !hasHost {
							frame.Text("先选一台主机").Muted()
						} else {
							frame.Text(host.User + " · " + host.Address).MaxWidth(220).Ellipsis().Muted()
							if s := m.sessionFor(host); s != nil && (s.connected || s.dialing) {
								selectedLive = true
							}
							if strings.TrimSpace(host.KeyFile) != "" {
								frame.Text(filepath.Base(host.KeyFile)).MaxWidth(220).Ellipsis().Muted()
							}
						}
						frame.Row().Gap(gapSmall).Children(func() {
							if frame.Button(keyPickID, "密钥文件").Outline().Small().Clicked() {
								m.pickKeyFile()
							}
							if hasHost && strings.TrimSpace(host.KeyFile) != "" && frame.Button(keyClearID, "清除").Ghost().Small().Clicked() {
								m.clearKeyFile()
							}
						})
						if m.ShowNewHost || m.ShowEditHost {
							frame.Field(&m.address, addressID, "地址", "example.com")
							frame.Field(&m.user, userID, "用户", "alice")
							frame.Field(&m.port, portID, "端口", "22")
							frame.Row().Gap(gapSmall).Children(func() {
								if m.ShowEditHost {
									if frame.Button(saveID, "更新").Small().Clicked() {
										m.saveNewHost()
									}
									if frame.Button(cancelID, "取消").Outline().Small().Clicked() {
										m.cancelEdit()
									}
								} else {
									if frame.Button(saveID, "保存").Small().Clicked() {
										m.saveNewHost()
									}
									if frame.Button(cancelID, "取消").Outline().Small().Clicked() {
										m.ShowNewHost = false
										m.address.SetText("")
										m.user.SetText("")
										m.port.SetText("")
									}
								}
							})
						}
						if hasHost && !selectedLive {
							keyChosen := strings.TrimSpace(host.KeyFile) != ""
							if keyChosen {
								if frame.Field(&m.password, passwordID, "密钥口令", "未加密可留空").Submitted() {
									m.connectSelected()
								}
							} else if m.remembered && !m.changePassword {
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
						if frame.Button(connectID, m.connectLabel()).FillWidth().Clicked() {
							m.connectSelected()
						}
						if frame.Button(deleteID, "删除").Ghost().Small().Clicked() {
							m.deleteSelected()
						}
					})
					m.paintLoad(frame, ink)
				})
				frame.Splitter(splitID, &m.split).Mins(240, 420)
				frame.Column().Grow(1).FillHeight().Background(ink.window).Children(func() {
					frame.Row().FillWidth().Height(52).PadX(16).Gap(gapSmall).Items(flex.Center).Background(ink.chrome).Children(func() {
						view := 0
						if m.showFiles {
							view = 1
						}
						m.ensurePanes()
						if m.panes[m.focus].showFiles {
							view = 1
						}
						if i, ok := frame.Choice(viewChoice, []string{"终端", "文件"}, view).ChosenIndex(); ok && i != view {
							m.panes[m.focus].showFiles = i == 1
							m.showFiles = i == 1
							if m.showFiles {
								m.openFiles()
							}
						}
						if frame.Button(addPaneID, "分栏").Outline().Small().Clicked() {
							m.addPane()
						}
						frame.Spacer()
						frame.Text(sessionHint(m)).Grow(1).MaxWidth(280).Ellipsis().Muted()
					})
					if prompt := updatePrompt(m); prompt != "" {
						frame.Column().FillWidth().Pad(gapLarge).Gap(gapSmall).Background(ink.elevated).Children(func() {
							frame.Text(prompt)
							frame.Row().Gap(gapSmall).Children(func() {
								if !m.updateBusy && frame.Button(updateYesID, "更新").Small().Clicked() {
									m.beginUpdate()
								}
								if frame.Button(updateNoID, "稍后").Outline().Small().Clicked() {
									m.updateDismissed = true
								}
							})
						})
					}
					if prompt := fingerprintPrompt(m); prompt != "" {
						frame.Column().FillWidth().Pad(gapLarge).Gap(gapSmall).Background(ink.elevated).Children(func() {
							frame.Text(prompt)
							frame.Row().Gap(gapSmall).Children(func() {
								if frame.Button(acceptID, "接受").Small().Clicked() {
									m.acceptHostKey()
								}
								if frame.Button(rejectID, "拒绝").DestructiveOutline().Small().Clicked() {
									m.rejectHostKey()
								}
							})
						})
					}
					m.paintWorkArea(frame, ink, cellW, cellH, target)
					terminal = m.paneBox
				})
			})
		})
		frame.End()
		if rect := logoBox.Rect(); rect.Width > 1 {
			canvas.DrawBitmap(logoBitmap(), rect)
		}
		area := terminal.Rect()
		m.lastTerminalHeight = area.Height
		m.fitGrid(canvas, area.Width, area.Height, cellW, cellH)
		m.noteResize()
	}
	return config
}

func themed(ink skiffPalette, dark bool) vivaui.Theme {
	theme := vivaui.DarkTheme()
	if !dark {
		theme = vivaui.LightTheme()
	}
	theme.Background = ink.window
	theme.Foreground = ink.ink
	theme.Card = ink.chrome
	theme.CardForeground = ink.ink
	theme.Muted = ink.elevated
	theme.MutedForeground = ink.muted
	theme.Border = ink.line
	theme.Hover = ink.elevated
	theme.Primary = ink.accent
	theme.PrimaryForeground = ink.window
	theme.Accent = ink.elevated
	theme.Success = ink.success
	theme.Danger = ink.danger
	theme.DangerForeground = ink.ink
	return theme
}

func themePath(hostsPath string) string {
	if strings.TrimSpace(hostsPath) == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(hostsPath), "theme.txt")
}

func themeChosen(hostsPath string) bool {
	path := themePath(hostsPath)
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func loadTheme(hostsPath string) bool {
	body, err := os.ReadFile(themePath(hostsPath))
	if err != nil {
		return true
	}
	return strings.TrimSpace(string(body)) != "light"
}

func (m *Model) saveTheme() {
	if m == nil {
		return
	}
	path := themePath(m.path)
	if path == "" {
		return
	}
	value := "dark"
	if !m.dark {
		value = "light"
	}
	_ = os.WriteFile(path, []byte(value), 0o644)
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
	m := &Model{path: path, Hosts: hosts, Selected: selected, Status: "未连接", passwords: store.SystemPasswords(), dark: loadTheme(path)}
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
	// 规范化错误格式：如果不是已知的特定错误格式，加上前缀
	if !strings.HasPrefix(reason, "连接") && !strings.HasPrefix(reason, "主机") && !strings.HasPrefix(reason, "密钥") && !strings.HasPrefix(reason, "端口") {
		reason = "连接失败: " + reason
	}
	m.Status = reason
}

// Run 打开主窗口。调用方负责准备要显示的主机和文本。
func Run(m *Model) error {
	if m != nil {
		go m.checkUpdate()
	}
	var canvas vivaui.Canvas
	return app.RunCanvas(NewConfig(m), &canvas)
}

func (m *Model) pickKeyFile() {
	host, ok := selectedHost(m)
	if !ok {
		m.Status = errNoHostSelected
		return
	}
	paths, err := app.OpenFiles(app.OpenFileOptions{Title: "选择私钥文件"})
	if err != nil {
		if !errors.Is(err, app.ErrCanceled) {
			m.Status = "取消"
		}
		return
	}
	if len(paths) == 0 {
		return
	}
	m.setHostKeyFile(host, paths[0])
}

func (m *Model) clearKeyFile() {
	host, ok := selectedHost(m)
	if !ok {
		return
	}
	m.setHostKeyFile(host, "")
}

func (m *Model) setHostKeyFile(host store.Host, path string) {
	path = strings.TrimSpace(path)
	if m.path != "" {
		if err := store.Open(m.path).SetKeyFile(host.Address, host.User, host.Port, path); err != nil {
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
		for i := range m.Hosts {
			if m.Hosts[i].Address == host.Address && m.Hosts[i].User == host.User && m.Hosts[i].Port == host.Port {
				m.Hosts[i].KeyFile = path
			}
		}
	}
	if path == "" {
		m.Status = "已清除密钥文件"
		return
	}
	m.Status = "已选择密钥文件"
}

func (m *Model) toggleNewHost() {
	m.ShowNewHost = !m.ShowNewHost
	if m.ShowNewHost && strings.TrimSpace(m.port.Text()) == "" {
		m.port.SetText("22")
	}
}

func (m *Model) startEditHost(index int) {
	if index < 0 || index >= len(m.Hosts) {
		return
	}
	m.ShowEditHost = true
	m.EditingIndex = index
	host := m.Hosts[index]
	m.address.SetText(host.Address)
	m.user.SetText(host.User)
	m.port.SetText(strconv.Itoa(host.Port))
}

func (m *Model) cancelEdit() {
	m.ShowEditHost = false
	m.EditingIndex = -1
	m.address.SetText("")
	m.user.SetText("")
	m.port.SetText("")
}

func (m *Model) saveNewHost() {
	portText := strings.TrimSpace(m.port.Text())
	if portText == "" {
		portText = "22"
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		m.Status = errPortInvalid
		return
	}
	host := store.Host{
		Address: strings.TrimSpace(m.address.Text()),
		User:    strings.TrimSpace(m.user.Text()),
		Port:    port,
	}
	if host.Address == "" || host.User == "" {
		m.Status = errHostInvalid
		return
	}
	if m.ShowEditHost && m.EditingIndex >= 0 && m.EditingIndex < len(m.Hosts) {
		// 编辑模式：检查是否与其他主机重复
		original := m.Hosts[m.EditingIndex]
		for i, existing := range m.Hosts {
			if i == m.EditingIndex {
				continue
			}
			if existing.Address == host.Address && existing.User == host.User && existing.Port == host.Port {
				m.Status = errHostExists
				return
			}
		}
		if m.path != "" {
			if err := store.Open(m.path).Delete(original.Address, original.User, original.Port); err != nil {
				m.Status = err.Error()
				return
			}
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
			m.Hosts[m.EditingIndex] = host
		}
		m.ShowEditHost = false
		m.EditingIndex = -1
		m.Status = "已编辑 " + host.Address
	} else {
		// 新增模式
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
		m.Status = "已添加 " + host.Address
	}
	m.address.SetText("")
	m.user.SetText("")
	m.port.SetText("")
}

func (m *Model) deleteSelected() {
	host, ok := selectedHost(m)
	if !ok {
		m.Status = errNoHostSelected
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
			if extra := m.pendingEchoText(); extra != "" {
				line += extra
			}
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

func sessionHint(m *Model) string {
	switch {
	case m == nil:
		return "先选择主机"
	case m.connected:
		return "会话已就绪"
	case m.dialing:
		return "正在建立会话"
	case m.Selected >= 0:
		return "点连接开始会话"
	default:
		return "先选择主机"
	}
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
	if host, ok := activeHost(m); ok && strings.TrimSpace(host.Address) != "" {
		title += " — " + strings.TrimSpace(host.Address)
	}
	return title + " — " + statusText(m.Status)
}

func activeHost(m *Model) (store.Host, bool) {
	if s := m.focusedConn(); s != nil {
		return s.host, true
	}
	return selectedHost(m)
}

func selectedHost(m *Model) (store.Host, bool) {
	if m == nil || m.Selected < 0 || m.Selected >= len(m.Hosts) {
		return store.Host{}, false
	}
	return m.Hosts[m.Selected], true
}

func themeLabel(dark bool) string {
	if dark {
		return "暗色"
	}
	return "亮色"
}

func promptBorder(ink skiffPalette, hostKey bool) vivaui.Color {
	if hostKey {
		return ink.danger
	}
	return ink.accent
}

func hostCardStyle(ink skiffPalette, selected bool) vivaui.Color {
	if selected {
		return ink.selected
	}
	return ink.window
}

func statusPillText(status string) string {
	text := statusText(status)
	runes := []rune(text)
	if len(runes) <= 12 {
		return text
	}
	return string(runes[:12]) + "…"
}

func statusText(status string) string {
	if strings.TrimSpace(status) == "" {
		return "未连接"
	}
	return status
}

func statusColor(ink skiffPalette, status string) vivaui.Color {
	switch statusText(status) {
	case "已连接":
		return ink.success
	case "正在连接", "可直接连接":
		return ink.accent
	default:
		if strings.Contains(status, "失败") || strings.Contains(status, "拒绝") || strings.Contains(status, "错误") {
			return ink.danger
		}
		return ink.muted
	}
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

// fitGrid 按等宽字的真实格子和终端内边距计算行列，并通知远端。
func terminalGridSize(width, height, cellW, cellH float32) (int, int) {
	if cellW < 1 {
		cellW = 1
	}
	if cellH < 1 {
		cellH = 1
	}
	// width 和 height 是含内边距的边框盒，只减一次，不能再减。
	innerW := width - terminalPad*2
	innerH := height - terminalPad*2
	if innerW < cellW {
		innerW = cellW
	}
	if innerH < cellH {
		innerH = cellH
	}
	cols := int(innerW / cellW)
	rows := int(innerH / cellH)
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	return cols, rows
}

func (m *Model) fitGrid(canvas *vivaui.Canvas, width, height, cellW, cellH float32) {
	if m == nil || m.Grid == nil {
		return
	}
	if cellW < 1 || cellH < 1 {
		if canvas != nil {
			cellW, cellH = canvas.MonoCellSize(terminalFontSize, vivaui.MonoSystem)
		}
	}
	if cellW < 1 {
		cellW = terminalFontSize * 0.6
	}
	if cellH < 1 {
		cellH = terminalFontSize * 1.18
	}
	// 响应式调整：小终端宽度下减少列数限制
	cols, rows := terminalGridSize(width, height, cellW, cellH)
	if width < minTerminalWidth && cols > 40 {
		cols = 40
	}
	if m.stableCols > 0 && absInt(cols-m.stableCols) <= 1 && absInt(rows-m.stableRows) <= 1 {
		return
	}
	m.stableCols, m.stableRows = cols, rows
	m.Grid.Resize(cols, rows)
}
