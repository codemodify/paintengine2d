package paintengine2d

// ImagePattern tiles an image across the plane: a dither, a desktop
// pattern, a woven texture. Origin is where a tile's top-left corner sits in
// user space, and Scale is the size of one image pixel in user units (0
// means 1). Sampling is nearest, so dithers stay crisp; the image must not
// change while a paint that uses it is recorded or drawn.
type ImagePattern struct {
	Image  *Image
	Origin Point
	Scale  float32
}

// Pattern returns a fill paint that tiles img from origin.
func Pattern(img *Image, origin Point) Paint {
	return Paint{Shader: ImagePattern{Image: img, Origin: origin}, Style: StyleFill, AntiAlias: true, Stroke: DefaultStroke()}
}

func (p ImagePattern) scale() float32 {
	if p.Scale <= 0 {
		return 1
	}
	return p.Scale
}

// Shade implements [Shader].
func (p ImagePattern) Shade(x, y float32, xform Matrix) Color {
	inv, ok := xform.Invert()
	if !ok {
		inv = Identity()
	}
	return p.shadeUser(inv.Transform(Point{x, y}))
}

// Prepare implements [PreparedShader]: the inverse transform, once per draw.
func (p ImagePattern) Prepare(xform Matrix) Shader {
	inv, ok := xform.Invert()
	if !ok {
		inv = Identity()
	}
	return preparedPattern{p: p, inv: inv}
}

type preparedPattern struct {
	p   ImagePattern
	inv Matrix
}

func (pp preparedPattern) Shade(x, y float32, _ Matrix) Color {
	return pp.p.shadeUser(pp.inv.Transform(Point{x, y}))
}

// shadeUser is the pattern's straight-alpha colour at user point u.
func (p ImagePattern) shadeUser(u Point) Color {
	img := p.Image
	if img == nil || img.Width <= 0 || img.Height <= 0 {
		return Color{}
	}
	s := p.scale()
	ix := wrapIndex(int(floor32((u.X-p.Origin.X)/s)), img.Width)
	iy := wrapIndex(int(floor32((u.Y-p.Origin.Y)/s)), img.Height)
	r, g, b, a := img.PremulAt(ix, iy)
	if a == 0 {
		return Color{}
	}
	fa := float32(a) / 255
	return Color{R: float32(r) / 255 / fa, G: float32(g) / 255 / fa, B: float32(b) / 255 / fa, A: fa}
}

func wrapIndex(i, n int) int {
	i %= n
	if i < 0 {
		i += n
	}
	return i
}

func floor32(v float32) float32 {
	i := float32(int(v))
	if v < i {
		i--
	}
	return i
}
