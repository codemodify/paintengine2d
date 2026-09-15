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
	// Hourglass: the two horizontal edges are the top and bottom, the two
	// diagonals cross in the middle. Even-odd fills an upper and a lower
	// triangle and leaves the left/right lobes empty.
	img := NewImage(40, 40)
	ctx := NewContext(img)
	p := NewPath()
	p.MoveTo(6, 6)
	p.LineTo(34, 6)
	p.LineTo(6, 34)
	p.LineTo(34, 34)
	p.Close()
	ctx.DrawPath(p, Paint{Color: White, FillRule: FillEvenOdd})
	assertAlpha(t, img, 20, 10, 200, 255, "upper triangle")
	assertAlpha(t, img, 20, 30, 200, 255, "lower triangle")
	assertAlpha(t, img, 8, 20, 0, 20, "left lobe is outside")
	assertAlpha(t, img, 32, 20, 0, 20, "right lobe is outside")
}

// Regression: this bow-tie has two distinct X and two distinct Y values, so
// the old isClosedRectPath treated it as an axis-aligned rectangle and the
// fill fast path painted a solid box. The real even-odd shape is a left and
// a right triangle.
func TestBowtieIsNotARectFastPath(t *testing.T) {
	img := NewImage(40, 40)
	ctx := NewContext(img)
	p := NewPath()
	p.MoveTo(6, 6)
	p.LineTo(34, 34)
	p.LineTo(34, 6)
	p.LineTo(6, 34)
	p.Close()
	ctx.DrawPath(p, Paint{Color: White, FillRule: FillEvenOdd})
	assertAlpha(t, img, 8, 10, 200, 255, "left lobe")
	assertAlpha(t, img, 32, 10, 200, 255, "right lobe")
	assertAlpha(t, img, 20, 10, 0, 20, "middle of a bow-tie row is outside")
	if isClosedRectPath(p) {
		t.Fatal("bow-tie must not match the rectangle fast path")
	}
}

// Regression: a triangle whose final LineTo returns to the start also has
// only two distinct X and two distinct Y values.
func TestRightTriangleIsNotARectFastPath(t *testing.T) {
	img := NewImage(40, 40)
	ctx := NewContext(img)
	p := NewPath()
	p.MoveTo(5, 5)
	p.LineTo(35, 5)
	p.LineTo(35, 35)
	p.LineTo(5, 5)
	p.Close()
	if isClosedRectPath(p) {
		t.Fatal("right triangle must not match the rectangle fast path")
	}
	ctx.DrawPath(p, Fill(White))
	assertAlpha(t, img, 30, 10, 200, 255, "inside the triangle")
	assertAlpha(t, img, 8, 32, 0, 20, "corner outside the triangle")
}

func TestRectFastPathStillMatchesRects(t *testing.T) {
	if !isClosedRectPath(RectPath(XYWH(2, 3, 10, 6))) {
		t.Fatal("RectPath must keep the fast path")
	}
	p := NewPath()
	p.MoveTo(0, 0)
	p.LineTo(10, 0)
	p.LineTo(10, 8)
	p.LineTo(0, 8)
	p.LineTo(0, 0) // explicit closing line
	p.Close()
	if !isClosedRectPath(p) {
		t.Fatal("explicitly closed rectangle must keep the fast path")
	}
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

// An open stroke with square caps used to leave its outline unclosed (the
// cap starts on its extended corner), which filled a band from the start
// cap across to the far arm of a check mark.
func TestSquareCapOpenStrokeLeavesNoBand(t *testing.T) {
	img := NewImage(16, 16)
	ctx := NewContext(img)
	p := NewPath()
	p.MoveTo(3.92, 7.92)
	p.LineTo(7.04, 11.04)
	p.LineTo(12.88, 5.12)
	ctx.DrawPath(p, Paint{Color: RGB(0, 0, 0), Style: StyleStroke, Stroke: Stroke{Width: 2, Cap: CapSquare, Join: JoinMiter, MiterLimit: 4}})
	// Inside the V, above the corner: nothing may be painted there.
	for _, pt := range [][2]int{{7, 6}, {6, 7}, {8, 6}} {
		if _, _, _, a := img.PremulAt(pt[0], pt[1]); a > 8 {
			t.Fatalf("pixel %v inside the check mark is painted (alpha %d)", pt, a)
		}
	}
	// The square cap still extends past the start point.
	if _, _, _, a := img.PremulAt(3, 6); a == 0 {
		t.Fatal("the square start cap is missing")
	}
}
