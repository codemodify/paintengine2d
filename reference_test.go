package paintengine2d

import (
	"math"
	"testing"
)

// Scenarios rewritten from public engine *themes* (AGG demos, Skia canvas
// contracts, Cairo clips, JUCE nested xforms, Blend2D compositing, LibGfx
// painter, NanoVG arcs). No upstream source or goldens are vendored.

func TestAGGDashedCircle(t *testing.T) {
	img := NewImage(64, 64)
	ctx := NewContext(img)
	ctx.DrawCircle(Pt(32, 32), 22, Paint{
		Color: White,
		Style: StyleStroke,
		Stroke: Stroke{
			Width: 3, Cap: CapButt, Join: JoinMiter, MiterLimit: 4,
			Dash: []float32{8, 6},
		},
	})
	if countOpaque(img, 180) < 20 {
		t.Fatalf("dashed ring should paint dashes, n=%d", countOpaque(img, 180))
	}
	if alphaAt(img, 32, 32) > 10 {
		t.Fatalf("circle hole should stay empty, a=%d", alphaAt(img, 32, 32))
	}
}

func TestSkiaRotateThenClipThenIdentity(t *testing.T) {
	img := NewImage(48, 48)
	ctx := NewContext(img)
	ctx.Translate(24, 24)
	ctx.Rotate(math.Pi / 6)
	ctx.ClipRect(XYWH(-10, -8, 20, 16))
	ctx.SetMatrix(Identity())
	ctx.SetColor(White)
	ctx.FillRect(XYWH(0, 0, 48, 48))
	n := countOpaque(img, 200)
	if n == 0 || n == 48*48 {
		t.Fatalf("rotated clip coverage n=%d", n)
	}
	if ctx.DeviceClipBounds().Empty() {
		t.Fatal("rotated clip should leave a scissor AABB")
	}
}

func TestCairoNestedClipIntersect(t *testing.T) {
	img := NewImage(40, 40)
	ctx := NewContext(img)
	ctx.ClipRect(XYWH(4, 4, 28, 28))
	ctx.ClipPath(CirclePath(Pt(20, 20), 12))
	ctx.ClipRect(XYWH(20, 4, 16, 32))
	ctx.SetColor(White)
	ctx.FillRect(XYWH(0, 0, 40, 40))
	assertAlpha(t, img, 24, 20, 200, 255, "triple intersection")
	assertAlpha(t, img, 12, 20, 0, 20, "left lobe clipped by last rect")
	assertAlpha(t, img, 2, 2, 0, 0, "outside all")
}

func TestJUCENestedOriginSave(t *testing.T) {
	img := NewImage(48, 24)
	ctx := NewContext(img)
	ctx.SetColor(White)
	ctx.Translate(6, 4)
	ctx.Save()
	ctx.Translate(10, 0)
	ctx.Scale(2, 1)
	ctx.FillRect(XYWH(0, 0, 4, 8))
	ctx.Restore()
	ctx.SetStroke(StrokePaint(White, 2))
	ctx.StrokeRect(XYWH(0, 12, 8, 6))
	assertAlpha(t, img, 18, 6, 250, 255, "inner scaled fill")
	assertAlpha(t, img, 8, 16, 200, 255, "outer stroke after restore")
	assertAlpha(t, img, 20, 16, 0, 10, "inner translate discarded")
}

func TestLibGfxRoundClipButton(t *testing.T) {
	img := NewImage(64, 32)
	ctx := NewContext(img)
	ctx.ClipPath(RoundRectPath(XYWH(6, 6, 52, 20), 8, 8))
	ctx.DrawRect(XYWH(0, 0, 64, 32), Linear(LinearGradient{
		Start: Pt(0, 6), End: Pt(0, 26),
		Stops: []GradientStop{
			{0, RGB(0.35, 0.60, 0.95)},
			{1, RGB(0.15, 0.35, 0.80)},
		},
	}))
	assertAlpha(t, img, 32, 16, 250, 255, "button interior")
	assertAlpha(t, img, 6, 6, 0, 80, "sharp corner outside round clip")
	if !ctx.QuickReject(XYWH(0, 0, 4, 4)) {
		t.Fatal("tightened scissor should reject the canvas corner")
	}
}

func TestBlend2DCheckerOver(t *testing.T) {
	img := NewImage(16, 16)
	ctx := NewContext(img)
	ctx.Clear(RGB(0.2, 0.2, 0.2))
	for y := 0; y < 16; y += 4 {
		for x := 0; x < 16; x += 4 {
			if ((x/4)+(y/4))&1 == 0 {
				ctx.DrawRect(XYWH(float32(x), float32(y), 4, 4), Fill(RGBA(1, 0, 0, 0.5)))
			}
		}
	}
	r, _, _, a := rgbAt(img, 2, 2)
	if a < 200 || r < 80 {
		t.Fatalf("checker cell r=%d a=%d", r, a)
	}
	r2, _, _, a2 := rgbAt(img, 6, 2)
	if a2 < 200 || r2 > 80 {
		t.Fatalf("gap should stay background r=%d a=%d", r2, a2)
	}
}

func TestNanoVGArcRing(t *testing.T) {
	img := NewImage(48, 48)
	ctx := NewContext(img)
	ctx.DrawArc(Pt(24, 24), 16, 16, -math.Pi/2, math.Pi*1.25, Paint{
		Color:  RGB(0.2, 0.8, 0.5),
		Style:  StyleStroke,
		Stroke: Stroke{Width: 4, Cap: CapRound, Join: JoinRound, MiterLimit: 4},
	})
	if alphaAt(img, 24, 8) < 80 {
		t.Fatalf("arc should start at 12 o'clock, a=%d", alphaAt(img, 24, 8))
	}
	if alphaAt(img, 24, 24) > 15 {
		t.Fatalf("ring hole a=%d", alphaAt(img, 24, 24))
	}
}

func TestQuickRejectAfterPathClip(t *testing.T) {
	img := NewImage(32, 32)
	ctx := NewContext(img)
	ctx.ClipPath(CirclePath(Pt(16, 16), 6))
	if ctx.QuickReject(XYWH(0, 0, 2, 2)) == false {
		t.Fatal("tightened scissor should reject a far corner")
	}
	if ctx.QuickReject(XYWH(14, 14, 4, 4)) {
		t.Fatal("circle interior should not reject")
	}
}

func TestDamageClippedByClipAndCanvas(t *testing.T) {
	img := NewImage(32, 32)
	ctx := NewContext(img)
	var d Damage
	ctx.SetDamage(&d)
	ctx.ClipRect(XYWH(8, 8, 8, 8))
	ctx.SetColor(White)
	ctx.FillRect(XYWH(-20, -20, 80, 80))
	if d.Empty() {
		t.Fatal("draw should dirty")
	}
	b := d.Bounds()
	if b.Min.X < 7.5 || b.Max.X > 16.5 || b.Min.Y < 7.5 || b.Max.Y > 16.5 {
		t.Fatalf("damage should be clipped to scissor ∩ canvas, got %+v", b)
	}
}
