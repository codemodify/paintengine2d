package paintengine2d

import "testing"

func TestRectCanonAndIntersect(t *testing.T) {
	r := XYWH(10, 8, -4, 2)
	if r.Min.X != 6 || r.Max.X != 10 || r.Dy() != 2 {
		t.Fatalf("canon: %+v", r)
	}
	s := r.Intersect(XYWH(8, 7, 10, 10))
	if s.Min.X != 8 || s.Max.X != 10 || s.Min.Y != 8 || s.Max.Y != 10 {
		t.Fatalf("intersect: %+v", s)
	}
	if !XYWH(0, 0, 1, 1).Overlaps(XYWH(0.5, 0.5, 1, 1)) {
		t.Fatal("expected overlap")
	}
	if XYWH(0, 0, 1, 1).Overlaps(XYWH(1, 0, 1, 1)) {
		t.Fatal("half-open edges should not overlap")
	}
}

func TestPointOps(t *testing.T) {
	p := Pt(3, 4)
	if p.Len() != 5 {
		t.Fatalf("len %v", p.Len())
	}
	n := Pt(10, 0).Normalize()
	if n != Pt(1, 0) {
		t.Fatalf("norm %+v", n)
	}
	if Pt(1, 0).Perp() != Pt(0, 1) {
		t.Fatal("perp")
	}
}

func TestMatrixRoundTrip(t *testing.T) {
	m := Translation(10, 20).Mul(Rotation(0.4)).Mul(Scaling(2, 3))
	inv, ok := m.Invert()
	if !ok {
		t.Fatal("singular")
	}
	p := Pt(5, -3)
	got := inv.Transform(m.Transform(p))
	if !got.Near(p, 1e-4) {
		t.Fatalf("roundtrip %+v", got)
	}
	id := Identity()
	if !id.IsIdentity() || !id.IsTranslation() || !id.IsAxisAligned() {
		t.Fatal("identity flags")
	}
}

func TestRectUnionInsetCenter(t *testing.T) {
	r := XYWH(0, 0, 10, 8).Union(XYWH(8, 6, 10, 8))
	if r.Min.X != 0 || r.Max.X != 18 || r.Max.Y != 14 {
		t.Fatalf("union %+v", r)
	}
	s := XYWH(0, 0, 10, 10).Inset(2)
	if s.Dx() != 6 || s.Center() != Pt(5, 5) {
		t.Fatalf("inset/center %+v %+v", s, s.Center())
	}
	if !XYWH(0, 0, 0, 0).Empty() {
		t.Fatal("empty")
	}
}

func TestColorPremul(t *testing.T) {
	c := RGBA(1, 0, 0, 0.5)
	r, g, b, a := c.Premul8()
	if a != 128 || r != 128 || g != 0 || b != 0 {
		t.Fatalf("premul %d %d %d %d", r, g, b, a)
	}
	back := FromPremul8(r, g, b, a)
	if abs32(back.R-1) > 0.02 || abs32(back.A-0.5) > 0.02 {
		t.Fatalf("unpremul %+v", back)
	}
}
