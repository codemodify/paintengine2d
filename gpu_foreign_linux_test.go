//go:build linux && cgo

package paintengine2d

import "testing"

// A picture this engine did not draw.
//
// Blit takes an Image, which is CPU memory by definition, so a video frame
// cost three copies and a full texture upload every frame: the decoder
// rasterized into its own buffer, the application copied that into Go memory,
// the engine copied it into a texture. At 1280x720 and 60 frames a second that
// is about 210 MB/s of upload, and it is why a player caps its render size.
//
// A foreign texture is the other way round — the caller renders into a texture
// the engine blends — so these tests are about the handle's lifetime and about
// the drawing being an *ordinary* blit: the same clip, transform and tint as
// everything around it, because a video frame that composited differently from
// the interface over it would be a second compositor.

func TestAForeignTargetIsDrawnLikeAnyOtherPicture(t *testing.T) {
	d := gpuDev(t, 64, 64)
	defer d.Close()
	if !SupportsForeign(d) {
		t.Fatal("a GPU device does not support a foreign texture")
	}

	ft, err := NewForeignTarget(d, 16, 16)
	if err != nil {
		t.Fatalf("NewForeignTarget: %v", err)
	}
	defer d.ReleaseForeign(ft)
	if !ft.Valid() || ft.Texture == 0 || ft.FBO == 0 {
		t.Fatalf("target %+v", ft)
	}

	// The caller's own content in the target.
	if !d.ClearForeign(ft, Red) {
		t.Fatal("ClearForeign refused a fresh target")
	}

	// Drawn through the ordinary canvas, with the clip the canvas has.
	d.Clear(Transparent)
	ctx := NewContextDevice(d)
	ctx.Save()
	ctx.ClipRect(XYWH(0, 0, 32, 64))
	if !ctx.DrawForeign(ft, Rect{}, XYWH(0, 0, 64, 64)) {
		t.Fatal("DrawForeign refused a valid target on a GPU device")
	}
	ctx.Restore()

	img := d.Snapshot()
	if img == nil {
		t.Fatal("no snapshot")
	}
	if r, g, b, _ := img.PremulAt(8, 32); r < 200 || g > 60 || b > 60 {
		t.Errorf("inside the clip the frame is {%d %d %d}, want red", r, g, b)
	}
	// Nothing outside the clip: a foreign texture obeys it like any blit.
	if _, _, _, a := img.PremulAt(48, 32); a > 8 {
		t.Errorf("outside the clip alpha is %d, want nothing drawn", a)
	}
}

// The tint and the global alpha reach it too, so a frame can be faded with the
// panel it sits in.
func TestAForeignTextureTakesTheCanvasAlpha(t *testing.T) {
	d := gpuDev(t, 32, 32)
	defer d.Close()
	ft, err := NewForeignTarget(d, 8, 8)
	if err != nil {
		t.Fatal(err)
	}
	defer d.ReleaseForeign(ft)
	d.ClearForeign(ft, White)

	d.Clear(Transparent)
	ctx := NewContextDevice(d)
	ctx.SetAlpha(0.5)
	if !ctx.DrawForeign(ft, Rect{}, XYWH(0, 0, 32, 32)) {
		t.Fatal("DrawForeign refused")
	}
	_, _, _, a := d.Snapshot().PremulAt(16, 16)
	if a < 100 || a > 155 {
		t.Errorf("a half-transparent canvas drew alpha %d, want about 128", a)
	}
}

// The CPU device cannot, and says so before anybody builds a renderer for it.
func TestTheCPUDeviceDoesNotSupportForeignTextures(t *testing.T) {
	cpu := NewCPUDevice(NewImage(16, 16))
	if SupportsForeign(cpu) {
		t.Error("the CPU device claims it can draw a foreign texture")
	}
	if _, err := NewForeignTarget(cpu, 8, 8); err == nil {
		t.Error("the CPU device made a foreign target")
	}
	ctx := NewContextDevice(cpu)
	if ctx.DrawForeign(&ForeignTexture{Texture: 1, Width: 8, Height: 8}, Rect{}, XYWH(0, 0, 8, 8)) {
		t.Error("a CPU canvas claimed to draw a foreign texture")
	}
}

// A target does not outlive its device, and says so rather than letting a blit
// sample whatever got its texture name next.
func TestAForeignTargetDiesWithItsDevice(t *testing.T) {
	d := gpuDev(t, 32, 32)
	ft, err := NewForeignTarget(d, 8, 8)
	if err != nil {
		t.Fatalf("NewForeignTarget: %v", err)
	}
	if !ft.Valid() {
		t.Fatal("a fresh target is not valid")
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if ft.Valid() {
		t.Error("a target outlived the device that owns its GL names")
	}
	if ft.Texture != 0 || ft.FBO != 0 {
		t.Errorf("a dead target still names texture %d fbo %d", ft.Texture, ft.FBO)
	}
	other := gpuDev(t, 32, 32)
	defer other.Close()
	ctx := NewContextDevice(other)
	if ctx.DrawForeign(ft, Rect{}, XYWH(0, 0, 32, 32)) {
		t.Error("a dead target was drawn")
	}
	if other.ClearForeign(ft, Red) {
		t.Error("a dead target was cleared")
	}
}

// Releasing one gives up its names and leaves the rest alone.
func TestReleasingOneForeignTargetKeepsTheOthers(t *testing.T) {
	d := gpuDev(t, 32, 32)
	defer d.Close()
	a, err := NewForeignTarget(d, 8, 8)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewForeignTarget(d, 8, 8)
	if err != nil {
		t.Fatal(err)
	}
	d.ReleaseForeign(a)
	if a.Valid() {
		t.Error("a released target is still valid")
	}
	if !b.Valid() {
		t.Error("releasing one target invalidated another")
	}
	d.ClearForeign(b, Green)
	d.Clear(Transparent)
	ctx := NewContextDevice(d)
	if !ctx.DrawForeign(b, Rect{}, XYWH(0, 0, 32, 32)) {
		t.Fatal("the surviving target did not draw")
	}
	if r, g, bl, _ := d.Snapshot().PremulAt(16, 16); g < 200 || r > 60 || bl > 60 {
		t.Errorf("the surviving target drew {%d %d %d}, want green", r, g, bl)
	}
}

// An adopted texture is the caller's: the engine draws it and does not delete
// it, and has no framebuffer of its own for it.
func TestAnAdoptedTextureIsDrawnAndNotOwned(t *testing.T) {
	d := gpuDev(t, 32, 32)
	defer d.Close()
	made, err := NewForeignTarget(d, 8, 8)
	if err != nil {
		t.Fatal(err)
	}
	d.ClearForeign(made, Blue)

	adopted, err := d.AdoptTexture(made.Texture, 8, 8)
	if err != nil {
		t.Fatalf("AdoptTexture: %v", err)
	}
	if adopted.FBO != 0 {
		t.Error("an adopted texture was given a framebuffer")
	}
	if d.ClearForeign(adopted, Red) {
		t.Error("an adopted texture was cleared through a framebuffer it has not got")
	}
	d.Clear(Transparent)
	ctx := NewContextDevice(d)
	if !ctx.DrawForeign(adopted, Rect{}, XYWH(0, 0, 32, 32)) {
		t.Fatal("an adopted texture did not draw")
	}
	if r, g, b, _ := d.Snapshot().PremulAt(16, 16); b < 200 || r > 60 || g > 60 {
		t.Errorf("the adopted texture drew {%d %d %d}, want blue", r, g, b)
	}
	// Releasing the adoption leaves the texture it borrowed alone — asked of
	// the pixels, because Valid is only this side's bookkeeping and a deleted
	// GL name would still look valid from here.
	d.ReleaseForeign(adopted)
	if !made.Valid() {
		t.Error("releasing an adoption killed the texture it borrowed")
	}
	d.Clear(Transparent)
	if !ctx.DrawForeign(made, Rect{}, XYWH(0, 0, 32, 32)) {
		t.Fatal("the real target stopped drawing when its adoption was released")
	}
	if r, g, b, a := d.Snapshot().PremulAt(16, 16); b < 200 || r > 60 || g > 60 || a < 200 {
		t.Errorf("after releasing its adoption the target draws {%d %d %d %d}, want blue — "+
			"the borrowed texture was deleted with the adoption", r, g, b, a)
	}
}

// Flipped turns a bottom-up texture the right way up. A uniform colour cannot
// show that, so this reads the quad the blit built: the V coordinates swap and
// nothing else does.
func TestFlippedSwapsTheTexturesVerticalCoordinates(t *testing.T) {
	d := gpuDev(t, 32, 32)
	defer d.Close()
	ft, err := NewForeignTarget(d, 8, 8)
	if err != nil {
		t.Fatal(err)
	}
	defer d.ReleaseForeign(ft)
	ctx := NewContextDevice(d)

	quadOf := func(flip bool) []float32 {
		ft.Flipped = flip
		d.Clear(Transparent)
		if !ctx.DrawForeign(ft, XYWH(0, 0, 8, 4), XYWH(0, 0, 32, 32)) {
			t.Fatal("DrawForeign refused")
		}
		return append([]float32(nil), d.quad...)
	}
	up, down := quadOf(false), quadOf(true)
	if len(up) != 24 || len(down) != 24 {
		t.Fatalf("quads are %d and %d floats, want 24", len(up), len(down))
	}
	for i := 0; i < 24; i += 4 {
		// x, y and u are the same; v is the one that moves.
		for j := 0; j < 3; j++ {
			if up[i+j] != down[i+j] {
				t.Errorf("vertex %d component %d changed: %v vs %v", i/4, j, up[i+j], down[i+j])
			}
		}
	}
	// The source box was the texture's top half, so unflipped V runs 0..0.5
	// and flipped it runs 0.5..0.
	if up[3] != 0 || down[3] != 0.5 {
		t.Errorf("first vertex V is %v unflipped and %v flipped, want 0 and 0.5", up[3], down[3])
	}
}
