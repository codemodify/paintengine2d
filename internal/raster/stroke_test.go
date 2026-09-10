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
