package paintengine2d

import "testing"

// Winding tests in the spirit of AGG compound shapes and Serenity/LibGfx
// painter coverage: even-odd vs non-zero on overlapping contours.

func TestNonZeroOverlappingCirclesFilled(t *testing.T) {
	img := NewImage(64, 32)
	ctx := NewContext(img)
	p := overlappingCircles(Pt(22, 16), Pt(42, 16), 14, 14)
	ctx.DrawPath(p, Paint{Color: White, FillRule: FillNonZero})
	assertAlpha(t, img, 22, 16, 250, 255, "left center")
	assertAlpha(t, img, 42, 16, 250, 255, "right center")
	assertAlpha(t, img, 32, 16, 250, 255, "overlap stays filled (non-zero)")
	assertAlpha(t, img, 2, 2, 0, 0, "outside")
}

func TestEvenOddOverlappingCirclesHole(t *testing.T) {
	img := NewImage(64, 32)
	ctx := NewContext(img)
	p := overlappingCircles(Pt(22, 16), Pt(42, 16), 14, 14)
	ctx.DrawPath(p, Paint{Color: White, FillRule: FillEvenOdd})
	assertAlpha(t, img, 22, 16, 250, 255, "left lobe")
	assertAlpha(t, img, 42, 16, 250, 255, "right lobe")
	assertAlpha(t, img, 32, 16, 0, 40, "overlap is a hole (even-odd)")
}

func TestOppositeWindingCancelsNonZero(t *testing.T) {
	// Outer CW rect, inner CCW rect: non-zero should punch a hole.
	img := NewImage(40, 40)
	ctx := NewContext(img)
	p := NewPath()
	p.MoveTo(4, 4)
	p.LineTo(36, 4)
	p.LineTo(36, 36)
	p.LineTo(4, 36)
	p.Close()
	p.MoveTo(12, 12)
	p.LineTo(12, 28)
	p.LineTo(28, 28)
	p.LineTo(28, 12)
	p.Close()
	ctx.DrawPath(p, Paint{Color: White, FillRule: FillNonZero})
	assertAlpha(t, img, 8, 8, 250, 255, "outer ring")
	assertAlpha(t, img, 20, 20, 0, 20, "cancelled hole")
}

func TestSelfIntersectingBowtieEvenOdd(t *testing.T) {
	img := NewImage(40, 40)
	ctx := NewContext(img)
	p := NewPath()
	p.MoveTo(6, 6)
	p.LineTo(34, 34)
	p.LineTo(34, 6)
	p.LineTo(6, 34)
	p.Close()
	ctx.DrawPath(p, Paint{Color: White, FillRule: FillEvenOdd})
	assertAlpha(t, img, 12, 10, 200, 255, "upper triangle")
	assertAlpha(t, img, 28, 30, 200, 255, "lower triangle")
}

func TestQuadAndCubicFillClosed(t *testing.T) {
	img := NewImage(48, 32)
	ctx := NewContext(img)
	p := NewPath()
	p.MoveTo(4, 28)
	p.QuadTo(24, 0, 44, 28)
	p.Close()
	ctx.DrawPath(p, Fill(White))
	assertAlpha(t, img, 24, 20, 200, 255, "quad interior")
	assertAlpha(t, img, 2, 2, 0, 0, "outside quad")

	c := NewPath()
	c.MoveTo(8, 4)
	c.CubicTo(8, 40, 40, 40, 40, 4)
	c.Close()
	img2 := NewImage(48, 32)
	ctx2 := NewContext(img2)
	ctx2.DrawPath(c, Fill(White))
	assertAlpha(t, img2, 24, 16, 200, 255, "cubic interior")
}
