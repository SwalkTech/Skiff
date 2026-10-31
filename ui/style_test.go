package ui

import "testing"

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
