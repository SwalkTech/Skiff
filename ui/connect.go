package ui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/SwalkTech/Skiff/session"
	"github.com/SwalkTech/Skiff/store"
	"github.com/SwalkTech/Skiff/term"
	"github.com/vivaui/viva/app"
	vivaui "github.com/vivaui/viva/ui"
	"golang.org/x/crypto/ssh"
)

var errHostKeyRejected = errors.New("主机密钥被拒绝")

func (m *Model) hostHasPassword(host store.Host) bool {
	key := store.PasswordTarget(host.Address, host.User, host.Port)
	if m.passwordCache == nil {
		m.passwordCache = map[string]bool{}
	}
	if saved, ok := m.passwordCache[key]; ok {
		return saved
	}
	_, ok := m.passwordStore().Load(host.Address, host.User, host.Port)
	m.passwordCache[key] = ok
	return ok
}

func (m *Model) passwordStore() store.PasswordStore {
	if m.passwords != nil {
		return m.passwords
	}
	return store.SystemPasswords()
}

func (m *Model) recallPassword(host store.Host) {
	if saved, ok := m.passwordStore().Load(host.Address, host.User, host.Port); ok {
		m.password.SetText(saved)
		m.remembered = true
		return
	}
	m.password.SetText("")
	m.remembered = false
}

func (m *Model) passwordFor(host store.Host) string {
	if password := strings.TrimSpace(m.password.Text()); password != "" {
		return password
	}
	m.recallPassword(host)
	return strings.TrimSpace(m.password.Text())
}

func (m *Model) rememberPassword(host store.Host, password string) {
	password = strings.TrimSpace(password)
	if password == "" {
		return
	}
	if err := m.passwordStore().Save(host.Address, host.User, host.Port, password); err != nil {
		return
	}
	if m.passwordCache == nil {
		m.passwordCache = map[string]bool{}
	}
	m.passwordCache[store.PasswordTarget(host.Address, host.User, host.Port)] = true
	m.password.SetText(password)
	m.remembered = true
	m.changePassword = false
}

func (m *Model) forgetPassword(host store.Host) {
	_ = m.passwordStore().Delete(host.Address, host.User, host.Port)
	delete(m.passwordCache, store.PasswordTarget(host.Address, host.User, host.Port))
	if selected, ok := selectedHost(m); !ok || selected == host {
		m.password.SetText("")
		m.remembered = false
	}
}

func (m *Model) connectLabel() string {
	if host, ok := selectedHost(m); ok {
		if s := m.sessionFor(host); s != nil {
			switch {
			case s.connected:
				return "断开"
			case s.dialing:
				return "取消"
			}
		}
	}
	switch {
	case m.connected:
		return "断开"
	case m.dialing:
		return "取消"
	default:
		return "连接"
	}
}

func (m *Model) connectSelected() {
	host, ok := selectedHost(m)
	if !ok {
		m.Status = "请先选择一台主机"
		return
	}
	if s := m.sessionFor(host); s != nil && (s.connected || s.dialing) {
		m.disconnectConn(s)
		return
	}
	password := m.passwordFor(host)
	if password == "" {
		m.Status = "请输入口令"
		return
	}
	s := m.openConn(host)
	s.secret = password
	m.assignFocused(s)
	cols, rows := 80, 24
	if m.Grid != nil {
		cols, rows = m.Grid.Size()
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.generation++
	gen := s.generation
	if s.cancel != nil {
		s.cancel()
	}
	s.cancel = cancel
	s.dialErr = ""
	s.dialOK = false
	s.askFingerprint = ""
	s.askAuthorized = ""
	s.mu.Unlock()
	s.dialing = true
	s.connected = false
	s.Fingerprint = ""
	s.Grid = nil
	s.Screen = ""
	s.Status = "正在连接"
	m.bindFocus()
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				m.publishConnErr(s, gen, fmt.Errorf("连接中断: %v", recovered), password)
			}
		}()
		m.dialConn(ctx, s, gen, host, password, cols, rows)
	}()
}

func (m *Model) dial(ctx context.Context, gen int, host store.Host, password string, cols, rows int) {
	address := net.JoinHostPort(host.Address, strconv.Itoa(host.Port))
	keys := store.OpenKeys(store.KeysPath(m.path))
	client, err := session.Dial(ctx, address, session.Config{
		User: host.User,
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			return m.confirmHostKey(ctx, gen, host, keys, key)
		},
		Password: func() (string, error) {
			return password, nil
		},
	})
	if err != nil {
		m.publishDialErr(gen, err, password)
		return
	}
	m.mu.Lock()
	if m.generation != gen {
		m.mu.Unlock()
		client.Close()
		return
	}
	m.client = client
	m.mu.Unlock()
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	if err := client.StartPTY(cols, rows); err != nil {
		client.Close()
		m.publishDialErr(gen, err, password)
		return
	}
	ch := make(chan []byte, 64)
	m.mu.Lock()
	if m.generation != gen {
		m.mu.Unlock()
		client.Close()
		return
	}
	m.writeCh = ch
	m.dialOK = true
	m.mu.Unlock()
	go m.writeLoop(ctx, client, ch)
	go m.readLoop(gen, client)
	go m.watchLoad(ctx, gen)
	m.wakeUp()
}

func (m *Model) confirmHostKey(ctx context.Context, gen int, host store.Host, keys *store.Keys, key ssh.PublicKey) error {
	if key == nil {
		return errors.New("session: 缺少主机公钥")
	}
	authorized := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))
	if trusted, ok := keys.Trusted(host.Address, host.Port); ok && trusted == authorized {
		return nil
	}
	reply := make(chan error, 1)
	m.mu.Lock()
	if m.generation != gen {
		m.mu.Unlock()
		return ctx.Err()
	}
	m.keyReply = reply
	m.askFingerprint = ssh.FingerprintSHA256(key)
	m.askAuthorized = authorized
	m.mu.Unlock()
	m.wakeUp()
	select {
	case err := <-reply:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Model) acceptHostKey() {
	if s := m.promptingConn(); s != nil {
		s.mu.Lock()
		authorized := s.askAuthorized
		reply := s.keyReply
		host := s.host
		s.keyReply = nil
		s.askFingerprint = ""
		s.askAuthorized = ""
		s.mu.Unlock()
		if authorized != "" {
			_ = store.OpenKeys(store.KeysPath(m.path)).Remember(host.Address, host.Port, authorized)
		}
		s.Fingerprint = ""
		s.Status = "正在连接"
		m.bindFocus()
		sendReply(reply, nil)
		return
	}
	m.mu.Lock()
	authorized := m.askAuthorized
	reply := m.keyReply
	m.keyReply = nil
	m.askFingerprint = ""
	m.askAuthorized = ""
	m.mu.Unlock()
	if host, ok := selectedHost(m); ok && authorized != "" {
		_ = store.OpenKeys(store.KeysPath(m.path)).Remember(host.Address, host.Port, authorized)
	}
	m.Fingerprint = ""
	m.Status = "正在连接"
	sendReply(reply, nil)
}

func (m *Model) rejectHostKey() {
	m.mu.Lock()
	reply := m.keyReply
	m.keyReply = nil
	m.askFingerprint = ""
	m.askAuthorized = ""
	m.mu.Unlock()
	m.Fingerprint = ""
	m.Status = errHostKeyRejected.Error()
	sendReply(reply, errHostKeyRejected)
}

func sendReply(reply chan error, err error) {
	if reply == nil {
		return
	}
	select {
	case reply <- err:
	default:
	}
}

func (m *Model) publishDialErr(gen int, err error, secret string) {
	if err == nil {
		return
	}
	msg := strings.ReplaceAll(err.Error(), secret, "******")
	m.mu.Lock()
	if m.generation != gen {
		m.mu.Unlock()
		return
	}
	m.dialErr = msg
	m.mu.Unlock()
	m.wakeUp()
}

func (m *Model) readLoop(gen int, client *session.Client) {
	buf := make([]byte, 4096)
	for {
		n, err := client.Read(buf)
		m.mu.Lock()
		current := m.generation == gen
		if current && n > 0 {
			m.pending = append(m.pending, append([]byte(nil), buf[:n]...)...)
		}
		if current && err != nil {
			m.readErr = err
		}
		m.mu.Unlock()
		if current {
			m.wakeUp()
		}
		if err != nil || !current {
			return
		}
	}
}

func (m *Model) wake() {
	m.applyFiles()
	m.applyLoad()
	m.applyUpdateNotice()
	for _, s := range m.sessions {
		m.applyConn(s)
	}
	m.bindFocus()
	m.mu.Lock()
	data := m.pending
	m.pending = nil
	fingerprint := m.askFingerprint
	dialErr := m.dialErr
	m.dialErr = ""
	connected := m.dialOK
	m.dialOK = false
	readErr := m.readErr
	m.readErr = nil
	m.mu.Unlock()

	if fingerprint != "" {
		m.Fingerprint = fingerprint
		m.Status = "正在连接"
	}
	if m.Grid == nil && (len(data) > 0 || connected) {
		m.Grid = term.New(80, 24)
	}
	if len(data) > 0 {
		m.altScreen = altScreenAfter(data, m.altScreen)
		if m.altScreen {
			m.pendingEcho = nil
		} else {
			data, m.pendingEcho = consumeEcho(data, m.pendingEcho)
		}
	}
	if len(data) > 0 && m.Grid != nil {
		m.Grid.Write(data)
	}
	if connected {
		m.connected = true
		m.dialing = false
		m.Fingerprint = ""
		m.Status = "已连接"
		if host, ok := selectedHost(m); ok {
			m.rememberPassword(host, m.password.Text())
		}
	}
	if dialErr != "" {
		m.dialing = false
		m.connected = false
		m.Fail(dialErr)
		m.Fingerprint = ""
	}
	if readErr != nil {
		m.connected = false
		m.dialing = false
		if errors.Is(readErr, io.EOF) || errors.Is(readErr, session.ErrClosed) {
			if m.Status == "已连接" {
				m.Status = "已断开"
			}
			return
		}
		m.Fail(readErr.Error())
	}
}

func (m *Model) disconnect() {
	m.mu.Lock()
	m.generation++
	cancel := m.cancel
	client := m.client
	reply := m.keyReply
	m.cancel = nil
	m.client = nil
	m.keyReply = nil
	m.pending = nil
	m.askFingerprint = ""
	m.askAuthorized = ""
	m.dialErr = ""
	m.dialOK = false
	m.readErr = nil
	m.writeCh = nil
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	sendReply(reply, errHostKeyRejected)
	m.closeFiles()
	if client != nil {
		client.Close()
	}
	m.connected = false
	m.dialing = false
	m.Fingerprint = ""
	m.Status = "未连接"
	m.load = serverLoad{}
	m.loadOK = false
	m.loadNote = ""
	m.pendingEcho = nil
	m.altScreen = false
	m.stableCols = 0
	m.stableRows = 0
}

func (m *Model) wakeUp() {
	if m.wakePending.CompareAndSwap(false, true) {
		time.AfterFunc(8*time.Millisecond, func() {
			m.wakePending.Store(false)
			m.invalidator.Invalidate()
		})
	}
}

func (m *Model) writeLoop(ctx context.Context, client *session.Client, ch <-chan []byte) {
	for {
		select {
		case <-ctx.Done():
			return
		case buf, ok := <-ch:
			if !ok {
				return
			}
		drain:
			for {
				select {
				case more, ok := <-ch:
					if !ok {
						break drain
					}
					buf = append(buf, more...)
				default:
					break drain
				}
			}
			client.Write(buf)
		}
	}
}

func (m *Model) writeSession(p []byte) {
	if len(p) == 0 {
		return
	}
	var ch chan []byte
	if s := m.focusedConn(); s != nil {
		s.mu.Lock()
		if s.connected {
			ch = s.writeCh
		}
		s.mu.Unlock()
	}
	if ch == nil && m.connected {
		ch = m.writeCh
	}
	if ch == nil {
		return
	}
	buf := append([]byte(nil), p...)
	select {
	case ch <- buf:
	default:
		go func() { ch <- buf }()
	}
}

func (m *Model) noteResize() {
	if !m.connected || m.Grid == nil {
		return
	}
	cols, rows := m.Grid.Size()
	if cols == m.sentCols && rows == m.sentRows {
		return
	}
	m.sentCols, m.sentRows = cols, rows
	m.mu.Lock()
	client := m.client
	m.mu.Unlock()
	if client == nil {
		return
	}
	go func() {
		m.writeMu.Lock()
		defer m.writeMu.Unlock()
		client.Resize(cols, rows)
	}()
}

func (m *Model) onKey(event vivaui.KeyEvent) vivaui.EventResult {
	if idx := m.paneIndex(event.Target); idx >= 0 {
		m.focus = idx
		m.bindFocus()
	}
	if m.connected && (event.Target == terminalID || event.Target == paneTermID(m.focus)) && !event.Release {
		if seq := keySequence(event); len(seq) > 0 {
			m.writeSession(seq)
			return vivaui.EventResult{Handled: true}
		}
	}
	return m.split.Key(event)
}

func (m *Model) onShortcut(event vivaui.KeyEvent) vivaui.EventResult {
	if m.connected && event.Modifiers&vivaui.ModifierControl != 0 && strings.EqualFold(event.Text, "c") {
		m.writeSession([]byte{3})
		return vivaui.EventResult{Handled: true}
	}
	return vivaui.EventResult{}
}

func (m *Model) onTerminalText(event app.TextEvent) (app.TextState, error) {
	if s := m.focusedConn(); s == nil || !s.connected {
		if !m.connected {
			return app.TextState{}, nil
		}
	}
	redraw := false
	switch event.Kind {
	case app.TextInsert, app.TextPaste:
		redraw = m.noteLocalEcho(event.Text)
		m.writeSession([]byte(event.Text))
	case app.TextCommandEvent:
		if event.Command == app.TextDeleteBackward {
			redraw = m.popLocalEcho()
		}
		m.writeSession(commandBytes(event.Command))
	}
	return app.TextState{Redraw: redraw}, nil
}

func keySequence(event vivaui.KeyEvent) []byte {
	if event.Modifiers&vivaui.ModifierControl != 0 && strings.EqualFold(event.Text, "c") {
		return []byte{3}
	}
	switch event.Key {
	case vivaui.KeyEnter:
		return []byte{'\r'}
	case vivaui.KeyBackspace:
		return []byte{0x7f}
	case vivaui.KeyTab:
		return []byte{'\t'}
	case vivaui.KeyUp:
		return []byte("\x1b[A")
	case vivaui.KeyDown:
		return []byte("\x1b[B")
	case vivaui.KeyRight:
		return []byte("\x1b[C")
	case vivaui.KeyLeft:
		return []byte("\x1b[D")
	case vivaui.KeyHome:
		return []byte("\x1b[H")
	case vivaui.KeyEnd:
		return []byte("\x1b[F")
	case vivaui.KeyDelete:
		return []byte("\x1b[3~")
	default:
		if event.Text != "" && event.Modifiers&(vivaui.ModifierControl|vivaui.ModifierAlt) == 0 {
			return []byte(event.Text)
		}
		return nil
	}
}

func commandBytes(command app.TextCommand) []byte {
	switch command {
	case app.TextInsertNewline, app.TextInsertLineBreak:
		return []byte{'\r'}
	case app.TextInsertTab:
		return []byte{'\t'}
	case app.TextDeleteBackward:
		return []byte{0x7f}
	case app.TextDeleteForward:
		return []byte("\x1b[3~")
	case app.TextMoveLeft:
		return []byte("\x1b[D")
	case app.TextMoveRight:
		return []byte("\x1b[C")
	default:
		return nil
	}
}
