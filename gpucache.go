package paintengine2d

import (
	"math"

	"github.com/codemodify/paintengine2d/internal/raster"
)

// tessCache stores flattened contours and triangle-fan verts so a GPU
// Fill/Stroke of the same path+xform does not CPU-flatten every frame.
type tessCache struct {
	m            map[tessKey]*tessEntry
	hits, misses int
	flattens     int
	strokePool   raster.StrokePool
	verbs        []raster.Verb
	pts          []raster.Vec2
	contours     [][]raster.Vec2
	closed       []bool
	outlineStore [][]raster.Vec2
}

type tessKind uint8

const (
	tessFill tessKind = iota
	tessStroke
)

type tessKey struct {
	hash              uint64
	kind              tessKind
	xa, xb, xc        uint32
	xd, xe, xf        uint32
	tol               uint32
	width, miter      uint32
	dashOff, dashHash uint64
	cap, join, rule   uint8
}

type tessEntry struct {
	contours [][]raster.Vec2
	closed   []bool
	verts    []float32
	box      Rect
}

const tessCacheCap = 256

func newTessCache() *tessCache {
	return &tessCache{m: make(map[tessKey]*tessEntry)}
}

func (c *tessCache) get(k tessKey) *tessEntry {
	if c == nil || c.m == nil {
		return nil
	}
	e, ok := c.m[k]
	if !ok {
		c.misses++
		return nil
	}
	c.hits++
	return e
}

func (c *tessCache) put(k tessKey, e *tessEntry) {
	if c.m == nil {
		c.m = make(map[tessKey]*tessEntry)
	}
	if len(c.m) >= tessCacheCap {
		c.m = make(map[tessKey]*tessEntry)
	}
	c.m[k] = e
}

func (c *tessCache) lookupFill(path *Path, xform Matrix, rule FillRule) *tessEntry {
	k := fillTessKey(path, xform, rule)
	if e := c.get(k); e != nil {
		return e
	}
	e := c.buildFill(path, xform)
	if e == nil {
		return nil
	}
	c.put(k, e)
	return e
}

func (c *tessCache) lookupStroke(path *Path, xform Matrix, st Stroke) *tessEntry {
	k := strokeTessKey(path, xform, st)
	if e := c.get(k); e != nil {
		return e
	}
	e := c.buildStroke(path, xform, st)
	if e == nil {
		return nil
	}
	c.put(k, e)
	return e
}

func fillTessKey(path *Path, xform Matrix, rule FillRule) tessKey {
	return tessKey{
		hash: hashPath(path),
		kind: tessFill,
		xa:   math.Float32bits(xform.A),
		xb:   math.Float32bits(xform.B),
		xc:   math.Float32bits(xform.C),
		xd:   math.Float32bits(xform.D),
		xe:   math.Float32bits(xform.E),
		xf:   math.Float32bits(xform.F),
		tol:  math.Float32bits(flattenTol()),
		rule: uint8(rule),
	}
}

func strokeTessKey(path *Path, xform Matrix, st Stroke) tessKey {
	scale := xform.ApproxScale()
	tol := flattenTol()
	if scale > 1e-8 {
		tol = flattenTol() / scale
	}
	return tessKey{
		hash:     hashPath(path),
		kind:     tessStroke,
		xa:       math.Float32bits(xform.A),
		xb:       math.Float32bits(xform.B),
		xc:       math.Float32bits(xform.C),
		xd:       math.Float32bits(xform.D),
		xe:       math.Float32bits(xform.E),
		xf:       math.Float32bits(xform.F),
		tol:      math.Float32bits(tol),
		width:    math.Float32bits(st.Width),
		miter:    math.Float32bits(st.MiterLimit),
		dashOff:  uint64(math.Float32bits(st.DashOffset)),
		dashHash: hashFloats(st.Dash),
		cap:      uint8(st.Cap),
		join:     uint8(st.Join),
	}
}

func hashPath(path *Path) uint64 {
	if path == nil {
		return 0
	}
	h := uint64(14695981039346656037)
	mix := func(v uint64) {
		h ^= v
		h *= 1099511628211
	}
	mix(uint64(len(path.verbs)))
	mix(uint64(len(path.pts)))
	for _, v := range path.verbs {
		mix(uint64(v))
	}
	for _, p := range path.pts {
		mix(uint64(math.Float32bits(p.X)))
		mix(uint64(math.Float32bits(p.Y)))
	}
	return h
}

func hashFloats(s []float32) uint64 {
	h := uint64(14695981039346656037)
	for _, v := range s {
		h ^= uint64(math.Float32bits(v))
		h *= 1099511628211
	}
	return h
}

func (c *tessCache) pack(path *Path) {
	c.verbs = c.verbs[:0]
	c.pts = c.pts[:0]
	if path == nil {
		return
	}
	for _, v := range path.verbs {
		c.verbs = append(c.verbs, raster.Verb(v))
	}
	for _, p := range path.pts {
		c.pts = append(c.pts, raster.Vec2{X: p.X, Y: p.Y})
	}
}

func (c *tessCache) buildFill(path *Path, xform Matrix) *tessEntry {
	c.flattens++
	c.pack(path)
	if !xform.IsIdentity() {
		for i := range c.pts {
			q := xform.Transform(Point{c.pts[i].X, c.pts[i].Y})
			c.pts[i] = raster.Vec2{X: q.X, Y: q.Y}
		}
	}
	raster.Flatten(c.verbs, c.pts, flattenTol(), &c.contours, &c.closed)
	if len(c.contours) == 0 {
		return nil
	}
	return snapshotTess(c.contours, c.closed)
}

func (c *tessCache) buildStroke(path *Path, xform Matrix, st Stroke) *tessEntry {
	scale := xform.ApproxScale()
	if !finite32(scale) || scale < 1e-8 {
		return nil
	}
	c.flattens++
	c.pack(path)
	tol := flattenTol() / scale
	raster.Flatten(c.verbs, c.pts, tol, &c.contours, &c.closed)
	user, closed := c.contours, c.closed
	if len(st.Dash) > 0 {
		user, closed = raster.Dash(user, closed, st.Dash, st.DashOffset)
	}
	outlines := c.strokePool.Expand(user, closed, raster.StrokeOpts{
		Width: st.Width, Cap: int(st.Cap), Join: int(st.Join), MiterLimit: st.MiterLimit,
	})
	if len(outlines) == 0 {
		return nil
	}
	for len(c.outlineStore) < len(outlines) {
		c.outlineStore = append(c.outlineStore, nil)
	}
	c.contours = c.contours[:0]
	c.closed = c.closed[:0]
	for i, cont := range outlines {
		tc := c.outlineStore[i]
		if cap(tc) < len(cont) {
			tc = make([]raster.Vec2, len(cont))
		} else {
			tc = tc[:len(cont)]
		}
		for j, p := range cont {
			q := xform.Transform(Point{p.X, p.Y})
			tc[j] = raster.Vec2{X: q.X, Y: q.Y}
		}
		c.outlineStore[i] = tc
		c.contours = append(c.contours, tc)
		c.closed = append(c.closed, true)
	}
	return snapshotTess(c.contours, c.closed)
}

func snapshotTess(contours [][]raster.Vec2, closed []bool) *tessEntry {
	e := &tessEntry{
		contours: make([][]raster.Vec2, len(contours)),
		closed:   append([]bool(nil), closed...),
	}
	for i, c := range contours {
		e.contours[i] = append([]raster.Vec2(nil), c...)
		e.verts = tessellateFan(c, e.verts)
	}
	e.box = contourBoundsOf(e.contours)
	return e
}

func tessellateFan(c []raster.Vec2, dst []float32) []float32 {
	if len(c) < 3 {
		return dst
	}
	for i := 1; i+1 < len(c); i++ {
		dst = append(dst,
			c[0].X, c[0].Y, 0, 0,
			c[i].X, c[i].Y, 0, 0,
			c[i+1].X, c[i+1].Y, 0, 0,
		)
	}
	return dst
}

func contourBoundsOf(contours [][]raster.Vec2) Rect {
	var minX, minY, maxX, maxY float32
	n := 0
	for _, c := range contours {
		for _, p := range c {
			if n == 0 {
				minX, minY, maxX, maxY = p.X, p.Y, p.X, p.Y
			} else {
				if p.X < minX {
					minX = p.X
				}
				if p.Y < minY {
					minY = p.Y
				}
				if p.X > maxX {
					maxX = p.X
				}
				if p.Y > maxY {
					maxY = p.Y
				}
			}
			n++
		}
	}
	if n == 0 {
		return Rect{}
	}
	return Rect{Min: Point{minX, minY}, Max: Point{maxX, maxY}}.Inset(-1)
}
