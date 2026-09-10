package paintengine2d

import (
	"math"
	"testing"
)

// UI chrome a toolkit paints on day one: buttons, focus rings, scroll
// thumbs, overlapping dirty regions. Engine only — no widgets.

func TestFocusRingOutsideButtonFill(t *testing.T) {
	img := NewImage(80, 36)
	ctx := NewContext(img)
	box := XYWH(10, 6, 60, 24)
	ctx.DrawRoundRect(box, 6, 6, Fill(RGB(0.23, 0.51, 0.93)))
	ring := box.Inset(-3)
	ctx.DrawRoundRect(ring, 8, 8, Paint{
		Color:  RGB(0.45, 0.75, 1.0),
		Style:  StyleStroke,
		Stroke: Stroke{Width: 2, Cap: CapRound, Join: JoinRound, MiterLimit: 4},
	})
	assertAlpha(t, img, 40, 18, 250, 255, "button fill")
	if countPartial(img, 20, 240) < 8 {
		t.Fatalf("focus ring should AA, partial=%d", countPartial(img, 20, 240))
	}
	// Ring is inset(-3) around the fill: the stroke lives outside the button.
	if alphaAt(img, 7, 18) < 20 {
		t.Fatalf("focus ring west a=%d", alphaAt(img, 7, 18))
	}
}

func TestScrollThumbOnTrack(t *testing.T) {
	img := NewImage(20, 80)
	ctx := NewContext(img)
	track := XYWH(6, 4, 8, 72)
	ctx.DrawRoundRect(track, 4, 4, Fill(RGB(0.16, 0.17, 0.20)))
	thumb := XYWH(7, 18, 6, 22)
	ctx.DrawRoundRect(thumb, 3, 3, Fill(RGB(0.45, 0.48, 0.55)))
	assertAlpha(t, img, 10, 28, 200, 255, "thumb")
	assertAlpha(t, img, 10, 8, 20, 255, "track above thumb")
	assertAlpha(t, img, 2, 40, 0, 5, "outside scrollbar")
}

func TestOverlappingDamageMergesThenPaints(t *testing.T) {
	img := NewImage(96, 48)
	ctx := NewContext(img)
	var dirty Damage
	ctx.SetDamage(&dirty)
	ctx.Clear(RGB(0.10, 0.11, 0.14))
	dirty.Reset()
	a := XYWH(8, 8, 40, 28)
	b := XYWH(32, 12, 40, 24)
	dirty.Add(a)
	dirty.Add(b)
	if dirty.Count() != 1 {
		t.Fatalf("overlapping dirty boxes should merge, count=%d", dirty.Count())
	}
	if !dirty.Overlaps(a) || !dirty.Overlaps(b) {
		t.Fatal("union must cover both widgets")
	}
	ctx.DrawRoundRect(a, 4, 4, Fill(RGBA(0.95, 0.35, 0.28, 0.85)))
	ctx.DrawRoundRect(b, 4, 4, Fill(RGBA(0.25, 0.55, 0.95, 0.85)))
	assertAlpha(t, img, 16, 20, 180, 255, "left button")
	assertAlpha(t, img, 60, 24, 180, 255, "right button")
	if alphaAt(img, 40, 24) < 180 {
		t.Fatalf("overlap should be painted, a=%d", alphaAt(img, 40, 24))
	}
}

func TestClipEmptyAndClipRoundRect(t *testing.T) {
	img := NewImage(48, 48)
	ctx := NewContext(img)
	if ctx.ClipEmpty() {
		t.Fatal("full canvas is not empty")
	}
	ctx.ClipRect(XYWH(80, 80, 10, 10))
	if !ctx.ClipEmpty() {
		t.Fatal("off-canvas clip should be empty")
	}
	ctx.Restore() // no save — still empty scissor
	ctx = NewContext(img)
	ctx.ClipRoundRect(XYWH(8, 8, 24, 24), 6, 6)
	if ctx.ClipEmpty() {
		t.Fatal("round-rect clip should keep pixels")
	}
	ctx.SetColor(White)
	ctx.FillRect(XYWH(0, 0, 48, 48))
	assertAlpha(t, img, 20, 20, 250, 255, "inside round clip")
	assertAlpha(t, img, 2, 2, 0, 5, "outside round clip")
	assertAlpha(t, img, 8, 8, 0, 80, "round corner outside fill")
}

func TestThinDiagonalAAStress(t *testing.T) {
	img := NewImage(64, 64)
	ctx := NewContext(img)
	ctx.Clear(Transparent)
	// Hairline-ish diagonals at several slopes (still width > 0).
	for i, w := range []float32{0.55, 0.8, 1.0} {
		y0 := float32(6 + i*18)
		ctx.DrawLine(Pt(4, y0), Pt(60, y0+14), Paint{
			Color:  White,
			Style:  StyleStroke,
			Stroke: Stroke{Width: w, Cap: CapButt, Join: JoinMiter, MiterLimit: 4},
		})
	}
	ctx.DrawLine(Pt(8, 56), Pt(56, 8), Paint{
		Color:  RGB(0.3, 0.8, 1),
		Style:  StyleStroke,
		Stroke: Stroke{Width: 0.7, Cap: CapRound, Join: JoinRound, MiterLimit: 4},
	})
	if countPartial(img, 8, 240) < 20 {
		t.Fatalf("thin diagonals should leave an AA rim, partial=%d", countPartial(img, 8, 240))
	}
	if countOpaque(img, 30) < 15 {
		t.Fatalf("thin strokes should still be visible, n=%d", countOpaque(img, 30))
	}
}

func TestTinyGlyphAAAndIntegerBlit(t *testing.T) {
	atlas := NewBitmapAtlas(White)
	img := NewImage(72, 24)
	ctx := NewContext(img)
	ctx.Clear(RGB(0.10, 0.11, 0.14))
	ctx.DrawLabel("ok", atlas, Pt(2, 4), Paint{Color: White, Filter: FilterNearest})
	ctx.DrawLabel("%/=", atlas, Pt(28, 4), Paint{Color: White, Filter: FilterNearest})
	if countOpaque(img, 80) < 10 {
		t.Fatalf("tiny glyphs should blit, n=%d", countOpaque(img, 80))
	}
	// Sub-pixel origin still paints something (nearest samples).
	ctx.DrawLabel("i", atlas, Pt(56.4, 8.6), Paint{Color: White, Filter: FilterNearest})
	if countOpaque(img, 80) < 12 {
		t.Fatalf("subpixel origin glyph missing, n=%d", countOpaque(img, 80))
	}
}

func TestClipIntersectRotatedTransform(t *testing.T) {
	img := NewImage(64, 64)
	ctx := NewContext(img)
	ctx.Translate(32, 32)
	ctx.Rotate(math.Pi / 5)
	ctx.ClipRect(XYWH(-12, -12, 24, 24))
	if ctx.ClipEmpty() {
		t.Fatal("rotated clip should keep a diamond-ish box")
	}
	if ctx.QuickReject(XYWH(-4, -4, 8, 8)) {
		t.Fatal("center of rotated clip should paint")
	}
	if !ctx.QuickReject(XYWH(40, 40, 4, 4)) {
		t.Fatal("far widget after rotate+clip")
	}
	ctx.SetColor(White)
	ctx.FillRect(XYWH(-20, -20, 40, 40))
	assertAlpha(t, img, 32, 32, 250, 255, "rotated clip center")
	assertAlpha(t, img, 2, 2, 0, 10, "canvas corner outside rotated clip")
}

func TestClipPathThenScaleFill(t *testing.T) {
	img := NewImage(48, 48)
	ctx := NewContext(img)
	ctx.ClipPath(CirclePath(Pt(24, 24), 14))
	ctx.Translate(24, 24)
	ctx.Scale(2, 2)
	ctx.DrawRect(XYWH(-6, -6, 12, 12), Fill(RGB(0.9, 0.3, 0.25)))
	assertAlpha(t, img, 24, 24, 250, 255, "scaled fill inside circle")
	assertAlpha(t, img, 2, 24, 0, 5, "left of circle clip")
	// Circle rim stays AA even after the scaled fill.
	if countPartial(img, 8, 240) < 6 {
		t.Fatalf("clip path should keep an AA rim, partial=%d", countPartial(img, 8, 240))
	}
}

func TestWrapPaddedShmStress(t *testing.T) {
	const w, h, pad = 40, 28, 48
	stride := w*4 + pad
	buf := make([]byte, h*stride+17) // extra tail past last row
	for i := range buf {
		buf[i] = 0x6E
	}
	img := WrapImage(buf[:h*stride], w, h, stride)
	if img == nil {
		t.Fatal("wrap")
	}
	ctx := NewContext(img)
	var dirty Damage
	ctx.SetDamage(&dirty)
	ctx.Clear(RGB(0.10, 0.11, 0.14))
	dirty.Reset()
	btn := XYWH(4, 4, 32, 12)
	thumb := XYWH(30, 18, 6, 8)
	dirty.Add(btn)
	dirty.Add(thumb)
	ctx.ClipRoundRect(XYWH(2, 2, 36, 24), 4, 4)
	ctx.DrawRoundRect(btn, 3, 3, Fill(RGB(0.23, 0.51, 0.93)))
	ctx.DrawRoundRect(btn.Inset(-2), 5, 5, StrokePaint(RGB(0.5, 0.8, 1), 1.25))
	ctx.DrawLabel("Go", NewBitmapAtlas(White), Pt(12, 6), Paint{Color: White, Filter: FilterNearest})
	src := NewImage(6, 8)
	src.Clear(RGB(0.5, 0.52, 0.58))
	ctx.DrawImageRectPaint(src, XYWH(0, 0, 6, 8), thumb, Paint{Color: White, Filter: FilterNearest})
	if _, _, _, a := img.PremulAt(10, 8); a < 200 {
		t.Fatalf("button in shm a=%d", a)
	}
	if _, _, _, a := img.PremulAt(32, 20); a < 200 {
		t.Fatalf("thumb blit in shm a=%d", a)
	}
	for y := 0; y < h; y++ {
		for i := w * 4; i < stride; i++ {
			if buf[y*stride+i] != 0x6E {
				t.Fatalf("shm padding clobbered y=%d i=%d", y, i)
			}
		}
	}
	for i := h * stride; i < len(buf); i++ {
		if buf[i] != 0x6E {
			t.Fatalf("tail past last row clobbered i=%d", i)
		}
	}
	if dirty.Empty() || dirty.Count() < 1 {
		t.Fatal("draws should record damage")
	}
}

func TestFillCircleWarmZeroAllocs(t *testing.T) {
	img := NewImage(64, 64)
	ctx := NewContext(img)
	p := Fill(RGB(0.2, 0.5, 0.9))
	ctx.DrawCircle(Pt(32, 32), 14, p)
	n := testing.AllocsPerRun(40, func() {
		ctx.DrawCircle(Pt(32, 32), 14, p)
	})
	if n != 0 {
		t.Fatalf("warm circle fill allocs/op = %.1f, want 0", n)
	}
}

func TestBlitNearest1to1ZeroAllocs(t *testing.T) {
	src := NewImage(16, 12)
	src.Clear(RGB(0.2, 0.6, 0.9))
	dst := NewImage(32, 24)
	ctx := NewContext(dst)
	sr := XYWH(0, 0, 16, 12)
	dr := XYWH(4, 4, 16, 12)
	p := Paint{Color: White, Filter: FilterNearest}
	ctx.DrawImageRectPaint(src, sr, dr, p)
	n := testing.AllocsPerRun(50, func() {
		ctx.DrawImageRectPaint(src, sr, dr, p)
	})
	if n != 0 {
		t.Fatalf("1:1 nearest blit allocs/op = %.1f, want 0", n)
	}
}

func TestPadded1to1BlitZeroAllocs(t *testing.T) {
	const w, h, pad = 10, 8, 12
	stride := w*4 + pad
	buf := make([]byte, h*stride)
	src := WrapImage(buf, w, h, stride)
	src.Clear(RGB(0.4, 0.2, 0.8))
	dst := NewImage(20, 16)
	ctx := NewContext(dst)
	sr := XYWH(0, 0, 10, 8)
	dr := XYWH(2, 2, 10, 8)
	p := Paint{Color: White, Filter: FilterNearest}
	ctx.DrawImageRectPaint(src, sr, dr, p)
	n := testing.AllocsPerRun(50, func() {
		ctx.DrawImageRectPaint(src, sr, dr, p)
	})
	if n != 0 {
		t.Fatalf("padded 1:1 blit allocs/op = %.1f, want 0", n)
	}
}
