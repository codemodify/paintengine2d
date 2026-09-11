package paintengine2d

// Context is the public 2D canvas — the analogue of Skia's SkCanvas and
// JUCE's Graphics. It owns a transform / clip / paint stack and forwards
// drawing to a [Device].
//
// This is the stable paint surface for two future consumers (not in this
// repo): a desktop UI framework (widget Paint into a buffer/swapchain) and
// a desktop environment / window manager (borders, titlebars, panels into
// X11/Wayland surfaces). Platform windowing stays out of this package.
//
// A Context is not safe for concurrent use.
type Context struct {
	dev     Device
	stack   []ctxState
	cur     ctxState
	scratch *Path // reused by DrawRect / DrawCircle / …
	damage  *Damage
	// clipScratch is a reusable pixmap for ClipPath coverage.
	clipScratch *Image
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

// Image returns a CPU-readable pixmap. [CPUDevice] returns the live target;
// [GPUDevice] returns a read-back snapshot. Other devices return nil.
func (c *Context) Image() *Image {
	if d, ok := c.dev.(*CPUDevice); ok {
		return d.Image()
	}
	type imager interface{ Image() *Image }
	if i, ok := c.dev.(imager); ok {
		return i.Image()
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

// SaveCount is the number of unmatched [Save] calls (0 at the root).
func (c *Context) SaveCount() int { return len(c.stack) }

// Size is the device pixmap size in pixels.
func (c *Context) Size() (w, h int) { return c.dev.Size() }

// DeviceClipBounds is the current clip as a device-space AABB (canvas ∩ scissor).
// A path mask may be tighter; this box is conservative and allocation-free.
func (c *Context) DeviceClipBounds() Rect {
	w, h := c.dev.Size()
	canvas := XYWH(0, 0, float32(w), float32(h))
	if c.cur.clip.hasScissor {
		return canvas.Intersect(c.cur.clip.scissor)
	}
	return canvas
}

// LocalClipBounds maps [Context.DeviceClipBounds] into user space.
// Returns empty if the current matrix is not invertible.
func (c *Context) LocalClipBounds() Rect {
	dev := c.DeviceClipBounds()
	if dev.Empty() {
		return Rect{}
	}
	inv, ok := c.cur.xform.Invert()
	if !ok {
		return Rect{}
	}
	return inv.TransformRect(dev)
}

// QuickReject reports whether r (user space) lies fully outside the current
// clip. A retained UI layer can skip painting a child whose bounds reject.
// The test uses the scissor AABB (conservative if a path mask is active).
func (c *Context) QuickReject(r Rect) bool {
	if r.Empty() || !r.Finite() || !c.cur.xform.Finite() {
		return true
	}
	dev := c.cur.xform.TransformRect(r)
	if !dev.Finite() {
		return true
	}
	return !dev.Overlaps(c.DeviceClipBounds())
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

// SetMatrix replaces the current transform. Non-finite matrices are ignored.
func (c *Context) SetMatrix(m Matrix) {
	if m.Finite() {
		c.cur.xform = m
	}
}

// Transform post-multiplies m (apply m in user space, then the current xform).
// Non-finite matrices are ignored.
func (c *Context) Transform(m Matrix) {
	if m.Finite() {
		c.cur.xform = c.cur.xform.Mul(m)
	}
}

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
// Width <= 0 is kept as-is and makes a later [Context.StrokePath] a no-op.
func (c *Context) SetStroke(p Paint) {
	p.Style = StyleStroke
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
func (c *Context) Clear(col Color) {
	c.dev.Clear(col)
	c.markDirtyDevice()
}

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

// ClipEmpty reports whether the current clip has no remaining device pixels.
// A retained UI layer can skip painting after a clip that missed the canvas.
func (c *Context) ClipEmpty() bool {
	return c.DeviceClipBounds().Empty()
}

// ClipRoundRect intersects the clip with a rounded rectangle in user space.
func (c *Context) ClipRoundRect(r Rect, rx, ry float32) {
	p := c.pathScratch()
	p.AddRoundRect(r, rx, ry)
	c.ClipPath(p)
}

// ClipPath intersects the clip with path filled with the non-zero rule.
// Typical UI clips are rounded rects and circles. For even-odd clips
// (stars, compound holes) use [Context.ClipPathRule].
func (c *Context) ClipPath(path *Path) {
	c.ClipPathRule(path, FillNonZero)
}

// ClipPathRule is [Context.ClipPath] with an explicit interior rule.
func (c *Context) ClipPathRule(path *Path, rule FillRule) {
	if path == nil || path.Empty() || !c.cur.xform.Finite() {
		c.cur.clip.scissor = Rect{}
		c.cur.clip.hasScissor = true
		return
	}
	w, h := c.dev.Size()
	canvas := XYWH(0, 0, float32(w), float32(h))
	box := c.cur.xform.TransformRect(path.Bounds()).Inset(-2)
	if c.cur.clip.hasScissor {
		box = box.Intersect(c.cur.clip.scissor)
	}
	box = box.Intersect(canvas)
	if box.Empty() {
		c.cur.clip.scissor = Rect{}
		c.cur.clip.hasScissor = true
		c.cur.clip.mask = nil
		return
	}
	x0, y0, x1, y1 := clampPixelBounds(box, w, h)
	if x0 >= x1 || y0 >= y1 {
		c.cur.clip.scissor = Rect{}
		c.cur.clip.hasScissor = true
		c.cur.clip.mask = nil
		return
	}
	bw, bh := x1-x0, y1-y0
	maskImg := c.ensureClipScratch(bw, bh)
	tmp := NewCPUDevice(maskImg)
	shifted := Translation(-float32(x0), -float32(y0)).Mul(c.cur.xform)
	clip := c.cur.clip.export()
	if clip.HasScissor {
		clip.Scissor = clip.Scissor.Translate(Pt(-float32(x0), -float32(y0)))
	}
	if clip.Mask != nil {
		clip.MaskX -= x0
		clip.MaskY -= y0
	}
	tmp.Clear(Transparent)
	maskPaint := Fill(White)
	maskPaint.FillRule = rule
	tmp.Fill(path, shifted, maskPaint, clip)
	newMask := make([]byte, bw*bh)
	for i := 0; i < bw*bh; i++ {
		newMask[i] = maskImg.Pix[i*4+3]
	}
	if c.cur.clip.mask != nil {
		prev := c.cur.clip.export()
		for y := 0; y < bh; y++ {
			for x := 0; x < bw; x++ {
				b := prev.maskAt(x0+x, y0+y)
				newMask[y*bw+x] = uint8(uint16(newMask[y*bw+x]) * uint16(b) / 255)
			}
		}
	}
	c.cur.clip.mask = newMask
	c.cur.clip.maskX, c.cur.clip.maskY = x0, y0
	c.cur.clip.maskW, c.cur.clip.maskH = bw, bh
	c.tightenScissorFromMask()
}

func (c *Context) ensureClipScratch(w, h int) *Image {
	need := w * h * 4
	if c.clipScratch == nil {
		c.clipScratch = NewImage(w, h)
		return c.clipScratch
	}
	if cap(c.clipScratch.Pix) < need {
		c.clipScratch.Pix = make([]byte, need)
	} else {
		c.clipScratch.Pix = c.clipScratch.Pix[:need]
	}
	c.clipScratch.Width = w
	c.clipScratch.Height = h
	c.clipScratch.Stride = w * 4
	return c.clipScratch
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

// SetDamage attaches a dirty-rect tracker. Draw calls record device-space
// bounds; the UI layer presents [Damage.Rects] or [Damage.Bounds].
func (c *Context) SetDamage(d *Damage) { c.damage = d }

// Damage returns the tracker set by [Context.SetDamage], or nil.
func (c *Context) Damage() *Damage { return c.damage }

func (c *Context) markDirtyUser(r Rect) {
	if c.damage == nil || r.Empty() || !r.Finite() || !c.cur.xform.Finite() {
		return
	}
	dev := c.cur.xform.TransformRect(r)
	if !dev.Finite() {
		return
	}
	if c.cur.clip.hasScissor {
		dev = dev.Intersect(c.cur.clip.scissor)
	}
	w, h := c.dev.Size()
	dev = dev.Intersect(XYWH(0, 0, float32(w), float32(h)))
	c.damage.Add(dev)
}

func (c *Context) markDirtyDevice() {
	if c.damage == nil {
		return
	}
	w, h := c.dev.Size()
	c.damage.Add(XYWH(0, 0, float32(w), float32(h)))
}

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
	b := path.Bounds()
	if paint.Style != StyleFill && paint.Stroke.Width > 0 {
		pad := paint.Stroke.Width
		if paint.Stroke.Join == JoinMiter && paint.Stroke.MiterLimit > 1 {
			if m := paint.Stroke.Width * paint.Stroke.MiterLimit * 0.5; m > pad {
				pad = m
			}
		}
		b = b.Inset(-pad)
	}
	c.markDirtyUser(b)
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

// StrokeRect strokes r with the convenience stroke paint.
func (c *Context) StrokeRect(r Rect) { c.DrawRect(r, c.cur.stroke) }

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

// DrawArc draws an elliptical arc centered at center. start and sweep are
// radians (0 = +X, positive clockwise). Style comes from paint (stroke is
// the usual UI choice for progress rings and knobs).
func (c *Context) DrawArc(center Point, rx, ry, start, sweep float32, paint Paint) {
	p := c.pathScratch()
	p.AddArc(center, rx, ry, start, sweep)
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
	c.markDirtyUser(dst)
}

// DrawImageRectPaint is [Context.DrawImageRect] with a color tint:
// paint.Color RGB multiplies premul blit samples; Color.A modulates coverage.
// A white atlas or icon sheet can be themed by setting Color to the UI accent.
func (c *Context) DrawImageRectPaint(img *Image, src, dst Rect, paint Paint) {
	if img == nil {
		return
	}
	c.dev.Blit(img, src, dst, c.cur.xform, paint, c.clip())
	c.markDirtyUser(dst)
}
