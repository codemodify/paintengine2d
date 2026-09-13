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
	if got := d.damageForAge(cur, 1); len(got) != 1 {
		t.Fatalf("age 1 repaints only this frame, got %d rects", len(got))
	}
	got := d.damageForAge(cur, 3)
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
	if n := len(d.damageForAge(cur, maxBufferAge+5)); n > maxBufferAge {
		t.Fatalf("age beyond the ring returned %d frames", n)
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
