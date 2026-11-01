package ui

import (
	"context"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SwalkTech/Skiff/session"
	"github.com/SwalkTech/Skiff/store"
	"github.com/SwalkTech/Skiff/term"
	"github.com/vivaui/viva/flex"
	vivaui "github.com/vivaui/viva/ui"
	"golang.org/x/crypto/ssh"
)

// conn 是一台主机的一条连接。后台读写只碰这个对象。
type conn struct {
	id              int
	host            store.Host
	secret          string
	mu              sync.Mutex
	client          *session.Client
	cancel          context.CancelFunc
	generation      int
	writeCh         chan []byte
	keyReply        chan error
	pending         []byte
	askFingerprint  string
	askAuthorized   string
	dialErr         string
	dialOK          bool
	readErr         error
	connected       bool
	dialing         bool
	Grid            *term.Grid
	Screen          string
	Status          string
	Fingerprint     string
	pendingEcho     []rune
	altScreen       bool
	sentCols        int
	sentRows        int
	stableCols      int
	stableRows      int
	load            serverLoad
	loadOK          bool
	loadNote        string
	pendingLoad     *serverLoad
	pendingLoadNote string
}

type pane struct {
	sessionID int
	showFiles bool
}

func (m *Model) ensurePanes() {
	if m == nil {
		return
	}
	if len(m.panes) == 0 {
		m.panes = []pane{{}}
	}
	if m.focus < 0 || m.focus >= len(m.panes) {
		m.focus = 0
	}
}

func sameHost(a, b store.Host) bool {
	return a.Address == b.Address && a.User == b.User && a.Port == b.Port
}

func (m *Model) sessionFor(host store.Host) *conn {
	for _, s := range m.sessions {
		if s != nil && sameHost(s.host, host) {
			return s
		}
	}
	return nil
}

func (m *Model) connByID(id int) *conn {
	if id == 0 {
		return nil
	}
	for _, s := range m.sessions {
		if s != nil && s.id == id {
			return s
		}
	}
	return nil
}

func (m *Model) focusedConn() *conn {
	m.ensurePanes()
	return m.connByID(m.panes[m.focus].sessionID)
}

func (m *Model) openConn(host store.Host) *conn {
	if s := m.sessionFor(host); s != nil {
		return s
	}
	m.nextSession++
	s := &conn{id: m.nextSession, host: host, Status: "未连接"}
	m.sessions = append(m.sessions, s)
	return s
}

func (m *Model) focusEmptyPane() bool {
	m.ensurePanes()
	for i, item := range m.panes {
		if item.sessionID == 0 {
			m.focus = i
			m.bindFocus()
			return true
		}
	}
	if len(m.panes) >= 3 {
		return false
	}
	m.addPane()
	return true
}

func (m *Model) assignFocused(s *conn) {
	m.ensurePanes()
	if s == nil {
		m.panes[m.focus].sessionID = 0
	} else {
		m.panes[m.focus].sessionID = s.id
	}
	m.bindFocus()
}

func (m *Model) addPane() {
	m.ensurePanes()
	if len(m.panes) >= 3 {
		return
	}
	m.panes = append(m.panes, pane{})
	m.focus = len(m.panes) - 1
	m.bindFocus()
}

func (m *Model) closePane(index int) {
	m.ensurePanes()
	if len(m.panes) <= 1 || index < 0 || index >= len(m.panes) {
		return
	}
	m.panes = append(m.panes[:index], m.panes[index+1:]...)
	if index < m.focus {
		m.focus--
	}
	if m.focus >= len(m.panes) {
		m.focus = len(m.panes) - 1
	}
	if m.focus < 0 {
		m.focus = 0
	}
	m.bindFocus()
}

func (m *Model) showSession(s *conn) {
	if s == nil {
		return
	}
	m.ensurePanes()
	for i, item := range m.panes {
		if item.sessionID == s.id {
			m.focus = i
			m.bindFocus()
			return
		}
	}
	m.assignFocused(s)
}

func (m *Model) bindFocus() {
	s := m.focusedConn()
	if s == nil {
		if len(m.sessions) == 0 {
			return
		}
		m.connected = false
		m.dialing = false
		m.client = nil
		m.writeCh = nil
		m.keyReply = nil
		if m.Status == "已连接" || m.Status == "正在连接" {
			m.Status = "未连接"
		}
		return
	}
	m.client = s.client
	m.connected = s.connected
	m.dialing = s.dialing
	m.Grid = s.Grid
	m.Screen = s.Screen
	m.Fingerprint = s.Fingerprint
	if s.Status != "" {
		m.Status = s.Status
	}
	m.writeCh = s.writeCh
	m.keyReply = s.keyReply
	m.askAuthorized = s.askAuthorized
	m.load = s.load
	m.loadOK = s.loadOK
	m.loadNote = s.loadNote
	m.pendingEcho = append([]rune(nil), s.pendingEcho...)
	m.altScreen = s.altScreen
	m.sentCols, m.sentRows = s.sentCols, s.sentRows
	m.stableCols, m.stableRows = s.stableCols, s.stableRows
	if m.focus >= 0 && m.focus < len(m.panes) {
		m.showFiles = m.panes[m.focus].showFiles
	}
}

func (m *Model) syncFocus() {
	s := m.focusedConn()
	if s == nil {
		return
	}
	s.pendingEcho = append([]rune(nil), m.pendingEcho...)
	s.altScreen = m.altScreen
	s.Grid = m.Grid
	s.sentCols, s.sentRows = m.sentCols, m.sentRows
	s.stableCols, s.stableRows = m.stableCols, m.stableRows
	s.connected = m.connected
	s.dialing = m.dialing
	s.client = m.client
	s.writeCh = m.writeCh
}

func (m *Model) promptingConn() *conn {
	if s := m.focusedConn(); s != nil && (s.Fingerprint != "" || s.askFingerprint != "") {
		return s
	}
	for _, s := range m.sessions {
		if s != nil && (s.Fingerprint != "" || s.askFingerprint != "") {
			return s
		}
	}
	return nil
}

func (m *Model) applyConn(s *conn) {
	if s == nil {
		return
	}
	s.mu.Lock()
	data := s.pending
	s.pending = nil
	fingerprint := s.askFingerprint
	dialErr := s.dialErr
	s.dialErr = ""
	connected := s.dialOK
	s.dialOK = false
	readErr := s.readErr
	s.readErr = nil
	load := s.pendingLoad
	loadNote := s.pendingLoadNote
	s.pendingLoad = nil
	s.pendingLoadNote = ""
	s.mu.Unlock()

	if fingerprint != "" {
		s.Fingerprint = fingerprint
		s.Status = "正在连接"
		m.showSession(s)
	}
	if s.Grid == nil && (len(data) > 0 || connected) {
		s.Grid = term.New(80, 24)
	}
	if len(data) > 0 {
		s.altScreen = altScreenAfter(data, s.altScreen)
		if s.altScreen {
			s.pendingEcho = nil
		} else {
			data, s.pendingEcho = consumeEcho(data, s.pendingEcho)
		}
	}
	if len(data) > 0 && s.Grid != nil {
		s.Grid.Write(data)
	}
	if connected {
		s.connected = true
		s.dialing = false
		s.Fingerprint = ""
		s.Status = "已连接"
		if s.secret != "" {
			m.rememberPassword(s.host, s.secret)
			s.secret = ""
		}
	}
	if dialErr != "" {
		s.dialing = false
		s.connected = false
		s.Status = dialErr
		s.Fingerprint = ""
		s.secret = ""
	}
	if readErr != nil {
		s.connected = false
		s.dialing = false
		if errors.Is(readErr, io.EOF) || errors.Is(readErr, session.ErrClosed) {
			if s.Status == "已连接" {
				s.Status = "已断开"
			}
		} else {
			s.Status = readErr.Error()
		}
	}
	if load != nil {
		s.load = *load
		s.loadOK = loadNote == ""
		s.loadNote = loadNote
	}
}

func (m *Model) disconnectConn(s *conn) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.generation++
	cancel := s.cancel
	client := s.client
	reply := s.keyReply
	s.cancel = nil
	s.client = nil
	s.keyReply = nil
	s.pending = nil
	s.askFingerprint = ""
	s.askAuthorized = ""
	s.dialErr = ""
	s.dialOK = false
	s.readErr = nil
	s.writeCh = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	sendReply(reply, errHostKeyRejected)
	if client != nil {
		client.Close()
	}
	s.connected = false
	s.dialing = false
	s.Status = "未连接"
	s.Fingerprint = ""
	s.secret = ""
	s.pendingEcho = nil
	s.altScreen = false
	next := m.sessions[:0]
	for _, item := range m.sessions {
		if item != s {
			next = append(next, item)
		}
	}
	m.sessions = next
	for i := range m.panes {
		if m.panes[i].sessionID == s.id {
			m.panes[i].sessionID = 0
			m.panes[i].showFiles = false
		}
	}
	if m.focusedConn() == nil {
		m.connected = false
		m.dialing = false
		m.client = nil
		m.writeCh = nil
		m.Fingerprint = ""
		m.Status = "未连接"
		m.closeFiles()
	}
	m.bindFocus()
}

func (m *Model) disconnectAll() {
	sessions := append([]*conn(nil), m.sessions...)
	for _, s := range sessions {
		m.disconnectConn(s)
	}
	m.connected = false
	m.dialing = false
	m.client = nil
	m.Status = "未连接"
	m.Fingerprint = ""
	m.closeFiles()
}

func (m *Model) publishConnErr(s *conn, gen int, err error, secret string) {
	if s == nil || err == nil {
		return
	}
	msg := redactSecret(err.Error(), secret)
	s.mu.Lock()
	if s.generation != gen {
		s.mu.Unlock()
		return
	}
	s.dialErr = msg
	s.mu.Unlock()
	m.wakeUp()
}

func (m *Model) confirmConnKey(ctx context.Context, s *conn, gen int, host store.Host, keys *store.Keys, key ssh.PublicKey) error {
	if key == nil {
		return errors.New("session: 缺少主机公钥")
	}
	authorized := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))
	if trusted, ok := keys.Trusted(host.Address, host.Port); ok && trusted == authorized {
		return nil
	}
	reply := make(chan error, 1)
	s.mu.Lock()
	if s.generation != gen {
		s.mu.Unlock()
		return ctx.Err()
	}
	s.keyReply = reply
	s.askFingerprint = ssh.FingerprintSHA256(key)
	s.askAuthorized = authorized
	s.mu.Unlock()
	m.wakeUp()
	select {
	case err := <-reply:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Model) dialConn(ctx context.Context, s *conn, gen int, host store.Host, password string, cols, rows int) {
	address := net.JoinHostPort(host.Address, strconv.Itoa(host.Port))
	keys := store.OpenKeys(store.KeysPath(m.path))
	signers, pass := authCallbacks(host, password)
	client, err := session.Dial(ctx, address, session.Config{
		User: host.User,
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			return m.confirmConnKey(ctx, s, gen, host, keys, key)
		},
		Password: pass,
		Signers:  signers,
	})
	if err != nil {
		m.publishConnErr(s, gen, err, password)
		return
	}
	s.mu.Lock()
	if s.generation != gen {
		s.mu.Unlock()
		client.Close()
		return
	}
	s.client = client
	s.mu.Unlock()
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	if err := client.StartPTY(cols, rows); err != nil {
		client.Close()
		m.publishConnErr(s, gen, err, password)
		return
	}
	ch := make(chan []byte, 64)
	s.mu.Lock()
	if s.generation != gen {
		s.mu.Unlock()
		client.Close()
		return
	}
	s.writeCh = ch
	s.dialOK = true
	s.mu.Unlock()
	go m.writeLoop(ctx, client, ch)
	go m.readConn(s, gen, client)
	go m.watchConn(ctx, s, gen)
	m.wakeUp()
}

func (m *Model) readConn(s *conn, gen int, client *session.Client) {
	buf := make([]byte, 4096)
	for {
		n, err := client.Read(buf)
		s.mu.Lock()
		current := s.generation == gen
		if current && n > 0 {
			s.pending = append(s.pending, append([]byte(nil), buf[:n]...)...)
		}
		if current && err != nil {
			s.readErr = err
		}
		s.mu.Unlock()
		if current {
			m.wakeUp()
		}
		if err != nil || !current {
			return
		}
	}
}

func (m *Model) watchConn(ctx context.Context, s *conn, gen int) {
	var cpu cpuSample
	var net netSample
	m.sampleConn(ctx, s, gen, &cpu, &net)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !m.sampleConn(ctx, s, gen, &cpu, &net) {
				return
			}
		}
	}
}

func (m *Model) sampleConn(ctx context.Context, s *conn, gen int, cpu *cpuSample, net *netSample) bool {
	if s == nil || ctx.Err() != nil {
		return false
	}
	s.mu.Lock()
	current := s.generation == gen && s.client != nil
	client := s.client
	s.mu.Unlock()
	if !current {
		return false
	}
	out, err := client.Output(loadScript)
	if err != nil {
		s.mu.Lock()
		s.pendingLoad = &serverLoad{}
		s.pendingLoadNote = "无法读取负载"
		s.mu.Unlock()
		m.wakeUp()
		return ctx.Err() == nil && s.generation == gen
	}
	load, nextCPU, nextNet, err := parseLoad(string(out), time.Now(), *cpu, *net)
	if err != nil {
		s.mu.Lock()
		s.pendingLoad = &serverLoad{}
		s.pendingLoadNote = "无法读取负载"
		s.mu.Unlock()
		m.wakeUp()
		return s.generation == gen
	}
	*cpu = nextCPU
	*net = nextNet
	s.mu.Lock()
	s.pendingLoad = &load
	s.pendingLoadNote = ""
	s.mu.Unlock()
	m.wakeUp()
	return s.generation == gen
}

func paneTermID(index int) vivaui.ID {
	return paneTerms.ID(vivaui.Index(index))
}

func (m *Model) acceptsTerminal(id vivaui.ID) bool {
	if m == nil || !m.connected && (m.focusedConn() == nil || !m.focusedConn().connected) {
		return false
	}
	m.ensurePanes()
	return id == terminalID || id == paneTermID(m.focus)
}

func (m *Model) paneIndex(id vivaui.ID) int {
	m.ensurePanes()
	for i := range m.panes {
		if id == paneTermID(i) {
			return i
		}
	}
	if id == terminalID {
		return m.focus
	}
	return -1
}

func (m *Model) paintWorkArea(frame *flex.Frame, ink skiffPalette, cellW, cellH float32, target string) {
	m.ensurePanes()
	m.bindFocus()
	closeAt := -1
	frame.Row().Grow(1).FillWidth().Gap(gapSmall).Children(func() {
		n := len(m.panes)
		for i := 0; i < n; i++ {
			if m.paintPane(frame, ink, i, cellW, cellH, target) {
				closeAt = i
			}
		}
	})
	if closeAt >= 0 {
		m.closePane(closeAt)
	}
}

func (m *Model) paintPane(frame *flex.Frame, ink skiffPalette, index int, cellW, cellH float32, target string) bool {
	if index < 0 || index >= len(m.panes) {
		return false
	}
	focused := index == m.focus
	c := m.connByID(m.panes[index].sessionID)
	closeRequested := false
	border := paneBorder(ink, focused)
	col := frame.Column().Grow(1).FillHeight().MinWidth(280).Background(border)
	if col.Children(func() {
		header := frame.Row().FillWidth().Height(36).PadX(8).Gap(gapSmall).Items(flex.Center).Background(ink.chrome)
		header.Hit(paneFocus.ID(vivaui.Index(index)), "聚焦这一栏")
		if header.Children(func() {
			labels, ids := m.sessionChoices()
			if len(labels) > 0 {
				current := paneChoiceIndex(m.panes[index].sessionID, ids)
				if n, ok := frame.Choice(panePick(index), labels, current).ChosenIndex(); ok && n < len(ids) && ids[n] != m.panes[index].sessionID {
					m.panes[index].sessionID = ids[n]
					if focused {
						m.bindFocus()
					}
				}
			} else {
				frame.Text("空栏").Muted()
			}
			frame.Spacer()
			if len(m.panes) > 1 && frame.Button(closePane.ID(vivaui.Index(index)), "关栏").Ghost().Small().Clicked() {
				closeRequested = true
			}
		}).Clicked() && !focused {
			m.focus = index
			m.bindFocus()
		}
		body := frame.Column().Grow(1).Fill().MinHeight(180).Background(ink.terminal).Pad(terminalPad)
		if focused && m.panes[index].showFiles && m.connected {
			body.Children(func() { m.paintFiles(frame, ink) })
		} else if focused && c == nil {
			if m.connected {
				body.HitWith(terminalID, "终端输入", vivaui.HitOptions{Role: vivaui.RoleTerminal})
			}
			lines := terminalLines(m)
			m.noteTerminal(lines)
			body.Children(func() {
				m.paintTerminalLines(frame, ink, lines, m.Grid, m.connected, m.dialing, target, cellW, cellH)
			})
		} else {
			connected := c != nil && c.connected
			if focused && connected {
				body.HitWith(paneTermID(index), "终端输入", vivaui.HitOptions{Role: vivaui.RoleTerminal})
			}
			body.Children(func() {
				if c == nil {
					frame.Text("这一栏还没有会话").Color(ink.muted)
					return
				}
				lines := connLines(c)
				m.paintTerminalLines(frame, ink, lines, c.Grid, connected, c.dialing, c.host.User+"@"+c.host.Address, cellW, cellH)
			})
		}
		if focused {
			m.paneBox = body
		}
	}).Clicked() && !focused && !closeRequested {
		m.focus = index
		m.bindFocus()
	}
	return closeRequested
}

func paneBorder(ink skiffPalette, focused bool) vivaui.Color {
	if focused {
		return ink.selected
	}
	return ink.chrome
}

func paneChoiceIndex(sessionID int, ids []int) int {
	for n, id := range ids {
		if id == sessionID {
			return n
		}
	}
	return -1
}

func panePick(index int) vivaui.Scope {
	switch index {
	case 1:
		return panePick1
	case 2:
		return panePick2
	default:
		return panePick0
	}
}

func (m *Model) sessionChoices() ([]string, []int) {
	labels := make([]string, 0, len(m.sessions))
	ids := make([]int, 0, len(m.sessions))
	for _, s := range m.sessions {
		if s == nil {
			continue
		}
		label := s.host.User + "@" + s.host.Address
		if s.connected {
			label += " 已连接"
		} else if s.dialing {
			label += " 连接中"
		}
		labels = append(labels, label)
		ids = append(ids, s.id)
	}
	return labels, ids
}

func connLines(s *conn) []string {
	if s == nil || s.Grid == nil {
		if s == nil || strings.TrimSpace(s.Screen) == "" {
			if s == nil {
				return []string{"未连接"}
			}
			return []string{statusText(s.Status)}
		}
		return strings.Split(s.Screen, "\n")
	}
	_, rows := s.Grid.Size()
	x, cursorY := s.Grid.Cursor()
	last := cursorY
	for y := rows - 1; y > last; y-- {
		if s.Grid.Row(y) != "" {
			last = y
			break
		}
	}
	lines := make([]string, 0, last+1)
	for y := 0; y <= last; y++ {
		line := s.Grid.Row(y)
		if y == cursorY {
			line = withCursor(line, x) + string(s.pendingEcho)
		}
		if line == "" {
			line = " "
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return []string{statusText(s.Status)}
	}
	return lines
}

func (m *Model) paintTerminalLines(frame *flex.Frame, ink skiffPalette, lines []string, grid *term.Grid, connected, dialing bool, target string, cellW, cellH float32) {
	empty := grid == nil
	if empty && !connected {
		hint := "选择左边的主机，然后点连接"
		if _, ok := selectedHost(m); ok && m.remembered {
			hint = "口令已记住，点连接或再点一次主机"
		} else if _, ok := selectedHost(m); ok {
			hint = "输入口令后点连接，之后不用再填"
		}
		frame.Text(hint).Color(ink.muted)
	}
	if dialing && empty {
		frame.Text("正在连接 " + target).Color(ink.ink)
	}
	if connected && empty {
		frame.Text("点击这里输入").Color(ink.muted)
	}
	if grid != nil {
		cols, _ := grid.Size()
		_, cursorY := grid.Cursor()
		last := cursorY
		_, rows := grid.Size()
		for y := rows - 1; y > last; y-- {
			if grid.Row(y) != "" {
				last = y
				break
			}
		}
		lineWidth := float32(cols) * cellW
		if lineWidth < cellW {
			lineWidth = cellW
		}
		for y := 0; y <= last; y++ {
			frame.Row().Height(cellH).MaxWidth(lineWidth).Children(func() {
				for _, run := range grid.LineRuns(y) {
					text := frame.Text(run.Text).Mono().FontSize(terminalFontSize).Height(cellH)
					if run.Bold {
						text.Bold()
					}
					if run.FG.Set {
						text.Color(vivaui.RGB(run.FG.R, run.FG.G, run.FG.B))
					} else {
						text.Color(ink.ink)
					}
					if run.BG.Set {
						text.Background(vivaui.RGB(run.BG.R, run.BG.G, run.BG.B))
					}
				}
			})
		}
		return
	}
	for _, line := range lines {
		if empty && !connected && line == statusText(m.Status) {
			continue
		}
		frame.Text(line).Mono().FontSize(terminalFontSize).Height(cellH).Color(ink.ink)
	}
}
