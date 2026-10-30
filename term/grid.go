package term

import "unicode/utf8"

// Grid 按行列保存终端已经显示的字符和光标位置。
//
// 调用方必须串行使用同一个 Grid。本类型不保存网络连接，也不主动刷新界面。
type Grid struct {
	cols int
	rows int
	cell []rune
	x    int
	y    int
	wrap bool

	state   int
	params  []int
	value   int
	has     bool
	partial []byte
}

const (
	stateText = iota
	stateEsc
	stateCSI
)

// New 创建至少有一行一列的网格。小于 1 的尺寸会提升到 1。
func New(cols, rows int) *Grid {
	g := &Grid{}
	g.Resize(cols, rows)
	return g
}

// Size 返回当前列数和行数。
func (g *Grid) Size() (cols, rows int) {
	return g.cols, g.rows
}

// Cursor 返回光标所在列和行，原点在左上角。
func (g *Grid) Cursor() (x, y int) {
	return g.x, g.y
}

// RuneAt 返回指定位置的字符。空单元格为零值。
func (g *Grid) RuneAt(x, y int) rune {
	if x < 0 || y < 0 || x >= g.cols || y >= g.rows {
		return 0
	}
	return g.cell[y*g.cols+x]
}

// Row 返回一行的可见文本，保留中间空格，去掉行尾空单元格。
func (g *Grid) Row(y int) string {
	if y < 0 || y >= g.rows {
		return ""
	}
	row := g.cell[y*g.cols : (y+1)*g.cols]
	end := len(row)
	for end > 0 && row[end-1] == 0 {
		end--
	}
	return string(row[:end])
}

// Write 把终端输出写入网格。它识别可打印字符、回车、换行、退格和清屏。
func (g *Grid) Write(p []byte) (int, error) {
	n := len(p)
	if len(g.partial) > 0 {
		p = append(append([]byte(nil), g.partial...), p...)
		g.partial = nil
	}
	for len(p) > 0 {
		if !utf8.FullRune(p) {
			if len(p) > utf8.UTFMax {
				g.feed(utf8.RuneError)
				p = p[1:]
				continue
			}
			g.partial = append([]byte(nil), p...)
			break
		}
		r, size := utf8.DecodeRune(p)
		p = p[size:]
		g.feed(r)
	}
	return n, nil
}

// Resize 改变网格尺寸。原有内容按左上角对齐保留，超出部分被丢弃，不做重排。
func (g *Grid) Resize(cols, rows int) {
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	next := make([]rune, cols*rows)
	if g.cell != nil {
		width := g.cols
		if width > cols {
			width = cols
		}
		height := g.rows
		if height > rows {
			height = rows
		}
		for y := 0; y < height; y++ {
			copy(next[y*cols:y*cols+width], g.cell[y*g.cols:y*g.cols+width])
		}
	}
	g.cell = next
	g.cols = cols
	g.rows = rows
	if g.x >= cols {
		g.x = cols - 1
	}
	if g.y >= rows {
		g.y = rows - 1
	}
	if g.x < cols-1 {
		g.wrap = false
	}
}

func (g *Grid) feed(r rune) {
	switch g.state {
	case stateText:
		g.feedText(r)
	case stateEsc:
		g.state = stateText
		if r == '[' {
			g.state = stateCSI
			g.params = g.params[:0]
			g.value = 0
			g.has = false
		}
	case stateCSI:
		g.feedCSI(r)
	}
}

func (g *Grid) feedText(r rune) {
	switch r {
	case '\n':
		g.wrap = false
		g.lineFeed()
	case '\r':
		g.wrap = false
		g.x = 0
	case '\b':
		g.wrap = false
		g.backspace()
	case 0x1b:
		g.state = stateEsc
	default:
		if r >= 0x20 {
			g.put(r)
		}
	}
}

func (g *Grid) feedCSI(r rune) {
	switch {
	case r >= '0' && r <= '9':
		g.value = g.value*10 + int(r-'0')
		g.has = true
	case r == ';' || r == ':':
		g.pushParam()
	case r == '?':
		// 私有模式标记不参与当前支持的清屏命令。
	default:
		g.pushParam()
		if r == 'J' && param(g.params, 0) == 2 {
			g.clear()
		}
		g.state = stateText
	}
}

func (g *Grid) pushParam() {
	if !g.has {
		g.params = append(g.params, 0)
	} else {
		g.params = append(g.params, g.value)
	}
	g.value = 0
	g.has = false
}

func (g *Grid) put(r rune) {
	// 写满一行后先保留光标，下一个可打印字符再换行。
	// 这样 "\r\n" 不会因为行已经写满而多滚动一次。
	if g.wrap {
		g.wrap = false
		g.x = 0
		g.lineFeed()
	}
	g.cell[g.y*g.cols+g.x] = r
	if g.x+1 >= g.cols {
		g.wrap = true
		return
	}
	g.x++
}

func (g *Grid) backspace() {
	if g.x == 0 {
		return
	}
	g.x--
}

func (g *Grid) lineFeed() {
	if g.y+1 < g.rows {
		g.y++
		return
	}
	copy(g.cell[:g.cols*(g.rows-1)], g.cell[g.cols:])
	clear(g.cell[g.cols*(g.rows-1):])
}

func (g *Grid) clear() {
	clear(g.cell)
	g.x = 0
	g.y = 0
	g.wrap = false
}

func param(params []int, index int) int {
	if index < 0 || index >= len(params) {
		return 0
	}
	return params[index]
}
