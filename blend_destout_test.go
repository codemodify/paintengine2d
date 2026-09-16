package paintengine2d

import "testing"

// Dest-out is the eraser a translucent window frame needs: the rounded
// corners of an opaque window are punched out of the painted surface.

func TestDestOutPunchesOpaquePixels(t *testing.T) {
	img := NewImage(16, 16)
	ctx := NewContext(img)
	ctx.Clear(Black)
	ctx.DrawRect(XYWH(4, 4, 8, 8), Paint{Color: White, Blend: BlendDestOut})
	if _, _, _, a := rgbAt(img, 8, 8); a != 0 {
		t.Fatalf("punched pixel a=%d, want 0", a)
	}
	if _, _, _, a := rgbAt(img, 1, 1); a != 255 {
		t.Fatalf("pixel outside the punch a=%d, want 255", a)
	}
}

func TestDestOutHalfAlphaHalvesDestination(t *testing.T) {
	img := NewImage(8, 8)
	ctx := NewContext(img)
	ctx.Clear(White)
	ctx.DrawRect(XYWH(0, 0, 8, 8), Paint{Color: RGBA(0, 0, 0, 0.5), Blend: BlendDestOut})
	r, g, b, a := rgbAt(img, 4, 4)
	if a < 120 || a > 136 {
		t.Fatalf("half-erased alpha=%d, want ~128", a)
	}
	// Premultiplied: the colour is scaled with the alpha, staying white.
	if r != a || g != a || b != a {
		t.Fatalf("premultiplied rgba=%d,%d,%d,%d", r, g, b, a)
	}
}

func TestDestOutRoundedCornerKeepsAntiAliasing(t *testing.T) {
	img := NewImage(32, 32)
	ctx := NewContext(img)
	ctx.Clear(Black)
	// Punch everything outside a rounded rect out of the top-left corner.
	ctx.Save()
	ctx.ClipRect(XYWH(0, 0, 12, 12))
	p := NewPath()
	p.AddRect(XYWH(0, 0, 12, 12))
	p.AddRoundRect(XYWH(0, 0, 32, 32), 10, 10)
	ctx.DrawPath(p, Paint{Color: White, Blend: BlendDestOut, FillRule: FillEvenOdd, AntiAlias: true})
	ctx.Restore()
	if _, _, _, a := rgbAt(img, 0, 0); a != 0 {
		t.Fatalf("the corner outside the curve a=%d, want 0", a)
	}
	if _, _, _, a := rgbAt(img, 11, 11); a != 255 {
		t.Fatalf("inside the curve a=%d, want 255", a)
	}
	if _, _, _, a := rgbAt(img, 20, 20); a != 255 {
		t.Fatalf("away from the corner a=%d, want 255", a)
	}
	// The curve itself is anti-aliased: some pixel on it is partly erased.
	partial := false
	for y := 0; y < 12; y++ {
		for x := 0; x < 12; x++ {
			if _, _, _, a := rgbAt(img, x, y); a > 0 && a < 255 {
				partial = true
			}
		}
	}
	if !partial {
		t.Fatal("no anti-aliased pixel along the punched curve")
	}
}

func TestDestOutInScene(t *testing.T) {
	rec := NewRecorder(16, 16)
	rec.Clear(Black)
	ctx := NewContextDevice(rec)
	ctx.DrawRect(XYWH(0, 0, 8, 8), Paint{Color: White, Blend: BlendDestOut})
	scene := rec.Finish()
	img := NewImage(16, 16)
	DrawScene(scene, NewCPUDevice(img))
	if _, _, _, a := rgbAt(img, 4, 4); a != 0 {
		t.Fatalf("replayed punch a=%d, want 0", a)
	}
	if _, _, _, a := rgbAt(img, 12, 12); a != 255 {
		t.Fatalf("replayed keep a=%d, want 255", a)
	}
	// A damage replay of the punched box repaints it the same way.
	img2 := NewImage(16, 16)
	DrawSceneRects(scene, NewCPUDevice(img2), []Rect{XYWH(0, 0, 16, 16)})
	if _, _, _, a := rgbAt(img2, 4, 4); a != 0 {
		t.Fatalf("damage replay punch a=%d, want 0", a)
	}
}

func TestGPUDestOut(t *testing.T) {
	if !GPUAvailable() {
		t.Skip("no EGL/GLES")
	}
	dev, err := NewGPUDevice(32, 32)
	if err != nil {
		t.Fatal(err)
	}
	defer dev.Close()
	ctx := NewContextDevice(dev)
	ctx.Clear(Black)
	ctx.DrawRect(XYWH(8, 8, 16, 16), Paint{Color: White, Blend: BlendDestOut})
	img := ctx.Image()
	if img == nil {
		t.Fatal("gpu snapshot nil")
	}
	if _, _, _, a := img.PremulAt(16, 16); a > 8 {
		t.Fatalf("punched pixel a=%d, want 0", a)
	}
	if _, _, _, a := img.PremulAt(2, 2); a < 248 {
		t.Fatalf("pixel outside the punch a=%d, want 255", a)
	}
	// The next draw is src-over again.
	ctx.DrawRect(XYWH(8, 8, 4, 4), Fill(White))
	img = ctx.Image()
	if r, _, _, a := img.PremulAt(10, 10); a < 248 || r < 248 {
		t.Fatalf("src-over after dest-out rgba=%d,_,_,%d", r, a)
	}
}
