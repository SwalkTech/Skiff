package term

// Color 是远端 SGR 指定的一种颜色。Set 为假时表示用界面默认色。
type Color struct {
	R, G, B uint8
	Set     bool
}

// Run 是一行里颜色相同的一段文字。
type Run struct {
	Text string
	FG   Color
	BG   Color
	Bold bool
}

type style struct {
	fg, bg Color
	bold   bool
}

func (g *Grid) applySGR(params []int) {
	if len(params) == 0 {
		g.pen = style{}
		return
	}
	for i := 0; i < len(params); i++ {
		switch p := params[i]; {
		case p == 0:
			g.pen = style{}
		case p == 1:
			g.pen.bold = true
		case p == 22:
			g.pen.bold = false
		case p >= 30 && p <= 37:
			g.pen.fg = ansiColor(p-30, g.pen.bold)
		case p >= 90 && p <= 97:
			g.pen.fg = ansiColor(p-90, true)
		case p == 39:
			g.pen.fg = Color{}
		case p >= 40 && p <= 47:
			g.pen.bg = ansiColor(p-40, false)
		case p >= 100 && p <= 107:
			g.pen.bg = ansiColor(p-100, true)
		case p == 49:
			g.pen.bg = Color{}
		case p == 38 || p == 48:
			col, used := extColor(params[i+1:])
			i += used
			if p == 38 {
				g.pen.fg = col
			} else {
				g.pen.bg = col
			}
		}
	}
}

func extColor(params []int) (Color, int) {
	if len(params) == 0 {
		return Color{}, 0
	}
	switch params[0] {
	case 5:
		if len(params) < 2 {
			return Color{}, 1
		}
		return color256(params[1]), 2
	case 2:
		if len(params) < 4 {
			return Color{}, len(params)
		}
		return Color{R: uint8(params[1]), G: uint8(params[2]), B: uint8(params[3]), Set: true}, 4
	default:
		return Color{}, 1
	}
}

func ansiColor(index int, bright bool) Color {
	if bright {
		index += 8
	}
	return color256(index)
}

func color256(n int) Color {
	if n < 0 {
		n = 0
	}
	if n > 255 {
		n = 255
	}
	if n < 16 {
		return Color{R: ansi16[n][0], G: ansi16[n][1], B: ansi16[n][2], Set: true}
	}
	if n >= 232 {
		shade := uint8(8 + (n-232)*10)
		return Color{R: shade, G: shade, B: shade, Set: true}
	}
	n -= 16
	r := n / 36
	g := (n / 6) % 6
	b := n % 6
	return Color{R: cube(r), G: cube(g), B: cube(b), Set: true}
}

func cube(n int) uint8 {
	if n == 0 {
		return 0
	}
	return uint8(55 + n*40)
}

var ansi16 = [16][3]uint8{
	{0, 0, 0},
	{205, 0, 0},
	{0, 205, 0},
	{205, 205, 0},
	{0, 0, 238},
	{205, 0, 205},
	{0, 205, 205},
	{229, 229, 229},
	{127, 127, 127},
	{255, 0, 0},
	{0, 255, 0},
	{255, 255, 0},
	{92, 92, 255},
	{255, 0, 255},
	{0, 255, 255},
	{255, 255, 255},
}

// LineRuns 返回一行的颜色分段，并在光标列插入光标标记。
func (g *Grid) LineRuns(y int) []Run {
	if g == nil || y < 0 || y >= g.rows {
		return nil
	}
	end := g.cols
	for end > 0 && g.cell[y*g.cols+end-1] == 0 && (g.attr == nil || !g.attr[y*g.cols+end-1].bg.Set) {
		end--
	}
	cursorX, cursorY := g.Cursor()
	showCursor := y == cursorY
	if showCursor && cursorX+1 > end {
		end = cursorX + 1
		if end > g.cols {
			end = g.cols
		}
	}
	var runs []Run
	var current style
	var buf []rune
	flush := func() {
		if len(buf) == 0 {
			return
		}
		runs = append(runs, Run{Text: string(buf), FG: current.fg, BG: current.bg, Bold: current.bold})
		buf = buf[:0]
	}
	for x := 0; x < end; x++ {
		if showCursor && x == cursorX {
			flush()
			runs = append(runs, Run{Text: "▍", FG: current.fg, BG: current.bg, Bold: true})
		}
		next := style{}
		if g.attr != nil && x < g.cols {
			next = g.attr[y*g.cols+x]
		}
		r := rune(0)
		if x < g.cols {
			r = g.cell[y*g.cols+x]
		}
		if r == 0 {
			r = ' '
		}
		if len(buf) > 0 && next != current {
			flush()
		}
		current = next
		buf = append(buf, r)
	}
	if showCursor && cursorX >= end {
		flush()
		runs = append(runs, Run{Text: "▍", Bold: true})
	}
	flush()
	if len(runs) == 0 {
		return []Run{{Text: " "}}
	}
	return runs
}
