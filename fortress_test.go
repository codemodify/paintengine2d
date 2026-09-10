package paintengine2d

import (
	"bytes"
	"math"
	"testing"
)

// Extra correctness cases a UI kit hits on day one: 1:1 icon blits,
// padded shm round-trips, gradient strokes, closed dashes, labels under
// transform. Themes from Skia/Cairo/AGG — rewritten here.

func TestNearest1to1IsExact(t *testing.T) {
	src := NewImage(5, 4)
	for y := 0; y < 4; y++ {
		for x := 0; x < 5; x++ {
			src.SetColor(x, y, Bytes(uint8(40+x*20), uint8(10+y*30), 80, 255))
		}
	}
	dst := NewImage(12, 10)
	ctx := NewContext(dst)
	ctx.DrawImageRectPaint(src, XYWH(0, 0, 5, 4), XYWH(2, 3, 5, 4), Paint{Color: White, Filter: FilterNearest})
	for y := 0; y < 4; y++ {
		for x := 0; x < 5; x++ {
			sr, sg, sb, sa := src.PremulAt(x, y)
			dr, dg, db, da := dst.PremulAt(2+x, 3+y)
			if sr != dr || sg != dg || sb != db || sa != da {
				t.Fatalf("1:1 nearest (%d,%d) src=%d,%d,%d,%d dst=%d,%d,%d,%d",
					x, y, sr, sg, sb, sa, dr, dg, db, da)
			}
		}
	}
}

func TestNearest1to1WithClipMatches(t *testing.T) {
	src := NewImage(8, 8)
	src.Clear(RGB(0.2, 0.7, 0.3))
	dst := NewImage(20, 20)
	ctx := NewContext(dst)
	ctx.ClipRect(XYWH(4, 4, 6, 6))
	ctx.DrawImageRectPaint(src, XYWH(0, 0, 8, 8), XYWH(2, 2, 8, 8), Paint{Color: White, Filter: FilterNearest})
	assertAlpha(t, dst, 5, 5, 250, 255, "inside clip")
	assertAlpha(t, dst, 2, 2, 0, 0, "outside clip")
}

func TestNearest1to1FromPaddedSource(t *testing.T) {
	const w, h, pad = 5, 4, 10
	stride := w*4 + pad
	buf := make([]byte, h*stride)
	src := WrapImage(buf, w, h, stride)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			src.SetColor(x, y, RGB(float32(x)/4, 0.2, float32(y)/3))
		}
	}
	dst := NewImage(8, 6)
	NewContext(dst).DrawImageRectPaint(src, XYWH(0, 0, 5, 4), XYWH(1, 1, 5, 4), Paint{Color: White, Filter: FilterNearest})
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			sr, sg, sb, sa := src.PremulAt(x, y)
			dr, dg, db, da := dst.PremulAt(1+x, 1+y)
			if sr != dr || sg != dg || sb != db || sa != da {
				t.Fatalf("padded 1:1 mismatch at %d,%d", x, y)
			}
		}
	}
}

func TestPaddedPNGRoundTrip(t *testing.T) {
	const w, h, pad = 6, 5, 8
	stride := w*4 + pad
	buf := make([]byte, h*stride)
	for i := range buf {
		buf[i] = 0x11
	}
	img := WrapImage(buf, w, h, stride)
	img.SetColor(1, 2, RGB(0.2, 0.6, 0.9))
	var bufPNG bytes.Buffer
	if err := img.WritePNG(&bufPNG); err != nil {
		t.Fatal(err)
	}
	got, err := DecodePNG(&bufPNG)
	if err != nil {
		t.Fatal(err)
	}
	if got.Width != w || got.Height != h {
		t.Fatalf("size %dx%d", got.Width, got.Height)
	}
	_, g, b, a := got.PremulAt(1, 2)
	if a < 240 || g < 140 || b < 200 {
		t.Fatalf("png pixel g=%d b=%d a=%d", g, b, a)
	}
}

func TestRotatedBlitPaintsInterior(t *testing.T) {
	src := NewImage(6, 6)
	src.Clear(White)
	dst := NewImage(40, 40)
	ctx := NewContext(dst)
	ctx.Translate(20, 20)
	ctx.Rotate(0.4)
	ctx.DrawImageRectPaint(src, XYWH(0, 0, 6, 6), XYWH(-10, -10, 20, 20), Paint{Color: White, Filter: FilterNearest})
	if alphaAt(dst, 20, 20) < 200 {
		t.Fatalf("rotated blit center a=%d", alphaAt(dst, 20, 20))
	}
	if alphaAt(dst, 1, 1) > 5 {
		t.Fatalf("canvas corner should stay empty, a=%d", alphaAt(dst, 1, 1))
	}
}

func TestGradientStroke(t *testing.T) {
	img := NewImage(64, 16)
	ctx := NewContext(img)
	ctx.DrawLine(Pt(4, 8), Pt(60, 8), Paint{
		Style:  StyleStroke,
		Stroke: Stroke{Width: 8, Cap: CapButt, Join: JoinMiter, MiterLimit: 4},
		Shader: LinearGradient{
			Start: Pt(4, 8), End: Pt(60, 8),
			Stops: []GradientStop{{0, RGB(1, 0, 0)}, {1, RGB(0, 0, 1)}},
		},
	})
	r, _, _, a := rgbAt(img, 8, 8)
	if a < 200 || r < 150 {
		t.Fatalf("stroke start should be red, r=%d a=%d", r, a)
	}
	_, _, b, a2 := rgbAt(img, 56, 8)
	if a2 < 200 || b < 150 {
		t.Fatalf("stroke end should be blue, b=%d a=%d", b, a2)
	}
}

func TestOddDashArrayDoubles(t *testing.T) {
	img := NewImage(80, 12)
	ctx := NewContext(img)
	// SVG: [10] → [10,10]
	ctx.DrawLine(Pt(2, 6), Pt(78, 6), Paint{
		Color: White, Style: StyleStroke,
		Stroke: Stroke{Width: 4, Cap: CapButt, Join: JoinMiter, MiterLimit: 4, Dash: []float32{10}},
	})
	on, off := 0, 0
	for x := 6; x < 74; x++ {
		if alphaAt(img, x, 6) > 180 {
			on++
		} else if alphaAt(img, x, 6) < 15 {
			off++
		}
	}
	if on < 12 || off < 12 {
		t.Fatalf("odd dash should pair with itself, on=%d off=%d", on, off)
	}
}

func TestClosedDashedRectHasGaps(t *testing.T) {
	img := NewImage(48, 48)
	ctx := NewContext(img)
	ctx.DrawRect(XYWH(8, 8, 32, 32), Paint{
		Color: White, Style: StyleStroke,
		Stroke: Stroke{Width: 3, Cap: CapButt, Join: JoinMiter, MiterLimit: 4, Dash: []float32{8, 8}},
	})
	if countOpaque(img, 180) < 20 {
		t.Fatalf("dashed rect should paint, n=%d", countOpaque(img, 180))
	}
	if alphaAt(img, 24, 24) > 10 {
		t.Fatalf("hollow dashed rect, a=%d", alphaAt(img, 24, 24))
	}
}

func TestNonUniformScaleStrokeVisible(t *testing.T) {
	img := NewImage(64, 32)
	ctx := NewContext(img)
	ctx.Translate(4, 8)
	ctx.Scale(4, 1)
	ctx.DrawLine(Pt(1, 8), Pt(14, 8), StrokePaint(White, 2))
	if countOpaque(img, 80) < 10 {
		t.Fatalf("non-uniform scaled stroke should remain visible, n=%d", countOpaque(img, 80))
	}
}

func TestVeryThickStrokeDoesNotPanic(t *testing.T) {
	img := NewImage(32, 32)
	ctx := NewContext(img)
	ctx.DrawLine(Pt(4, 16), Pt(28, 16), StrokePaint(White, 40))
	if countOpaque(img, 200) < 20 {
		t.Fatalf("thick stroke should fill a band, n=%d", countOpaque(img, 200))
	}
}

func TestLabelUnderScale(t *testing.T) {
	img := NewImage(64, 32)
	ctx := NewContext(img)
	ctx.Translate(4, 4)
	ctx.Scale(2, 2)
	ctx.DrawLabel("OK", NewBitmapAtlas(White), Pt(1, 1), Paint{Color: White, Filter: FilterNearest})
	if countOpaque(img, 80) < 8 {
		t.Fatalf("scaled label should blit, n=%d", countOpaque(img, 80))
	}
}

func TestDrawGlyphsRespectsClipPath(t *testing.T) {
	img := NewImage(48, 20)
	ctx := NewContext(img)
	ctx.ClipPath(CirclePath(Pt(10, 10), 8))
	ctx.DrawLabel("HI", NewBitmapAtlas(White), Pt(2, 4), Paint{Color: White, Filter: FilterNearest})
	assertAlpha(t, img, 40, 10, 0, 0, "beyond circle clip")
}

func TestAllZeroDashIsSolid(t *testing.T) {
	img := NewImage(32, 12)
	ctx := NewContext(img)
	ctx.DrawLine(Pt(2, 6), Pt(30, 6), Paint{
		Color: White, Style: StyleStroke,
		Stroke: Stroke{Width: 3, Cap: CapButt, Join: JoinMiter, MiterLimit: 4, Dash: []float32{0, 0}},
	})
	if alphaAt(img, 16, 6) < 200 {
		t.Fatalf("all-zero dash is solid, a=%d", alphaAt(img, 16, 6))
	}
}

func TestQuickRejectAfterRotate(t *testing.T) {
	img := NewImage(32, 32)
	ctx := NewContext(img)
	ctx.Translate(16, 16)
	ctx.Rotate(math.Pi / 2)
	ctx.ClipRect(XYWH(-6, -6, 12, 12))
	if ctx.QuickReject(XYWH(-2, -2, 4, 4)) {
		t.Fatal("widget under rotated clip should paint")
	}
	if !ctx.QuickReject(XYWH(20, 20, 4, 4)) {
		t.Fatal("far widget should reject")
	}
}

func TestWrapImageExactLastRow(t *testing.T) {
	const w, h, pad = 3, 2, 4
	stride := w*4 + pad
	need := (h-1)*stride + w*4
	buf := make([]byte, need) // tight: last row has no trailing pad
	img := WrapImage(buf, w, h, stride)
	if img == nil {
		t.Fatal("exact last-row wrap")
	}
	ctx := NewContext(img)
	ctx.Clear(White)
	if _, _, _, a := img.PremulAt(2, 1); a < 250 {
		t.Fatal("last pixel of last row")
	}
}

func TestArcHelpersClosedAndOpen(t *testing.T) {
	p := NewPath()
	p.AddArc(Pt(0, 0), 10, 10, 0, math.Pi*2)
	if p.Empty() {
		t.Fatal("full-circle arc")
	}
	img := NewImage(16, 16)
	ctx := NewContext(img)
	ctx.DrawArc(Pt(8, 8), 0, 6, 0, 1, StrokePaint(White, 2)) // rx=0 no-op
	if countOpaque(img, 1) != 0 {
		t.Fatal("degenerate arc")
	}
}
