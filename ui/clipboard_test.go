package ui

import (
	"testing"

	"github.com/SwalkTech/Skiff/term"
	vivaui "github.com/vivaui/viva/ui"
)

func TestPastePayloadTurnsNewlinesIntoCarriageReturns(t *testing.T) {
	got := string(pastePayload("a\r\nb\nc\r"))
	if got != "a\rb\rc\r" {
		t.Fatalf("粘贴内容 = %q", got)
	}
	if pastePayload("a\x00b") != nil {
		t.Fatal("含 NUL 的文本仍然被粘贴")
	}
	if pastePayload("") != nil {
		t.Fatal("空文本被粘贴")
	}
}

func TestSelectionTextReadsCells(t *testing.T) {
	grid := term.New(8, 3)
	if _, err := grid.Write([]byte("hello\r\nworld")); err != nil {
		t.Fatal(err)
	}
	got := selectionText(grid, 1, 0, 3, 1)
	if got != "ello\nworl" {
		t.Fatalf("选区 = %q", got)
	}
	if reverse := selectionText(grid, 3, 1, 1, 0); reverse != got {
		t.Fatalf("反向选区 = %q", reverse)
	}
}

func TestRowPiecesMarksSelection(t *testing.T) {
	runs := []term.Run{{Text: "hello"}}
	pieces := rowPieces(runs, 0, true, 1, 0, 3, 0)
	if len(pieces) != 3 || pieces[0].text != "h" || pieces[0].selected || pieces[1].text != "ell" || !pieces[1].selected || pieces[2].text != "o" || pieces[2].selected {
		t.Fatalf("分段 = %+v", pieces)
	}
}

func TestPasteClipboardWritesSession(t *testing.T) {
	ch := make(chan []byte, 1)
	model := &Model{
		connected: true,
		writeCh:   ch,
		clipboard: func() (string, error) { return "echo hi\n", nil },
	}
	result := model.pasteClipboard()
	if !result.Handled {
		t.Fatal("粘贴没有被处理")
	}
	select {
	case got := <-ch:
		if string(got) != "echo hi\r" {
			t.Fatalf("写入会话 = %q", got)
		}
	default:
		t.Fatal("剪贴板没有写入会话")
	}
}

func TestCopyShortcutUsesSelectionInsteadOfInterrupt(t *testing.T) {
	grid := term.New(8, 2)
	if _, err := grid.Write([]byte("hello\r\nworld")); err != nil {
		t.Fatal(err)
	}
	ch := make(chan []byte, 1)
	model := &Model{connected: true, writeCh: ch, Grid: grid, hasSel: true, selToX: 1}
	result, ok := model.terminalChord(vivaui.KeyEvent{Modifiers: vivaui.ModifierControl, Text: "c"})
	if !ok || result.ClipboardText != "he" {
		t.Fatalf("复制 = %q, ok %v", result.ClipboardText, ok)
	}
	select {
	case got := <-ch:
		t.Fatalf("有选区时仍然发送了 %q", got)
	default:
	}

	model.hasSel = false
	result, ok = model.terminalChord(vivaui.KeyEvent{Modifiers: vivaui.ModifierControl, Text: "c"})
	if !ok {
		t.Fatal("没有选区时 Ctrl+C 没有处理")
	}
	select {
	case got := <-ch:
		if string(got) != "\x03" {
			t.Fatalf("中断字节 = %q", got)
		}
	default:
		t.Fatal("没有选区时没有发送中断")
	}
	if result.ClipboardText != "" {
		t.Fatalf("中断时复制了 %q", result.ClipboardText)
	}
}

func TestDragCopiesTerminalCells(t *testing.T) {
	grid := term.New(8, 2)
	if _, err := grid.Write([]byte("hello\r\nworld")); err != nil {
		t.Fatal(err)
	}
	model := &Model{
		connected: true,
		Grid:      grid,
		termArea:  vivaui.Rect{Width: 400, Height: 300},
		termCellW: 10,
		termCellH: 20,
	}
	down := model.onPointer(vivaui.PointerEvent{
		Kind: vivaui.PointerDown, Button: vivaui.PointerButtonPrimary, Target: terminalID,
		X: terminalPad + 5, Y: terminalPad + 5,
	})
	if !model.selecting || down.ClipboardText != "" {
		t.Fatal("按下时就开始复制")
	}
	model.onPointer(vivaui.PointerEvent{
		Kind: vivaui.PointerMove, Button: vivaui.PointerButtonPrimary, Target: terminalID, Captured: true,
		X: terminalPad + 35, Y: terminalPad + 5,
	})
	up := model.onPointer(vivaui.PointerEvent{
		Kind: vivaui.PointerUp, Button: vivaui.PointerButtonPrimary, Target: terminalID, Captured: true,
		X: terminalPad + 35, Y: terminalPad + 5,
	})
	if up.ClipboardText != "hell" {
		t.Fatalf("拖选复制 = %q", up.ClipboardText)
	}
}

func TestRightClickPastes(t *testing.T) {
	ch := make(chan []byte, 1)
	model := &Model{
		connected: true,
		writeCh:   ch,
		clipboard: func() (string, error) { return "pwd", nil },
	}
	model.onPointer(vivaui.PointerEvent{
		Kind: vivaui.PointerDown, Button: vivaui.PointerButtonSecondary, Target: terminalID,
	})
	select {
	case got := <-ch:
		if string(got) != "pwd" {
			t.Fatalf("右键粘贴 = %q", got)
		}
	default:
		t.Fatal("右键没有粘贴")
	}
}
