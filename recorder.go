package paintengine2d

// Recorder is a [Device] that records draw calls into a retained [Scene]
// instead of rasterizing. Pair it with [NewContextDevice] so widgets paint
// through the usual [Context] API, then [Finish] and [DrawScene].
type Recorder struct {
	w, h   int
	root   *GroupNode
	stack  []*GroupNode
	nodes  int
	reused int
}

// NewRecorder starts an empty recording of a w×h target.
func NewRecorder(w, h int) *Recorder {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	root := &GroupNode{Xform: Identity()}
	return &Recorder{
		w:     w,
		h:     h,
		root:  root,
		stack: []*GroupNode{root},
	}
}

func (r *Recorder) cur() *GroupNode {
	if r == nil || len(r.stack) == 0 {
		return nil
	}
	return r.stack[len(r.stack)-1]
}

func (r *Recorder) add(n Node) {
	g := r.cur()
	if g == nil {
		return
	}
	g.Children = append(g.Children, n)
	r.nodes++
}

// BeginGroup opens a named transform root. The returned node is filled
// until [Recorder.EndGroup] and can be cached for a later [Recorder.Attach].
func (r *Recorder) BeginGroup(id uint64, xform Matrix) *GroupNode {
	if r == nil {
		return nil
	}
	if !xform.Finite() {
		xform = Identity()
	}
	g := &GroupNode{ID: id, Xform: xform}
	r.add(g)
	r.stack = append(r.stack, g)
	return g
}

// EndGroup closes the current [Recorder.BeginGroup]. Extra ends are ignored.
func (r *Recorder) EndGroup() {
	if r == nil || len(r.stack) <= 1 {
		return
	}
	r.stack = r.stack[:len(r.stack)-1]
}

// Attach splices a previously recorded group into the current parent
// (retained-layer reuse). Increments [Scene.Reused].
func (r *Recorder) Attach(g *GroupNode) {
	if r == nil || g == nil {
		return
	}
	r.add(g)
	r.reused++
}

// Finish snapshots the recording. The recorder can still be inspected;
// start a new [NewRecorder] for the next frame.
func (r *Recorder) Finish() *Scene {
	if r == nil {
		return nil
	}
	return &Scene{
		Root:   r.root,
		Nodes:  r.nodes,
		Reused: r.reused,
		W:      r.w,
		H:      r.h,
	}
}

// Size implements [Device].
func (r *Recorder) Size() (w, h int) {
	if r == nil {
		return 0, 0
	}
	return r.w, r.h
}

// Clear implements [Device].
func (r *Recorder) Clear(c Color) {
	if r == nil {
		return
	}
	r.add(&drawOp{kind: opClear, color: c})
}

// Fill implements [Device].
func (r *Recorder) Fill(path *Path, xform Matrix, paint Paint, clip Clip) {
	if r == nil || path == nil || path.Empty() || !xform.Finite() {
		return
	}
	r.add(&drawOp{
		kind:  opFill,
		path:  path.Clone(),
		xform: xform,
		paint: clonePaint(paint),
		clip:  cloneClip(clip),
	})
}

// Stroke implements [Device].
func (r *Recorder) Stroke(path *Path, xform Matrix, paint Paint, clip Clip) {
	if r == nil || path == nil || path.Empty() || !xform.Finite() {
		return
	}
	r.add(&drawOp{
		kind:  opStroke,
		path:  path.Clone(),
		xform: xform,
		paint: clonePaint(paint),
		clip:  cloneClip(clip),
	})
}

// Blit implements [Device].
func (r *Recorder) Blit(src *Image, srcRect, dstRect Rect, xform Matrix, paint Paint, clip Clip) {
	if r == nil || src == nil || dstRect.Empty() || !xform.Finite() {
		return
	}
	r.add(&drawOp{
		kind:  opBlit,
		src:   src,
		srcR:  srcRect,
		dstR:  dstRect,
		xform: xform,
		paint: clonePaint(paint),
		clip:  cloneClip(clip),
	})
}

func cloneClip(c Clip) Clip {
	if c.Mask != nil {
		c.Mask = append([]byte(nil), c.Mask...)
	}
	return c
}

var _ Device = (*Recorder)(nil)
