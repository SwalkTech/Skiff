package ui

import (
	"strings"
	"testing"

	"github.com/SwalkTech/Skiff/store"
)

func TestHostCardStyleDiffersFromHover(t *testing.T) {
	ink := colors(true)
	bg := hostCardStyle(ink, true)
	if bg == ink.elevated {
		t.Fatal("选中填充和悬停相同")
	}
	bg = hostCardStyle(ink, false)
	if bg == ink.accent {
		t.Fatal("未选中主机不该使用强调背景")
	}
}

func TestStatusPillTextTruncates(t *testing.T) {
	long := "已选择 alice@very-long.example"
	got := statusPillText(long)
	if len([]rune(got)) > 13 {
		t.Fatalf("状态胶囊过长: %q", got)
	}
	if got == long || got[len(got)-len("…"):] != "…" && !containsEllipsis(got) {
		t.Fatalf("长状态没有省略: %q", got)
	}
	if statusPillText("已连接") != "已连接" {
		t.Fatal("短状态不该被截断")
	}
}

func containsEllipsis(s string) bool {
	return len(s) > 0 && s[len(s)-3:] == "…" || (len([]rune(s)) > 0 && []rune(s)[len([]rune(s))-1] == '…')
}

func TestLightTerminalDiffersFromWindow(t *testing.T) {
	ink := colors(false)
	if ink.terminal == ink.window || ink.terminal == ink.chrome || ink.terminal == ink.elevated {
		t.Fatal("浅色终端和窗口或卡片贴在一起")
	}
}

func TestTerminalGridSubtractsPaddingOnce(t *testing.T) {
	cols, rows := terminalGridSize(200, 100, 10, 20)
	if cols != 16 || rows != 3 {
		t.Fatalf("格子 = %d x %d，希望 16 x 3", cols, rows)
	}
}

func TestLoadPanelHiddenUntilConnected(t *testing.T) {
	if showLoadPanel(false) {
		t.Fatal("未连接时仍显示负载卡")
	}
	if !showLoadPanel(true) {
		t.Fatal("连接后没有显示负载")
	}
}

func TestLoadDetailOmitsRepeatedCPU(t *testing.T) {
	lines := loadDetailLines(serverLoad{CPU: 42, MemUsed: 1024, MemTotal: 2048})
	for _, line := range lines {
		if strings.Contains(line, "CPU") {
			t.Fatalf("负载正文重复了标题里的 CPU: %q", line)
		}
	}
}

func TestAuthCallbacksPreferKeyFile(t *testing.T) {
	host := store.Host{Address: "example.com", User: "alice", Port: 22, KeyFile: "id_rsa"}
	signers, pass := authCallbacks(host, "phrase")
	if signers == nil || pass != nil {
		t.Fatal("选择了密钥文件时仍走口令登录")
	}
	signers, pass = authCallbacks(store.Host{Address: "example.com", User: "alice", Port: 22}, "secret")
	if signers != nil || pass == nil {
		t.Fatal("没有密钥文件时没有走口令登录")
	}
}

func TestThemeLabelShowsCurrentTheme(t *testing.T) {
	if themeLabel(true) != "暗色" || themeLabel(false) != "亮色" {
		t.Fatal("主题按钮没有显示当前主题")
	}
}

func TestFileRowStyleMarksSelection(t *testing.T) {
	ink := colors(true)
	bg := fileRowStyle(ink, true)
	if bg == ink.window {
		t.Fatal("选中文件不应该用窗口底色")
	}
	bg = fileRowStyle(ink, false)
	if bg != ink.window {
		t.Fatal("未选中文件应该用窗口底色")
	}
}

func TestPromptBorderSeparatesHostKey(t *testing.T) {
	ink := colors(true)
	host := promptBorder(ink, true)
	update := promptBorder(ink, false)
	if host == update {
		t.Fatal("主机密钥提示和更新提示应该用不同背景")
	}
	if host != ink.danger {
		t.Fatal("主机密钥提示应该用危险色")
	}
}

func TestWindowIconUsesLogo(t *testing.T) {
	icon := windowIcon()
	if icon.Size != 256 || len(icon.RGBA) != 256*256*4 {
		t.Fatalf("窗口图标 = %d，字节 %d", icon.Size, len(icon.RGBA))
	}
	bitmap := logoBitmap()
	if bitmap.Width != 256 || bitmap.Height != 256 || len(bitmap.Pix) != 256*256*4 {
		t.Fatal("标题栏图标没有画出来")
	}
}

func TestPaneBorderMarksFocus(t *testing.T) {
	ink := colors(true)
	color := paneBorder(ink, true)
	if color != ink.selected {
		t.Fatal("焦点分栏应该用选中背景")
	}
	idle := paneBorder(ink, false)
	if idle != ink.chrome {
		t.Fatal("非焦点分栏应该用栏面色")
	}
}
