package paintengine2d

import (
	"math"
	"testing"
)

func TestDamageCoalesceOverlaps(t *testing.T) {
	var d Damage
	d.Add(XYWH(0, 0, 10, 10))
	d.Add(XYWH(8, 8, 10, 10))
	if len(d.Rects) != 1 {
		t.Fatalf("expected merge, got %d rects", len(d.Rects))
	}
	b := d.Bounds()
	if b.Min.X != 0 || b.Max.X != 18 || b.Max.Y != 18 {
		t.Fatalf("union %+v", b)
	}
}

func TestDamageClipAndReset(t *testing.T) {
	var d Damage
	d.Add(XYWH(-4, -4, 20, 8))
	d.ClipTo(XYWH(0, 0, 10, 10))
	if d.Empty() || d.Bounds().Min.X < 0 {
		t.Fatalf("clipped %+v", d.Bounds())
	}
	d.Reset()
	if !d.Empty() {
		t.Fatal("reset")
	}
}

func TestDamageMaxRectsCollapses(t *testing.T) {
	d := Damage{MaxRects: 3}
	d.Add(XYWH(0, 0, 2, 2))
	d.Add(XYWH(10, 0, 2, 2))
	d.Add(XYWH(20, 0, 2, 2))
	d.Add(XYWH(30, 0, 2, 2))
	if len(d.Rects) != 1 {
		t.Fatalf("should collapse, got %d", len(d.Rects))
	}
	if d.Bounds().Dx() < 30 {
		t.Fatalf("collapsed union too small %+v", d.Bounds())
	}
}

func TestDamageMergesBeforeMaxRectsCollapse(t *testing.T) {
	// 3 isolated boxes at MaxRects=3, then a 4th that overlaps the first.
	// Old code appended then collapsed the whole union; a UI layer would
	// redraw every widget. Merge-first keeps the other two isolated.
	d := Damage{MaxRects: 3}
	d.Add(XYWH(0, 0, 4, 4))
	d.Add(XYWH(20, 0, 4, 4))
	d.Add(XYWH(40, 0, 4, 4))
	d.Add(XYWH(2, 2, 4, 4)) // overlaps first
	if len(d.Rects) != 3 {
		t.Fatalf("overlapping 4th should merge, not collapse, got %d %+v", len(d.Rects), d.Rects)
	}
	if !d.Overlaps(XYWH(0, 0, 6, 6)) {
		t.Fatal("merged first cluster")
	}
	if d.Overlaps(XYWH(10, 10, 2, 2)) {
		t.Fatal("gap between isolated dirty boxes must stay clean")
	}
}

func TestDamageMergesTouchingEdges(t *testing.T) {
	var d Damage
	d.Add(XYWH(0, 0, 10, 10))
	d.Add(XYWH(10, 0, 10, 10)) // share an edge
	if len(d.Rects) != 1 {
		t.Fatalf("touching edges should coalesce, got %d", len(d.Rects))
	}
	if d.Bounds().Dx() != 20 || d.Bounds().Dy() != 10 {
		t.Fatalf("union %+v", d.Bounds())
	}
}

func TestDamageIgnoresNonFinite(t *testing.T) {
	var d Damage
	nan := float32(math.NaN())
	d.Add(Rect{Min: Pt(0, 0), Max: Pt(nan, 4)})
	if !d.Empty() {
		t.Fatalf("NaN dirty rect should be ignored, got %+v", d.Rects)
	}
	d.Add(XYWH(1, 1, 4, 4))
	if d.Empty() {
		t.Fatal("finite rect should record")
	}
}

func TestContextRecordsDamage(t *testing.T) {
	img := NewImage(40, 40)
	ctx := NewContext(img)
	var d Damage
	ctx.SetDamage(&d)
	ctx.SetColor(White)
	ctx.FillRect(XYWH(4, 6, 8, 8))
	if d.Empty() {
		t.Fatal("fill should dirty")
	}
	b := d.Bounds()
	if b.Min.X > 4 || b.Max.X < 12 || b.Min.Y > 6 || b.Max.Y < 14 {
		t.Fatalf("dirty bounds %+v", b)
	}
	d.Reset()
	ctx.Translate(10, 0)
	ctx.FillRect(XYWH(0, 0, 5, 5))
	if d.Bounds().Min.X < 9 {
		t.Fatalf("damage should follow transform %+v", d.Bounds())
	}
}
