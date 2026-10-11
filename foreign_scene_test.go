package paintengine2d

import "testing"

// **A foreign texture survives retained painting.**
//
// Recording and replaying is the toolkit's default path — a widget is
// handed a context over a recorder, not over the window's device — so a
// foreign draw that only worked immediately was a feature unreachable
// from the one place it was built for. Reported against
// media-player-video: the same red texture drew immediately and produced
// nothing recorded, and SupportsForeign said false for the context the
// widget actually had.
func TestAForeignTextureSurvivesTheRecorder(t *testing.T) {
	if !GPUAvailable() {
		t.Skip("no EGL/GLES")
	}
	const w, h = 64, 48
	dev, err := NewGPUDevice(w, h)
	if err != nil {
		t.Fatal(err)
	}
	defer dev.Close()

	// A red foreign texture, made on the device that will draw it.
	tex, err := NewForeignTarget(dev, 16, 16)
	if err != nil {
		t.Fatalf("the GPU device cannot make a foreign target: %v", err)
	}
	if !dev.ClearForeign(tex, RGB(1, 0, 0)) {
		t.Fatal("clearing the foreign target failed")
	}

	// A recorder is what retained painting hands a widget.
	rec := NewRecorder(w, h)
	if !SupportsForeign(rec) {
		t.Fatal("a recorder says it cannot take a foreign texture, so a widget would never try")
	}
	rc := NewContextDevice(rec)
	rc.DrawRect(XYWH(0, 0, w, h), Fill(RGB(0, 0, 0)))
	if !rc.DrawForeign(tex, Rect{}, XYWH(8, 8, 32, 24)) {
		t.Fatal("the recorder refused the foreign draw")
	}

	// Replayed onto the GPU device, the draw happens.
	DrawScene(rec.Finish(), dev)
	img := dev.Snapshot()
	if img == nil {
		t.Fatal("no snapshot")
	}
	at := func(x, y int) (uint8, uint8, uint8) {
		i := (y*img.Width + x) * 4
		return img.Pix[i], img.Pix[i+1], img.Pix[i+2]
	}
	if r, g, b := at(20, 20); r < 200 || g > 60 || b > 60 {
		t.Errorf("inside the blit the pixel is (%d,%d,%d), want red", r, g, b)
	}
	if r, g, b := at(2, 2); r > 60 || g > 60 || b > 60 {
		t.Errorf("outside the blit the pixel is (%d,%d,%d), want the black ground", r, g, b)
	}
}

// A recorder cannot make a foreign texture: that needs a GL context, and
// the two halves are separate capabilities for exactly this reason.
func TestARecorderCannotMakeAForeignTarget(t *testing.T) {
	rec := NewRecorder(32, 32)
	if _, err := NewForeignTarget(rec, 8, 8); err != ErrForeignUnsupported {
		t.Errorf("a recorder made a foreign target, or failed differently: %v", err)
	}
}

// A released texture is not drawn when the scene is replayed later: a
// scene outlives the frame that recorded it, and the GL name behind the
// texture can be gone by then.
//
// The replay is onto a counting device rather than a real one, because a
// real GPU device refuses a dead texture itself — so against it this
// test passes whether or not the scene checks anything, which is what
// the first version of it did.
func TestAReleasedForeignTextureIsNotReplayed(t *testing.T) {
	live := &ForeignTexture{Texture: 7, Width: 8, Height: 8}
	rec := NewRecorder(48, 32)
	NewContextDevice(rec).DrawForeign(live, Rect{}, XYWH(4, 4, 24, 16))
	scene := rec.Finish()

	// Alive: the device is asked to draw it.
	var dev countingForeign
	DrawScene(scene, &dev)
	if dev.blits != 1 {
		t.Fatalf("a live texture was blitted %d times, want 1", dev.blits)
	}

	// Dead: it is not. The texture is the same object the scene holds,
	// so this is the case a frame later.
	live.Texture = 0
	dev.blits = 0
	DrawScene(scene, &dev)
	if dev.blits != 0 {
		t.Errorf("a released texture was blitted %d times, want none", dev.blits)
	}
}

// countingForeign is a Device that can take a foreign texture and does
// nothing but count.
type countingForeign struct {
	CPUDevice
	blits int
}

func (d *countingForeign) BlitForeign(*ForeignTexture, Rect, Rect, Matrix, Paint, Clip) {
	d.blits++
}
