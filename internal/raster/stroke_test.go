package raster

import "testing"

func TestExpandStrokeOpenAndClosed(t *testing.T) {
	open := [][]Vec2{{{0, 0}, {20, 0}}}
	out := ExpandStroke(open, []bool{false}, StrokeOpts{Width: 4, Cap: CapButt, Join: JoinMiter, MiterLimit: 4})
	if len(out) != 1 || len(out[0]) < 4 {
		t.Fatalf("open stroke outline %+v", out)
	}
	closed := [][]Vec2{{{0, 0}, {10, 0}, {10, 10}, {0, 10}, {0, 0}}}
	out2 := ExpandStroke(closed, []bool{true}, StrokeOpts{Width: 2, Cap: CapRound, Join: JoinRound, MiterLimit: 4})
	if len(out2) != 1 || len(out2[0]) < 6 {
		t.Fatalf("closed stroke outline %+v", out2)
	}
}

func TestExpandStrokeIgnoresDots(t *testing.T) {
	out := ExpandStroke([][]Vec2{{{1, 1}}}, []bool{false}, StrokeOpts{Width: 3, MiterLimit: 4})
	if len(out) != 0 {
		t.Fatalf("point contour should produce no outline, %v", out)
	}
}

func TestStrokePoolMatchesExpandStroke(t *testing.T) {
	open := [][]Vec2{{{0, 0}, {20, 0}, {20, 10}}}
	closed := [][]Vec2{{{0, 0}, {10, 0}, {10, 10}, {0, 10}, {0, 0}}}
	opt := StrokeOpts{Width: 4, Cap: CapRound, Join: JoinRound, MiterLimit: 4}
	var pool StrokePool
	a := ExpandStroke(open, []bool{false}, opt)
	b := pool.Expand(open, []bool{false}, opt)
	if !outlinesEq(a, b) {
		t.Fatalf("open pool vs expand\n%v\n%v", a, b)
	}
	c := ExpandStroke(closed, []bool{true}, opt)
	d := pool.Expand(closed, []bool{true}, opt)
	if !outlinesEq(c, d) {
		t.Fatalf("closed pool vs expand")
	}
	// Warm reuse must stay bit-identical to a fresh expand.
	e := pool.Expand(open, []bool{false}, opt)
	if !outlinesEq(a, e) {
		t.Fatal("warm pool drifted")
	}
}

func outlinesEq(a, b [][]Vec2) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if len(a[i]) != len(b[i]) {
			return false
		}
		for j := range a[i] {
			if abs32(a[i][j].X-b[i][j].X) > 1e-5 || abs32(a[i][j].Y-b[i][j].Y) > 1e-5 {
				return false
			}
		}
	}
	return true
}
