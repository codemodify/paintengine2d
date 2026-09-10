package paintengine2d

import "testing"

func TestGradientTileClampRepeatMirror(t *testing.T) {
	// Gradient defined on [0, 16]; sample at x=8 (mid), x=2 (start-ish),
	// x=40 (past the end) under each tile mode.
	mk := func(tile TileMode) *Image {
		img := NewImage(48, 8)
		ctx := NewContext(img)
		ctx.DrawRect(XYWH(0, 0, 48, 8), Linear(LinearGradient{
			Start: Pt(0, 0),
			End:   Pt(16, 0),
			Stops: []GradientStop{
				{Offset: 0, Color: RGB(1, 0, 0)},
				{Offset: 1, Color: RGB(0, 0, 1)},
			},
			Tile: tile,
		}))
		return img
	}

	clamp := mk(TileClamp)
	r, _, b, _ := rgbAt(clamp, 40, 4)
	if b < 180 || r > 40 {
		t.Fatalf("clamp past end should stay blue, r=%d b=%d", r, b)
	}

	repeat := mk(TileRepeat)
	// x=40 → t = 40/16 = 2.5 → frac 0.5 → midpoint (magenta-ish).
	rr, _, br, _ := rgbAt(repeat, 40, 4)
	if rr < 60 || br < 60 {
		t.Fatalf("repeat at t=2.5 should be mid blend, r=%d b=%d", rr, br)
	}
	// x=2 is near the start of a cycle → red.
	r0, _, b0, _ := rgbAt(repeat, 2, 4)
	if r0 < 180 || b0 > 80 {
		t.Fatalf("repeat near cycle start should be red, r=%d b=%d", r0, b0)
	}

	mirror := mk(TileMirror)
	// x=24 → t=1.5 → mirrored back to 0.5 → mid blend.
	rm, _, bm, _ := rgbAt(mirror, 24, 4)
	if rm < 60 || bm < 60 {
		t.Fatalf("mirror at t=1.5 should be mid, r=%d b=%d", rm, bm)
	}
}

func TestGradientSingleStopAndEmpty(t *testing.T) {
	img := NewImage(16, 8)
	ctx := NewContext(img)
	ctx.DrawRect(XYWH(0, 0, 16, 8), Linear(LinearGradient{
		Start: Pt(0, 0),
		End:   Pt(16, 0),
		Stops: []GradientStop{{Offset: 0.3, Color: RGB(0, 1, 0)}},
	}))
	_, g, _, a := rgbAt(img, 8, 4)
	if a < 250 || g < 200 {
		t.Fatalf("single stop should be green, g=%d a=%d", g, a)
	}

	img2 := NewImage(8, 8)
	ctx2 := NewContext(img2)
	ctx2.DrawRect(XYWH(0, 0, 8, 8), Linear(LinearGradient{Start: Pt(0, 0), End: Pt(8, 0)}))
	if countOpaque(img2, 1) != 0 {
		t.Fatal("empty stops should paint nothing")
	}
}

func TestGradientZeroLengthFallsBack(t *testing.T) {
	img := NewImage(12, 8)
	ctx := NewContext(img)
	ctx.DrawRect(XYWH(0, 0, 12, 8), Linear(LinearGradient{
		Start: Pt(4, 4),
		End:   Pt(4, 4),
		Stops: []GradientStop{
			{Offset: 0, Color: RGB(1, 0, 0)},
			{Offset: 1, Color: RGB(0, 0, 1)},
		},
	}))
	r, _, b, a := rgbAt(img, 6, 4)
	if a < 200 || r < 150 || b > 80 {
		t.Fatalf("zero-length gradient should use first stop, r=%d b=%d a=%d", r, b, a)
	}
}

func TestGradientRespectsTransform(t *testing.T) {
	img := NewImage(32, 16)
	ctx := NewContext(img)
	ctx.Translate(16, 0)
	ctx.DrawRect(XYWH(0, 0, 16, 16), Linear(LinearGradient{
		Start: Pt(0, 0),
		End:   Pt(16, 0),
		Stops: []GradientStop{
			{Offset: 0, Color: RGB(1, 0, 0)},
			{Offset: 1, Color: RGB(0, 0, 1)},
		},
	}))
	r, _, _, _ := rgbAt(img, 17, 8)
	_, _, b, _ := rgbAt(img, 30, 8)
	if r < 150 {
		t.Fatalf("left of translated gradient should be red, r=%d", r)
	}
	if b < 150 {
		t.Fatalf("right of translated gradient should be blue, b=%d", b)
	}
}

func TestTileParamHelpers(t *testing.T) {
	if g := tileParam(-1, TileClamp); g != 0 {
		t.Fatalf("clamp neg %v", g)
	}
	if g := tileParam(2.25, TileRepeat); g < 0.24 || g > 0.26 {
		t.Fatalf("repeat 2.25 → %v", g)
	}
	if g := tileParam(1.25, TileMirror); g < 0.74 || g > 0.76 {
		t.Fatalf("mirror 1.25 → %v", g)
	}
}
