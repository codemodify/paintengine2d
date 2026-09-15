package paintengine2d

import "testing"

// A coverage mask reads everywhere as premultiplied white at its coverage,
// and keeps its format through the image operations.
func TestImageA8ReadsAsWhite(t *testing.T) {
	m := NewImageA8(4, 3)
	if m.Format != FormatA8 || m.BytesPerPixel() != 1 || len(m.Pix) != 12 || m.RowStride() != 4 {
		t.Fatalf("mask layout %+v", m)
	}
	half := White.WithAlpha(0.5)
	_, _, _, ha := half.Premul8()
	m.SetColor(1, 1, half)
	if r, g, b, a := m.PremulAt(1, 1); a != ha || r != a || g != a || b != a {
		t.Fatalf("premul %d %d %d %d, want white at %d", r, g, b, a, ha)
	}
	if n := m.NRGBAAt(1, 1); n.R != 255 || n.G != 255 || n.B != 255 || n.A != ha {
		t.Fatalf("nrgba %+v", n)
	}
	if c := m.Clone(); c.Format != FormatA8 || len(c.Pix) != 12 || c.Pix[1*4+1] != ha {
		t.Fatal("a clone keeps the format and the pixels")
	}
	if s := m.SubImage(1, 1, 3, 3); s.Format != FormatA8 || s.Width != 2 || s.Pix[0] != ha {
		t.Fatal("a sub-image keeps the format")
	}
	quarter := White.WithAlpha(0.25)
	_, _, _, qa := quarter.Premul8()
	m.Clear(quarter)
	for i, v := range m.Pix {
		if v != qa {
			t.Fatalf("clear wrote %d at %d, want %d", v, i, qa)
		}
	}
	m.ClearRect(XYWH(0, 0, 2, 1), Color{})
	if m.Pix[0] != 0 || m.Pix[1] != 0 || m.Pix[2] != qa {
		t.Fatalf("clear rect %v", m.Pix[:4])
	}
	rgba := NewImage(4, 3)
	rgba.CopyFrom(m, XYWH(0, 0, 4, 3), 0, 0)
	if r, g, b, a := rgba.PremulAt(2, 0); a != qa || r != qa || g != qa || b != qa {
		t.Fatal("a mask copies into RGBA as white at its coverage")
	}
	back := NewImageA8(4, 3)
	back.CopyFrom(rgba, XYWH(0, 0, 4, 3), 0, 0)
	if back.Pix[2] != qa || back.Pix[0] != 0 {
		t.Fatal("RGBA copies into a mask as its alpha")
	}
	m.Pix[4] = 200
	m.Scroll(1, 0, XYWH(0, 0, 4, 3))
	if m.Pix[5] != 200 {
		t.Fatal("scroll moves one byte per pixel")
	}
}

// a8Twins is a coverage mask and the white RGBA image it stands for.
func a8Twins() (mask, twin *Image) {
	mask, twin = NewImageA8(13, 9), NewImage(13, 9)
	for y := 0; y < 9; y++ {
		for x := 0; x < 13; x++ {
			a := uint8((x*37 + y*91) % 256)
			mask.Pix[y*13+x] = a
			i := twin.pixIndex(x, y)
			twin.Pix[i], twin.Pix[i+1], twin.Pix[i+2], twin.Pix[i+3] = a, a, a, a
		}
	}
	return mask, twin
}

// a8Draws blit a glyph sheet the ways text and icons are drawn: 1:1 under
// a translation, scaled, rotated, bilinear and nearest, tinted and faded.
var a8Draws = []struct {
	name  string
	xf    Matrix
	dst   Rect
	paint Paint
}{
	{"1:1 nearest", Translation(3, 2), XYWH(0, 0, 13, 9), Paint{Color: RGB(0.9, 0.3, 0.1), Filter: FilterNearest}},
	{"scaled bilinear", Identity(), XYWH(1.5, 2.25, 31, 20), Paint{Color: RGB(0.1, 0.5, 0.9)}},
	{"rotated", Translation(20, 4).Mul(Rotation(0.4)), XYWH(0, 0, 13, 9), Paint{Color: RGB(0.2, 0.7, 0.3)}},
	{"faded nearest", Translation(5, 5), XYWH(0, 0, 13, 9), Paint{Color: RGBA(0, 0, 0, 0.6), Filter: FilterNearest}},
	{"untinted", Translation(2, 20), XYWH(0, 0, 13, 9), Paint{Filter: FilterNearest}},
}

// A mask draws exactly as its white RGBA twin on the CPU.
func TestBlitA8MatchesWhiteRGBA(t *testing.T) {
	mask, twin := a8Twins()
	for _, d := range a8Draws {
		a, b := NewImage(48, 40), NewImage(48, 40)
		a.Clear(RGB(0.8, 0.8, 0.7))
		b.Clear(RGB(0.8, 0.8, 0.7))
		NewCPUDevice(a).Blit(mask, XYWH(0, 0, 13, 9), d.dst, d.xf, d.paint, Clip{})
		NewCPUDevice(b).Blit(twin, XYWH(0, 0, 13, 9), d.dst, d.xf, d.paint, Clip{})
		for i := range a.Pix {
			if a.Pix[i] != b.Pix[i] {
				t.Fatalf("%s: byte %d is %d from the mask, %d from its twin", d.name, i, a.Pix[i], b.Pix[i])
			}
		}
	}
}
