package paintengine2d

import (
	"runtime"
	"testing"
	"time"
)

// Regression: blitTint treated Color.A == 0 as "untinted", so a widget
// fading out snapped back to fully opaque on the last frame.
func TestTintAlphaZeroIsInvisible(t *testing.T) {
	icon := NewImage(8, 8)
	icon.Clear(Black)

	img := NewImage(20, 20)
	ctx := NewContext(img)
	ctx.Clear(White)
	ctx.DrawImageRectPaint(icon, XYWH(0, 0, 8, 8), XYWH(4, 4, 8, 8), Paint{Color: RGBA(1, 1, 1, 0)})
	if r, _, _, _ := img.PremulAt(8, 8); r != 255 {
		t.Fatalf("alpha-0 tint must paint nothing, got R=%d", r)
	}

	ctx.Clear(White)
	ctx.DrawImageRectPaint(icon, XYWH(0, 0, 8, 8), XYWH(4, 4, 8, 8), Paint{Color: RGBA(1, 1, 1, 0.5)})
	r, _, _, _ := img.PremulAt(8, 8)
	if r < 100 || r > 160 {
		t.Fatalf("alpha-0.5 tint should be about half, got R=%d", r)
	}

	// The zero-value paint still means "untinted".
	ctx.Clear(White)
	ctx.DrawImageRect(icon, XYWH(0, 0, 8, 8), XYWH(4, 4, 8, 8))
	if r, _, _, _ := img.PremulAt(8, 8); r != 0 {
		t.Fatalf("zero-value paint must blit unmodulated, got R=%d", r)
	}
}

func TestGlyphTintAlphaZeroIsInvisible(t *testing.T) {
	atlas := NewBitmapAtlas(White)
	img := NewImage(20, 20)
	ctx := NewContext(img)
	ctx.Clear(Black)
	ctx.DrawLabel("A", atlas, Pt(2, 2), Paint{Color: RGBA(1, 1, 1, 0)})
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			if r, _, _, _ := img.PremulAt(x, y); r != 0 {
				t.Fatalf("alpha-0 glyph tint painted at %d,%d (R=%d)", x, y, r)
			}
		}
	}
}

// Paint.Opacity is a layer alpha on top of color/shader alpha.
func TestPaintOpacity(t *testing.T) {
	img := NewImage(20, 20)
	ctx := NewContext(img)

	ctx.Clear(White)
	p := Fill(Black)
	p.Opacity = 0.5
	ctx.DrawRect(XYWH(0, 0, 20, 20), p)
	if r, _, _, _ := img.PremulAt(10, 10); r < 100 || r > 160 {
		t.Fatalf("half-opacity fill R=%d", r)
	}

	// Zero Opacity keeps the old meaning: fully opaque.
	ctx.Clear(White)
	ctx.DrawRect(XYWH(0, 0, 20, 20), Fill(Black))
	if r, _, _, _ := img.PremulAt(10, 10); r != 0 {
		t.Fatalf("default opacity must be opaque, got R=%d", r)
	}

	// Opacity also modulates a gradient shader and a blit.
	ctx.Clear(White)
	g := Linear(LinearGradient{Start: Pt(0, 0), End: Pt(20, 0), Stops: []GradientStop{
		{Offset: 0, Color: Black}, {Offset: 1, Color: Black},
	}})
	g.Opacity = 0.5
	ctx.DrawRect(XYWH(0, 0, 20, 20), g)
	if r, _, _, _ := img.PremulAt(10, 10); r < 100 || r > 160 {
		t.Fatalf("half-opacity gradient R=%d", r)
	}

	icon := NewImage(8, 8)
	icon.Clear(Black)
	ctx.Clear(White)
	ctx.DrawImageRectPaint(icon, XYWH(0, 0, 8, 8), XYWH(4, 4, 8, 8), Paint{Opacity: 0.5})
	if r, _, _, _ := img.PremulAt(8, 8); r < 100 || r > 160 {
		t.Fatalf("half-opacity blit R=%d", r)
	}
}

// Regression: finite but enormous control points overflowed to NaN during
// subdivision; the NaN comparison never terminated so the flattener ran to
// the depth cap and allocated ~1.4 GB for a single curve.
func TestHugeCurveIsBounded(t *testing.T) {
	img := NewImage(64, 64)
	ctx := NewContext(img)
	p := NewPath()
	p.MoveTo(0, 0)
	p.CubicTo(3e38, 3e38, -3e38, 3e38, 10, 10)
	p.Close()

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	before := ms.TotalAlloc
	start := time.Now()
	ctx.DrawPath(p, Fill(Black))
	elapsed := time.Since(start)
	runtime.ReadMemStats(&ms)
	grew := ms.TotalAlloc - before

	if elapsed > 2*time.Second {
		t.Fatalf("huge cubic took %v", elapsed)
	}
	if grew > 64<<20 {
		t.Fatalf("huge cubic allocated %d MB", grew>>20)
	}
}

func TestHugeQuadAndNaNPathAreSafe(t *testing.T) {
	img := NewImage(32, 32)
	ctx := NewContext(img)
	p := NewPath()
	p.MoveTo(0, 0)
	p.QuadTo(1e30, 1e30, 10, 10)
	p.Close()
	ctx.DrawPath(p, Fill(Black)) // must not hang or panic

	q := NewPath()
	q.MoveTo(1, 1)
	q.LineTo(20, 20)
	q.CubicTo(1e38, -1e38, 1e38, 1e38, 30, 5)
	q.Close()
	ctx.StrokePath(q)
}

// Regression: a zero-length subpath with a round or square cap paints a dot
// (Skia/Cairo behaviour); it used to paint nothing.
func TestZeroLengthSubpathCaps(t *testing.T) {
	check := func(cap Cap, want bool) {
		t.Helper()
		img := NewImage(20, 20)
		ctx := NewContext(img)
		ctx.Clear(White)
		st := StrokePaint(Black, 6)
		st.Stroke.Cap = cap
		ctx.DrawLine(Pt(10, 10), Pt(10, 10), st)
		r, _, _, _ := img.PremulAt(10, 10)
		if want && r != 0 {
			t.Fatalf("cap %v should paint a dot, got R=%d", cap, r)
		}
		if !want && r != 255 {
			t.Fatalf("butt cap should paint nothing, got R=%d", r)
		}
	}
	check(CapRound, true)
	check(CapSquare, true)
	check(CapButt, false)
}

// Regression: Image.Scroll / CopyFrom moved pixels without bumping Epoch, so
// a GPU texture cached for that image stayed stale.
func TestInPlacePixelMovesBumpEpoch(t *testing.T) {
	im := NewImage(16, 16)
	im.Clear(White)
	e := im.Epoch
	im.Scroll(0, 4, XYWH(0, 0, 16, 16))
	if im.Epoch == e {
		t.Fatal("Scroll must bump Epoch")
	}
	if im.Dirty.Empty() {
		t.Fatal("Scroll must record a dirty box")
	}
	e = im.Epoch
	src := NewImage(8, 8)
	src.Clear(Red)
	im.CopyFrom(src, XYWH(0, 0, 8, 8), 2, 2)
	if im.Epoch == e {
		t.Fatal("CopyFrom must bump Epoch")
	}
}

// Images carry a process-unique identity so a cache cannot confuse two
// images that happened to reuse an address.
func TestImageIDsAreUnique(t *testing.T) {
	a, b := NewImage(4, 4), NewImage(4, 4)
	if a.ID == 0 || b.ID == 0 || a.ID == b.ID {
		t.Fatalf("ids %d %d", a.ID, b.ID)
	}
	if a.Clone().ID == a.ID {
		t.Fatal("Clone must get a fresh id")
	}
	if w := WrapImage(make([]byte, 64), 4, 4, 0); w.ID == 0 || w.ID == a.ID {
		t.Fatalf("WrapImage id %d", w.ID)
	}
	var lit Image
	if lit.UID() == 0 || lit.UID() != lit.UID() {
		t.Fatal("struct-literal Image must get a stable lazy id")
	}
}

// Dirty rectangles are whole pixels: a consumer that erases them rounds
// outward, so the recorded box must too.
func TestDamageRoundsOutward(t *testing.T) {
	var d Damage
	d.Add(XYWH(2.25, 3.75, 4.5, 1.1))
	b := d.Bounds()
	if b.Min.X != 2 || b.Min.Y != 3 || b.Max.X != 7 || b.Max.Y != 5 {
		t.Fatalf("damage not snapped to pixels: %+v", b)
	}
}
