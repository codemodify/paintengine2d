package paintengine2d

// Node is one entry in a retained [Scene] (a group or a recorded draw).
type Node interface {
	sceneNode()
}

// GroupNode is a transform root with children. A UI layer caches these
// between frames and [Recorder.Attach]es them (scroll offset = new Xform).
type GroupNode struct {
	ID       uint64
	Xform    Matrix
	Children []Node
}

func (*GroupNode) sceneNode() {}

// Scene is a retained display list produced by [Recorder.Finish] and
// replayed with [DrawScene]. Analogous to SkPicture / a Qt Quick node tree.
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

// DrawScene replays s onto dev. CPU replays each op through [Device].
// [GPUDevice] batches consecutive opaque axis-aligned rect fills into one
// draw. A nil scene or device is a no-op.
func DrawScene(s *Scene, dev Device) {
	if s == nil || s.Root == nil || dev == nil {
		return
	}
	w := sceneWalker{dev: dev}
	if g, ok := dev.(*GPUDevice); ok && GPUAvailable() {
		w.batch = g
	}
	w.walk(s.Root, Identity())
	w.flush()
}

type sceneWalker struct {
	dev   Device
	batch rectBatcher
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
		for _, ch := range t.Children {
			w.walk(ch, xf)
		}
	case *drawOp:
		if t == nil {
			return
		}
		switch t.kind {
		case opClear:
			w.flush()
			w.dev.Clear(t.color)
		case opFill:
			xf := acc.Mul(t.xform)
			clip := mapClip(t.clip, acc)
			w.fill(t.path, xf, t.paint, clip)
		case opStroke:
			w.flush()
			w.dev.Stroke(t.path, acc.Mul(t.xform), t.paint, mapClip(t.clip, acc))
		case opBlit:
			w.flush()
			w.dev.Blit(t.src, t.srcR, t.dstR, acc.Mul(t.xform), t.paint, mapClip(t.clip, acc))
		}
	}
}

func (w *sceneWalker) fill(path *Path, xform Matrix, paint Paint, clip Clip) {
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
