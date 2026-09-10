package paintengine2d

import "image/color"

// Color is a straight-alpha sRGB color. Components are in [0, 1].
// The rasterizer converts to premultiplied 8-bit RGBA when blending.
type Color struct {
	R, G, B, A float32
}

// RGB returns an opaque color.
func RGB(r, g, b float32) Color { return Color{R: r, G: g, B: b, A: 1} }

// RGBA returns a straight-alpha color.
func RGBA(r, g, b, a float32) Color { return Color{R: r, G: g, B: b, A: a} }

// Gray returns an opaque gray.
func Gray(v float32) Color { return Color{R: v, G: v, B: v, A: 1} }

// Bytes constructs a color from 8-bit sRGB channels (straight alpha).
func Bytes(r, g, b, a uint8) Color {
	return Color{
		R: float32(r) / 255,
		G: float32(g) / 255,
		B: float32(b) / 255,
		A: float32(a) / 255,
	}
}

// Common colors.
var (
	Transparent = Color{A: 0}
	Black       = Color{A: 1}
	White       = Color{R: 1, G: 1, B: 1, A: 1}
	Red         = Color{R: 1, A: 1}
	Green       = Color{G: 1, A: 1}
	Blue        = Color{B: 1, A: 1}
)

// Clamp returns c with each component limited to [0, 1].
func (c Color) Clamp() Color {
	return Color{
		R: clamp32(c.R, 0, 1),
		G: clamp32(c.G, 0, 1),
		B: clamp32(c.B, 0, 1),
		A: clamp32(c.A, 0, 1),
	}
}

// WithAlpha returns c with A replaced.
func (c Color) WithAlpha(a float32) Color {
	c.A = a
	return c
}

// Lerp interpolates toward q in straight-alpha sRGB (good enough for v0 stops).
func (c Color) Lerp(q Color, t float32) Color {
	return Color{
		R: c.R + (q.R-c.R)*t,
		G: c.G + (q.G-c.G)*t,
		B: c.B + (q.B-c.B)*t,
		A: c.A + (q.A-c.A)*t,
	}
}

// NRGBA returns the straight 8-bit sRGB encoding of c.
func (c Color) NRGBA() color.NRGBA {
	c = c.Clamp()
	return color.NRGBA{
		R: uint8(c.R*255 + 0.5),
		G: uint8(c.G*255 + 0.5),
		B: uint8(c.B*255 + 0.5),
		A: uint8(c.A*255 + 0.5),
	}
}

// Premul8 returns premultiplied 8-bit sRGB channels.
func (c Color) Premul8() (r, g, b, a uint8) {
	c = c.Clamp()
	a = uint8(c.A*255 + 0.5)
	r = uint8(c.R*c.A*255 + 0.5)
	g = uint8(c.G*c.A*255 + 0.5)
	b = uint8(c.B*c.A*255 + 0.5)
	return
}

// FromNRGBA converts a straight 8-bit color.
func FromNRGBA(c color.NRGBA) Color { return Bytes(c.R, c.G, c.B, c.A) }

// FromPremul8 converts a premultiplied 8-bit pixel to straight-alpha Color.
func FromPremul8(r, g, b, a uint8) Color {
	if a == 0 {
		return Transparent
	}
	af := float32(a) / 255
	return Color{
		R: float32(r) / float32(a),
		G: float32(g) / float32(a),
		B: float32(b) / float32(a),
		A: af,
	}
}
