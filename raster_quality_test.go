package paintengine2d

import (
	"math"
	"testing"
)

func TestCircleInteriorAndAARim(t *testing.T) {
	img := NewImage(80, 80)
	ctx := NewContext(img)
	ctx.Clear(Transparent)
	ctx.DrawCircle(Pt(40, 40), 20, Fill(RGB(1, 0, 0)))

	// Center must be fully covered red.
	r, g, b, a := img.PremulAt(40, 40)
	if a < 250 || r < 250 || g != 0 || b != 0 {
		t.Fatalf("center %d %d %d %d", r, g, b, a)
	}
	// Far outside must stay transparent.
	_, _, _, a = img.PremulAt(2, 2)
	if a != 0 {
		t.Fatalf("outside alpha %d", a)
	}
	// Rim must contain fractional coverage (the AA proof).
	partial := 0
	for y := 0; y < 80; y++ {
		for x := 0; x < 80; x++ {
			_, _, _, a := img.PremulAt(x, y)
			if a > 8 && a < 240 {
				partial++
			}
		}
	}
	if partial < 40 {
		t.Fatalf("expected a visible AA rim, got %d partial pixels", partial)
	}
}

func TestFillRectExactInterior(t *testing.T) {
	img := NewImage(32, 32)
	ctx := NewContext(img)
	ctx.SetColor(RGB(0, 1, 0))
	ctx.FillRect(XYWH(8, 8, 10, 10))
	r, g, b, a := img.PremulAt(10, 10)
	if r != 0 || g != 255 || b != 0 || a != 255 {
		t.Fatalf("interior %d %d %d %d", r, g, b, a)
	}
	_, _, _, a = img.PremulAt(0, 0)
	if a != 0 {
		t.Fatalf("outside %d", a)
	}
}

func TestFractionalRectAA(t *testing.T) {
	img := NewImage(16, 16)
	ctx := NewContext(img)
	ctx.DrawRect(XYWH(2.25, 2.25, 6.5, 6.5), Fill(White))
	_, _, _, a0 := img.PremulAt(2, 2)
	_, _, _, a1 := img.PremulAt(5, 5)
	if a1 < 250 {
		t.Fatalf("interior should be solid, alpha=%d", a1)
	}
	if a0 == 0 || a0 == 255 {
		t.Fatalf("corner pixel should be partial, alpha=%d", a0)
	}
}

func TestLinearGradientEndpoints(t *testing.T) {
	img := NewImage(64, 8)
	ctx := NewContext(img)
	g := LinearGradient{
		Start: Pt(0, 0),
		End:   Pt(64, 0),
		Stops: []GradientStop{
			{0, RGB(1, 0, 0)},
			{1, RGB(0, 0, 1)},
		},
	}
	ctx.DrawRect(XYWH(0, 0, 64, 8), Linear(g))
	r0, _, b0, a0 := img.PremulAt(0, 4)
	r1, _, b1, a1 := img.PremulAt(63, 4)
	if a0 < 250 || a1 < 250 {
		t.Fatalf("opaque ends %d %d", a0, a1)
	}
	if r0 < 200 || b0 > 40 {
		t.Fatalf("left should be red, got r=%d b=%d", r0, b0)
	}
	if b1 < 200 || r1 > 40 {
		t.Fatalf("right should be blue, got r=%d b=%d", r1, b1)
	}
}

func TestSaveRestoreClipAndTransform(t *testing.T) {
	img := NewImage(40, 40)
	ctx := NewContext(img)
	ctx.SetColor(White)
	ctx.Save()
	ctx.ClipRect(XYWH(0, 0, 10, 40))
	ctx.FillRect(XYWH(0, 0, 40, 40))
	ctx.Restore()
	_, _, _, aClip := img.PremulAt(5, 20)
	_, _, _, aOut := img.PremulAt(20, 20)
	if aClip < 250 || aOut != 0 {
		t.Fatalf("clip a=%d out=%d", aClip, aOut)
	}

	ctx.Translate(20, 20)
	ctx.FillRect(XYWH(0, 0, 5, 5))
	_, _, _, aT := img.PremulAt(22, 22)
	if aT < 250 {
		t.Fatalf("translated fill missing, a=%d", aT)
	}
}

func TestStrokeHasWidth(t *testing.T) {
	img := NewImage(60, 20)
	ctx := NewContext(img)
	ctx.DrawLine(Pt(5, 10), Pt(55, 10), StrokePaint(White, 6))
	_, _, _, mid := img.PremulAt(30, 10)
	_, _, _, above := img.PremulAt(30, 10-2)
	_, _, _, far := img.PremulAt(30, 1)
	if mid < 250 {
		t.Fatalf("stroke center a=%d", mid)
	}
	if above < 200 {
		t.Fatalf("stroke should cover ±3px, a=%d", above)
	}
	if far != 0 {
		t.Fatalf("far pixel should be empty, a=%d", far)
	}
}

func TestEvenOddNestedRects(t *testing.T) {
	img := NewImage(80, 80)
	ctx := NewContext(img)
	p := NewPath()
	p.AddRect(XYWH(10, 10, 60, 60))
	p.AddRect(XYWH(25, 25, 30, 30))
	ctx.DrawPath(p, Paint{Color: White, FillRule: FillEvenOdd})
	_, _, _, hole := img.PremulAt(40, 40)
	_, _, _, ring := img.PremulAt(15, 40)
	if hole > 20 {
		t.Fatalf("even-odd inner rect should be a hole, a=%d", hole)
	}
	if ring < 200 {
		t.Fatalf("even-odd ring should be filled, a=%d", ring)
	}
}

func TestEvenOddPentagramHole(t *testing.T) {
	img := NewImage(80, 80)
	ctx := NewContext(img)
	ctx.DrawPath(pentagram(Pt(40, 40), 32), Paint{Color: White, FillRule: FillEvenOdd})
	_, _, _, hole := img.PremulAt(40, 40)
	_, _, _, tip := img.PremulAt(40, 12)
	if hole > 40 {
		t.Fatalf("pentagram even-odd center should be a hole, a=%d", hole)
	}
	if tip < 180 {
		t.Fatalf("pentagram tip should be filled, a=%d", tip)
	}
}

func TestImageRoundTripPNG(t *testing.T) {
	img := NewImage(8, 4)
	img.SetColor(2, 1, RGBA(0.2, 0.4, 0.8, 1))
	path := t.TempDir() + "/t.png"
	if err := img.WritePNGFile(path); err != nil {
		t.Fatal(err)
	}
	got, err := DecodePNGFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Width != 8 || got.Height != 4 {
		t.Fatalf("size %dx%d", got.Width, got.Height)
	}
	c := FromPremul8(got.PremulAt(2, 1))
	if abs32(c.B-0.8) > 0.05 {
		t.Fatalf("roundtrip %+v", c)
	}
}

func TestDrawImage(t *testing.T) {
	src := NewImage(4, 4)
	src.Clear(RGB(0, 0, 1))
	dst := NewImage(16, 16)
	ctx := NewContext(dst)
	ctx.DrawImage(src, 4, 5)
	_, _, b, a := dst.PremulAt(5, 6)
	if a < 250 || b < 250 {
		t.Fatalf("blit %d %d", b, a)
	}
	_, _, _, out := dst.PremulAt(0, 0)
	if out != 0 {
		t.Fatalf("outside blit %d", out)
	}
}

func TestClipPathCircle(t *testing.T) {
	img := NewImage(40, 40)
	ctx := NewContext(img)
	ctx.ClipPath(CirclePath(Pt(20, 20), 8))
	ctx.SetColor(White)
	ctx.FillRect(XYWH(0, 0, 40, 40))
	_, _, _, in := img.PremulAt(20, 20)
	_, _, _, out := img.PremulAt(2, 2)
	if in < 250 || out != 0 {
		t.Fatalf("clip path in=%d out=%d", in, out)
	}
}

func starPath(c Point, outer, inner float32, n int) *Path {
	p := NewPath()
	for i := 0; i < n*2; i++ {
		r := outer
		if i%2 == 1 {
			r = inner
		}
		ang := float32(-math.Pi/2) + float32(i)*float32(math.Pi)/float32(n)
		x := c.X + r*cos32(ang)
		y := c.Y + r*sin32(ang)
		if i == 0 {
			p.MoveTo(x, y)
		} else {
			p.LineTo(x, y)
		}
	}
	p.Close()
	return p
}

// pentagram is a self-intersecting {5/2} star; even-odd leaves a hole.
func pentagram(c Point, r float32) *Path {
	p := NewPath()
	for i := 0; i < 5; i++ {
		ang := -math.Pi/2 + float64(i)*4*math.Pi/5
		x := c.X + r*float32(math.Cos(ang))
		y := c.Y + r*float32(math.Sin(ang))
		if i == 0 {
			p.MoveTo(x, y)
		} else {
			p.LineTo(x, y)
		}
	}
	p.Close()
	return p
}

func cos32(a float32) float32 { return float32(math.Cos(float64(a))) }
func sin32(a float32) float32 { return float32(math.Sin(float64(a))) }
