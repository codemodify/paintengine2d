//go:build linux && cgo

package paintengine2d

import "testing"

// The partial-present blit must repaint the damage of every frame the back
// buffer has not seen. With EGL_KHR_partial_update and a buffer age of N,
// that is this frame plus the previous N-1 — repainting only the current
// frame's rects is what left stale tiles on Wayland.
func TestPresentDamageAgeRing(t *testing.T) {
	d := &GPUDevice{}
	f1 := []Rect{XYWH(0, 0, 10, 10)}
	f2 := []Rect{XYWH(20, 0, 10, 10)}
	f3 := []Rect{XYWH(40, 0, 10, 10)}
	d.recordFrameDamage(f1)
	d.recordFrameDamage(f2)
	d.recordFrameDamage(f3)

	cur := []Rect{XYWH(60, 0, 10, 10)}
	if got, full := d.damageForAge(cur, 1); len(got) != 1 || full {
		t.Fatalf("age 1 repaints only this frame, got %d rects (full=%v)", len(got), full)
	}
	got, full := d.damageForAge(cur, 3)
	if full {
		t.Fatal("no full frame in the history, yet a full blit was requested")
	}
	if len(got) != 3 {
		t.Fatalf("age 3 should repaint 3 frames of damage, got %d: %+v", len(got), got)
	}
	var u Rect
	for _, r := range got {
		u = u.Union(r)
	}
	if u.Min.X != 20 || u.Max.X != 70 {
		t.Fatalf("age-3 damage union %+v; want the last two frames plus the current one", u)
	}
	// Never reaches past the ring.
	if got, _ := d.damageForAge(cur, maxBufferAge+5); len(got) > maxBufferAge {
		t.Fatalf("age beyond the ring returned %d frames", len(got))
	}
}

// A full-surface present (nil rects) must be remembered as "everything
// changed". It used to be recorded as no damage, so the next partial present
// into a 2-frame-old back buffer repaired only its own rects and the rest of
// the window reverted to the frame before the full repaint — the gallery's
// vanishing menus and reappearing dialogs on KDE Wayland.
func TestPresentDamageAgeRingFullFrame(t *testing.T) {
	d := &GPUDevice{}
	hover := []Rect{XYWH(0, 0, 50, 20)}
	d.recordFrameDamage(hover) // frame 1: partial (hover)
	d.recordFrameDamage(nil)   // frame 2: full (menu opened)

	cur := []Rect{XYWH(0, 0, 100, 30)} // frame 3: partial (title release)
	if got, full := d.damageForAge(cur, 1); full || len(got) != 1 {
		t.Fatalf("age 1 buffer is the full frame's own result: want only this frame's rects, got %d (full=%v)", len(got), full)
	}
	if _, full := d.damageForAge(cur, 2); !full {
		t.Fatal("age 2 buffer predates the full frame: a full blit is required")
	}
	if _, full := d.damageForAge(cur, 3); !full {
		t.Fatal("age 3 buffer predates the full frame: a full blit is required")
	}

	// The full frame ages out of reach after enough partial frames.
	for i := 0; i < maxBufferAge; i++ {
		d.recordFrameDamage(hover)
	}
	if _, full := d.damageForAge(cur, 3); full {
		t.Fatal("full frame fell out of the window but still forces a full blit")
	}

	// A reset (resize / new target) leaves no repairable history.
	d.resetDamageHistory()
	if _, full := d.damageForAge(cur, 2); !full {
		t.Fatal("after a reset every aged buffer needs a full blit")
	}
	d.recordFrameDamage(hover) // first frame after the reset counts as full
	if _, full := d.damageForAge(cur, 2); !full {
		t.Fatal("age 2 reaches back past the reset: still needs a full blit")
	}
	if _, full := d.damageForAge(cur, 1); full {
		t.Fatal("age 1 buffer is the previous frame's own result")
	}
	d.recordFrameDamage(hover)
	if _, full := d.damageForAge(cur, 2); full {
		t.Fatal("age 2 only spans a partial frame now")
	}
	if _, full := d.damageForAge(cur, 3); !full {
		t.Fatal("age 3 spans the first post-reset frame: needs a full blit")
	}
}

// The texture cache is keyed by Image.ID (not by address), bounded, and
// explicitly releasable.
func TestGPUTextureCacheBudgetAndRelease(t *testing.T) {
	d := gpuDev(t, 64, 64)
	defer d.Close()
	d.SetTextureBudget(256 << 10) // 256 KiB ≈ 4 of the images below

	var keep *Image
	for i := 0; i < 12; i++ {
		img := NewImage(128, 128) // 64 KiB each
		img.Clear(Red)
		if i == 0 {
			keep = img
		}
		d.Blit(img, XYWH(0, 0, 128, 128), XYWH(0, 0, 32, 32), Identity(), Paint{}, Clip{})
	}
	if d.TextureBytes() > 256<<10 {
		t.Fatalf("texture cache ignored its budget: %d bytes", d.TextureBytes())
	}
	if len(d.texCache) >= 12 {
		t.Fatalf("nothing was evicted: %d entries", len(d.texCache))
	}

	d.SetTextureBudget(0)
	atlas := NewImage(32, 32)
	atlas.Clear(Blue)
	d.Blit(atlas, XYWH(0, 0, 32, 32), XYWH(0, 0, 32, 32), Identity(), Paint{}, Clip{})
	if _, ok := d.texCache[atlas.UID()]; !ok {
		t.Fatal("atlas should be cached by image id")
	}
	before := d.TextureBytes()
	d.ReleaseImage(atlas)
	if _, ok := d.texCache[atlas.UID()]; ok {
		t.Fatal("ReleaseImage must drop the texture")
	}
	if d.TextureBytes() >= before {
		t.Fatalf("ReleaseImage did not free bytes: %d → %d", before, d.TextureBytes())
	}
	_ = keep
	if err := d.Err(); err != nil {
		t.Fatalf("device error: %v", err)
	}
}
