package paintengine2d

import "testing"

// Canvas / Skia-style API contract tests: save/restore, transform order,
// empty and degenerate geometry, Clear vs clip.

func TestRestoreWithoutSaveIsNoop(t *testing.T) {
	img := NewImage(16, 16)
	ctx := NewContext(img)
	ctx.Restore()
	ctx.Restore()
	ctx.SetColor(White)
	ctx.FillRect(XYWH(2, 2, 8, 8))
	assertAlpha(t, img, 4, 4, 250, 255, "draw after extra restore")
}

func TestNestedSaveRestorePaintsAndClip(t *testing.T) {
	img := NewImage(32, 32)
	ctx := NewContext(img)
	ctx.SetColor(White)
	ctx.Save()
	ctx.SetColor(RGB(1, 0, 0))
	ctx.ClipRect(XYWH(0, 0, 16, 32))
	ctx.Save()
	ctx.ClipRect(XYWH(0, 0, 32, 10))
	ctx.FillRect(XYWH(0, 0, 32, 32))
	ctx.Restore()
	ctx.FillRect(XYWH(0, 12, 32, 4))
	ctx.Restore()
	ctx.FillRect(XYWH(20, 20, 6, 6))

	assertAlpha(t, img, 4, 4, 250, 255, "inner clip")
	assertAlpha(t, img, 20, 4, 0, 0, "outside inner clip")
	assertAlpha(t, img, 4, 14, 250, 255, "outer clip after restore")
	assertAlpha(t, img, 20, 14, 0, 0, "outer clip still active")
	r, _, _, a := rgbAt(img, 22, 22)
	if a < 250 || r != 255 {
		t.Fatalf("fully restored fill should be white, r=%d a=%d", r, a)
	}
}

func TestTransformOrderTranslateThenScale(t *testing.T) {
	img := NewImage(40, 16)
	ctx := NewContext(img)
	ctx.Translate(10, 0)
	ctx.Scale(2, 1)
	ctx.SetColor(White)
	ctx.FillRect(XYWH(0, 2, 5, 8))
	// Device x = 10 + 2*userX → [10, 20).
	assertAlpha(t, img, 12, 6, 250, 255, "mapped interior")
	assertAlpha(t, img, 4, 6, 0, 0, "before translate")
	assertAlpha(t, img, 24, 6, 0, 0, "past scaled width")
}

func TestTransformOrderScaleThenTranslate(t *testing.T) {
	img := NewImage(40, 16)
	ctx := NewContext(img)
	ctx.Scale(2, 1)
	ctx.Translate(10, 0)
	ctx.SetColor(White)
	ctx.FillRect(XYWH(0, 2, 5, 8))
	// Device x = 2*(10+userX) → [20, 30).
	assertAlpha(t, img, 22, 6, 250, 255, "mapped interior")
	assertAlpha(t, img, 12, 6, 0, 0, "unscaled translate would land here")
}

func TestNestedTranslateLikeJUCE(t *testing.T) {
	img := NewImage(40, 20)
	ctx := NewContext(img)
	ctx.SetColor(White)
	ctx.Translate(8, 4)
	ctx.Save()
	ctx.Translate(8, 0)
	ctx.FillRect(XYWH(0, 0, 6, 6))
	ctx.Restore()
	ctx.FillRect(XYWH(0, 10, 6, 6))
	assertAlpha(t, img, 18, 6, 250, 255, "inner translate")
	assertAlpha(t, img, 10, 6, 0, 0, "inner only")
	assertAlpha(t, img, 10, 16, 250, 255, "outer after restore")
	assertAlpha(t, img, 18, 16, 0, 0, "inner translate gone")
}

func TestEmptyPathIsNoop(t *testing.T) {
	img := NewImage(12, 12)
	ctx := NewContext(img)
	ctx.DrawPath(NewPath(), Fill(White))
	ctx.DrawPath(nil, Fill(White))
	ctx.StrokePath(NewPath())
	if countOpaque(img, 1) != 0 {
		t.Fatal("empty path painted pixels")
	}
}

func TestDegeneratePathIsNoop(t *testing.T) {
	img := NewImage(16, 16)
	ctx := NewContext(img)
	p := NewPath()
	p.MoveTo(4, 4)
	ctx.DrawPath(p, Fill(White))

	q := NewPath()
	q.MoveTo(8, 8)
	q.LineTo(8, 8)
	q.Close()
	ctx.DrawPath(q, Fill(White))

	if countOpaque(img, 1) != 0 {
		t.Fatal("degenerate path painted pixels")
	}
}

func TestZeroSizeRectIsNoop(t *testing.T) {
	img := NewImage(16, 16)
	ctx := NewContext(img)
	ctx.SetColor(White)
	ctx.FillRect(XYWH(4, 4, 0, 8))
	ctx.FillRect(XYWH(4, 4, 8, 0))
	ctx.DrawRect(Rect{}, Fill(White))
	if countOpaque(img, 1) != 0 {
		t.Fatal("zero-size rect painted pixels")
	}
}

func TestClearIgnoresClip(t *testing.T) {
	img := NewImage(20, 20)
	ctx := NewContext(img)
	ctx.ClipRect(XYWH(0, 0, 6, 20))
	ctx.Clear(White)
	assertAlpha(t, img, 2, 10, 250, 255, "inside former clip")
	assertAlpha(t, img, 15, 10, 250, 255, "Clear must ignore clip")
}

func TestSetMatrixReplacesConcat(t *testing.T) {
	img := NewImage(24, 16)
	ctx := NewContext(img)
	ctx.Translate(100, 0)
	ctx.SetMatrix(Translation(4, 2))
	ctx.SetColor(White)
	ctx.FillRect(XYWH(0, 0, 6, 6))
	assertAlpha(t, img, 6, 4, 250, 255, "replaced matrix")
	assertAlpha(t, img, 20, 4, 0, 0, "old translate discarded")
}

func TestContextImageAndDevice(t *testing.T) {
	img := NewImage(8, 8)
	ctx := NewContext(img)
	if ctx.Image() != img {
		t.Fatal("Image() should return the CPU pixmap")
	}
	if ctx.Device() == nil {
		t.Fatal("Device()")
	}
	w, h := ctx.Device().Size()
	if w != 8 || h != 8 {
		t.Fatalf("size %d×%d", w, h)
	}
}

func TestNewContextDeviceNilPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	NewContextDevice(nil)
}
