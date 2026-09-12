package raster

import "testing"

func TestBuildEdgesSkipsHorizontal(t *testing.T) {
	edges := BuildEdges([][]Vec2{
		{{0, 0}, {10, 0}, {10, 5}, {0, 5}, {0, 0}},
	}, nil)
	for _, e := range edges {
		if e.Y0 == e.Y1 {
			t.Fatalf("horizontal survived: %+v", e)
		}
		if e.Y0 > e.Y1 {
			t.Fatalf("not sorted Y: %+v", e)
		}
	}
	if len(edges) != 2 {
		t.Fatalf("expected 2 verticals, got %d", len(edges))
	}
}

func TestRectCoverageFractional(t *testing.T) {
	cover := make([]uint16, 8)
	RectCoverage(cover, 0, 1.25, 0, 4.5, 1, 0, 8)
	if cover[1] == 0 || cover[1] == 255 {
		t.Fatalf("leading pixel should be partial, %d", cover[1])
	}
	if cover[2] < 250 {
		t.Fatalf("full pixel, %d", cover[2])
	}
	if cover[4] == 0 || cover[4] == 255 {
		t.Fatalf("trailing pixel should be partial, %d", cover[4])
	}
	if cover[0] != 0 || cover[5] != 0 {
		t.Fatalf("outside %v", cover)
	}
}

func TestCoverageRowEmpty(t *testing.T) {
	var r Rasterizer
	r.ResetEdges(nil)
	c := r.CoverageRow(0, 8, FillNonZero, 0, 8)
	for _, v := range c {
		if v != 0 {
			t.Fatal("empty edges")
		}
	}
}

func TestCoverageRowSharedVertex(t *testing.T) {
	// Closed unit square: vertices meet exactly on sample lines.
	edges := BuildEdges([][]Vec2{
		{{1, 1}, {5, 1}, {5, 5}, {1, 5}, {1, 1}},
	}, nil)
	var r Rasterizer
	r.ResetEdges(edges)
	c := r.CoverageRow(3, 8, FillNonZero, 0, 8)
	if c[3] < 250 {
		t.Fatalf("interior coverage %d", c[3])
	}
	if c[0] != 0 || c[7] != 0 {
		t.Fatalf("outside %v", c)
	}
}

func TestEdgeBoundsEmpty(t *testing.T) {
	if _, _, _, _, ok := EdgeBounds(nil); ok {
		t.Fatal("empty edges")
	}
}

func TestEdgeBoundsTight(t *testing.T) {
	edges := BuildEdges([][]Vec2{
		{{10, 20}, {40, 20}, {40, 35}, {10, 35}, {10, 20}},
	}, nil)
	minX, minY, maxX, maxY, ok := EdgeBounds(edges)
	if !ok {
		t.Fatal("expected bounds")
	}
	if minX < 9.9 || maxX > 40.1 || minY < 19.9 || maxY > 35.1 {
		t.Fatalf("bounds %v %v %v %v", minX, minY, maxX, maxY)
	}
}

func TestRectCoverageClearsOnlyClip(t *testing.T) {
	cover := make([]uint16, 16)
	for i := range cover {
		cover[i] = 99
	}
	RectCoverage(cover, 0, 4, 0, 8, 1, 4, 8)
	if cover[0] != 99 || cover[15] != 99 {
		t.Fatalf("outside clip must stay untouched, %v", cover)
	}
	if cover[5] < 250 {
		t.Fatalf("interior %d", cover[5])
	}
}

func TestMergeIsectsCombinesDirs(t *testing.T) {
	in := []isect{{x: 1, dir: 1}, {x: 1, dir: -1}, {x: 4, dir: 1}}
	sortIsects(in)
	out := mergeIsects(in)
	if len(out) != 1 || out[0].x != 4 {
		t.Fatalf("zero-dir pair should drop, got %+v", out)
	}
}
