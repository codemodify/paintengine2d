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
	// intern deduplicates recorded path snapshots: a list of identical
	// rows records one Path, not one per row.
	intern map[uint64][]*Path
	// paths, when set, keeps snapshots across frames (see [PathCache]).
	paths *PathCache
	// slab hands out draw ops in small blocks instead of one allocation
	// per op.
	slab []drawOp
}

// opSlab is the draw ops per block: small, so a cached group that outlives
// its frame pins little memory.
const opSlab = 32

func (r *Recorder) newOp() *drawOp {
	if len(r.slab) == cap(r.slab) {
		r.slab = make([]drawOp, 0, opSlab)
	}
	r.slab = r.slab[:len(r.slab)+1]
	return &r.slab[len(r.slab)-1]
}

// UsePathCache makes the recorder intern paths in c, which outlives it:
// shapes recorded every frame are then cloned once, not once per frame.
func (r *Recorder) UsePathCache(c *PathCache) {
	if r != nil {
		r.paths = c
	}
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
// Call [BakeGroup] on a pane after recording so [DrawScene] blits it
// through [GroupNode.Xform] instead of re-rasterizing children.
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
	if g.backdrop {
		for _, p := range r.stack {
			p.backdrop = true
		}
	}
}

// internPath returns an immutable snapshot of path, reusing an equal one
// recorded earlier in this scene. Recorded paths are never mutated, so
// sharing is safe and a repeated widget shape costs one clone per scene.
func (r *Recorder) internPath(path *Path) *Path {
	if r.paths != nil {
		return r.paths.intern(path)
	}
	h := hashPath(path)
	if r.intern == nil {
		r.intern = make(map[uint64][]*Path)
	}
	for _, c := range r.intern[h] {
		if pathEqual(c, path) {
			return c
		}
	}
	c := path.Clone()
	r.intern[h] = append(r.intern[h], c)
	return c
}

func pathEqual(a, b *Path) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil || len(a.verbs) != len(b.verbs) || len(a.pts) != len(b.pts) {
		return false
	}
	for i := range a.verbs {
		if a.verbs[i] != b.verbs[i] {
			return false
		}
	}
	for i := range a.pts {
		if a.pts[i] != b.pts[i] {
			return false
		}
	}
	return true
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
	op := r.newOp()
	*op = drawOp{kind: opClear, color: c}
	r.add(op)
}

// Fill implements [Device].
func (r *Recorder) Fill(path *Path, xform Matrix, paint Paint, clip Clip) {
	if r == nil || path == nil || path.Empty() || !xform.Finite() {
		return
	}
	op := r.newOp()
	*op = drawOp{
		kind:  opFill,
		path:  r.internPath(path),
		xform: xform,
		paint: clonePaint(paint),
		clip:  cloneClip(clip),
	}
	r.add(op)
}

// Stroke implements [Device].
func (r *Recorder) Stroke(path *Path, xform Matrix, paint Paint, clip Clip) {
	if r == nil || path == nil || path.Empty() || !xform.Finite() {
		return
	}
	op := r.newOp()
	*op = drawOp{
		kind:  opStroke,
		path:  r.internPath(path),
		xform: xform,
		paint: clonePaint(paint),
		clip:  cloneClip(clip),
	}
	r.add(op)
}

// Blit implements [Device].
func (r *Recorder) Blit(src *Image, srcRect, dstRect Rect, xform Matrix, paint Paint, clip Clip) {
	if r == nil || src == nil || dstRect.Empty() || !xform.Finite() {
		return
	}
	op := r.newOp()
	*op = drawOp{
		kind:  opBlit,
		src:   src,
		srcR:  srcRect,
		dstR:  dstRect,
		xform: xform,
		paint: clonePaint(paint),
		clip:  cloneClip(clip),
	}
	r.add(op)
}

// BackdropBlur implements [BackdropBlurrer]: the blur is replayed on the
// target device, and every group up to the root is marked so damage over
// its region repaints the whole region first.
func (r *Recorder) BackdropBlur(rect Rect, xform Matrix, radius float32, clip Clip) {
	if r == nil || rect.Empty() || radius <= 0 || !xform.Finite() {
		return
	}
	op := r.newOp()
	*op = drawOp{kind: opBackdrop, dstR: rect, xform: xform, radius: radius, clip: cloneClip(clip)}
	r.add(op)
	for _, g := range r.stack {
		g.backdrop = true
	}
}

// cloneClip retains the recorded clip. The coverage mask is shared by
// reference: [Context.clip] marks an exported mask immutable, so the next
// ClipPath allocates a fresh buffer instead of overwriting these bytes.
// Deep-copying here cost one full mask per recorded op — a 33-glyph label
// under a round-rect clip allocated megabytes every frame.
func cloneClip(c Clip) Clip { return c }

var _ Device = (*Recorder)(nil)
