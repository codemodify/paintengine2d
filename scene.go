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
//
// # Clipping
//
// Clip / ClipPath are expressed in the *parent's* coordinate space: they are
// applied with the matrix accumulated down to this node's parent and are
// deliberately NOT multiplied by this node's own Xform. A scroll view is
// therefore a fixed viewport (the group Clip) plus a moving content
// transform (Xform): changing Xform slides the children under a clip that
// stays where it is.
//
// Clips recorded into the children (the per-op [Clip] a [Context] resolves)
// belong to the content and do move with Xform, as they always have. Put the
// viewport on the group, not on the ops, when the group will be re-attached
// with a different Xform.
//
// The zero value has no clip, so scenes built before v0.11 replay unchanged.
//
// A ClipPath is rasterized to a mask and cached on the node, so a Scene is
// replayed by one goroutine at a time (as it always was).
type GroupNode struct {
	ID       uint64
	Xform    Matrix
	Children []Node

	// Clip is a parent-space rectangle. It only applies when HasClip is
	// set, so an empty Clip can legitimately mean "nothing is visible".
	Clip    Rect
	HasClip bool
	// ClipPath, when non-nil, further restricts the group to the interior
	// of a parent-space path (rounded viewports, circular avatars). It is
	// rasterized to a coverage mask and cached until the path, the rule or
	// the parent matrix changes.
	ClipPath *Path
	ClipRule FillRule

	Layer       *Image
	LayerOrigin Point
	LayerOpaque bool

	// backdrop marks a subtree that blurs what lies under it
	// ([Context.BackdropBlur]): damage touching its region must repaint
	// the whole region, or the blur would read its own old output.
	backdrop bool

	clipCache groupClipCache
}

// groupClipCache memoizes the rasterized GroupNode.ClipPath mask.
type groupClipCache struct {
	valid bool
	m     Matrix
	path  *Path
	rule  FillRule
	clip  Clip
}

// SetClipRect sets a parent-space rectangular clip on the group.
func (g *GroupNode) SetClipRect(r Rect) {
	if g == nil {
		return
	}
	g.Clip = r.Canon()
	g.HasClip = true
	g.clipCache.valid = false
}

// SetClipPath sets a parent-space path clip (intersected with Clip when
// HasClip is set). A nil path clears it.
func (g *GroupNode) SetClipPath(p *Path, rule FillRule) {
	if g == nil {
		return
	}
	g.ClipPath = p
	g.ClipRule = rule
	g.clipCache.valid = false
}

// ClearClip removes the group clip.
func (g *GroupNode) ClearClip() {
	if g == nil {
		return
	}
	g.Clip = Rect{}
	g.HasClip = false
	g.ClipPath = nil
	g.clipCache = groupClipCache{}
}

// hasGroupClip reports whether g restricts its children.
func (g *GroupNode) hasGroupClip() bool {
	return g != nil && (g.HasClip || g.ClipPath != nil)
}

// deviceClipFor resolves the group clip into device space under m (the
// matrix accumulated down to the group's parent).
func (g *GroupNode) deviceClipFor(m Matrix, w, h int) Clip {
	var out Clip
	if g.HasClip {
		out.HasScissor = true
		out.Scissor = m.TransformRect(g.Clip)
	}
	if g.ClipPath == nil || g.ClipPath.Empty() {
		return out
	}
	if c := &g.clipCache; c.valid && c.path == g.ClipPath && c.rule == g.ClipRule && c.m == m {
		return intersectClips(out, c.clip)
	}
	mask := rasterizeClipPath(g.ClipPath, g.ClipRule, m, w, h)
	g.clipCache = groupClipCache{valid: true, m: m, path: g.ClipPath, rule: g.ClipRule, clip: mask}
	return intersectClips(out, mask)
}

// rasterizeClipPath renders path (user space, mapped by m) into an 8-bit
// coverage clip in device space.
func rasterizeClipPath(path *Path, rule FillRule, m Matrix, w, h int) Clip {
	box := m.TransformRect(path.Bounds()).Inset(-2)
	box = box.Intersect(XYWH(0, 0, float32(w), float32(h)))
	if box.Empty() {
		return Clip{HasScissor: true}
	}
	x0, y0, x1, y1 := clampPixelBounds(box, w, h)
	if x0 >= x1 || y0 >= y1 {
		return Clip{HasScissor: true}
	}
	bw, bh := x1-x0, y1-y0
	img := NewImage(bw, bh)
	dev := NewCPUDevice(img)
	paint := Fill(White)
	paint.FillRule = rule
	dev.Fill(path, Translation(-float32(x0), -float32(y0)).Mul(m), paint, Clip{})
	mask := make([]byte, bw*bh)
	for i := range mask {
		mask[i] = img.Pix[i*4+3]
	}
	return Clip{
		HasScissor: true,
		Scissor:    XYWH(float32(x0), float32(y0), float32(bw), float32(bh)),
		Mask:       mask,
		MaskX:      x0,
		MaskY:      y0,
		MaskW:      bw,
		MaskH:      bh,
	}
}

// intersectClips combines two device-space clips.
func intersectClips(a, b Clip) Clip {
	switch {
	case a.Mask == nil && b.Mask == nil:
		if !a.HasScissor {
			return b
		}
		if !b.HasScissor {
			return a
		}
		return Clip{HasScissor: true, Scissor: a.Scissor.Intersect(b.Scissor)}
	case a.Mask == nil:
		if a.HasScissor {
			b.Scissor = scissorOf(b).Intersect(a.Scissor)
			b.HasScissor = true
		}
		return b
	case b.Mask == nil:
		if b.HasScissor {
			a.Scissor = scissorOf(a).Intersect(b.Scissor)
			a.HasScissor = true
		}
		return a
	}
	// Two masks: multiply them over the intersection box.
	box := scissorOf(a).Intersect(scissorOf(b))
	if box.Empty() {
		return Clip{HasScissor: true}
	}
	x0, y0, x1, y1 := box.IntBounds()
	bw, bh := x1-x0, y1-y0
	if bw <= 0 || bh <= 0 {
		return Clip{HasScissor: true}
	}
	mask := make([]byte, bw*bh)
	for y := 0; y < bh; y++ {
		for x := 0; x < bw; x++ {
			va := a.maskAt(x0+x, y0+y)
			vb := b.maskAt(x0+x, y0+y)
			mask[y*bw+x] = uint8((uint16(va)*uint16(vb) + 127) / 255)
		}
	}
	return Clip{
		HasScissor: true,
		Scissor:    XYWH(float32(x0), float32(y0), float32(bw), float32(bh)),
		Mask:       mask,
		MaskX:      x0,
		MaskY:      y0,
		MaskW:      bw,
		MaskH:      bh,
	}
}

func scissorOf(c Clip) Rect {
	if c.HasScissor {
		return c.Scissor
	}
	if c.Mask != nil {
		return XYWH(float32(c.MaskX), float32(c.MaskY), float32(c.MaskW), float32(c.MaskH))
	}
	return Rect{Min: Point{-1e9, -1e9}, Max: Point{1e9, 1e9}}
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
	opBackdrop
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
	radius     float32 // opBackdrop: the blur's standard deviation, user units
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

// DrawSceneDamage replays s onto the dirty boxes (device pixels).
//
// A nil dirty is a full replay. An empty dirty is a no-op (and tells
// [GPUDevice.Present] to skip the swap). Recorded [Device.Clear] becomes
// [ClearRect] of the dirty box, so a hover does not wipe the surface.
//
// The scene is replayed once per dirty rectangle, each pass scissored to
// that one box. Replaying once against the union would composite a
// translucent op twice into the gaps between boxes (it is painted for every
// box it overlaps, but only the boxes are erased first).
//
// Ops whose device bounds miss the box are skipped, as are whole groups —
// except groups that contain a recorded [Device.Clear], which must run so
// the background is restored where a widget was removed.
//
// After replay, a [GPUDevice] stores the dirty list so [GPUDevice.Present]
// / [Context.Present] can swap-with-damage without a second copy of the
// rects.
//
// A box that touches a [Context.BackdropBlur] region grows to cover the
// whole region (the content under a blur is repainted before the blur
// reads it); dirty then holds the grown boxes, which are what the caller
// must present.
func DrawSceneDamage(s *Scene, dev Device, dirty *Damage) {
	if s == nil || s.Root == nil || dev == nil {
		return
	}
	if dirty != nil && dirty.Empty() {
		setPresentDamage(dev, emptyPresent)
		return
	}
	var batch rectBatcher
	if g, ok := dev.(*GPUDevice); ok && gpuUsable(g) {
		batch = g
	}
	clears := make(map[*GroupNode]bool)
	if dirty == nil {
		w := sceneWalker{dev: dev, batch: batch, clears: clears}
		w.walk(s.Root, Identity())
		w.flush()
		setPresentDamage(dev, nil)
		return
	}
	// Snapshot: a Device call must not observe a half-mutated list.
	rects := widenForBackdrops(s, append([]Rect(nil), dirty.Rects...))
	if s.Root.backdrop {
		dirty.Rects = append(dirty.Rects[:0], rects...)
	}
	for _, r := range rects {
		if r.Empty() {
			continue
		}
		w := sceneWalker{dev: dev, batch: batch, clears: clears, rect: r, hasRect: true}
		w.walk(s.Root, Identity())
		w.flush()
	}
	setPresentDamage(dev, rects)
}

// emptyPresent is a non-nil empty slice: SetPresentDamage skips the swap.
var emptyPresent = []Rect{}

func setPresentDamage(dev Device, rects []Rect) {
	if p, ok := dev.(presentDamager); ok {
		p.SetPresentDamage(rects)
	}
}

type sceneWalker struct {
	dev     Device
	batch   rectBatcher
	rect    Rect
	hasRect bool
	// gclip is the accumulated device-space clip from enclosing
	// [GroupNode.Clip] / ClipPath.
	gclip  Clip
	hasG   bool
	clears map[*GroupNode]bool
	rects  []Rect
	color  Color
	clip   Clip
	has    bool
}

func (w *sceneWalker) walk(n Node, acc Matrix) {
	switch t := n.(type) {
	case *GroupNode:
		if t == nil {
			return
		}
		xf := acc.Mul(t.Xform)
		savedClip, savedHas := w.gclip, w.hasG
		if t.hasGroupClip() {
			dw, dh := w.dev.Size()
			gc := t.deviceClipFor(acc, dw, dh)
			if w.hasG {
				w.gclip = intersectClips(w.gclip, gc)
			} else {
				w.gclip, w.hasG = gc, true
			}
			if w.hasRect && w.gclip.HasScissor && !w.gclip.Scissor.Overlaps(w.rect) {
				w.gclip, w.hasG = savedClip, savedHas
				return
			}
			w.flush()
		}
		if t.Layer != nil {
			w.blitLayer(t, xf)
		} else if !w.skipGroup(t, xf) {
			for _, ch := range t.Children {
				w.walk(ch, xf)
			}
		}
		if t.hasGroupClip() {
			w.flush()
			w.gclip, w.hasG = savedClip, savedHas
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
		case opBackdrop:
			b, ok := w.dev.(BackdropBlurrer)
			if !ok || w.skipRect(opDeviceBounds(t, acc)) {
				return
			}
			w.flush()
			b.BackdropBlur(t.dstR, acc.Mul(t.xform), t.radius, w.clipOp(t.clip, acc))
		}
	}
}

// skipGroup reports whether the whole subtree misses the dirty box. A
// group that contains a recorded Clear is never skipped: the Clear repaints
// the background where an op used to be, and its bounds are not part of
// GroupLocalBounds.
func (w *sceneWalker) skipGroup(g *GroupNode, xf Matrix) bool {
	if !w.hasRect {
		return false
	}
	if w.groupHasClear(g) {
		return false
	}
	b := GroupLocalBounds(g)
	if b.Empty() {
		return false
	}
	return w.skipRect(xf.TransformRect(b))
}

// groupHasClear reports (memoized) whether the subtree records a Clear.
func (w *sceneWalker) groupHasClear(g *GroupNode) bool {
	if g == nil {
		return false
	}
	if w.clears != nil {
		if v, ok := w.clears[g]; ok {
			return v
		}
	}
	found := false
	for _, ch := range g.Children {
		switch t := ch.(type) {
		case *drawOp:
			if t != nil && t.kind == opClear {
				found = true
			}
		case *GroupNode:
			if t != nil && t.Layer == nil && w.groupHasClear(t) {
				found = true
			}
		}
		if found {
			break
		}
	}
	if w.clears != nil {
		w.clears[g] = found
	}
	return found
}

func (w *sceneWalker) skipRect(r Rect) bool {
	if !w.hasRect {
		return false
	}
	return !r.Overlaps(w.rect)
}

// clipOp resolves a recorded op clip: mapped by the accumulated matrix,
// intersected with any enclosing group clip and with the dirty box.
func (w *sceneWalker) clipOp(c Clip, acc Matrix) Clip {
	clip := mapClip(c, acc)
	if w.hasG {
		clip = intersectClips(clip, w.gclip)
	}
	if !w.hasRect {
		return clip
	}
	return clip.IntersectDevice(w.rect)
}

func (w *sceneWalker) clear(c Color) {
	w.flush()
	if !w.hasRect {
		if w.hasG {
			w.clearBox(scissorOf(w.gclip), c)
			return
		}
		w.dev.Clear(c)
		return
	}
	box := w.rect
	if w.hasG {
		box = box.Intersect(scissorOf(w.gclip))
	}
	w.clearBox(box, c)
}

func (w *sceneWalker) clearBox(r Rect, c Color) {
	if r.Empty() {
		return
	}
	if w.hasG && w.gclip.Mask != nil {
		// A masked group clip cannot be honoured by a raw ClearRect.
		w.dev.Fill(RectPath(r), Identity(), Fill(c), intersectClips(Clip{HasScissor: true, Scissor: r}, w.gclip))
		return
	}
	if clr, ok := w.dev.(clearRector); ok {
		clr.ClearRect(r, c)
		return
	}
	w.dev.Fill(RectPath(r), Identity(), Fill(c), Clip{HasScissor: true, Scissor: r})
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
	clip := w.clipOp(Clip{}, Identity())
	if g.LayerOpaque && clip.Mask == nil && w.copyOpaqueLayer(src, dest, xform, clip) {
		return
	}
	w.dev.Blit(src, srcR, srcR, xform, Paint{Color: White, Filter: FilterNearest}, clip)
}

func (w *sceneWalker) copyOpaqueLayer(src *Image, dest Rect, xform Matrix, clip Clip) bool {
	cpu, ok := w.dev.(*CPUDevice)
	if !ok || cpu.img == nil || !xform.IsTranslation() {
		return false
	}
	dx, okX := nearInt(xform.E)
	dy, okY := nearInt(xform.F)
	if !okX || !okY {
		return false
	}
	part := dest
	if clip.HasScissor {
		part = part.Intersect(clip.Scissor)
	}
	if part.Empty() {
		return true
	}
	px0, py0, px1, py1 := part.IntBounds()
	ox := px0 - dx
	oy := py0 - dy
	cpu.img.CopyFrom(src, XYWH(float32(ox), float32(oy), float32(px1-px0), float32(py1-py0)), px0, py0)
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
	return paint.isOpaqueSolid()
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

// mapClip moves a recorded op clip by the accumulated group transform.
// The clip belongs to the content, so it travels with it.
//
// A coverage mask is resampled when the transform is not an integer
// translation; before v0.11 a fractional translation truncated the offset
// (off-by-one) and any rotation/scale dropped the mask entirely, which
// un-clipped the op.
func mapClip(c Clip, m Matrix) Clip {
	if m.IsIdentity() {
		return c
	}
	if c.HasScissor {
		c.Scissor = m.TransformRect(c.Scissor)
	}
	if c.Mask == nil {
		return c
	}
	if m.IsTranslation() {
		if dx, ok := nearInt(m.E); ok {
			if dy, ok2 := nearInt(m.F); ok2 {
				c.MaskX += dx
				c.MaskY += dy
				return c
			}
		}
	}
	return resampleMask(c, m)
}

// resampleMask rebuilds a coverage mask under an arbitrary affine map.
func resampleMask(c Clip, m Matrix) Clip {
	inv, ok := m.Invert()
	if !ok {
		c.Mask = nil
		return c
	}
	src := XYWH(float32(c.MaskX), float32(c.MaskY), float32(c.MaskW), float32(c.MaskH))
	dst := m.TransformRect(src)
	x0, y0, x1, y1 := dst.IntBounds()
	bw, bh := x1-x0, y1-y0
	if bw <= 0 || bh <= 0 || bw*bh > 1<<24 {
		c.Mask = nil
		return c
	}
	out := make([]byte, bw*bh)
	for y := 0; y < bh; y++ {
		for x := 0; x < bw; x++ {
			p := inv.Transform(Pt(float32(x0+x)+0.5, float32(y0+y)+0.5))
			sx := int(p.X)
			sy := int(p.Y)
			if p.X < 0 {
				sx--
			}
			if p.Y < 0 {
				sy--
			}
			out[y*bw+x] = c.maskAt(sx, sy)
		}
	}
	c.Mask = out
	c.MaskX, c.MaskY = x0, y0
	c.MaskW, c.MaskH = bw, bh
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
	case opBlit, opBackdrop:
		return xf.TransformRect(op.dstR)
	default:
		return Rect{}
	}
}

// backdropRegions appends the device regions the backdrop ops under g read:
// each op's rectangle grown by how far its blur reaches.
func backdropRegions(g *GroupNode, acc Matrix, out []Rect) []Rect {
	if g == nil || !g.backdrop {
		return out
	}
	xf := acc.Mul(g.Xform)
	for _, ch := range g.Children {
		switch t := ch.(type) {
		case *GroupNode:
			out = backdropRegions(t, xf, out)
		case *drawOp:
			if t != nil && t.kind == opBackdrop {
				m := xf.Mul(t.xform)
				reach := float32(blurReach(t.radius * m.ApproxScale()))
				out = append(out, m.TransformRect(t.dstR).Inset(-reach))
			}
		}
	}
	return out
}

// widenForBackdrops grows every dirty box that touches a backdrop region
// to cover the region, so the content under a blur is repainted before
// the blur reads it.
func widenForBackdrops(s *Scene, dirty []Rect) []Rect {
	if s == nil || s.Root == nil || !s.Root.backdrop {
		return dirty
	}
	regions := backdropRegions(s.Root, Identity(), nil)
	if len(regions) == 0 {
		return dirty
	}
	var d Damage
	for _, r := range dirty {
		for grown := true; grown; {
			grown = false
			for _, g := range regions {
				if r.Overlaps(g) && !g.Empty() {
					if u := r.Union(g); u != r {
						r, grown = u, true
					}
				}
			}
		}
		d.Add(r)
	}
	return d.Rects
}

// opDeviceBounds is the op's device box padded by one pixel. The padding
// covers the AA fringe for geometry and, for blits, the pixel a fractional
// destination edge still touches — a dirty rect is rounded outward, so an
// exact float test would skip a blit whose pixels the rect erased.
func opDeviceBounds(op *drawOp, acc Matrix) Rect {
	b := opContentBounds(op, acc)
	if b.Empty() {
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
