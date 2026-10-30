package ui

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SwalkTech/Skiff/store"
	"github.com/vivaui/viva/app"
	vivaui "github.com/vivaui/viva/ui"
	"golang.org/x/crypto/ssh"
)

func TestConnectConfirmsHostKeyBeforePassword(t *testing.T) {
	var auths atomic.Int32
	addr := startBannerServer(t, &auths, "secret-password")
	host, port := splitHostPort(t, addr)
	path := filepath.Join(t.TempDir(), "hosts.json")
	model, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	model.Hosts = []store.Host{{Address: host, User: "alice", Port: port}}
	model.Selected = 0
	model.password.SetText("secret-password")
	driver := newDesk(t, model)

	clickID(t, driver, connectID)
	waitLabel(t, driver, "确认主机密钥")
	if auths.Load() != 0 {
		t.Fatalf("确认密钥前服务器收到了 %d 次认证", auths.Load())
	}
	if hasLabel(driver.Snapshot(), "secret-password") {
		t.Fatal("确认界面显示了口令")
	}

	clickID(t, driver, acceptID)
	waitLabel(t, driver, "skiff-ready")
	snapshot := driver.Snapshot()
	if !hasExactLabel(snapshot, "已连接") {
		t.Fatal("接受密钥后没有连上")
	}
	if hasLabel(snapshot, "secret-password") {
		t.Fatal("连接后界面显示了口令")
	}
	raw, err := os.ReadFile(store.KeysPath(path))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) == "" || containsText(raw, "secret-password") || containsText(raw, "password") {
		t.Fatalf("密钥文件不应包含口令: %s", raw)
	}

	clickID(t, driver, connectID)
	if model.connected {
		t.Fatal("断开后会话仍标记为已连接")
	}
	if !hasLabel(driver.Snapshot(), "未连接") {
		t.Fatal("断开后状态没有回到未连接")
	}
}

func TestRejectHostKeyDoesNotAuthenticate(t *testing.T) {
	var auths atomic.Int32
	addr := startBannerServer(t, &auths, "secret-password")
	host, port := splitHostPort(t, addr)
	model := &Model{
		Hosts:    []store.Host{{Address: host, User: "alice", Port: port}},
		Selected: 0,
		Status:   "未连接",
	}
	model.password.SetText("secret-password")
	driver := newDesk(t, model)
	clickID(t, driver, connectID)
	waitLabel(t, driver, "确认主机密钥")
	clickID(t, driver, rejectID)
	waitLabel(t, driver, "主机密钥被拒绝")
	if auths.Load() != 0 {
		t.Fatalf("拒绝密钥后服务器收到了 %d 次认证", auths.Load())
	}
	if model.connected {
		t.Fatal("拒绝密钥后仍标记为已连接")
	}
}

func waitLabel(t *testing.T, driver *app.Driver, want string) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-driver.Wakes():
		case <-time.After(30 * time.Millisecond):
		}
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		if hasLabel(driver.Snapshot(), want) {
			return
		}
	}
	t.Fatalf("超时，没有看到 %s", want)
}

func splitHostPort(t *testing.T, addr string) (string, int) {
	t.Helper()
	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	return host, port
}

func containsText(haystack []byte, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && indexText(haystack, needle) >= 0
}

func indexText(haystack []byte, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if string(haystack[i:i+len(needle)]) == needle {
			return i
		}
	}
	return -1
}

func startBannerServer(t *testing.T, auths *atomic.Int32, password string) string {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	config := &ssh.ServerConfig{
		PasswordCallback: func(conn ssh.ConnMetadata, got []byte) (*ssh.Permissions, error) {
			auths.Add(1)
			if conn.User() == "alice" && string(got) == password {
				return nil, nil
			}
			return nil, errors.New("认证失败")
		},
	}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go serveBanner(conn, config)
		}
	}()
	return listener.Addr().String()
}

func serveBanner(conn net.Conn, config *ssh.ServerConfig) {
	serverConn, channels, requests, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return
	}
	defer serverConn.Close()
	go ssh.DiscardRequests(requests)
	for newChannel := range channels {
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "不支持的通道")
			continue
		}
		channel, reqs, err := newChannel.Accept()
		if err != nil {
			continue
		}
		go func() {
			defer channel.Close()
			for req := range reqs {
				ok := req.Type == "pty-req" || req.Type == "shell" || req.Type == "window-change"
				if req.WantReply {
					req.Reply(ok, nil)
				}
				if req.Type == "shell" {
					_, _ = channel.Write([]byte("skiff-ready\r\n"))
				}
			}
		}()
	}
}

type memoryPasswords struct {
	saved map[string]string
}

func (m *memoryPasswords) Load(address, user string, port int) (string, bool) {
	password, ok := m.saved[store.PasswordTarget(address, user, port)]
	return password, ok && password != ""
}

func (m *memoryPasswords) Save(address, user string, port int, password string) error {
	if m.saved == nil {
		m.saved = map[string]string{}
	}
	m.saved[store.PasswordTarget(address, user, port)] = password
	return nil
}

func (m *memoryPasswords) Delete(address, user string, port int) error {
	delete(m.saved, store.PasswordTarget(address, user, port))
	return nil
}

var _ = vivaui.KeyEnter
