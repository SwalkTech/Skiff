package ui

import (
	"strings"

	"github.com/SwalkTech/Skiff/term"
	"github.com/vivaui/viva/app"
	vivaui "github.com/vivaui/viva/ui"
)

func pastePayload(text string) []byte {
	if text == "" || strings.ContainsRune(text, 0) {
		return nil
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.ReplaceAll(text, "\n", "\r")
	return []byte(text)
}

func normalizeSelection(x0, y0, x1, y1 int) (int, int, int, int) {
	if y0 > y1 || (y0 == y1 && x0 > x1) {
		return x1, y1, x0, y0
	}
	return x0, y0, x1, y1
}

func selectionText(g *term.Grid, x0, y0, x1, y1 int) string {
	if g == nil {
		return ""
	}
	x0, y0, x1, y1 = normalizeSelection(x0, y0, x1, y1)
	cols, rows := g.Size()
	if cols < 1 || rows < 1 || y1 < 0 || y0 >= rows || x1 < 0 || x0 >= cols {
		return ""
	}
	var lines []string
	for y := y0; y <= y1; y++ {
		if y < 0 || y >= rows {
			continue
		}
		start, end := 0, cols-1
		if y == y0 {
			start = x0
		}
		if y == y1 {
			end = x1
		}
		if start < 0 {
			start = 0
		}
		if end >= cols {
			end = cols - 1
		}
		var b strings.Builder
		for x := start; x <= end; x++ {
			r := g.RuneAt(x, y)
			if r == 0 {
				r = ' '
			}
			b.WriteRune(r)
		}
		lines = append(lines, strings.TrimRight(b.String(), " "))
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

type termPiece struct {
	text     string
	fg, bg   term.Color
	bold     bool
	selected bool
}

func rowPieces(runs []term.Run, y int, active bool, x0, y0, x1, y1 int) []termPiece {
	if len(runs) == 0 {
		return nil
	}
	if !active || y < 0 {
		return piecesOf(runs)
	}
	x0, y0, x1, y1 = normalizeSelection(x0, y0, x1, y1)
	if y < y0 || y > y1 {
		return piecesOf(runs)
	}
	start, end := 0, int(^uint(0)>>1)
	if y == y0 {
		start = x0
	}
	if y == y1 {
		end = x1
	}
	var out []termPiece
	col := 0
	for _, run := range runs {
		rs := []rune(run.Text)
		i := 0
		for i < len(rs) {
			selected := col >= start && col <= end
			j := i + 1
			next := col + 1
			for j < len(rs) && (next >= start && next <= end) == selected {
				j++
				next++
			}
			out = append(out, termPiece{text: string(rs[i:j]), fg: run.FG, bg: run.BG, bold: run.Bold, selected: selected})
			col = next
			i = j
		}
	}
	return out
}

func piecesOf(runs []term.Run) []termPiece {
	out := make([]termPiece, 0, len(runs))
	for _, run := range runs {
		if run.Text == "" {
			continue
		}
		out = append(out, termPiece{text: run.Text, fg: run.FG, bg: run.BG, bold: run.Bold})
	}
	return out
}

func (m *Model) readClipboard() (string, error) {
	if m != nil && m.clipboard != nil {
		return m.clipboard()
	}
	return app.ClipboardText()
}

func (m *Model) pasteClipboard() vivaui.EventResult {
	text, err := m.readClipboard()
	if err != nil || text == "" {
		return vivaui.EventResult{Handled: true}
	}
	payload := pastePayload(text)
	if len(payload) == 0 {
		return vivaui.EventResult{Handled: true}
	}
	cleared := m.dropSelection()
	m.writeSession(payload)
	return vivaui.EventResult{Handled: true, Repaint: cleared}
}

func (m *Model) copyText(all bool) string {
	if m == nil || m.Grid == nil {
		return ""
	}
	if m.hasSel {
		return selectionText(m.Grid, m.selFromX, m.selFromY, m.selToX, m.selToY)
	}
	if !all {
		return ""
	}
	cols, rows := m.Grid.Size()
	return selectionText(m.Grid, 0, 0, cols-1, rows-1)
}

func (m *Model) dropSelection() bool {
	if m == nil || (!m.hasSel && !m.selecting) {
		return false
	}
	m.hasSel = false
	m.selecting = false
	return true
}

func (m *Model) terminalChord(event vivaui.KeyEvent) (vivaui.EventResult, bool) {
	if m == nil || event.Release || event.Modifiers&vivaui.ModifierAlt != 0 {
		return vivaui.EventResult{}, false
	}
	if event.Key == vivaui.KeyInsert && event.Modifiers == vivaui.ModifierShift {
		return m.pasteClipboard(), true
	}
	if event.Key == vivaui.KeyInsert && event.Modifiers == vivaui.ModifierControl {
		if text := m.copyText(false); text != "" {
			return vivaui.EventResult{Handled: true, ClipboardText: text}, true
		}
		return vivaui.EventResult{}, false
	}
	ctrl := event.Modifiers&vivaui.ModifierControl != 0 || event.Modifiers&vivaui.ModifierMeta != 0
	if !ctrl || event.Text == "" {
		return vivaui.EventResult{}, false
	}
	shift := event.Modifiers&vivaui.ModifierShift != 0
	switch strings.ToLower(event.Text) {
	case "v":
		return m.pasteClipboard(), true
	case "c":
		if shift || m.hasSel {
			if text := m.copyText(shift); text != "" {
				return vivaui.EventResult{Handled: true, ClipboardText: text}, true
			}
			if shift {
				return vivaui.EventResult{Handled: true}, true
			}
		}
		m.dropSelection()
		m.writeSession([]byte{3})
		return vivaui.EventResult{Handled: true, Repaint: true}, true
	default:
		return vivaui.EventResult{}, false
	}
}

func (m *Model) pointInTerminal(x, y float32) bool {
	if m == nil || m.termCellW < 1 || m.termCellH < 1 || m.Grid == nil {
		return false
	}
	cols, rows := m.Grid.Size()
	left := m.termArea.X + terminalPad
	top := m.termArea.Y + terminalPad
	right := left + float32(cols)*m.termCellW
	bottom := top + float32(rows)*m.termCellH
	return x >= left && y >= top && x < right && y < bottom
}

func (m *Model) cellAt(x, y float32) (int, int) {
	cols, rows := 1, 1
	if m.Grid != nil {
		cols, rows = m.Grid.Size()
	}
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	if m.termCellW < 1 || m.termCellH < 1 {
		return 0, 0
	}
	col := int((x - (m.termArea.X + terminalPad)) / m.termCellW)
	row := int((y - (m.termArea.Y + terminalPad)) / m.termCellH)
	if col < 0 {
		col = 0
	}
	if row < 0 {
		row = 0
	}
	if col >= cols {
		col = cols - 1
	}
	if row >= rows {
		row = rows - 1
	}
	return col, row
}

func (m *Model) onPointer(event vivaui.PointerEvent) vivaui.EventResult {
	inTerm := m.acceptsTerminal(event.Target) || (event.Captured && m.selecting)
	if !inTerm {
		if event.Kind == vivaui.PointerDown && m.dropSelection() {
			return vivaui.EventResult{Repaint: true}
		}
		return vivaui.EventResult{}
	}
	switch event.Kind {
	case vivaui.PointerDown:
		if event.Button == vivaui.PointerButtonSecondary {
			return m.pasteClipboard()
		}
		if event.Button != vivaui.PointerButtonPrimary || !m.pointInTerminal(event.X, event.Y) {
			if m.dropSelection() {
				return vivaui.EventResult{Repaint: true}
			}
			return vivaui.EventResult{}
		}
		col, row := m.cellAt(event.X, event.Y)
		m.selecting = true
		m.selFromX, m.selFromY = col, row
		m.selToX, m.selToY = col, row
		m.hasSel = false
		return vivaui.EventResult{Repaint: true}
	case vivaui.PointerMove:
		if !m.selecting {
			return vivaui.EventResult{}
		}
		col, row := m.cellAt(event.X, event.Y)
		if col == m.selToX && row == m.selToY {
			return vivaui.EventResult{}
		}
		m.selToX, m.selToY = col, row
		m.hasSel = m.selFromX != col || m.selFromY != row
		return vivaui.EventResult{Repaint: true}
	case vivaui.PointerUp:
		if !m.selecting || event.Button != vivaui.PointerButtonPrimary {
			return vivaui.EventResult{}
		}
		m.selecting = false
		if !m.hasSel {
			return vivaui.EventResult{Repaint: true}
		}
		text := m.copyText(false)
		if text == "" {
			return vivaui.EventResult{Repaint: true}
		}
		return vivaui.EventResult{ClipboardText: text, Repaint: true}
	default:
		return vivaui.EventResult{}
	}
}

func selectionColor(dark bool) vivaui.Color {
	if dark {
		return vivaui.RGB(46, 92, 180)
	}
	return vivaui.RGB(46, 110, 210)
}
