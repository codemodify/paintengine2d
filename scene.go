package paintengine2d

// Node is one entry in a retained [Scene] (a group or a recorded draw).
type Node interface {
	sceneNode()
}

// GroupNode is a transform root with children. A UI layer caches these
// between frames and [Recorder.Attach]es them (scroll offset / splitter
// slide = new Xform).
//
// [BakeGroup] snapshots Children into Layer (local space). [DrawScene]
// then blits the layer through Xform and does not walk Children — a
// splitter drag is two blits, not two pane rasters. An opaque integer
// translation uses [Image.CopyFrom] (memcpy) on the CPU backend.
type GroupNode struct {
	ID          uint64
	Xform       Matrix
	Children    []Node
	Layer       *Image
	LayerOrigin Point
	LayerOpaque bool
}

func (*GroupNode) sceneNode() {}

// HasLayer reports whether [BakeGroup] stored a replay pixmap.
func (g *GroupNode) HasLayer() bool { return g != nil && g.Layer != nil }

// InvalidateLayer drops a baked pixmap so the next [DrawScene] walks
// Children again. Call after mutating Children.
func (g *GroupNode) InvalidateLayer() {
	if g == nil {
		return
	}
	g.Layer = nil
	g.LayerOrigin = Point{}
	g.LayerOpaque = false
}

// Scene is a retained display list produced by [Recorder.Finish] and
// replayed with [DrawScene] / [DrawSceneDamage]. Analogous to SkPicture /
// a Qt Quick node tree.
//
// Nodes is the number of groups and draw ops recorded (including Attach).
// Reused counts [Recorder.Attach] hits.
type Scene struct {
	Root   *GroupNode
	Nodes  int
	Reused int
	W, H   int
}

const (
	opClear uint8 = iota
	opFill
	opStroke
	opBlit
)

type drawOp struct {
	kind       uint8
	path       *Path
	xform      Matrix
	paint      Paint
	clip       Clip
	src        *Image
	srcR, dstR Rect
	color      Color
}

func (*drawOp) sceneNode() {}

type rectBatcher interface {
	fillOpaqueRects(rects []Rect, color Color, clip Clip)
}

type clearRector interface {
	ClearRect(Rect, Color)
}

type presentDamager interface {
	SetPresentDamage([]Rect)
}

// DrawScene replays s onto dev (full surface). Same as
// [DrawSceneDamage] with a nil dirty tracker. [GPUDevice] batches
// consecutive opaque axis-aligned rect fills. A nil scene or device is
// a no-op.
func DrawScene(s *Scene, dev Device) {
	DrawSceneDamage(s, dev, nil)
}

// DrawSceneRects replays s clipped to rects (device pixels). An empty
// list is a full replay ([DrawScene]).
func DrawSceneRects(s *Scene, dev Device, rects []Rect) {
	if len(rects) == 0 {
		DrawSceneDamage(s, dev, nil)
		return
	}
	var d Damage
	for _, r := range rects {
		d.Add(r)
	}
	DrawSceneDamage(s, dev, &d)
}

// DrawSceneDamage replays s onto the union of dirty (device pixels).
//
// A nil dirty is a full replay. An empty dirty is a no-op (and tells
// [GPUDevice.Present] to skip the swap). Recorded [Device.Clear] becomes
// [ClearRect] of each dirty box so a hover does not wipe the surface.
//
// Ops whose device bounds miss every dirty rect are skipped. Surviving
// ops get an extra device scissor of [Damage.Bounds]. Groups with a
// [GroupNode.Layer] are blitted; their children are not walked.
//
// After replay, a [GPUDevice] stores the dirty list so [GPUDevice.Present]
// / [Context.Present] can swap-with-damage without a second copy of the
// rects. uitoolkit should call this instead of Clear + [DrawScene].
func DrawSceneDamage(s *Scene, dev Device, dirty *Damage) {
	if s == nil || s.Root == nil || dev == nil {
		return
	}
	if dirty != nil && dirty.Empty() {
		setPresentDamage(dev, emptyPresent)
		return
	}
	w := sceneWalker{dev: dev, dirty: dirty}
	if g, ok := dev.(*GPUDevice); ok && GPUAvailable() {
		w.batch = g
	}
	w.walk(s.Root, Identity())
	w.flush()
	if dirty == nil {
		setPresentDamage(dev, nil)
		return
	}
	setPresentDamage(dev, dirty.Rects)
}

// emptyPresent is a non-nil empty slice: SetPresentDamage skips the swap.
var emptyPresent = []Rect{}

func setPresentDamage(dev Device, rects []Rect) {
	if p, ok := dev.(presentDamager); ok {
		p.SetPresentDamage(rects)
	}
}

type sceneWalker struct {
	dev   Device
	batch rectBatcher
	dirty *Damage
	rects []Rect
	color Color
	clip  Clip
	has   bool
}

func (w *sceneWalker) walk(n Node, acc Matrix) {
	switch t := n.(type) {
	case *GroupNode:
		if t == nil {
			return
		}
		xf := acc.Mul(t.Xform)
		if t.Layer != nil {
			w.blitLayer(t, xf)
			return
		}
		if w.skipGroup(t, xf) {
			return
		}
		for _, ch := range t.Children {
			w.walk(ch, xf)
		}
	case *drawOp:
		if t == nil {
			return
		}
		switch t.kind {
		case opClear:
			w.clear(t.color)
		case opFill:
			xf := acc.Mul(t.xform)
			if w.skipRect(opDeviceBounds(t, acc)) {
				return
			}
			w.fill(t.path, xf, t.paint, w.clipOp(t.clip, acc))
		case opStroke:
			if w.skipRect(opDeviceBounds(t, acc)) {
				return
			}
			w.flush()
			w.dev.Stroke(t.path, acc.Mul(t.xform), t.paint, w.clipOp(t.clip, acc))
		case opBlit:
			if w.skipRect(opDeviceBounds(t, acc)) {
				return
			}
			w.flush()
			w.dev.Blit(t.src, t.srcR, t.dstR, acc.Mul(t.xform), t.paint, w.clipOp(t.clip, acc))
		}
	}
}

func (w *sceneWalker) skipGroup(g *GroupNode, xf Matrix) bool {
	if w.dirty == nil {
		return false
	}
	b := GroupLocalBounds(g)
	if b.Empty() {
		return false
	}
	return w.skipRect(xf.TransformRect(b))
}

func (w *sceneWalker) skipRect(r Rect) bool {
	if w.dirty == nil {
		return false
	}
	return !w.dirty.Overlaps(r)
}

func (w *sceneWalker) clipOp(c Clip, acc Matrix) Clip {
	clip := mapClip(c, acc)
	if w.dirty == nil {
		return clip
	}
	return clip.IntersectDevice(w.dirty.Bounds())
}

func (w *sceneWalker) clear(c Color) {
	w.flush()
	if w.dirty == nil {
		w.dev.Clear(c)
		return
	}
	if clr, ok := w.dev.(clearRector); ok {
		for _, r := range w.dirty.Rects {
			clr.ClearRect(r, c)
		}
		return
	}
	p := Fill(c)
	for _, r := range w.dirty.Rects {
		if r.Empty() {
			continue
		}
		w.dev.Fill(RectPath(r), Identity(), p, Clip{HasScissor: true, Scissor: r})
	}
}

func (w *sceneWalker) blitLayer(g *GroupNode, xf Matrix) {
	src := g.Layer
	if src == nil {
		return
	}
	srcR := XYWH(0, 0, float32(src.Width), float32(src.Height))
	xform := xf.Mul(Translation(g.LayerOrigin.X, g.LayerOrigin.Y))
	dest := xform.TransformRect(srcR)
	if w.skipRect(dest) {
		return
	}
	w.flush()
	if g.LayerOpaque && w.copyOpaqueLayer(src, dest, xform) {
		return
	}
	w.dev.Blit(src, srcR, srcR, xform, Paint{Color: White, Filter: FilterNearest}, w.clipOp(Clip{}, Identity()))
}

func (w *sceneWalker) copyOpaqueLayer(src *Image, dest Rect, xform Matrix) bool {
	cpu, ok := w.dev.(*CPUDevice)
	if !ok || cpu.img == nil || !xform.IsTranslation() {
		return false
	}
	dx, okX := nearInt(xform.E)
	dy, okY := nearInt(xform.F)
	if !okX || !okY {
		return false
	}
	if w.dirty != nil {
		copied := false
		for _, r := range w.dirty.Rects {
			part := dest.Intersect(r)
			if part.Empty() {
				continue
			}
			ox := int(part.Min.X) - dx
			oy := int(part.Min.Y) - dy
			cpu.img.CopyFrom(src, XYWH(float32(ox), float32(oy), part.Dx(), part.Dy()), int(part.Min.X), int(part.Min.Y))
			copied = true
		}
		return copied
	}
	cpu.img.CopyFrom(src, XYWH(0, 0, float32(src.Width), float32(src.Height)), dx, dy)
	return true
}

func (w *sceneWalker) fill(path *Path, xform Matrix, paint Paint, clip Clip) {
	if clip.HasScissor && clip.Scissor.Empty() {
		return
	}
	if w.batch != nil && opaqueAxisAlignedRect(path, xform, paint, clip) {
		r := xform.TransformRect(path.Bounds())
		if w.has && (w.color != paint.Color || !clipBatchEqual(w.clip, clip)) {
			w.flush()
		}
		w.color = paint.Color
		w.clip = clip
		w.rects = append(w.rects, r)
		w.has = true
		return
	}
	w.flush()
	w.dev.Fill(path, xform, paint, clip)
}

func (w *sceneWalker) flush() {
	if !w.has || len(w.rects) == 0 {
		return
	}
	if w.batch != nil {
		w.batch.fillOpaqueRects(w.rects, w.color, w.clip)
	} else {
		p := Fill(w.color)
		for _, r := range w.rects {
			w.dev.Fill(RectPath(r), Identity(), p, w.clip)
		}
	}
	w.rects = w.rects[:0]
	w.has = false
}

func opaqueAxisAlignedRect(path *Path, xform Matrix, paint Paint, clip Clip) bool {
	if paint.Shader != nil || clip.Mask != nil || !xform.IsAxisAligned() || !isClosedRectPath(path) {
		return false
	}
	_, _, _, a := paint.Color.Premul8()
	return a == 255
}

func clipBatchEqual(a, b Clip) bool {
	if a.HasScissor != b.HasScissor || a.Mask != nil || b.Mask != nil {
		return false
	}
	if a.HasScissor && a.Scissor != b.Scissor {
		return false
	}
	return true
}

func mapClip(c Clip, m Matrix) Clip {
	if m.IsIdentity() {
		return c
	}
	if c.HasScissor {
		c.Scissor = m.TransformRect(c.Scissor)
	}
	if c.Mask != nil && m.IsTranslation() {
		c.MaskX += int(m.E)
		c.MaskY += int(m.F)
	} else if c.Mask != nil {
		c.Mask = nil
	}
	return c
}

func opContentBounds(op *drawOp, acc Matrix) Rect {
	if op == nil {
		return Rect{}
	}
	xf := acc.Mul(op.xform)
	switch op.kind {
	case opFill:
		if op.path == nil {
			return Rect{}
		}
		return xf.TransformRect(op.path.Bounds())
	case opStroke:
		if op.path == nil {
			return Rect{}
		}
		return xf.TransformRect(strokePadBounds(op.path.Bounds(), op.paint.Stroke))
	case opBlit:
		return xf.TransformRect(op.dstR)
	default:
		return Rect{}
	}
}

func opDeviceBounds(op *drawOp, acc Matrix) Rect {
	b := opContentBounds(op, acc)
	if b.Empty() || (op != nil && op.kind == opBlit) {
		return b
	}
	return b.Inset(-1)
}

func strokePadBounds(b Rect, s Stroke) Rect {
	if s.Width <= 0 {
		return b
	}
	pad := s.Width
	if s.Join == JoinMiter && s.MiterLimit > 1 {
		if m := s.Width * s.MiterLimit * 0.5; m > pad {
			pad = m
		}
	}
	return b.Inset(-pad)
}
