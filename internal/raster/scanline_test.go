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
