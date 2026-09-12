package paintengine2d

import "testing"

func TestFillRectAASmallOnLargeMatchesSmallCanvas(t *testing.T) {
	p := Fill(RGBA(0.23, 0.51, 0.93, 0.4))
	r := XYWH(24.5, 16.25, 80, 18)
	large := NewImage(1920, 1080)
	small := NewImage(160, 48)
	NewContext(large).DrawRect(r, p)
	NewContext(small).DrawRect(r, p)
	for y := 16; y < 36; y++ {
		for x := 24; x < 106; x++ {
			lr, lg, lb, la := large.PremulAt(x, y)
			sr, sg, sb, sa := small.PremulAt(x, y)
			if abs8(lr, sr) > 1 || abs8(lg, sg) > 1 || abs8(lb, sb) > 1 || abs8(la, sa) > 1 {
				t.Fatalf("pixel %d,%d large=%d,%d,%d,%d small=%d,%d,%d,%d",
					x, y, lr, lg, lb, la, sr, sg, sb, sa)
			}
		}
	}
	_, _, _, a := large.PremulAt(8, 8)
	if a != 0 {
		t.Fatalf("outside hover row should stay clear, a=%d", a)
	}
}

func abs8(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}

func TestClearRectLeavesOutsideUntouched(t *testing.T) {
	img := NewImage(64, 48)
	ctx := NewContext(img)
	ctx.Clear(RGB(0.10, 0.11, 0.14))
	br, bg, bb, ba := img.PremulAt(2, 2)
	ctx.ClearRect(XYWH(8, 10, 24, 12), RGB(0.9, 0.2, 0.2))
	r, g, b, a := img.PremulAt(12, 14)
	if a < 250 || r < 200 {
		t.Fatalf("cleared interior rgba=%d,%d,%d,%d", r, g, b, a)
	}
	r, g, b, a = img.PremulAt(2, 2)
	if r != br || g != bg || b != bb || a != ba {
		t.Fatalf("outside ClearRect clobbered %d,%d,%d,%d want %d,%d,%d,%d", r, g, b, a, br, bg, bb, ba)
	}
}

func TestImageClearRectPadUntouched(t *testing.T) {
	const w, h, pad = 8, 6, 12
	stride := w*4 + pad
	buf := make([]byte, h*stride)
	for i := range buf {
		buf[i] = 0x5A
	}
	im := WrapImage(buf, w, h, stride)
	im.ClearRect(XYWH(1, 1, 3, 2), White)
	for y := 0; y < h; y++ {
		for i := w * 4; i < stride; i++ {
			if buf[y*stride+i] != 0x5A {
				t.Fatalf("padding clobbered y=%d i=%d", y, i)
			}
		}
	}
	_, _, _, a := im.PremulAt(2, 2)
	if a < 250 {
		t.Fatalf("cleared pixel a=%d", a)
	}
	_, _, _, a = im.PremulAt(0, 0)
	if a != 0x5A {
		t.Fatalf("outside pixel should stay 0x5A, a=%d", a)
	}
}

func TestClipDeviceRectScissorsFill(t *testing.T) {
	img := NewImage(80, 60)
	ctx := NewContext(img)
	ctx.ClipDeviceRect(XYWH(10, 20, 16, 8))
	ctx.SetColor(White)
	ctx.FillRect(XYWH(0, 0, 80, 60))
	assertAlpha(t, img, 12, 22, 250, 255, "inside dirty scissor")
	assertAlpha(t, img, 2, 2, 0, 0, "outside dirty scissor")
	assertAlpha(t, img, 40, 22, 0, 0, "right of dirty scissor")
}

func TestClipToDamageEmptyRejects(t *testing.T) {
	img := NewImage(40, 40)
	ctx := NewContext(img)
	var d Damage
	ctx.SetDamage(&d)
	ctx.ClipToDamage()
	ctx.SetColor(White)
	ctx.FillRect(XYWH(0, 0, 40, 40))
	if countOpaque(img, 1) != 0 {
		t.Fatal("empty damage clip should reject")
	}
}

func TestClipToDamageMenuRow(t *testing.T) {
	img := NewImage(200, 160)
	ctx := NewContext(img)
	ctx.Clear(RGB(0.10, 0.11, 0.14))
	var d Damage
	ctx.SetDamage(&d)
	d.Reset()
	row := XYWH(8, 40, 180, 22)
	d.Add(row)
	ctx.Save()
	ctx.ClipToDamage()
	ctx.SetColor(RGB(0.23, 0.51, 0.93))
	ctx.FillRect(XYWH(0, 0, 200, 160))
	ctx.Restore()
	assertAlpha(t, img, 20, 48, 200, 255, "dirty row")
	r, g, b, _ := rgbAt(img, 20, 10)
	if r > 80 || g > 80 || b > 80 {
		t.Fatalf("clean chrome should stay background, got %d %d %d", r, g, b)
	}
}

func TestDrawPathQuickRejectsOutsideClip(t *testing.T) {
	img := NewImage(32, 32)
	ctx := NewContext(img)
	ctx.ClipRect(XYWH(0, 0, 8, 8))
	ctx.DrawCircle(Pt(28, 28), 4, Fill(White))
	if countOpaque(img, 1) != 0 {
		t.Fatal("rejected circle should not paint")
	}
}

func TestDrawLabelWarmZeroAllocs(t *testing.T) {
	img := NewImage(120, 20)
	ctx := NewContext(img)
	atlas := NewBitmapAtlas(White)
	p := Paint{Color: White, Filter: FilterNearest}
	ctx.DrawLabel("Open File", atlas, Pt(4, 4), p)
	n := testing.AllocsPerRun(50, func() {
		ctx.DrawLabel("Open File", atlas, Pt(4, 4), p)
	})
	if n != 0 {
		t.Fatalf("warm DrawLabel allocs/op = %.1f, want 0", n)
	}
}

func TestPresentCPUNoop(t *testing.T) {
	img := NewImage(8, 8)
	ctx := NewContext(img)
	if err := ctx.Present(); err != nil {
		t.Fatal(err)
	}
	var d Damage
	d.Add(XYWH(1, 1, 2, 2))
	ctx.SetDamage(&d)
	if err := ctx.PresentDamage(); err != nil {
		t.Fatal(err)
	}
	if err := NewCPUSurface(img).PresentRects(d.Rects); err != nil {
		t.Fatal(err)
	}
}

func TestGlyphsSkipOutsideClip(t *testing.T) {
	img := NewImage(80, 20)
	ctx := NewContext(img)
	atlas := NewBitmapAtlas(White)
	ctx.ClipRect(XYWH(0, 0, 4, 20))
	ctx.DrawLabel("HELLO", atlas, Pt(20, 4), Paint{Color: White, Filter: FilterNearest})
	if countOpaque(img, 1) != 0 {
		t.Fatal("clipped-away glyphs should not blit")
	}
}
