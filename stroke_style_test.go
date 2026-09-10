package paintengine2d

import "testing"

// Stroke cap/join/thin-width coverage inspired by AGG stroking demos.

func TestStrokeCapsDiffer(t *testing.T) {
	// Horizontal stroke ending at x=20. Butt stops at the endpoint;
	// square and round extend past it.
	type row struct {
		cap  Cap
		xOut int // pixel past the geometric end
	}
	for _, tc := range []row{{CapButt, 23}, {CapSquare, 23}, {CapRound, 23}} {
		img := NewImage(40, 16)
		ctx := NewContext(img)
		ctx.DrawLine(Pt(4, 8), Pt(20, 8), Paint{
			Color:  White,
			Style:  StyleStroke,
			Stroke: Stroke{Width: 8, Cap: tc.cap, Join: JoinMiter, MiterLimit: 4},
		})
		assertAlpha(t, img, 12, 8, 250, 255, tc.cap.string()+" body")
		a := alphaAt(img, tc.xOut, 8)
		switch tc.cap {
		case CapButt:
			if a > 20 {
				t.Fatalf("butt should not extend to x=%d, a=%d", tc.xOut, a)
			}
		case CapSquare, CapRound:
			if a < 100 {
				t.Fatalf("%s should extend to x=%d, a=%d", tc.cap.string(), tc.xOut, a)
			}
		}
	}
}

func (c Cap) string() string {
	switch c {
	case CapRound:
		return "round"
	case CapSquare:
		return "square"
	default:
		return "butt"
	}
}

func TestStrokeJoinsDiffer(t *testing.T) {
	// Right-angle polyline. Miter protrudes farther than bevel at the corner.
	draw := func(join Join) *Image {
		img := NewImage(36, 36)
		ctx := NewContext(img)
		p := NewPath()
		p.MoveTo(4, 20)
		p.LineTo(18, 20)
		p.LineTo(18, 6)
		ctx.DrawPath(p, Paint{
			Color:  White,
			Style:  StyleStroke,
			Stroke: Stroke{Width: 8, Cap: CapButt, Join: join, MiterLimit: 8},
		})
		return img
	}
	miter := draw(JoinMiter)
	bevel := draw(JoinBevel)
	round := draw(JoinRound)
	// 90° miter fills the box corner; bevel cuts it on a diagonal.
	if alphaAt(miter, 21, 22) < 200 {
		t.Fatalf("miter should fill the outer box corner, a=%d", alphaAt(miter, 21, 22))
	}
	if alphaAt(bevel, 21, 22) > 80 {
		t.Fatalf("bevel should cut the outer corner, a=%d", alphaAt(bevel, 21, 22))
	}
	if alphaAt(round, 20, 21) < 80 {
		t.Fatalf("round join should fill a fan, a=%d", alphaAt(round, 20, 21))
	}
}

func TestStrokeMiterLimitFallsBack(t *testing.T) {
	sharp := NewPath()
	sharp.MoveTo(4, 28)
	sharp.LineTo(16, 6)
	sharp.LineTo(28, 28)
	limited := NewImage(36, 36)
	ctx := NewContext(limited)
	ctx.DrawPath(sharp, Paint{
		Color:  White,
		Style:  StyleStroke,
		Stroke: Stroke{Width: 6, Cap: CapButt, Join: JoinMiter, MiterLimit: 1.1},
	})
	wide := NewImage(36, 36)
	ctx2 := NewContext(wide)
	ctx2.DrawPath(sharp.Clone(), Paint{
		Color:  White,
		Style:  StyleStroke,
		Stroke: Stroke{Width: 6, Cap: CapButt, Join: JoinMiter, MiterLimit: 12},
	})
	// A high miter limit grows a spike near the apex (16,6).
	if countOpaque(wide, 200) <= countOpaque(limited, 200) {
		t.Fatalf("high miter limit should cover more pixels (%d vs %d)",
			countOpaque(wide, 200), countOpaque(limited, 200))
	}
}

func TestVeryThinStrokeStillVisible(t *testing.T) {
	img := NewImage(32, 16)
	ctx := NewContext(img)
	ctx.DrawLine(Pt(2, 8), Pt(30, 8), Paint{
		Color:  White,
		Style:  StyleStroke,
		Stroke: Stroke{Width: 0.35, Cap: CapButt, Join: JoinMiter, MiterLimit: 4},
	})
	if countOpaque(img, 8) < 8 {
		t.Fatalf("thin stroke should leave AA coverage, opaque=%d", countOpaque(img, 8))
	}
}

func TestStrokeAndFill(t *testing.T) {
	img := NewImage(32, 32)
	ctx := NewContext(img)
	ctx.DrawCircle(Pt(16, 16), 8, Paint{
		Color:  RGB(1, 0, 0),
		Style:  StyleStrokeAndFill,
		Stroke: Stroke{Width: 4, Cap: CapRound, Join: JoinRound, MiterLimit: 4},
	})
	r, _, _, a := rgbAt(img, 16, 16)
	if a < 250 || r < 200 {
		t.Fatalf("filled interior r=%d a=%d", r, a)
	}
	// Outside the fill radius but on the stroke ring.
	r2, _, _, a2 := rgbAt(img, 16, 16-10)
	if a2 < 80 || r2 < 80 {
		t.Fatalf("stroke ring r=%d a=%d", r2, a2)
	}
}

func TestClosedStrokeHasNoCaps(t *testing.T) {
	img := NewImage(40, 40)
	ctx := NewContext(img)
	p := NewPath()
	p.AddRect(XYWH(8, 8, 24, 24))
	ctx.DrawPath(p, Paint{
		Color:  White,
		Style:  StyleStroke,
		Stroke: Stroke{Width: 4, Cap: CapSquare, Join: JoinMiter, MiterLimit: 4},
	})
	assertAlpha(t, img, 20, 8, 200, 255, "top edge")
	assertAlpha(t, img, 20, 20, 0, 20, "interior hollow")
}

func TestDiagonalStrokeAA(t *testing.T) {
	img := NewImage(32, 32)
	ctx := NewContext(img)
	ctx.DrawLine(Pt(3, 3), Pt(28, 28), StrokePaint(White, 2))
	if countPartial(img, 8, 240) < 8 {
		t.Fatalf("diagonal stroke should anti-alias, partial=%d", countPartial(img, 8, 240))
	}
	assertAlpha(t, img, 16, 16, 80, 255, "on the diagonal")
	assertAlpha(t, img, 28, 4, 0, 5, "off the diagonal")
}
