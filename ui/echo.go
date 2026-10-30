package ui

import (
	"strings"
	"unicode/utf8"
)

func (m *Model) noteLocalEcho(text string) bool {
	if m == nil || m.altScreen || m.Grid == nil || strings.ContainsAny(text, "\r\n") {
		return false
	}
	_, y := m.Grid.Cursor()
	if echoSuppressed(m.Grid.Row(y)) {
		return false
	}
	added := false
	for _, r := range text {
		if r < 0x20 || r == 0x7f {
			continue
		}
		m.pendingEcho = append(m.pendingEcho, r)
		added = true
	}
	return added
}

func (m *Model) popLocalEcho() bool {
	if m == nil || len(m.pendingEcho) == 0 {
		return false
	}
	m.pendingEcho = m.pendingEcho[:len(m.pendingEcho)-1]
	return true
}

func (m *Model) pendingEchoText() string {
	if m == nil || len(m.pendingEcho) == 0 {
		return ""
	}
	return string(m.pendingEcho)
}

func echoSuppressed(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "password") || strings.Contains(lower, "passphrase") || strings.Contains(line, "口令")
}

func altScreenAfter(data []byte, on bool) bool {
	for i := 0; i < len(data); i++ {
		if data[i] != 0x1b || i+1 >= len(data) || data[i+1] != '[' {
			continue
		}
		rest := data[i+2:]
		switch {
		case hasAlt(rest, "?1049h"), hasAlt(rest, "?1047h"), hasAlt(rest, "?47h"):
			on = true
		case hasAlt(rest, "?1049l"), hasAlt(rest, "?1047l"), hasAlt(rest, "?47l"):
			on = false
		}
	}
	return on
}

func hasAlt(data []byte, seq string) bool {
	return len(data) >= len(seq) && string(data[:len(seq)]) == seq
}

// consumeEcho 吃掉远端回显里已经在本地显示过的前缀。对不上时丢掉本地预测，让远端内容生效。
func consumeEcho(data []byte, pending []rune) ([]byte, []rune) {
	if len(pending) == 0 || len(data) == 0 {
		return data, pending
	}
	i := 0
	left := pending
	for len(left) > 0 && i < len(data) {
		if !utf8.FullRune(data[i:]) {
			return data[i:], left
		}
		r, size := utf8.DecodeRune(data[i:])
		if r != left[0] {
			return data[i:], nil
		}
		left = left[1:]
		i += size
	}
	return data[i:], left
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
