package paintengine2d

import "testing"

// Scenarios inspired by Cairo pattern tests, NanoVG gradient demos,
// and SVG dasharray — implemented here from scratch.

func TestRadialGradientCenterAndRim(t *testing.T) {
	img := NewImage(64, 64)
	ctx := NewContext(img)
	ctx.DrawCircle(Pt(32, 32), 28, Radial(RadialGradient{
		Center: Pt(32, 32),
		Radius: 28,
		Stops: []GradientStop{
			{Offset: 0, Color: RGB(1, 0, 0)},
			{Offset: 1, Color: RGB(0, 0, 1)},
		},
	}))
	r, _, b, a := rgbAt(img, 32, 32)
	if a < 250 || r < 200 || b > 40 {
		t.Fatalf("center should be red, r=%d b=%d a=%d", r, b, a)
	}
	r2, _, b2, a2 := rgbAt(img, 32, 32+26)
	if a2 < 80 || b2 < 80 {
		t.Fatalf("rim should trend blue, r=%d b=%d a=%d", r2, b2, a2)
	}
}

func TestRadialInnerRadius(t *testing.T) {
	img := NewImage(48, 48)
	ctx := NewContext(img)
	ctx.DrawRect(XYWH(0, 0, 48, 48), Radial(RadialGradient{
		Center: Pt(24, 24),
		Inner:  10,
		Radius: 22,
		Stops: []GradientStop{
			{Offset: 0, Color: RGB(0, 1, 0)},
			{Offset: 1, Color: RGB(1, 1, 0)},
		},
	}))
	_, g, _, _ := rgbAt(img, 24, 24)
	if g < 200 {
		t.Fatalf("inside inner radius should be first stop, g=%d", g)
	}
}

func TestDashedStrokeHasGaps(t *testing.T) {
	img := NewImage(80, 16)
	ctx := NewContext(img)
	ctx.DrawLine(Pt(4, 8), Pt(76, 8), Paint{
		Color: White,
		Style: StyleStroke,
		Stroke: Stroke{
			Width: 4, Cap: CapButt, Join: JoinMiter, MiterLimit: 4,
			Dash: []float32{8, 8},
		},
	})
	on := 0
	off := 0
	for x := 8; x < 72; x++ {
		if alphaAt(img, x, 8) > 200 {
			on++
		} else if alphaAt(img, x, 8) < 10 {
			off++
		}
	}
	if on < 16 || off < 16 {
		t.Fatalf("expected a dashed pattern, on=%d off=%d", on, off)
	}
}

func TestDashOffsetShiftsPattern(t *testing.T) {
	a := NewImage(64, 12)
	b := NewImage(64, 12)
	stroke := func(img *Image, off float32) {
		ctx := NewContext(img)
		ctx.DrawLine(Pt(2, 6), Pt(62, 6), Paint{
			Color: White,
			Style: StyleStroke,
			Stroke: Stroke{
				Width: 3, Cap: CapButt, Join: JoinMiter, MiterLimit: 4,
				Dash: []float32{10, 10}, DashOffset: off,
			},
		})
	}
	stroke(a, 0)
	stroke(b, 10)
	// Offset of one full "on" should invert the first interior sample.
	if alphaAt(a, 8, 6) < 200 {
		t.Fatalf("dash start should be on, a=%d", alphaAt(a, 8, 6))
	}
	if alphaAt(b, 8, 6) > 40 {
		t.Fatalf("offset 10 should start in a gap, a=%d", alphaAt(b, 8, 6))
	}
}

func TestZeroAndNegativeStrokeWidthNoop(t *testing.T) {
	img := NewImage(24, 24)
	ctx := NewContext(img)
	ctx.DrawPath(RectPath(XYWH(2, 2, 20, 20)), Paint{
		Color: White, Style: StyleStroke,
		Stroke: Stroke{Width: 0, Cap: CapButt, Join: JoinMiter, MiterLimit: 4},
	})
	ctx.DrawPath(RectPath(XYWH(2, 2, 20, 20)), Paint{
		Color: White, Style: StyleStroke,
		Stroke: Stroke{Width: -3, Cap: CapButt, Join: JoinMiter, MiterLimit: 4},
	})
	if countOpaque(img, 1) != 0 {
		t.Fatal("width <= 0 must not paint")
	}
}

func TestNearestVsBilinearScale(t *testing.T) {
	src := NewImage(2, 2)
	src.SetColor(0, 0, RGB(1, 0, 0))
	src.SetColor(1, 0, RGB(0, 0, 1))
	src.SetColor(0, 1, RGB(0, 0, 1))
	src.SetColor(1, 1, RGB(1, 0, 0))

	near := NewImage(16, 16)
	lin := NewImage(16, 16)
	cn := NewContext(near)
	cl := NewContext(lin)
	srcR := XYWH(0, 0, 2, 2)
	dstR := XYWH(0, 0, 16, 16)
	cn.DrawImageRectPaint(src, srcR, dstR, Paint{Color: White, Filter: FilterNearest})
	cl.DrawImageRectPaint(src, srcR, dstR, Paint{Color: White, Filter: FilterBilinear})

	// Nearest: a pixel well inside the top-left source texel stays pure red.
	r, _, b, _ := rgbAt(near, 3, 3)
	if r < 200 || b > 20 {
		t.Fatalf("nearest block should be red, r=%d b=%d", r, b)
	}
	// Bilinear at the center seam should mix.
	rl, _, bl, _ := rgbAt(lin, 8, 8)
	if rl < 40 || bl < 40 {
		t.Fatalf("bilinear seam should mix, r=%d b=%d", rl, bl)
	}
}

func TestFillRectZeroAllocs(t *testing.T) {
	img := NewImage(64, 64)
	ctx := NewContext(img)
	p := Fill(RGB(0.2, 0.3, 0.8))
	r := XYWH(4, 4, 56, 56)
	// Warm-up (scratch path, cover buf).
	ctx.DrawRect(r, p)
	n := testing.AllocsPerRun(50, func() { ctx.DrawRect(r, p) })
	if n > 0.5 {
		t.Fatalf("FillRect hot path allocs = %v, want 0", n)
	}
}

func TestUnsortedGradientStops(t *testing.T) {
	img := NewImage(32, 8)
	ctx := NewContext(img)
	ctx.DrawRect(XYWH(0, 0, 32, 8), Linear(LinearGradient{
		Start: Pt(0, 0), End: Pt(32, 0),
		Stops: []GradientStop{
			{Offset: 1, Color: RGB(0, 0, 1)},
			{Offset: 0, Color: RGB(1, 0, 0)},
		},
	}))
	r, _, b, _ := rgbAt(img, 1, 4)
	if r < 150 || b > 80 {
		t.Fatalf("unsorted stops should still start red, r=%d b=%d", r, b)
	}
}

func TestSaveRestoreClonesDash(t *testing.T) {
	img := NewImage(8, 8)
	ctx := NewContext(img)
	ctx.SetStroke(Paint{Color: White, Style: StyleStroke, Stroke: Stroke{Width: 2, Dash: []float32{1, 2}}})
	ctx.Save()
	ctx.cur.stroke.Stroke.Dash[0] = 99
	ctx.Restore()
	if ctx.cur.stroke.Stroke.Dash[0] != 1 {
		t.Fatalf("dash slice leaked across restore: %v", ctx.cur.stroke.Stroke.Dash)
	}
}
