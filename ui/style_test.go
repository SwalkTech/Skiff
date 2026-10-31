package ui

import (
	"strings"
	"testing"
)

func TestHostCardStyleDiffersFromHover(t *testing.T) {
	ink := colors(true)
	bg, border, width := hostCardStyle(ink, true)
	if bg == ink.elevated {
		t.Fatal("选中填充和悬停相同")
	}
	if border != ink.accent || width < 2 {
		t.Fatalf("选中边框 = %v 宽度 %v", border, width)
	}
	_, idleBorder, idleWidth := hostCardStyle(ink, false)
	if idleBorder == ink.accent || idleWidth >= 2 {
		t.Fatal("未选中主机不该使用强调边框")
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

func TestLoadDetailOmitsRepeatedCPU(t *testing.T) {
	lines := loadDetailLines(serverLoad{CPU: 42, MemUsed: 1024, MemTotal: 2048})
	for _, line := range lines {
		if strings.Contains(line, "CPU") {
			t.Fatalf("负载正文重复了标题里的 CPU: %q", line)
		}
	}
}

func TestThemeLabelShowsCurrentTheme(t *testing.T) {
	if themeLabel(true) != "暗色" || themeLabel(false) != "亮色" {
		t.Fatal("主题按钮没有显示当前主题")
	}
}

func TestFileRowStyleMarksSelection(t *testing.T) {
	ink := colors(true)
	_, border, width := fileRowStyle(ink, true)
	if border != ink.accent || width < 2 {
		t.Fatal("选中文件没有强调边框")
	}
	_, idle, idleWidth := fileRowStyle(ink, false)
	if idle == ink.accent || idleWidth >= 2 {
		t.Fatal("未选中文件不该使用强调边框")
	}
}

func TestPromptBorderSeparatesHostKey(t *testing.T) {
	ink := colors(true)
	host, hostWidth := promptBorder(ink, true)
	update, updateWidth := promptBorder(ink, false)
	if host == update || hostWidth <= updateWidth {
		t.Fatal("主机密钥提示和更新提示边框没有分开")
	}
	if host != ink.danger {
		t.Fatal("主机密钥提示没有使用危险色边框")
	}
}

func TestPaneBorderMarksFocus(t *testing.T) {
	ink := colors(true)
	color, width := paneBorder(ink, true)
	if color != ink.accent || width < 2 {
		t.Fatalf("焦点边框 = %v 宽度 %v", color, width)
	}
	idle, idleWidth := paneBorder(ink, false)
	if idle == ink.accent || idleWidth >= 2 {
		t.Fatal("非焦点分栏不该使用强调边框")
	}
}
