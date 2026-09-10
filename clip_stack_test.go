package paintengine2d

import "testing"

// Clip intersection tests inspired by Skia clip-stack and JUCE clip bounds.

func TestClipRectIntersection(t *testing.T) {
	img := NewImage(40, 40)
	ctx := NewContext(img)
	ctx.ClipRect(XYWH(4, 4, 24, 24))
	ctx.ClipRect(XYWH(16, 8, 20, 10))
	ctx.SetColor(White)
	ctx.FillRect(XYWH(0, 0, 40, 40))
	assertAlpha(t, img, 20, 12, 250, 255, "intersection")
	assertAlpha(t, img, 8, 12, 0, 0, "first clip only")
	assertAlpha(t, img, 20, 22, 0, 0, "outside second clip")
}

func TestClipPathThenClipRect(t *testing.T) {
	img := NewImage(48, 48)
	ctx := NewContext(img)
	ctx.ClipPath(CirclePath(Pt(24, 24), 16))
	ctx.ClipRect(XYWH(24, 0, 24, 48))
	ctx.SetColor(White)
	ctx.FillRect(XYWH(0, 0, 48, 48))
	assertAlpha(t, img, 30, 24, 250, 255, "right half of circle")
	assertAlpha(t, img, 16, 24, 0, 10, "left half clipped by rect")
	assertAlpha(t, img, 40, 24, 0, 0, "outside circle")
}

func TestClipRectThenClipPath(t *testing.T) {
	img := NewImage(48, 48)
	ctx := NewContext(img)
	ctx.ClipRect(XYWH(8, 8, 32, 32))
	ctx.ClipPath(CirclePath(Pt(8, 24), 14))
	ctx.SetColor(White)
	ctx.FillRect(XYWH(0, 0, 48, 48))
	assertAlpha(t, img, 14, 24, 200, 255, "overlap of rect and circle")
	assertAlpha(t, img, 4, 24, 0, 0, "circle outside rect scissor")
	assertAlpha(t, img, 30, 24, 0, 10, "rect but outside circle")
}

func TestEmptyClipRejectsDraws(t *testing.T) {
	img := NewImage(16, 16)
	ctx := NewContext(img)
	ctx.ClipRect(XYWH(4, 4, 0, 0))
	ctx.SetColor(White)
	ctx.FillRect(XYWH(0, 0, 16, 16))
	ctx.DrawCircle(Pt(8, 8), 6, Fill(White))
	if countOpaque(img, 1) != 0 {
		t.Fatal("empty clip should reject drawing")
	}
}

func TestEmptyClipPathRejectsDraws(t *testing.T) {
	img := NewImage(16, 16)
	ctx := NewContext(img)
	ctx.ClipPath(NewPath())
	ctx.SetColor(White)
	ctx.FillRect(XYWH(0, 0, 16, 16))
	if countOpaque(img, 1) != 0 {
		t.Fatal("empty clip path should reject drawing")
	}
}

func TestClipRestoredAfterSave(t *testing.T) {
	img := NewImage(24, 24)
	ctx := NewContext(img)
	ctx.SetColor(White)
	ctx.Save()
	ctx.ClipRect(XYWH(0, 0, 8, 24))
	ctx.Restore()
	ctx.FillRect(XYWH(12, 4, 8, 8))
	assertAlpha(t, img, 14, 8, 250, 255, "draw after clip restore")
}

func TestRotatedClipRectBecomesPath(t *testing.T) {
	img := NewImage(40, 40)
	ctx := NewContext(img)
	ctx.Translate(20, 20)
	ctx.Rotate(0.4)
	ctx.ClipRect(XYWH(-8, -8, 16, 16))
	ctx.SetMatrix(Identity())
	ctx.SetColor(White)
	ctx.FillRect(XYWH(0, 0, 40, 40))
	// Axis-aligned corners of the canvas should stay empty; the center filled.
	assertAlpha(t, img, 20, 20, 200, 255, "rotated clip center")
	assertAlpha(t, img, 1, 1, 0, 5, "canvas corner outside rotated clip")
}
