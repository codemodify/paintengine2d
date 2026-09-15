package paintengine2d

import "testing"

// stripes paints 2px black and white columns over the whole surface.
func stripes(ctx *Context, w, h int) {
	ctx.Clear(White)
	p := NewPath()
	for x := 0; x < w; x += 4 {
		p.AddRect(XYWH(float32(x), 0, 2, float32(h)))
	}
	ctx.DrawPath(p, Fill(Black))
}

func TestBackdropBlurCPU(t *testing.T) {
	img := NewImage(80, 60)
	ctx := NewContext(img)
	stripes(ctx, 80, 60)
	ctx.BackdropBlur(XYWH(20, 10, 40, 40), 4)
	// Inside: the stripes average to grey.
	for _, x := range []int{30, 31, 32, 33} {
		if r, _, _, _ := img.PremulAt(x, 30); r < 100 || r > 160 {
			t.Fatalf("blurred stripe at x=%d: %d, want grey", x, r)
		}
	}
	// Outside: untouched.
	if r, _, _, _ := img.PremulAt(8, 30); r != 0 {
		t.Fatalf("outside the rect a black stripe changed: %d", r)
	}
	if r, _, _, _ := img.PremulAt(10, 30); r != 255 {
		t.Fatalf("outside the rect a white stripe changed: %d", r)
	}
}

// A rounded clip keeps the blur inside the rounded shape.
func TestBackdropBlurRespectsClipPath(t *testing.T) {
	img := NewImage(80, 60)
	ctx := NewContext(img)
	stripes(ctx, 80, 60)
	ctx.Save()
	ctx.ClipRoundRect(XYWH(20, 10, 40, 40), 12, 12)
	ctx.BackdropBlur(XYWH(20, 10, 40, 40), 4)
	ctx.Restore()
	if r, _, _, _ := img.PremulAt(20, 10); r != 0 {
		t.Fatalf("the rounded corner was blurred: %d", r)
	}
	if r, _, _, _ := img.PremulAt(40, 30); r < 100 || r > 160 {
		t.Fatalf("the centre should be grey: %d", r)
	}
}

// A recorded blur replays the same, and a partial repaint inside its
// region matches a full one: the damage widens to the whole region, so the
// blur never reads its own output.
func TestBackdropBlurScene(t *testing.T) {
	const w, h = 80, 60
	record := func() *Scene {
		rec := NewRecorder(w, h)
		ctx := NewContextDevice(rec)
		stripes(ctx, w, h)
		ctx.BackdropBlur(XYWH(20, 10, 40, 40), 4)
		ctx.DrawRect(XYWH(20, 10, 40, 40), Fill(RGBA(0, 0, 1, 0.3)))
		return rec.Finish()
	}
	direct := NewImage(w, h)
	dc := NewContext(direct)
	stripes(dc, w, h)
	dc.BackdropBlur(XYWH(20, 10, 40, 40), 4)
	dc.DrawRect(XYWH(20, 10, 40, 40), Fill(RGBA(0, 0, 1, 0.3)))

	full := NewImage(w, h)
	DrawScene(record(), NewCPUDevice(full))
	if !sameImage(full, direct) {
		t.Fatal("the replayed blur differs from the direct one")
	}
	// Repaint a small box inside the blur, over the finished frame.
	part := full.Clone()
	d := Damage{Rects: []Rect{XYWH(36, 26, 6, 6)}}
	DrawSceneDamage(record(), NewCPUDevice(part), &d)
	if !sameImage(part, full) {
		t.Fatal("a partial repaint inside the blur drifted from the full frame")
	}
	if len(d.Rects) != 1 || d.Rects[0].Dx() < 40 {
		t.Fatalf("the damage should have widened over the blur region, got %v", d.Rects)
	}
}

func sameImage(a, b *Image) bool {
	if a.Width != b.Width || a.Height != b.Height {
		return false
	}
	for y := 0; y < a.Height; y++ {
		for x := 0; x < a.Width; x++ {
			ar, ag, ab, aa := a.PremulAt(x, y)
			br, bg, bb, ba := b.PremulAt(x, y)
			if ar != br || ag != bg || ab != bb || aa != ba {
				return false
			}
		}
	}
	return true
}
