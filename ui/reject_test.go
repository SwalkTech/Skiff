package ui

import (
	"errors"
	"testing"

	"github.com/SwalkTech/Skiff/store"
)

func TestRejectHostKeyRepliesToPane(t *testing.T) {
	model := &Model{Selected: -1, Status: "未连接"}
	model.ensurePanes()
	reply := make(chan error, 1)
	session := &conn{
		id:             1,
		host:           store.Host{Address: "example.com", User: "alice", Port: 22},
		Status:         "正在连接",
		Fingerprint:    "SHA256:abc",
		askFingerprint: "SHA256:abc",
		askAuthorized:  "ssh-ed25519 AAAA",
		keyReply:       reply,
		dialing:        true,
		secret:         "secret-password",
	}
	model.sessions = []*conn{session}
	model.panes[0].sessionID = session.id
	model.rejectHostKey()

	select {
	case err := <-reply:
		if !errors.Is(err, errHostKeyRejected) {
			t.Fatalf("回复 = %v", err)
		}
	default:
		t.Fatal("分栏拒绝没有解除拨号等待")
	}
	if session.Fingerprint != "" || session.askFingerprint != "" {
		t.Fatal("指纹确认没有清掉")
	}
	if session.secret != "" {
		t.Fatal("拒绝后口令仍留在会话上")
	}
	if session.dialing {
		t.Fatal("拒绝后仍显示正在连接")
	}
	if model.Fingerprint != "" {
		t.Fatal("界面仍保留指纹确认")
	}
	if model.Status != errHostKeyRejected.Error() {
		t.Fatalf("状态 = %q", model.Status)
	}
}
