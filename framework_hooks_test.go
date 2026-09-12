package paintengine2d

import "testing"

// Hooks a forthcoming retained UI framework will call. No widgets here —
// just the canvas queries and the paint-cycle shape.

func TestVersionIsFoundationReady(t *testing.T) {
	if Version != "0.9.1" {
		t.Fatalf("Version %q, want 0.9.1 incremental paint", Version)
	}
}

func TestSizeAndSaveCount(t *testing.T) {
	img := NewImage(80, 40)
	ctx := NewContext(img)
	w, h := ctx.Size()
	if w != 80 || h != 40 {
		t.Fatalf("size %dx%d", w, h)
	}
	if ctx.SaveCount() != 0 {
		t.Fatal("root save count")
	}
	ctx.Save()
	ctx.Save()
	if ctx.SaveCount() != 2 {
		t.Fatalf("save count %d", ctx.SaveCount())
	}
	ctx.Restore()
	if ctx.SaveCount() != 1 {
		t.Fatal("after one restore")
	}
}

func TestDeviceAndLocalClipBounds(t *testing.T) {
	img := NewImage(64, 64)
	ctx := NewContext(img)
	full := ctx.DeviceClipBounds()
	if full.Min.X != 0 || full.Max.X != 64 || full.Max.Y != 64 {
		t.Fatalf("full clip %+v", full)
	}
	ctx.ClipRect(XYWH(8, 10, 20, 12))
	dev := ctx.DeviceClipBounds()
	if dev.Min.X != 8 || dev.Min.Y != 10 || dev.Max.X != 28 || dev.Max.Y != 22 {
		t.Fatalf("scissor %+v", dev)
	}
	ctx.Translate(8, 10)
	loc := ctx.LocalClipBounds()
	if loc.Min.X > 0.1 || loc.Min.Y > 0.1 || loc.Max.X < 19.9 || loc.Max.Y < 11.9 {
		t.Fatalf("local clip %+v", loc)
	}
}

func TestQuickRejectWidgetBounds(t *testing.T) {
	img := NewImage(48, 48)
	ctx := NewContext(img)
	ctx.ClipRect(XYWH(0, 0, 24, 48))
	if ctx.QuickReject(XYWH(2, 2, 10, 10)) {
		t.Fatal("visible child should not reject")
	}
	if !ctx.QuickReject(XYWH(30, 2, 10, 10)) {
		t.Fatal("child left of clip should reject")
	}
	if !ctx.QuickReject(Rect{}) {
		t.Fatal("empty rejects")
	}
}

func TestDamageOverlapsForPartialRedraw(t *testing.T) {
	var d Damage
	d.Add(XYWH(10, 10, 8, 8))
	if !d.Overlaps(XYWH(14, 14, 10, 10)) {
		t.Fatal("overlapping widget should paint")
	}
	if d.Overlaps(XYWH(30, 30, 4, 4)) {
		t.Fatal("clean widget should skip")
	}
	if d.Overlaps(Rect{}) {
		t.Fatal("empty")
	}
}

func TestFrameworkPaintCycleSketch(t *testing.T) {
	// Retained-UI frame: dirty a control, skip siblings, paint with
	// save / translate / clip / restore. Not a widget kit — just the contract.
	img := NewImage(64, 48)
	ctx := NewContext(img)
	var dirty Damage
	ctx.SetDamage(&dirty)
	ctx.Clear(RGB(0.10, 0.11, 0.14))
	dirty.Reset()
	dirty.Add(XYWH(8, 8, 24, 16)) // framework: button invalidated

	type child struct{ box Rect }
	children := []child{
		{XYWH(8, 8, 24, 16)},
		{XYWH(36, 8, 20, 16)},
	}
	painted := 0
	for _, ch := range children {
		if !dirty.Overlaps(ch.box) || ctx.QuickReject(ch.box) {
			continue
		}
		ctx.Save()
		ctx.ClipRect(ch.box)
		ctx.DrawRoundRect(ch.box, 4, 4, Fill(RGB(0.23, 0.51, 0.93)))
		ctx.Restore()
		painted++
	}
	if painted != 1 {
		t.Fatalf("expected one dirty child, painted %d", painted)
	}
	assertAlpha(t, img, 16, 14, 200, 255, "dirty button")
	r, g, b, _ := rgbAt(img, 40, 14)
	if r > 80 || g > 80 || b > 80 {
		t.Fatalf("clean sibling should stay background, got %d %d %d", r, g, b)
	}
}

func TestFrameworkRecorderScene(t *testing.T) {
	rec := NewRecorder(64, 48)
	ctx := NewContextDevice(rec)
	ctx.Clear(RGB(0.10, 0.11, 0.14))
	g := rec.BeginGroup(1, Identity())
	ctx.DrawRect(XYWH(8, 8, 24, 16), Fill(RGB(0.23, 0.51, 0.93)))
	rec.EndGroup()
	s := rec.Finish()
	if s.Nodes < 2 || g == nil {
		t.Fatalf("scene %+v group=%v", s, g)
	}
	img := NewImage(64, 48)
	DrawScene(s, NewCPUDevice(img))
	assertAlpha(t, img, 16, 14, 200, 255, "recorded button")
}
