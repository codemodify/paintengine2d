package paintengine2d

import "testing"

// A shape recorded every frame is cloned once while it stays in use, and
// dropped a few seconds after the UI stops drawing it.
func TestPathCacheReusesAcrossFramesAndForgets(t *testing.T) {
	c := NewPathCache()
	rect := func() *Path {
		p := NewPath()
		p.AddRect(XYWH(1, 2, 30, 40))
		return p
	}
	var first *Path
	for f := 0; f < 10; f++ {
		rec := NewRecorder(64, 64)
		rec.UsePathCache(c)
		rec.Fill(rect(), Identity(), Fill(RGB(1, 0, 0)), Clip{})
		scene := rec.Finish()
		if scene == nil {
			t.Fatal("no scene")
		}
		got := c.intern(rect())
		if first == nil {
			first = got
		} else if got != first {
			t.Fatalf("frame %d: the same rect was cloned again", f)
		}
		c.EndFrame()
	}
	if c.Len() != 1 {
		t.Fatalf("cache holds %d paths, want 1", c.Len())
	}
	for f := 0; f < pathCacheKeep+2*pathCacheSweep; f++ {
		c.EndFrame()
	}
	if c.Len() != 0 {
		t.Fatalf("unused path survived %d frames (cache %d)", pathCacheKeep+2*pathCacheSweep, c.Len())
	}
}

// The cache hands out snapshots: changing the caller's path afterwards must
// not change what was recorded.
func TestPathCacheSnapshotsAreImmutable(t *testing.T) {
	c := NewPathCache()
	p := NewPath()
	p.AddRect(XYWH(0, 0, 10, 10))
	snap := c.intern(p)
	p.Reset()
	p.AddRect(XYWH(5, 5, 1, 1))
	if b := snap.Bounds(); b != XYWH(0, 0, 10, 10) {
		t.Fatalf("snapshot changed with its source: %v", b)
	}
}

// Many rect sub-paths whose pixels never touch fill like separate rects;
// touching ones take the scanline path (a shared edge pixel must not be
// blended twice).
func TestMultiRectFastPathMatchesSeparateRects(t *testing.T) {
	col := RGBA(0.2, 0.4, 0.8, 0.5)
	want := NewImage(40, 40)
	wd := NewCPUDevice(want)
	got := NewImage(40, 40)
	gd := NewCPUDevice(got)
	p := NewPath()
	for y := float32(0); y < 40; y += 4 {
		r := XYWH(3, y+0.5, 30, 1.5)
		one := NewPath()
		one.AddRect(r)
		wd.Fill(one, Identity(), Fill(col), Clip{})
		p.AddRect(r)
	}
	gd.Fill(p, Identity(), Fill(col), Clip{})
	for i := range want.Pix {
		if want.Pix[i] != got.Pix[i] {
			t.Fatalf("multi-rect fill differs from separate rects at byte %d", i)
		}
	}
	// Two rects sharing the pixel column at x=10: the union must be as
	// opaque as one rect there (no double blend, no seam).
	img := NewImage(20, 4)
	d := NewCPUDevice(img)
	q := NewPath()
	q.AddRect(XYWH(0, 0, 10.5, 4))
	q.AddRect(XYWH(10.5, 0, 9.5, 4))
	d.Fill(q, Identity(), Fill(RGB(0, 0, 0)), Clip{})
	if a := img.Pix[img.pixIndex(10, 1)+3]; a < 250 {
		t.Fatalf("touching rects left a seam: alpha %d at the shared pixel", a)
	}
}
