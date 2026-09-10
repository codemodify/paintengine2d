package paintengine2d

import "testing"

func TestOutOfBoundsFillDoesNotPanicOrWrap(t *testing.T) {
	img := NewImage(16, 16)
	ctx := NewContext(img)
	ctx.SetColor(White)
	ctx.FillRect(XYWH(-40, -40, 20, 20))
	ctx.FillRect(XYWH(80, 80, 20, 20))
	ctx.DrawCircle(Pt(-10, -10), 4, Fill(White))
	ctx.DrawCircle(Pt(40, 40), 4, Fill(White))
	if countOpaque(img, 1) != 0 {
		t.Fatal("out-of-bounds geometry should not wrap into the pixmap")
	}
}

func TestPartialOutOfBoundsFill(t *testing.T) {
	img := NewImage(16, 16)
	ctx := NewContext(img)
	ctx.SetColor(White)
	ctx.FillRect(XYWH(-4, 4, 12, 8))
	assertAlpha(t, img, 2, 8, 250, 255, "visible overlap")
	assertAlpha(t, img, 14, 8, 0, 0, "far side empty")
}

func TestLargeCoordinatesDoNotPanic(t *testing.T) {
	img := NewImage(12, 12)
	ctx := NewContext(img)
	ctx.SetColor(White)
	ctx.FillRect(XYWH(1e6, 1e6, 10, 10))
	ctx.DrawCircle(Pt(1e7, -1e7), 50, Fill(White))
	p := NewPath()
	p.MoveTo(1e5, 1e5)
	p.CubicTo(1e5+10, 1e5, 1e5, 1e5+10, 1e5+10, 1e5+10)
	ctx.DrawPath(p, StrokePaint(White, 2))
	if countOpaque(img, 1) != 0 {
		t.Fatal("huge coordinates should miss the small pixmap")
	}
}

func TestSubpixelPositionsShiftCoverage(t *testing.T) {
	a := NewImage(12, 12)
	b := NewImage(12, 12)
	ca := NewContext(a)
	cb := NewContext(b)
	ca.DrawRect(XYWH(3, 3, 5, 5), Fill(White))
	cb.DrawRect(XYWH(3.4, 3.4, 5, 5), Fill(White))
	if alphaAt(a, 3, 3) == 255 && alphaAt(b, 3, 3) == 255 {
		t.Fatal("subpixel shift should reduce coverage on the leading pixel")
	}
	if alphaAt(b, 3, 3) == 0 {
		t.Fatal("subpixel rect should still touch (3,3)")
	}
}

func TestImageZeroSize(t *testing.T) {
	img := NewImage(0, 0)
	ctx := NewContext(img)
	ctx.Clear(White)
	ctx.FillRect(XYWH(0, 0, 4, 4))
	if len(img.Pix) != 0 {
		t.Fatal("0×0 pixmap")
	}
}

func TestDamageStub(t *testing.T) {
	var d Damage
	d.Add(XYWH(1, 2, 3, 4))
	d.Add(Rect{})
	if len(d.Rects) != 1 {
		t.Fatalf("empty rects should be ignored, got %d", len(d.Rects))
	}
	if d.Bounds().Dx() != 3 {
		t.Fatalf("bounds %+v", d.Bounds())
	}
	d.Reset()
	if !d.Bounds().Empty() {
		t.Fatal("reset")
	}
}

func TestPathResetAndTransform(t *testing.T) {
	p := RectPath(XYWH(0, 0, 10, 10))
	p.Transform(Translation(5, 0))
	if p.Bounds().Min.X < 4 {
		t.Fatalf("transform bounds %+v", p.Bounds())
	}
	p.Reset()
	if !p.Empty() {
		t.Fatal("reset")
	}
	p.AddArc(Pt(0, 0), 4, 4, 0, 1.2)
	if p.Empty() {
		t.Fatal("arc")
	}
}

func TestMatrixSingular(t *testing.T) {
	m := Scaling(0, 1)
	if _, ok := m.Invert(); ok {
		t.Fatal("zero scale should be singular")
	}
}

func TestRoundRectClampsRadii(t *testing.T) {
	p := RoundRectPath(XYWH(0, 0, 10, 10), 100, 100)
	if p.Empty() {
		t.Fatal("over-large radii should still produce a path")
	}
}

func TestImageCloneAndSubImage(t *testing.T) {
	img := NewImage(8, 8)
	img.SetColor(2, 3, RGB(0, 1, 0))
	c := img.Clone()
	img.SetColor(2, 3, RGB(1, 0, 0))
	_, g, _, _ := c.PremulAt(2, 3)
	if g < 250 {
		t.Fatal("clone should be independent")
	}
	sub := c.SubImage(2, 3, 4, 5)
	if sub.Width != 2 || sub.Height != 2 {
		t.Fatalf("crop %dx%d", sub.Width, sub.Height)
	}
	_, g2, _, _ := sub.PremulAt(0, 0)
	if g2 < 250 {
		t.Fatal("subimage pixel")
	}
}
