package paintengine2d

import (
	"testing"
)

// Fuzz targets for path building, matrices, clip, and raster entry points.
// Seeds run under `go test`; longer campaigns: go test -fuzz=Fuzz -fuzztime=15s

func FuzzPathBuild(f *testing.F) {
	f.Add([]byte{0, 10, 10, 1, 20, 30})
	f.Add([]byte{0, 0, 0, 2, 40, 10, 20, 40, 4})
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 256 {
			raw = raw[:256]
		}
		p := NewPath()
		for i := 0; i+2 < len(raw); {
			op := raw[i] % 6
			i++
			x := float32(int8(raw[i]))
			y := float32(int8(raw[i+1]))
			i += 2
			switch op {
			case 0:
				p.MoveTo(x, y)
			case 1:
				p.LineTo(x, y)
			case 2:
				if i+2 >= len(raw) {
					return
				}
				p.QuadTo(x, y, float32(int8(raw[i])), float32(int8(raw[i+1])))
				i += 2
			case 3:
				if i+4 >= len(raw) {
					return
				}
				p.CubicTo(x, y, float32(int8(raw[i])), float32(int8(raw[i+1])),
					float32(int8(raw[i+2])), float32(int8(raw[i+3])))
				i += 4
			case 4:
				p.Close()
			default:
				p.AddCircle(Pt(x, y), abs32(y)+1)
			}
		}
		_ = p.Bounds()
		_ = p.Clone()
		p.Transform(Rotation(0.3).Mul(Translation(2, -1)))
		p.Reset()
	})
}

func FuzzMatrix(f *testing.F) {
	f.Add(float32(1), float32(0), float32(0), float32(1), float32(3), float32(-2))
	f.Add(float32(0), float32(1), float32(-1), float32(0), float32(0), float32(0))
	f.Fuzz(func(t *testing.T, a, b, c, d, e, ff float32) {
		m := Matrix{A: a, B: b, C: c, D: d, E: e, F: ff}
		p := m.Transform(Pt(1, 2))
		_ = p
		inv, ok := m.Invert()
		if ok {
			q := inv.Transform(m.Transform(Pt(4, -3)))
			_ = q
		}
		_ = m.Mul(Translation(e, ff)).ApproxScale()
		_ = m.TransformRect(XYWH(-2, -2, 5, 5))
	})
}

func FuzzRasterDraw(f *testing.F) {
	f.Add(uint8(1), uint8(10), uint8(10), uint8(40), uint8(30), uint8(2), uint8(3))
	f.Add(uint8(2), uint8(20), uint8(8), uint8(8), uint8(20), uint8(0), uint8(5))
	f.Fuzz(func(t *testing.T, kind, x0, y0, x1, y1, style, extra uint8) {
		img := NewImage(32, 32)
		ctx := NewContext(img)
		ctx.Clear(Transparent)
		p := NewPath()
		p.MoveTo(float32(x0%40), float32(y0%40))
		p.LineTo(float32(x1%40), float32(y1%40))
		p.QuadTo(float32(extra%40), 16, 8, float32(kind%40))
		if extra&1 == 1 {
			p.Close()
		}
		paint := Paint{
			Color: RGBA(1, 0.4, 0.2, 0.8),
			Style: Style(style % 3),
			Stroke: Stroke{
				Width:      float32(extra%20) - 2, // includes <= 0
				Cap:        Cap(kind % 3),
				Join:       Join(x0 % 3),
				MiterLimit: float32(y0 % 12),
				Dash:       []float32{float32(x1 % 8), float32(y1 % 8)},
			},
			FillRule: FillRule(extra % 2),
		}
		if kind%5 == 0 {
			paint.Shader = LinearGradient{
				Start: Pt(0, 0), End: Pt(32, 16),
				Stops: []GradientStop{{0, Red}, {1, Blue}},
				Tile:  TileMode(extra % 3),
			}
		}
		if kind%5 == 1 {
			paint.Shader = RadialGradient{
				Center: Pt(16, 16), Radius: float32(8 + extra%20),
				Stops: []GradientStop{{0, White}, {1, Black}},
			}
		}
		ctx.Save()
		if extra&2 == 2 {
			ctx.ClipRect(XYWH(float32(x0%16), float32(y0%16), float32(8+x1%16), float32(8+y1%16)))
		}
		ctx.Translate(float32(int8(x0)), float32(int8(y1)))
		ctx.Rotate(float32(extra) * 0.05)
		ctx.DrawPath(p, paint)
		ctx.Restore()
		ctx.DrawCircle(Pt(float32(x1%32), float32(y0%32)), float32(extra%12), Fill(Green))
	})
}

func FuzzClipAndImage(f *testing.F) {
	f.Add(int8(4), int8(4), int8(20), int8(12), uint8(1))
	f.Fuzz(func(t *testing.T, x, y, w, h int8, flags uint8) {
		dst := NewImage(24, 24)
		ctx := NewContext(dst)
		src := NewImage(6, 6)
		src.Clear(RGB(0.2, 0.6, 1))
		ctx.ClipRect(XYWH(float32(x), float32(y), float32(w), float32(h)))
		if flags&1 == 1 {
			ctx.ClipPath(CirclePath(Pt(12, 12), float32(4+flags%10)))
		}
		filt := FilterBilinear
		if flags&2 == 2 {
			filt = FilterNearest
		}
		ctx.DrawImageRectPaint(src, XYWH(0, 0, 6, 6), XYWH(float32(x), float32(y), 12, 12),
			Paint{Color: White.WithAlpha(0.7), Filter: filt})
		ctx.Clear(Transparent) // must not panic even with empty clip leftover
	})
}

func FuzzWrapImage(f *testing.F) {
	f.Add(uint8(8), uint8(4), uint8(16), uint8(3), uint8(2))
	f.Fuzz(func(t *testing.T, w8, h8, pad8, x8, y8 uint8) {
		w := int(w8%24) + 1
		h := int(h8%16) + 1
		pad := int(pad8 % 32)
		stride := w*4 + pad
		buf := make([]byte, h*stride)
		for i := range buf {
			buf[i] = 0xA5
		}
		img := WrapImage(buf, w, h, stride)
		if img == nil {
			return
		}
		ctx := NewContext(img)
		ctx.Clear(RGB(0.1, 0.2, 0.3))
		ctx.DrawRect(XYWH(float32(x8%16), float32(y8%16), 6, 4), Fill(White))
		ctx.DrawCircle(Pt(float32(w/2), float32(h/2)), 3, StrokePaint(Red, 1.5))
		src := NewImage(3, 3)
		src.Clear(Blue)
		ctx.DrawImageRectPaint(src, XYWH(0, 0, 3, 3), XYWH(1, 1, 5, 5), Paint{Color: White, Filter: FilterNearest})
		// Padding bytes must survive every paint.
		for y := 0; y < h; y++ {
			for i := w * 4; i < stride; i++ {
				if buf[y*stride+i] != 0xA5 {
					t.Fatalf("padding clobbered y=%d i=%d", y, i)
				}
			}
		}
		_ = img.Clone()
		_ = img.SubImage(0, 0, w, h)
	})
}

func FuzzDamage(f *testing.F) {
	f.Add(int8(0), int8(0), int8(8), int8(8), int8(6), int8(2), uint8(4), uint8(1))
	f.Fuzz(func(t *testing.T, x, y, w, h, x2, y2 int8, maxN, pad uint8) {
		d := Damage{MaxRects: int(maxN % 12), Pad: float32(pad % 8)}
		d.Add(XYWH(float32(x), float32(y), float32(w), float32(h)))
		d.Add(XYWH(float32(x2), float32(y2), 3, 3))
		d.Add(Rect{})
		_ = d.Overlaps(XYWH(0, 0, 4, 4))
		_ = d.Bounds()
		d.ClipTo(XYWH(-20, -20, 80, 80))
		if len(d.Rects) > 64 {
			t.Fatalf("damage list exploded: %d", len(d.Rects))
		}
	})
}

func FuzzTextHooks(f *testing.F) {
	f.Add("SAVE", int8(2), int8(2), uint8(1))
	f.Add("Ok!", int8(-4), int8(3), uint8(0))
	f.Fuzz(func(t *testing.T, text string, x, y int8, flags uint8) {
		if len(text) > 32 {
			text = text[:32]
		}
		atlas := NewBitmapAtlas(White)
		img := NewImage(64, 24)
		ctx := NewContext(img)
		if flags&1 == 1 {
			ctx.ClipRect(XYWH(4, 2, 40, 16))
		}
		if flags&2 == 2 {
			ctx.Translate(float32(x), float32(y))
			ctx.Rotate(0.15)
		}
		paint := Paint{Color: White, Filter: FilterNearest}
		if flags&4 == 4 {
			paint.Filter = FilterBilinear
			paint.Color = White.WithAlpha(0.6)
		}
		run := NullShaper{}.Shape(text, atlas)
		_ = run.Bounds(Pt(float32(x), float32(y)))
		ctx.DrawGlyphs(run, Pt(float32(x), float32(y)), paint)
		ctx.DrawLabel(text, atlas, Pt(1, 1), paint)
		ctx.DrawLabel(text, nil, Pt(0, 0), paint)
	})
}
