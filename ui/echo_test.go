package ui

import "testing"

func TestConsumeEchoMatchesPrefix(t *testing.T) {
	rest, left := consumeEcho([]byte("ab\r\n"), []rune{'a', 'b'})
	if string(rest) != "\r\n" || len(left) != 0 {
		t.Fatalf("rest=%q left=%q", rest, string(left))
	}
}

func TestConsumeEchoDropsPredictionOnMismatch(t *testing.T) {
	rest, left := consumeEcho([]byte("\x1b[31m"), []rune{'a'})
	if string(rest) != "\x1b[31m" || left != nil {
		t.Fatalf("rest=%q left=%v", rest, left)
	}
}

func TestConsumeEchoKeepsPartialRune(t *testing.T) {
	rest, left := consumeEcho([]byte{0xe4}, []rune{'你'})
	if len(rest) != 1 || string(left) != "你" {
		t.Fatalf("rest=%v left=%q", rest, string(left))
	}
}

func TestAltScreenToggle(t *testing.T) {
	if !altScreenAfter([]byte("\x1b[?1049h"), false) {
		t.Fatal("expected alternate screen on")
	}
	if altScreenAfter([]byte("\x1b[?1049l"), true) {
		t.Fatal("expected alternate screen off")
	}
}

func TestEchoSuppressedForPassword(t *testing.T) {
	if !echoSuppressed("Password:") || !echoSuppressed("请输入口令") {
		t.Fatal("password prompts should not predict echo")
	}
	if echoSuppressed("root@host:~# ") {
		t.Fatal("shell prompt should allow local echo")
	}
}
