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

// naiveCoverageRow is CoverageRow as it was written first: every whole
// pixel of every span added once for each sub-scanline. The difference
// array must give exactly the same coverage.
func naiveCoverageRow(r *Rasterizer, y, width, rule, clipX0, clipX1 int) []uint16 {
	cover := make([]uint16, width)
	r.syncActive(y)
	for s := 0; s < SamplesY; s++ {
		yy := float32(y) + (float32(s)+0.5)/SamplesY
		var xs []isect
		for _, i := range r.active {
			e := &r.edges[i]
			if yy < e.Y0 || yy >= e.Y1 {
				continue
			}
			xs = append(xs, isect{x: r.xAtFast(i, yy), dir: e.Dir})
		}
		if len(xs) < 2 {
			continue
		}
		sortIsects(xs)
		xs = mergeIsects(xs)
		wind := 0
		for i := 0; i < len(xs)-1; i++ {
			wind += int(xs[i].dir)
			if !filled(wind, rule) || xs[i+1].x <= xs[i].x {
				continue
			}
			x0, x1 := max(xs[i].x, float32(clipX0)), min(xs[i+1].x, float32(clipX1))
			for x := clipX0; x < clipX1; x++ {
				lo, hi := max(x0, float32(x)), min(x1, float32(x+1))
				if hi > lo {
					cover[x] += uint16((hi - lo) * float32(256/SamplesY))
				}
			}
		}
	}
	for i := range cover {
		cover[i] = min(cover[i], 255)
	}
	return cover
}

func TestCoverageRowMatchesSpanBySpan(t *testing.T) {
	// A ring with a hole, filled even-odd, and a star filled nonzero:
	// partial pixels, whole runs, and spans that start and end mid-row.
	star := []Vec2{{50, 2}, {61, 38}, {98, 38}, {68, 60}, {80, 97}, {50, 74}, {20, 97}, {32, 60}, {2, 38}, {39, 38}, {50, 2}}
	ring := [][]Vec2{
		{{3.3, 4.7}, {96.2, 4.7}, {96.2, 95.1}, {3.3, 95.1}, {3.3, 4.7}},
		{{30.5, 30.25}, {70.75, 30.25}, {70.75, 70.5}, {30.5, 70.5}, {30.5, 30.25}},
	}
	for _, c := range []struct {
		poly [][]Vec2
		rule int
	}{{[][]Vec2{star}, 0}, {ring, 1}} {
		var got, want Rasterizer
		got.ResetEdges(BuildEdges(c.poly, nil))
		want.ResetEdges(BuildEdges(c.poly, nil))
		for y := 0; y < 100; y++ {
			g := got.CoverageRow(y, 100, c.rule, 0, 100)
			w := naiveCoverageRow(&want, y, 100, c.rule, 0, 100)
			for x := range w {
				if d := int(g[x]) - int(w[x]); d < -1 || d > 1 {
					t.Fatalf("rule %d row %d px %d: %d, span by span %d", c.rule, y, x, g[x], w[x])
				}
			}
		}
	}
}
