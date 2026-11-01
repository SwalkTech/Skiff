package ui

import (
	"strings"
	"testing"

	"github.com/SwalkTech/Skiff/store"
)

func TestPaneChoiceIndexIgnoresEmptyPane(t *testing.T) {
	if got := paneChoiceIndex(0, []int{1, 2}); got != -1 {
		t.Fatalf("空栏下标 = %d，希望 -1", got)
	}
	if got := paneChoiceIndex(2, []int{1, 2}); got != 1 {
		t.Fatalf("匹配下标 = %d，希望 1", got)
	}
}

func TestAddPaneStartsEmpty(t *testing.T) {
	model := &Model{
		Selected: -1,
		sessions: []*conn{{id: 1, host: store.Host{Address: "a.example", User: "ann", Port: 22}}},
		panes:    []pane{{sessionID: 1}},
	}
	model.addPane()
	if len(model.panes) != 2 {
		t.Fatalf("分栏数 = %d", len(model.panes))
	}
	if model.panes[1].sessionID != 0 {
		t.Fatalf("新栏会话 = %d，希望为空", model.panes[1].sessionID)
	}
}

func TestWindowTitleFollowsFocusedPane(t *testing.T) {
	model := &Model{
		Status: "未连接",
		Hosts: []store.Host{
			{User: "ann", Address: "a.example", Port: 22},
			{User: "bob", Address: "b.example", Port: 22},
		},
		Selected: 0,
		sessions: []*conn{{id: 1, host: store.Host{User: "bob", Address: "b.example", Port: 22}}},
		panes:    []pane{{sessionID: 1}},
	}
	title := windowTitle(model)
	if !strings.Contains(title, "b.example") || strings.Contains(title, "a.example") {
		t.Fatalf("标题跟随了左侧列表而不是当前分栏: %q", title)
	}
	if model.connectLabel() != "连接" {
		t.Fatalf("选中主机未连接时按钮 = %q", model.connectLabel())
	}
	model.sessions[0].connected = true
	if model.connectLabel() != "连接" {
		t.Fatalf("按钮跟着了当前分栏而不是选中主机: %q", model.connectLabel())
	}
	model.Selected = 1
	if model.connectLabel() != "断开" {
		t.Fatalf("选中已连接主机时按钮 = %q", model.connectLabel())
	}
}
