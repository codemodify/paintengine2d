package paintengine2d

import "testing"

func TestImageEpochBumpsOnMutate(t *testing.T) {
	im := NewImage(4, 4)
	if im.Epoch != 0 {
		t.Fatalf("new image epoch %d", im.Epoch)
	}
	im.Clear(White)
	if im.Epoch == 0 {
		t.Fatal("Clear should bump epoch")
	}
	e := im.Epoch
	im.SetColor(1, 1, Black)
	if im.Epoch == e {
		t.Fatal("SetColor should bump epoch")
	}
	e = im.Epoch
	im.Bump()
	if im.Epoch != e+1 {
		t.Fatalf("Bump epoch %d want %d", im.Epoch, e+1)
	}
}

func TestFontAtlasEpoch(t *testing.T) {
	a := NewBitmapAtlas(White)
	if a.Epoch() == 0 {
		t.Fatal("baked atlas should have a non-zero image epoch")
	}
	a.Image.Touch()
	if a.Epoch() == 0 {
		t.Fatal("Touch should advance atlas epoch")
	}
}

func TestTessCacheHitsRepeatFill(t *testing.T) {
	c := newTessCache()
	p := CirclePath(Pt(32, 32), 16)
	xf := Identity()
	e1 := c.lookupFill(p, xf, FillNonZero)
	if e1 == nil || len(e1.verts) < 12 {
		t.Fatalf("first fill entry %+v", e1)
	}
	if c.flattens != 1 || c.misses != 1 || c.hits != 0 {
		t.Fatalf("cold stats flatten=%d miss=%d hit=%d", c.flattens, c.misses, c.hits)
	}
	e2 := c.lookupFill(p, xf, FillNonZero)
	if c.flattens != 1 || c.hits != 1 {
		t.Fatalf("repeat should hit cache flatten=%d hit=%d", c.flattens, c.hits)
	}
	if len(e2.verts) != len(e1.verts) {
		t.Fatalf("cached verts %d vs %d", len(e2.verts), len(e1.verts))
	}
	// Same geometry, different Path pointer — content hash must still hit.
	p2 := CirclePath(Pt(32, 32), 16)
	_ = c.lookupFill(p2, xf, FillNonZero)
	if c.flattens != 1 || c.hits != 2 {
		t.Fatalf("content hash miss flatten=%d hit=%d", c.flattens, c.hits)
	}
	moved := CirclePath(Pt(40, 32), 16)
	_ = c.lookupFill(moved, xf, FillNonZero)
	if c.flattens != 2 {
		t.Fatalf("moved path should flatten, flatten=%d", c.flattens)
	}
}

func TestTessCacheHitsRepeatStroke(t *testing.T) {
	c := newTessCache()
	p := RoundRectPath(XYWH(8, 8, 40, 24), 6, 6)
	st := Stroke{Width: 2, Cap: CapRound, Join: JoinRound, MiterLimit: 4}
	xf := Identity()
	e1 := c.lookupStroke(p, xf, st)
	if e1 == nil || len(e1.verts) < 12 {
		t.Fatalf("stroke entry %+v", e1)
	}
	_ = c.lookupStroke(p, xf, st)
	if c.flattens != 1 || c.hits != 1 {
		t.Fatalf("stroke cache flatten=%d hit=%d", c.flattens, c.hits)
	}
	st.Width = 4
	_ = c.lookupStroke(p, xf, st)
	if c.flattens != 2 {
		t.Fatalf("wider stroke should miss, flatten=%d", c.flattens)
	}
}

func TestHashPathStable(t *testing.T) {
	a := RectPath(XYWH(1, 2, 3, 4))
	b := RectPath(XYWH(1, 2, 3, 4))
	if hashPath(a) != hashPath(b) {
		t.Fatal("identical rects should hash equal")
	}
	c := RectPath(XYWH(1, 2, 3, 5))
	if hashPath(a) == hashPath(c) {
		t.Fatal("different rects should hash different")
	}
}
