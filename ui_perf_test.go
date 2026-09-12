package paintengine2d

import "testing"

func TestImageScrollDownExposesTop(t *testing.T) {
	im := NewImage(8, 6)
	im.Clear(RGB(0.1, 0.1, 0.1))
	im.ClearRect(XYWH(0, 0, 8, 2), RGB(1, 0, 0))
	im.Scroll(0, 2, XYWH(0, 0, 8, 6))
	r, _, _, a := im.PremulAt(2, 3)
	if a < 250 || r < 200 {
		t.Fatalf("scrolled red stripe should land at y=2..4, y=3 r=%d a=%d", r, a)
	}
	r, _, _, a = im.PremulAt(2, 0)
	if r < 200 {
		t.Fatalf("vacated top is left as-is (still red), r=%d a=%d", r, a)
	}
}

func TestContextScrollList(t *testing.T) {
	img := NewImage(40, 24)
	ctx := NewContext(img)
	ctx.Clear(RGB(0.1, 0.1, 0.12))
	ctx.SetColor(RGB(0.9, 0.2, 0.2))
	ctx.FillRect(XYWH(0, 0, 40, 8))
	ctx.Scroll(0, 8, XYWH(0, 0, 40, 24))
	assertAlpha(t, img, 4, 12, 200, 255, "scrolled band")
}

func TestCopyImageLayer(t *testing.T) {
	src := NewImage(10, 6)
	src.Clear(RGB(0.2, 0.8, 0.3))
	dst := NewImage(20, 12)
	dst.Clear(RGB(0.1, 0.1, 0.1))
	ctx := NewContext(dst)
	ctx.CopyImage(src, XYWH(0, 0, 10, 6), Pt(4, 3))
	_, g, _, a := dst.PremulAt(6, 5)
	if a < 250 || g < 180 {
		t.Fatalf("copied layer g=%d a=%d", g, a)
	}
	_, g, _, a = dst.PremulAt(1, 1)
	if g > 80 {
		t.Fatalf("outside copy should stay bg g=%d", g)
	}
}

func TestSaveRestoreClipChurnZeroAlloc(t *testing.T) {
	img := NewImage(64, 64)
	ctx := NewContext(img)
	ctx.SetColor(RGB(0.2, 0.4, 0.8))
	for i := 0; i < 4; i++ {
		ctx.Save()
		ctx.ClipRect(XYWH(4, 4, 40, 40))
		ctx.FillRect(XYWH(0, 0, 64, 64))
		ctx.Restore()
	}
	n := testing.AllocsPerRun(80, func() {
		ctx.Save()
		ctx.ClipRect(XYWH(4, 4, 40, 40))
		ctx.FillRect(XYWH(8, 8, 20, 12))
		ctx.Restore()
	})
	if n != 0 {
		t.Fatalf("Save/ClipRect/Fill/Restore allocs/op = %.1f, want 0", n)
	}
}

func TestSaveAfterClipPathDoesNotAllocOnSave(t *testing.T) {
	img := NewImage(48, 48)
	ctx := NewContext(img)
	ctx.ClipPath(CirclePath(Pt(24, 24), 16))
	n := testing.AllocsPerRun(40, func() {
		ctx.Save()
		ctx.Restore()
	})
	if n != 0 {
		t.Fatalf("Save/Restore after ClipPath allocs/op = %.1f, want 0 (COW mask)", n)
	}
}

func TestCPUSurfaceResizeKeepsDevice(t *testing.T) {
	s := NewCPUSurfaceSize(32, 24)
	dev := s.Device()
	ctx := NewContextSurface(s)
	if err := s.Resize(48, 36); err != nil {
		t.Fatal(err)
	}
	if s.Device() != dev {
		t.Fatal("Resize must keep the same CPUDevice so Context stays valid")
	}
	ctx.SyncSize()
	b := ctx.DeviceClipBounds()
	if b.Max.X != 48 || b.Max.Y != 36 {
		t.Fatalf("SyncSize clip %+v", b)
	}
	ctx.SetColor(White)
	ctx.FillRect(XYWH(0, 0, 48, 36))
	assertAlpha(t, s.Image(), 40, 30, 250, 255, "paint after resize")
}

func TestDrawLabelScrollCacheHits(t *testing.T) {
	img := NewImage(200, 20)
	ctx := NewContext(img)
	atlas := NewBitmapAtlas(White)
	p := Paint{Color: White, Filter: FilterNearest}
	labels := []string{"Alpha", "Bravo", "Charlie", "Delta", "Echo"}
	for _, s := range labels {
		ctx.DrawLabel(s, atlas, Pt(2, 4), p)
	}
	n := testing.AllocsPerRun(40, func() {
		for _, s := range labels {
			ctx.DrawLabel(s, atlas, Pt(2, 4), p)
		}
	})
	if n != 0 {
		t.Fatalf("cached list labels allocs/op = %.1f, want 0", n)
	}
}

func TestTouchRectUnionsDirty(t *testing.T) {
	im := NewImage(32, 32)
	im.TouchRect(XYWH(2, 2, 4, 4))
	im.TouchRect(XYWH(20, 8, 4, 4))
	b := im.Dirty
	if b.Min.X > 2 || b.Max.X < 24 || b.Min.Y > 2 || b.Max.Y < 12 {
		t.Fatalf("dirty union %+v", b)
	}
}
