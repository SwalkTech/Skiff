package term

import "testing"

func TestSGRKeepsRemoteColors(t *testing.T) {
	g := New(20, 2)
	g.Write([]byte("\x1b[32mhi\x1b[0m!\x1b[38;2;1;2;3mZ"))
	runs := g.LineRuns(0)
	if len(runs) < 3 {
		t.Fatalf("runs = %#v", runs)
	}
	if runs[0].Text == "" || !runs[0].FG.Set || runs[0].FG.G == 0 {
		t.Fatalf("绿色没有保留: %#v", runs[0])
	}
	if runs[1].FG.Set {
		t.Fatalf("复位后仍带颜色: %#v", runs[1])
	}
	var found bool
	for _, run := range runs {
		if run.Text == "Z" || (len(run.Text) > 0 && run.Text[0] == 'Z') {
			found = run.FG.Set && run.FG.R == 1 && run.FG.G == 2 && run.FG.B == 3
		}
	}
	if !found {
		t.Fatalf("真彩色没有保留: %#v", runs)
	}
}
