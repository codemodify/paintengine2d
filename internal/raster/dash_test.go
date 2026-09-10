package raster

import "testing"

func TestDashSplitsOpenContour(t *testing.T) {
	c := [][]Vec2{{{0, 0}, {40, 0}}}
	out, closed := Dash(c, []bool{false}, []float32{10, 10}, 0)
	if len(out) < 2 {
		t.Fatalf("expected multiple dashes, got %d", len(out))
	}
	for _, cl := range closed {
		if cl {
			t.Fatal("dashes should be open")
		}
	}
}

func TestDashEmptyIsSolid(t *testing.T) {
	c := [][]Vec2{{{0, 0}, {10, 0}}}
	out, _ := Dash(c, []bool{false}, nil, 0)
	if len(out) != 1 {
		t.Fatalf("empty dash should pass through, %d", len(out))
	}
	out2, _ := Dash(c, []bool{false}, []float32{0, 0}, 0)
	if len(out2) != 1 {
		t.Fatal("zero dash should pass through")
	}
}

func TestSampleNearestInBounds(t *testing.T) {
	pix := []byte{10, 20, 30, 40, 50, 60, 70, 80}
	r, g, b, a := SampleNearestPremul(pix, 2, 1, 8, 1.2, 0.1)
	if r != 50 || a != 80 {
		t.Fatalf("nearest %d %d %d %d", r, g, b, a)
	}
	r, g, b, a = SampleNearestPremul(pix, 2, 1, 8, -1, 0)
	if r|g|b|a != 0 {
		t.Fatal("outside")
	}
}
