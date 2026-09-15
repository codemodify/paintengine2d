package paintengine2d

import "testing"

// checker is a 2×2 black-and-white dither.
func checker() *Image {
	img := NewImage(2, 2)
	img.SetColor(0, 0, Black)
	img.SetColor(1, 1, Black)
	img.SetColor(1, 0, White)
	img.SetColor(0, 1, White)
	return img
}

// A pattern tiles from its origin, one image pixel per user unit (or per
// Scale), anchored to user space, not to the shape.
func TestImagePatternTiles(t *testing.T) {
	img := NewImage(12, 12)
	ctx := NewContext(img)
	ctx.Clear(RGBA(1, 0, 0, 1))
	ctx.DrawRect(XYWH(2, 2, 8, 8), Pattern(checker(), Pt(0, 0)))
	for _, c := range []struct {
		x, y int
		r    uint8
	}{{2, 2, 0}, {3, 2, 255}, {2, 3, 255}, {3, 3, 0}, {9, 9, 0}, {8, 9, 255}} {
		if r, g, _, _ := img.PremulAt(c.x, c.y); r != c.r || g != c.r {
			t.Errorf("(%d,%d): %d,%d, want %d", c.x, c.y, r, g, c.r)
		}
	}
	if r, g, _, _ := img.PremulAt(1, 1); r != 255 || g != 0 {
		t.Fatal("the pattern painted outside the rect")
	}
	// Moved origin and scale 2: 2×2 blocks, starting one unit in.
	ctx.Clear(RGBA(1, 0, 0, 1))
	ctx.DrawRect(XYWH(0, 0, 12, 12), Paint{Shader: ImagePattern{Image: checker(), Origin: Pt(1, 1), Scale: 2}})
	for _, c := range []struct {
		x, y int
		r    uint8
	}{{1, 1, 0}, {2, 2, 0}, {3, 1, 255}, {0, 0, 0}, {5, 5, 0}, {3, 3, 0}, {1, 3, 255}} {
		if r, _, _, _ := img.PremulAt(c.x, c.y); r != c.r {
			t.Errorf("scaled (%d,%d): %d, want %d", c.x, c.y, r, c.r)
		}
	}
}

func TestGPUImagePatternMatchesCPU(t *testing.T) {
	d := gpuDev(t, 16, 16)
	defer d.Close()
	cimg := NewImage(16, 16)
	for _, ctx := range []*Context{NewContextDevice(d), NewContext(cimg)} {
		ctx.Clear(RGBA(1, 0, 0, 1))
		p := NewPath()
		p.MoveTo(1, 1)
		p.LineTo(15, 1)
		p.LineTo(15, 15)
		p.LineTo(1, 15)
		p.Close()
		ctx.DrawPath(p, Paint{Shader: ImagePattern{Image: checker(), Origin: Pt(1, 0), Scale: 1}})
	}
	g := d.Snapshot()
	for y := 2; y < 14; y++ {
		for x := 2; x < 14; x++ {
			gr, _, _, _ := g.PremulAt(x, y)
			cr, _, _, _ := cimg.PremulAt(x, y)
			if d := int(gr) - int(cr); d > 8 || d < -8 {
				t.Fatalf("GPU %d vs CPU %d at (%d,%d)", gr, cr, x, y)
			}
		}
	}
}
