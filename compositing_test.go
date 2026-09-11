package paintengine2d

import "testing"

// Src-over / image-blit cases in the spirit of Blend2D compositing tests.

func TestSrcOverSemiTransparent(t *testing.T) {
	img := NewImage(16, 16)
	ctx := NewContext(img)
	ctx.Clear(Black)
	ctx.DrawRect(XYWH(0, 0, 16, 16), Fill(RGBA(1, 1, 1, 0.5)))
	r, g, b, a := rgbAt(img, 8, 8)
	if a < 250 {
		t.Fatalf("opaque destination after over, a=%d", a)
	}
	// 50% white over black → ~128 gray (premul = straight here since a=255).
	if r < 100 || r > 160 || r != g || g != b {
		t.Fatalf("expected mid gray, got %d %d %d", r, g, b)
	}
}

func TestLayeredAlphaDarkensCorrectly(t *testing.T) {
	img := NewImage(12, 12)
	ctx := NewContext(img)
	ctx.DrawRect(XYWH(0, 0, 12, 12), Fill(RGBA(1, 0, 0, 0.5)))
	ctx.DrawRect(XYWH(0, 0, 12, 12), Fill(RGBA(1, 0, 0, 0.5)))
	r, _, _, a := rgbAt(img, 6, 6)
	// Two 50% red overs: coverage accumulates; not still 128/128.
	if a < 170 {
		t.Fatalf("stacked alpha should rise, a=%d r=%d", a, r)
	}
	if r < 170 {
		t.Fatalf("stacked red should rise, r=%d", r)
	}
}

func TestDrawImageAlphaModulate(t *testing.T) {
	src := NewImage(4, 4)
	src.Clear(White)
	dst := NewImage(12, 12)
	ctx := NewContext(dst)
	ctx.DrawImageRectPaint(src, XYWH(0, 0, 4, 4), XYWH(2, 2, 4, 4), Paint{Color: White.WithAlpha(0.4)})
	_, _, _, a := rgbAt(dst, 3, 3)
	if a < 70 || a > 140 {
		t.Fatalf("modulated blit alpha=%d", a)
	}
	assertAlpha(t, dst, 0, 0, 0, 0, "outside blit")
}

func TestBlitRGBTintWhiteAtlas(t *testing.T) {
	src := NewImage(4, 4)
	src.Clear(White)
	dst := NewImage(12, 12)
	ctx := NewContext(dst)
	ctx.DrawImageRectPaint(src, XYWH(0, 0, 4, 4), XYWH(2, 2, 4, 4), Paint{Color: Red, Filter: FilterNearest})
	r, g, b, a := rgbAt(dst, 3, 3)
	if r < 250 || g > 2 || b > 2 || a < 250 {
		t.Fatalf("white atlas * red = red, got %d %d %d %d", r, g, b, a)
	}
	assertAlpha(t, dst, 0, 0, 0, 0, "outside tinted blit")
}

func TestBlitRGBTintPremulSample(t *testing.T) {
	src := NewImage(2, 2)
	src.Clear(RGBA(1, 1, 1, 0.5))
	dst := NewImage(4, 4)
	ctx := NewContext(dst)
	ctx.DrawImageRectPaint(src, XYWH(0, 0, 2, 2), XYWH(1, 1, 2, 2), Paint{Color: RGB(1, 0, 0), Filter: FilterNearest})
	r, g, b, a := rgbAt(dst, 1, 1)
	if r < 100 || r > 160 || g > 5 || b > 5 || a < 100 || a > 160 {
		t.Fatalf("premul white * red, got %d %d %d %d", r, g, b, a)
	}
}

func TestBlitRGBTintMultipliesSource(t *testing.T) {
	src := NewImage(2, 2)
	src.Clear(RGB(1, 0.5, 0))
	dst := NewImage(4, 4)
	ctx := NewContext(dst)
	ctx.DrawImageRectPaint(src, XYWH(0, 0, 2, 2), XYWH(1, 1, 2, 2), Paint{Color: RGB(0, 1, 1), Filter: FilterNearest})
	r, g, b, a := rgbAt(dst, 1, 1)
	if r > 5 || g < 100 || g > 160 || b > 5 || a < 250 {
		t.Fatalf("orange * (0,1,1), got %d %d %d %d", r, g, b, a)
	}
}

func TestBlitRGBTintScaledBilinear(t *testing.T) {
	src := NewImage(4, 4)
	src.Clear(White)
	dst := NewImage(24, 24)
	ctx := NewContext(dst)
	ctx.DrawImageRectPaint(src, XYWH(0, 0, 4, 4), XYWH(2, 2, 16, 16), Paint{Color: RGB(0.2, 0.55, 0.95), Filter: FilterBilinear})
	r, g, b, a := rgbAt(dst, 10, 10)
	if a < 200 || r > 90 || g < 100 || b < 200 {
		t.Fatalf("scaled bilinear tint, got %d %d %d %d", r, g, b, a)
	}
}

func TestBlitZeroPaintStaysUntinted(t *testing.T) {
	src := NewImage(2, 2)
	src.Clear(White)
	dst := NewImage(4, 4)
	NewContext(dst).DrawImageRectPaint(src, XYWH(0, 0, 2, 2), XYWH(1, 1, 2, 2), Paint{Filter: FilterNearest})
	r, g, b, a := rgbAt(dst, 1, 1)
	if r < 250 || g < 250 || b < 250 || a < 250 {
		t.Fatalf("zero paint should be unmodulated white, got %d %d %d %d", r, g, b, a)
	}
}

func TestBlitRGBTintWithAlpha(t *testing.T) {
	src := NewImage(4, 4)
	src.Clear(White)
	dst := NewImage(8, 8)
	ctx := NewContext(dst)
	ctx.Clear(Black)
	ctx.DrawImageRectPaint(src, XYWH(0, 0, 4, 4), XYWH(2, 2, 4, 4), Paint{Color: Red.WithAlpha(0.5), Filter: FilterNearest})
	r, g, b, a := rgbAt(dst, 3, 3)
	if r < 100 || r > 160 || g > 8 || b > 8 || a < 250 {
		t.Fatalf("50%% red over black, got %d %d %d %d", r, g, b, a)
	}
}

func TestDrawImageOutOfBounds(t *testing.T) {
	src := NewImage(8, 8)
	src.Clear(White)
	dst := NewImage(16, 16)
	ctx := NewContext(dst)
	ctx.DrawImage(src, -20, -20)
	ctx.DrawImage(src, 40, 40)
	if countOpaque(dst, 1) != 0 {
		t.Fatal("fully out-of-bounds blit should be a no-op")
	}
}

func TestDrawImagePartialOverlap(t *testing.T) {
	src := NewImage(8, 8)
	src.Clear(White)
	dst := NewImage(16, 16)
	ctx := NewContext(dst)
	ctx.DrawImage(src, 12, 12)
	assertAlpha(t, dst, 14, 14, 200, 255, "overlapping corner")
	assertAlpha(t, dst, 4, 4, 0, 0, "far side")
}

func TestClearTransparentThenDraw(t *testing.T) {
	img := NewImage(8, 8)
	ctx := NewContext(img)
	ctx.Clear(White)
	ctx.Clear(Transparent)
	if countOpaque(img, 1) != 0 {
		t.Fatal("Clear(Transparent) should wipe")
	}
	ctx.SetColor(White)
	ctx.FillRect(XYWH(2, 2, 4, 4))
	assertAlpha(t, img, 3, 3, 250, 255, "draw after clear")
}
