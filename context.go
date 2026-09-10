package paintengine2d

// Context is the public 2D canvas — the analogue of Skia's SkCanvas and
// JUCE's Graphics. It owns a transform / clip / paint stack and forwards
// drawing to a [Device].
//
// A Context is not safe for concurrent use.
type Context struct {
	dev     Device
	stack   []ctxState
	cur     ctxState
	scratch *Path // reused by DrawRect / DrawCircle / …
}

type ctxState struct {
	xform  Matrix
	clip   deviceClip
	fill   Paint
	stroke Paint
}

// NewContext draws into img using the CPU backend.
func NewContext(img *Image) *Context {
	return NewContextDevice(NewCPUDevice(img))
}

// NewContextDevice draws through an arbitrary [Device] (CPU today, GPU later).
func NewContextDevice(dev Device) *Context {
	if dev == nil {
		panic("paintengine2d: NewContextDevice(nil)")
	}
	w, h := dev.Size()
	c := &Context{dev: dev}
	c.cur = ctxState{
		xform: Identity(),
		clip: deviceClip{
			scissor:    XYWH(0, 0, float32(w), float32(h)),
			hasScissor: true,
		},
		fill:   Fill(Black),
		stroke: StrokePaint(Black, 1),
	}
	return c
}

// Device returns the low-level backend.
func (c *Context) Device() Device { return c.dev }

// Image returns the CPU pixmap when the device is a [CPUDevice]; otherwise nil.
func (c *Context) Image() *Image {
	if d, ok := c.dev.(*CPUDevice); ok {
		return d.Image()
	}
	return nil
}

// Save pushes transform, clip, and convenience paints.
func (c *Context) Save() {
	c.stack = append(c.stack, c.cur.clone())
}

// Restore pops the last [Save]. Extra restores are ignored.
func (c *Context) Restore() {
	n := len(c.stack)
	if n == 0 {
		return
	}
	c.cur = c.stack[n-1]
	c.stack = c.stack[:n-1]
}

func (s ctxState) clone() ctxState {
	out := s
	out.clip = s.clip.clone()
	out.fill = clonePaint(s.fill)
	out.stroke = clonePaint(s.stroke)
	return out
}

func clonePaint(p Paint) Paint {
	switch g := p.Shader.(type) {
	case LinearGradient:
		p.Shader = cloneLinear(g)
	case RadialGradient:
		p.Shader = cloneRadial(g)
	}
	if p.Stroke.Dash != nil {
		p.Stroke.Dash = append([]float32(nil), p.Stroke.Dash...)
	}
	return p
}

func cloneLinear(g LinearGradient) LinearGradient {
	g.Stops = append([]GradientStop(nil), g.Stops...)
	return g
}

func cloneRadial(g RadialGradient) RadialGradient {
	g.Stops = append([]GradientStop(nil), g.Stops...)
	return g
}

// Matrix returns the current user → device transform.
func (c *Context) Matrix() Matrix { return c.cur.xform }

// SetMatrix replaces the current transform.
func (c *Context) SetMatrix(m Matrix) { c.cur.xform = m }

// Transform post-multiplies m (apply m in user space, then the current xform).
func (c *Context) Transform(m Matrix) { c.cur.xform = c.cur.xform.Mul(m) }

// Translate shifts user space.
func (c *Context) Translate(x, y float32) { c.Transform(Translation(x, y)) }

// Scale scales about the origin of the current user space.
func (c *Context) Scale(sx, sy float32) { c.Transform(Scaling(sx, sy)) }

// Rotate rotates clockwise about the origin, in radians (+Y is down).
func (c *Context) Rotate(radians float32) { c.Transform(Rotation(radians)) }

// SetFill sets the paint used by [Context.FillPath] and [Context.FillRect].
func (c *Context) SetFill(p Paint) {
	p.Style = StyleFill
	c.cur.fill = p
}

// SetStroke sets the paint used by [Context.StrokePath]. Style is forced to stroke.
func (c *Context) SetStroke(p Paint) {
	p.Style = StyleStroke
	if p.Stroke.Width <= 0 {
		p.Stroke = DefaultStroke()
	}
	c.cur.stroke = p
}

// SetColor sets the fill color (and the stroke color, matching JUCE setColour).
func (c *Context) SetColor(col Color) {
	c.cur.fill.Color = col
	c.cur.fill.Shader = nil
	c.cur.stroke.Color = col
	c.cur.stroke.Shader = nil
}

// SetStrokeWidth updates the convenience stroke width.
func (c *Context) SetStrokeWidth(w float32) { c.cur.stroke.Stroke.Width = w }

// SetStrokeStyle updates cap/join/miter on the convenience stroke paint.
func (c *Context) SetStrokeStyle(cap Cap, join Join, miterLimit float32) {
	c.cur.stroke.Stroke.Cap = cap
	c.cur.stroke.Stroke.Join = join
	c.cur.stroke.Stroke.MiterLimit = miterLimit
}

// Clear fills the entire device, ignoring clip (reset-the-surface).
func (c *Context) Clear(col Color) { c.dev.Clear(col) }

// ClipRect intersects the clip with r in user space. If the current matrix
// keeps the rect axis-aligned, this is a scissor; otherwise it is a clip path.
func (c *Context) ClipRect(r Rect) {
	r = r.Canon()
	if r.Empty() {
		c.cur.clip.scissor = Rect{}
		c.cur.clip.hasScissor = true
		return
	}
	if c.cur.xform.IsAxisAligned() {
		dev := c.cur.xform.TransformRect(r)
		if c.cur.clip.hasScissor {
			c.cur.clip.scissor = c.cur.clip.scissor.Intersect(dev)
		} else {
			c.cur.clip.scissor = dev
			c.cur.clip.hasScissor = true
		}
		return
	}
	c.ClipPath(c.rectScratch(r))
}

func (c *Context) pathScratch() *Path {
	if c.scratch == nil {
		c.scratch = NewPath()
	}
	c.scratch.Reset()
	return c.scratch
}

func (c *Context) rectScratch(r Rect) *Path {
	p := c.pathScratch()
	p.AddRect(r)
	return p
}

// ClipPath intersects the clip with path (filled, current fill rule / transform).
func (c *Context) ClipPath(path *Path) {
	if path == nil || path.Empty() {
		c.cur.clip.scissor = Rect{}
		c.cur.clip.hasScissor = true
		return
	}
	w, h := c.dev.Size()
	// Rasterize coverage into a temporary CPU image, then AND with the mask.
	maskImg := NewImage(w, h)
	tmp := NewCPUDevice(maskImg)
	tmp.Fill(path, c.cur.xform, Fill(White), c.cur.clip.export())
	// Convert premul RGB (white * coverage) to A8. White premul means A is coverage.
	newMask := make([]byte, w*h)
	for i := 0; i < w*h; i++ {
		newMask[i] = maskImg.Pix[i*4+3]
	}
	if c.cur.clip.mask == nil {
		c.cur.clip.mask = newMask
		c.cur.clip.maskX, c.cur.clip.maskY = 0, 0
		c.cur.clip.maskW, c.cur.clip.maskH = w, h
	} else {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				a := newMask[y*w+x]
				b := c.cur.clip.export().maskAt(x, y)
				newMask[y*w+x] = uint8(uint16(a) * uint16(b) / 255)
			}
		}
		c.cur.clip.mask = newMask
		c.cur.clip.maskX, c.cur.clip.maskY = 0, 0
		c.cur.clip.maskW, c.cur.clip.maskH = w, h
	}
	// Tighten scissor to the mask's non-zero bounds when cheap enough.
	c.tightenScissorFromMask()
}

func (c *Context) tightenScissorFromMask() {
	m := c.cur.clip
	if m.mask == nil {
		return
	}
	minX, minY := m.maskW, m.maskH
	maxX, maxY := 0, 0
	for y := 0; y < m.maskH; y++ {
		row := m.mask[y*m.maskW : (y+1)*m.maskW]
		for x, v := range row {
			if v == 0 {
				continue
			}
			if x < minX {
				minX = x
			}
			if x+1 > maxX {
				maxX = x + 1
			}
			if y < minY {
				minY = y
			}
			if y+1 > maxY {
				maxY = y + 1
			}
		}
	}
	if minX >= maxX || minY >= maxY {
		c.cur.clip.scissor = Rect{}
		c.cur.clip.hasScissor = true
		return
	}
	box := XYWH(float32(m.maskX+minX), float32(m.maskY+minY), float32(maxX-minX), float32(maxY-minY))
	if c.cur.clip.hasScissor {
		c.cur.clip.scissor = c.cur.clip.scissor.Intersect(box)
	} else {
		c.cur.clip.scissor = box
		c.cur.clip.hasScissor = true
	}
}

func (c *Context) clip() Clip { return c.cur.clip.export() }

// DrawPath fills and/or strokes path according to paint.Style.
func (c *Context) DrawPath(path *Path, paint Paint) {
	if path == nil || path.Empty() {
		return
	}
	switch paint.Style {
	case StyleStroke:
		c.dev.Stroke(path, c.cur.xform, paint, c.clip())
	case StyleStrokeAndFill:
		fill := paint
		fill.Style = StyleFill
		c.dev.Fill(path, c.cur.xform, fill, c.clip())
		c.dev.Stroke(path, c.cur.xform, paint, c.clip())
	default:
		c.dev.Fill(path, c.cur.xform, paint, c.clip())
	}
}

// FillPath fills path with the convenience fill paint.
func (c *Context) FillPath(path *Path) { c.DrawPath(path, c.cur.fill) }

// StrokePath strokes path with the convenience stroke paint.
func (c *Context) StrokePath(path *Path) { c.DrawPath(path, c.cur.stroke) }

// DrawRect draws an axis-aligned rectangle with paint.
func (c *Context) DrawRect(r Rect, paint Paint) {
	c.DrawPath(c.rectScratch(r), paint)
}

// FillRect fills r with the convenience fill paint.
func (c *Context) FillRect(r Rect) { c.DrawRect(r, c.cur.fill) }

// DrawRoundRect draws a rounded rectangle.
func (c *Context) DrawRoundRect(r Rect, rx, ry float32, paint Paint) {
	p := c.pathScratch()
	p.AddRoundRect(r, rx, ry)
	c.DrawPath(p, paint)
}

// DrawOval draws an ellipse inscribed in r.
func (c *Context) DrawOval(r Rect, paint Paint) {
	p := c.pathScratch()
	p.AddEllipse(r.Center(), r.Dx()*0.5, r.Dy()*0.5)
	c.DrawPath(p, paint)
}

// DrawCircle draws a circle.
func (c *Context) DrawCircle(center Point, radius float32, paint Paint) {
	p := c.pathScratch()
	p.AddCircle(center, radius)
	c.DrawPath(p, paint)
}

// DrawLine strokes the segment a→b. Width / caps come from paint.Stroke
// (or a 1px default). Style is forced to stroke.
func (c *Context) DrawLine(a, b Point, paint Paint) {
	paint.Style = StyleStroke
	if paint.Stroke.Width <= 0 {
		paint.Stroke = DefaultStroke()
	}
	p := c.pathScratch()
	p.MoveTo(a.X, a.Y)
	p.LineTo(b.X, b.Y)
	c.DrawPath(p, paint)
}

// DrawImage blits img at (x, y) in user space (1:1 source pixels).
func (c *Context) DrawImage(img *Image, x, y float32) {
	if img == nil {
		return
	}
	src := XYWH(0, 0, float32(img.Width), float32(img.Height))
	c.DrawImageRect(img, src, src.Translate(Pt(x, y)))
}

// DrawImageRect blits src (in image pixels) into dst (user space).
func (c *Context) DrawImageRect(img *Image, src, dst Rect) {
	if img == nil {
		return
	}
	c.dev.Blit(img, src, dst, c.cur.xform, Paint{Color: White}, c.clip())
}

// DrawImageRectPaint is [Context.DrawImageRect] with an alpha modulator
// (paint.Color.A) and optional extra color tint reserved for later.
func (c *Context) DrawImageRectPaint(img *Image, src, dst Rect, paint Paint) {
	if img == nil {
		return
	}
	c.dev.Blit(img, src, dst, c.cur.xform, paint, c.clip())
}
