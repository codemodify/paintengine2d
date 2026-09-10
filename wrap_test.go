package paintengine2d

import "testing"

func TestWrapImagePaintsIntoCallerBuffer(t *testing.T) {
	const w, h = 32, 16
	buf := make([]byte, w*h*4)
	img := WrapImage(buf, w, h, 0)
	if img == nil {
		t.Fatal("wrap packed")
	}
	ctx := NewContext(img)
	ctx.Clear(RGB(0.10, 0.11, 0.14))
	ctx.DrawRect(XYWH(4, 2, 12, 8), Fill(White))
	if buf[img.pixIndex(6, 4)+3] < 250 {
		t.Fatal("draw must land in the caller buffer")
	}
}

func TestWrapImagePaddedStride(t *testing.T) {
	const w, h, pad = 8, 4, 16 // stride = 8*4+16 = 48
	stride := w*4 + pad
	buf := make([]byte, h*stride)
	for i := range buf {
		buf[i] = 0xAB
	}
	img := WrapImage(buf, w, h, stride)
	if img == nil || img.RowStride() != stride {
		t.Fatal("padded wrap")
	}
	ctx := NewContext(img)
	ctx.Clear(White)
	// Pixel columns are white; padding bytes stay 0xAB.
	if buf[0] != 255 || buf[3] != 255 {
		t.Fatalf("pixel 0 %v", buf[:4])
	}
	if buf[w*4] != 0xAB || buf[stride-1] != 0xAB {
		t.Fatal("padding must be left alone")
	}
}

func TestWrapImageRejectsShortBuffer(t *testing.T) {
	if WrapImage(make([]byte, 3), 2, 2, 0) != nil {
		t.Fatal("short buffer")
	}
	if WrapImage(make([]byte, 16), 4, 2, 4) != nil {
		t.Fatal("stride too small")
	}
}

func TestDecorationStripPaintCycle(t *testing.T) {
	// WM-style: paint a titlebar + border into a surface buffer. Not a WM.
	const w, h = 80, 48
	buf := make([]byte, w*h*4)
	img := WrapImage(buf, w, h, w*4)
	ctx := NewContext(img)
	var dirty Damage
	ctx.SetDamage(&dirty)
	ctx.Clear(RGB(0.12, 0.13, 0.16))
	dirty.Reset()
	title := XYWH(0, 0, float32(w), 18)
	dirty.Add(title) // compositor: titlebar invalidated
	if !dirty.Overlaps(title) {
		t.Fatal("titlebar dirty")
	}
	if dirty.Overlaps(XYWH(8, 24, 20, 16)) {
		t.Fatal("client area should stay clean")
	}
	ctx.Save()
	ctx.ClipRect(title)
	ctx.DrawRect(title, Fill(RGB(0.22, 0.24, 0.28)))
	ctx.DrawRect(XYWH(0, 17, float32(w), 1), Fill(RGB(0.4, 0.45, 0.5)))
	ctx.Restore()
	r, _, _, a := img.PremulAt(10, 8)
	if a < 200 || r < 40 {
		t.Fatalf("titlebar r=%d a=%d", r, a)
	}
}
