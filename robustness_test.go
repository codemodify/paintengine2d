package paintengine2d

import (
	"math"
	"testing"
)

func TestNonFinitePathCommandsAreIgnored(t *testing.T) {
	nan := float32(math.NaN())
	inf := float32(math.Inf(1))
	p := NewPath()
	p.MoveTo(nan, 1)
	p.LineTo(2, inf)
	p.QuadTo(nan, 0, 1, 1)
	p.CubicTo(0, nan, 1, inf, 2, 2)
	if !p.Empty() {
		t.Fatalf("non-finite commands should not append verbs, verbs=%d", len(p.Verbs()))
	}
	p.MoveTo(4, 4)
	p.LineTo(nan, 4)
	p.LineTo(12, 12)
	p.Close()
	if len(p.Points()) != 2 {
		t.Fatalf("only finite points should remain, got %d", len(p.Points()))
	}
}

func TestNonFiniteDrawDoesNotPanic(t *testing.T) {
	img := NewImage(24, 24)
	ctx := NewContext(img)
	nan := float32(math.NaN())
	ctx.SetMatrix(Matrix{A: nan, D: 1})
	if !ctx.Matrix().IsIdentity() {
		t.Fatal("non-finite SetMatrix should be ignored")
	}
	ctx.Transform(Matrix{A: 1, D: nan})
	ctx.DrawCircle(Pt(nan, 8), 6, Fill(White))
	ctx.DrawRect(XYWH(nan, 0, 4, 4), Fill(White))
	ctx.DrawLine(Pt(0, 0), Pt(inf32(), 8), StrokePaint(White, 2))
	ctx.DrawImageRect(NewImage(2, 2), XYWH(0, 0, 2, 2), XYWH(1, 1, 4, 4))
	ctx.SetMatrix(Scaling(0, 0))
	ctx.DrawCircle(Pt(12, 12), 8, StrokePaint(White, 3))
	// Finite draw still works afterwards.
	ctx.SetMatrix(Identity())
	ctx.DrawRect(XYWH(4, 4, 8, 8), Fill(White))
	if countOpaque(img, 200) == 0 {
		t.Fatal("finite draw after garbage should still paint")
	}
}

func inf32() float32 { return float32(math.Inf(1)) }

func TestBowtieDoesNotPanic(t *testing.T) {
	p := NewPath()
	p.MoveTo(4, 4)
	p.LineTo(28, 20)
	p.LineTo(4, 20)
	p.LineTo(28, 4)
	p.Close()
	nz := NewImage(32, 24)
	eo := NewImage(32, 24)
	NewContext(nz).DrawPath(p, Paint{Color: White, FillRule: FillNonZero})
	NewContext(eo).DrawPath(p, Paint{Color: White, FillRule: FillEvenOdd})
	if countOpaque(nz, 80)+countOpaque(eo, 80) == 0 {
		t.Fatal("self-intersecting quad should paint something")
	}
}

func TestSharedVertexWindingClosedRect(t *testing.T) {
	img := NewImage(20, 20)
	ctx := NewContext(img)
	ctx.DrawRect(XYWH(4, 4, 12, 12), Fill(White))
	assertAlpha(t, img, 10, 10, 250, 255, "interior")
	assertAlpha(t, img, 4, 4, 200, 255, "corner vertex")
	assertAlpha(t, img, 1, 1, 0, 5, "outside")
}

func TestScaledStrokeStaysVisible(t *testing.T) {
	img := NewImage(64, 64)
	ctx := NewContext(img)
	ctx.Translate(8, 8)
	ctx.Scale(4, 4)
	ctx.DrawCircle(Pt(6, 6), 4, StrokePaint(White, 0.5))
	if countPartial(img, 8, 240) < 10 {
		t.Fatalf("scaled stroke should keep an AA rim, partial=%d", countPartial(img, 8, 240))
	}
	if countOpaque(img, 200) < 20 {
		t.Fatalf("scaled stroke should cover a ring, opaque=%d", countOpaque(img, 200))
	}
	assertAlpha(t, img, 8+24, 8+24, 0, 40, "circle hole after scale")
}

func TestRoundRectFillAndStroke(t *testing.T) {
	img := NewImage(48, 32)
	ctx := NewContext(img)
	ctx.DrawRoundRect(XYWH(4, 4, 40, 24), 8, 8, Fill(RGB(0.2, 0.5, 0.95)))
	ctx.DrawRoundRect(XYWH(4, 4, 40, 24), 8, 8, StrokePaint(White, 2))
	assertAlpha(t, img, 24, 16, 250, 255, "roundrect interior")
	if countPartial(img, 8, 240) < 8 {
		t.Fatalf("roundrect corners should AA, partial=%d", countPartial(img, 8, 240))
	}
}

func TestClipPathNestedRoundRect(t *testing.T) {
	img := NewImage(40, 40)
	ctx := NewContext(img)
	ctx.ClipPath(RoundRectPath(XYWH(6, 6, 28, 28), 6, 6))
	ctx.ClipPath(CirclePath(Pt(20, 20), 10))
	ctx.SetColor(White)
	ctx.FillRect(XYWH(0, 0, 40, 40))
	assertAlpha(t, img, 20, 20, 250, 255, "nested clip center")
	assertAlpha(t, img, 2, 2, 0, 0, "outside both clips")
	assertAlpha(t, img, 8, 8, 0, 40, "roundrect corner outside circle")
}

func TestGlyphClipAndSubpixel(t *testing.T) {
	atlas := NewBitmapAtlas(White)
	img := NewImage(40, 16)
	ctx := NewContext(img)
	ctx.ClipRect(XYWH(0, 0, 14, 16))
	ctx.DrawLabel("HI", atlas, Pt(1.4, 3.2), Paint{Color: White, Filter: FilterNearest})
	if countOpaque(img, 80) < 4 {
		t.Fatalf("clipped label should still paint, n=%d", countOpaque(img, 80))
	}
	// Right of the clip stays empty.
	assertAlpha(t, img, 30, 8, 0, 0, "beyond clip")
}

func TestEmptyAndDuplicateStroke(t *testing.T) {
	img := NewImage(16, 16)
	ctx := NewContext(img)
	p := NewPath()
	p.MoveTo(4, 4)
	p.LineTo(4, 4)
	p.Close()
	ctx.DrawPath(p, StrokePaint(White, 3))
	ctx.DrawPath(NewPath(), Fill(White))
	ctx.DrawCircle(Pt(8, 8), 0, Fill(White))
	ctx.DrawLine(Pt(1, 1), Pt(1, 1), StrokePaint(White, 0))
}

func TestStrokeWarmPathBoundedAllocs(t *testing.T) {
	img := NewImage(64, 64)
	ctx := NewContext(img)
	p := NewPath()
	p.MoveTo(8, 32)
	p.CubicTo(16, 8, 48, 8, 56, 32)
	paint := Paint{Color: White, Style: StyleStroke, Stroke: Stroke{Width: 3, Cap: CapRound, Join: JoinRound, MiterLimit: 4}}
	ctx.DrawPath(p, paint) // warmup flatten / outline stores
	allocs := testing.AllocsPerRun(40, func() {
		ctx.DrawPath(p, paint)
	})
	if allocs > 24 {
		t.Fatalf("warm stroke allocs/op = %.1f, want <= 24", allocs)
	}
}
