package paintengine2d

import "testing"

// Drawing into a one-byte coverage image ([NewImageA8]) must leave exactly
// the alpha that the same drawing leaves in an RGBA image, since a white
// premultiplied pixel is its own coverage.
func TestA8TargetMatchesRGBAAlpha(t *testing.T) {
	const w, h = 64, 48
	white := Paint{Color: RGBA(1, 1, 1, 1)}
	half := Paint{Color: RGBA(1, 1, 1, 0.5)}
	glyph := NewImageA8(9, 9)
	for i := range glyph.Pix {
		glyph.Pix[i] = uint8(i * 3 % 256)
	}
	tile := NewImage(6, 6)
	tile.Clear(RGBA(1, 1, 1, 0.75))

	draws := []struct {
		name string
		draw func(*Context)
	}{
		{"rect on the tight fill", func(c *Context) { c.DrawRect(XYWH(4, 4, 20, 10), white) }},
		{"rect off the pixel grid", func(c *Context) { c.DrawRect(XYWH(4.35, 4.2, 20.4, 10.1), white) }},
		{"translucent rect", func(c *Context) { c.DrawRect(XYWH(2, 2, 30, 20), half) }},
		{"antialiased circle", func(c *Context) { c.DrawCircle(Pt(30, 24), 13, white) }},
		{"round rect", func(c *Context) { c.DrawRoundRect(XYWH(6, 6, 40, 30), 7, 7, white) }},
		{"a8 blit", func(c *Context) { c.DrawImage(glyph, 10, 10) }},
		{"rgba blit", func(c *Context) { c.DrawImage(tile, 20, 14) }},
		{"dest-out punch", func(c *Context) {
			c.DrawRect(XYWH(4, 4, 40, 30), white)
			c.DrawCircle(Pt(24, 19), 9, Paint{Color: RGBA(1, 1, 1, 1), Blend: BlendDestOut})
		}},
		{"clipped circle", func(c *Context) {
			c.Save()
			c.ClipRect(XYWH(8, 8, 24, 18))
			c.DrawCircle(Pt(24, 20), 14, white)
			c.Restore()
		}},
	}
	for _, d := range draws {
		t.Run(d.name, func(t *testing.T) {
			rgba := NewImage(w, h)
			d.draw(NewContext(rgba))
			mask := NewImageA8(w, h)
			d.draw(NewContext(mask))
			var worst, at int
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					want := int(rgba.Pix[y*rgba.RowStride()+x*4+3])
					got := int(mask.Pix[y*mask.RowStride()+x])
					if diff := abs(want - got); diff > worst {
						worst, at = diff, y*w+x
					}
				}
			}
			if worst > 1 {
				t.Fatalf("coverage differs by %d at (%d,%d)", worst, at%w, at/w)
			}
		})
	}
}

// A one-byte target keeps its own stride: a draw must never write past the
// row it is on (the bug this fixes wrote four bytes per pixel).
func TestA8TargetStaysInItsRows(t *testing.T) {
	img := NewImageA8(16, 4)
	ctx := NewContext(img)
	ctx.DrawRect(XYWH(0, 0, 16, 1), Paint{Color: RGBA(1, 1, 1, 1)})
	for x := 0; x < 16; x++ {
		if img.Pix[x] != 255 {
			t.Fatalf("row 0 pixel %d is %d, want 255", x, img.Pix[x])
		}
	}
	for i := 16; i < len(img.Pix); i++ {
		if img.Pix[i] != 0 {
			t.Fatalf("byte %d spilled past row 0: %d", i, img.Pix[i])
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
