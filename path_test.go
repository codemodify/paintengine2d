package paintengine2d

import "testing"

func TestPathRectAndClose(t *testing.T) {
	p := RectPath(XYWH(0, 0, 10, 6))
	if p.Empty() {
		t.Fatal("empty")
	}
	b := p.Bounds()
	if b.Dx() != 10 || b.Dy() != 6 {
		t.Fatalf("bounds %+v", b)
	}
	hasClose := false
	for _, v := range p.Verbs() {
		if v == VerbClose {
			hasClose = true
		}
	}
	if !hasClose {
		t.Fatal("expected close")
	}
}

func TestPathCloneIndependent(t *testing.T) {
	p := NewPath()
	p.MoveTo(0, 0)
	p.LineTo(1, 1)
	q := p.Clone()
	p.LineTo(2, 2)
	if len(q.Points()) != 2 {
		t.Fatalf("clone shared storage: %d", len(q.Points()))
	}
}

func TestPathHelpers(t *testing.T) {
	c := CirclePath(Pt(0, 0), 10)
	if c.Bounds().Dx() < 19 || c.Bounds().Dy() < 19 {
		t.Fatalf("circle bounds %+v", c.Bounds())
	}
	rr := RoundRectPath(XYWH(0, 0, 40, 20), 6, 6)
	if rr.Empty() {
		t.Fatal("round rect")
	}
	e := EllipsePath(Pt(5, 5), 8, 3)
	if e.Bounds().Dx() < 15 {
		t.Fatalf("ellipse bounds %+v", e.Bounds())
	}
}

func TestLineToWithoutMove(t *testing.T) {
	p := NewPath()
	p.LineTo(3, 4)
	if got := p.Points()[0]; got != Pt(3, 4) {
		t.Fatalf("implicit move %+v", got)
	}
}
