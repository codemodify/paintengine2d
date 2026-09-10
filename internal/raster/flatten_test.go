package raster

import "testing"

func TestFlattenLineAndClose(t *testing.T) {
	verbs := []Verb{Move, Line, Line, Close}
	pts := []Vec2{{0, 0}, {10, 0}, {10, 5}}
	var contours [][]Vec2
	var closed []bool
	Flatten(verbs, pts, 0.2, &contours, &closed)
	if len(contours) != 1 || !closed[0] {
		t.Fatalf("got %d contours closed=%v", len(contours), closed)
	}
	c := contours[0]
	if len(c) < 4 || !c[0].near(c[len(c)-1], 1e-3) {
		t.Fatalf("closed contour %+v", c)
	}
}

func TestFlattenCubicIncreasesPoints(t *testing.T) {
	verbs := []Verb{Move, Cubic}
	pts := []Vec2{{0, 0}, {0, 40}, {40, 40}, {40, 0}}
	var contours [][]Vec2
	var closed []bool
	Flatten(verbs, pts, 0.2, &contours, &closed)
	if len(contours) != 1 || len(contours[0]) < 6 {
		t.Fatalf("expected subdivided cubic, got %+v", contours)
	}
}

func TestFlattenReusesContourBacking(t *testing.T) {
	verbs := []Verb{Move, Line, Line, Close}
	pts := []Vec2{{0, 0}, {8, 0}, {8, 6}}
	var contours [][]Vec2
	var closed []bool
	Flatten(verbs, pts, 0.2, &contours, &closed)
	if len(contours) != 1 {
		t.Fatal(len(contours))
	}
	ptr := &contours[0][0]
	Flatten(verbs, pts, 0.2, &contours, &closed)
	if &contours[0][0] != ptr {
		t.Fatal("expected contour slice reuse on the second flatten")
	}
}
