package term

import "testing"

func TestWritePrintableAndNewlines(t *testing.T) {
	g := New(4, 2)
	if _, err := g.Write([]byte("中b\r\ncd")); err != nil {
		t.Fatalf("写入失败: %v", err)
	}
	if got := g.Row(0); got != "中b" {
		t.Fatalf("第一行 = %q", got)
	}
	if got := g.Row(1); got != "cd" {
		t.Fatalf("第二行 = %q", got)
	}
	x, y := g.Cursor()
	if x != 2 || y != 1 {
		t.Fatalf("光标 = (%d,%d)", x, y)
	}
}

func TestBackspaceMovesCursorWithoutErasing(t *testing.T) {
	g := New(4, 1)
	g.Write([]byte("ab\b"))
	if got := g.Row(0); got != "ab" {
		t.Fatalf("退格后 = %q", got)
	}
	if x, _ := g.Cursor(); x != 1 {
		t.Fatalf("退格后光标列 = %d", x)
	}
}

func TestWriteKeepsSplitUTF8(t *testing.T) {
	text := []byte("中")
	g := New(4, 1)
	g.Write(text[:2])
	g.Write(text[2:])
	if got := g.Row(0); got != "中" {
		t.Fatalf("分片后 = %q", got)
	}
}

func TestClearScreen(t *testing.T) {
	g := New(3, 2)
	g.Write([]byte("abc\r\n"))
	g.Write([]byte("\x1b[2J"))
	if g.Row(0) != "" || g.Row(1) != "" {
		t.Fatalf("清屏后仍有内容: %q %q", g.Row(0), g.Row(1))
	}
	if x, y := g.Cursor(); x != 0 || y != 0 {
		t.Fatalf("清屏后光标 = (%d,%d)", x, y)
	}
}

func TestOSCIsNotPrinted(t *testing.T) {
	g := New(24, 2)
	g.Write([]byte("\x1b]8;;https://example\x07你好"))
	if got := g.Row(0); got != "你好" {
		t.Fatalf("OSC 被写进网格: %q", got)
	}
}

func TestCursorAddressRewritesCell(t *testing.T) {
	g := New(6, 2)
	g.Write([]byte("abcdef"))
	g.Write([]byte("\x1b[1;1HX"))
	if got := g.Row(0); got != "Xbcdef" {
		t.Fatalf("定位后 = %q", got)
	}
}

func TestUnknownControlDoesNotPrint(t *testing.T) {
	g := New(8, 1)
	g.Write([]byte("a\x1b[31mb"))
	if got := g.Row(0); got != "ab" {
		t.Fatalf("颜色序列被写入网格: %q", got)
	}
}

func TestScrollAndResize(t *testing.T) {
	g := New(2, 2)
	g.Write([]byte("ab\r\ncd\r\nef"))
	if g.Row(0) != "cd" || g.Row(1) != "ef" {
		t.Fatalf("滚动后 = %q / %q", g.Row(0), g.Row(1))
	}
	g.Resize(1, 1)
	if cols, rows := g.Size(); cols != 1 || rows != 1 {
		t.Fatalf("尺寸 = %d x %d", cols, rows)
	}
	if got := g.Row(0); got != "c" {
		t.Fatalf("缩小后 = %q", got)
	}
}
