package ui

import (
	"testing"

	"github.com/SwalkTech/Skiff/store"
)

func TestFocusedPaneAcceptsTerminalInput(t *testing.T) {
	m := &Model{connected: true}
	m.ensurePanes()
	m.focus = 1
	m.panes = []pane{{}, {}}
	if !m.acceptsTerminal(paneTermID(1)) {
		t.Fatal("当前栏没有接收终端输入")
	}
	if m.acceptsTerminal(paneTermID(0)) {
		t.Fatal("别的栏不该抢走输入")
	}
}

func TestClosePaneLeavesTheOthersConnected(t *testing.T) {
	m := &Model{}
	a := m.openConn(store.Host{Address: "a.example", User: "ann", Port: 22})
	b := m.openConn(store.Host{Address: "b.example", User: "bob", Port: 22})
	a.connected = true
	b.connected = true
	m.assignFocused(a)
	m.addPane()
	m.panes[1].sessionID = b.id
	m.focus = 1
	m.closePane(0)
	if len(m.panes) != 1 {
		t.Fatalf("栏数 = %d，想留下 1 栏", len(m.panes))
	}
	if len(m.sessions) != 2 || !a.connected || !b.connected {
		t.Fatal("关掉一栏时把连接也断开了")
	}
	if m.focusedConn() != b {
		t.Fatal("剩下的不是原来的第二栏")
	}
}
