package raster

// SamplesY is the number of vertical sub-scanlines per pixel row.
// 8 is a good quality/speed tradeoff; coverage is quantized to 256 levels
// (each sample contributes up to 32).
const SamplesY = 8

// FillNonZero / FillEvenOdd match the public fill rules.
const (
	FillNonZero = 0
	FillEvenOdd = 1
)

// Edge is a directed, non-horizontal line segment with Y0 < Y1.
type Edge struct {
	X0, Y0, X1, Y1 float32
	Dir            int8 // +1 upward original, -1 if we swapped endpoints
}

func (e Edge) xAt(y float32) float32 {
	dy := e.Y1 - e.Y0
	if abs32(dy) < 1e-20 {
		return e.X0
	}
	t := (y - e.Y0) / dy
	x := e.X0 + t*(e.X1-e.X0)
	if x != x { // NaN
		return e.X0
	}
	return x
}

// BuildEdges converts flattened contours into directed edges.
func BuildEdges(contours [][]Vec2, dst []Edge) []Edge {
	dst = dst[:0]
	for _, c := range contours {
		if len(c) < 2 {
			continue
		}
		for i := 1; i < len(c); i++ {
			a, b := c[i-1], c[i]
			if !a.finite() || !b.finite() || a.Y == b.Y {
				continue
			}
			dir := int8(1)
			if a.Y > b.Y {
				a, b = b, a
				dir = -1
			}
			dst = append(dst, Edge{X0: a.X, Y0: a.Y, X1: b.X, Y1: b.Y, Dir: dir})
		}
	}
	return dst
}

type isect struct {
	x   float32
	dir int8
}

// Rasterizer holds reusable scratch for scanline AA.
type Rasterizer struct {
	edges  []Edge
	isects []isect
	cover  []uint16
}

// ResetEdges replaces the active edge list. The slice is borrowed until the
// next ResetEdges; CoverageRow only reads it.
func (r *Rasterizer) ResetEdges(edges []Edge) {
	r.edges = edges
}

// CoverageRow rasterizes one pixel row y into cover[0:width] as 0..255
// (inclusive). cover is allocated/resized as needed and returned.
func (r *Rasterizer) CoverageRow(y, width int, rule int, clipX0, clipX1 int) []uint16 {
	if width < 0 {
		width = 0
	}
	if cap(r.cover) < width {
		r.cover = make([]uint16, width)
	} else {
		r.cover = r.cover[:width]
	}
	if clipX0 < 0 {
		clipX0 = 0
	}
	if clipX1 > width {
		clipX1 = width
	}
	if clipX0 >= clipX1 {
		return r.cover
	}
	// Only the clip span is read by the blender; skip O(surface width) work.
	clear(r.cover[clipX0:clipX1])

	y0 := float32(y)
	y1 := y0 + 1
	// Quick reject: no edge overlaps this row.
	any := false
	for i := range r.edges {
		e := &r.edges[i]
		if e.Y1 > y0 && e.Y0 < y1 {
			any = true
			break
		}
	}
	if !any {
		return r.cover
	}

	const weight = 256 / SamplesY // 32
	for s := 0; s < SamplesY; s++ {
		yy := y0 + (float32(s)+0.5)/SamplesY
		r.isects = r.isects[:0]
		for i := range r.edges {
			e := &r.edges[i]
			if yy < e.Y0 || yy >= e.Y1 {
				continue
			}
			r.isects = append(r.isects, isect{x: e.xAt(yy), dir: e.Dir})
		}
		if len(r.isects) < 2 {
			continue
		}
		sortIsects(r.isects)
		r.isects = mergeIsects(r.isects)
		if len(r.isects) < 2 {
			continue
		}
		winding := 0
		for i := 0; i < len(r.isects)-1; i++ {
			winding += int(r.isects[i].dir)
			if !filled(winding, rule) {
				continue
			}
			x0 := r.isects[i].x
			x1 := r.isects[i+1].x
			if x1 <= x0 {
				continue
			}
			addSpan(r.cover, x0, x1, weight, clipX0, clipX1)
		}
	}
	// Clamp to 255 inside the clip span only.
	for i := clipX0; i < clipX1; i++ {
		if r.cover[i] > 255 {
			r.cover[i] = 255
		}
	}
	return r.cover
}

// EdgeBounds is the axis-aligned box of all edge endpoints.
// ok is false when there are no edges.
func EdgeBounds(edges []Edge) (minX, minY, maxX, maxY float32, ok bool) {
	if len(edges) == 0 {
		return 0, 0, 0, 0, false
	}
	e0 := edges[0]
	minX, maxX = e0.X0, e0.X0
	minY, maxY = e0.Y0, e0.Y0
	for i := range edges {
		e := &edges[i]
		if e.X0 < minX {
			minX = e.X0
		}
		if e.X1 < minX {
			minX = e.X1
		}
		if e.X0 > maxX {
			maxX = e.X0
		}
		if e.X1 > maxX {
			maxX = e.X1
		}
		if e.Y0 < minY {
			minY = e.Y0
		}
		if e.Y1 < minY {
			minY = e.Y1
		}
		if e.Y0 > maxY {
			maxY = e.Y0
		}
		if e.Y1 > maxY {
			maxY = e.Y1
		}
	}
	return minX, minY, maxX, maxY, true
}

func sortIsects(a []isect) {
	// Insertion sort: intersection lists are tiny (typically 2–16).
	for i := 1; i < len(a); i++ {
		x := a[i]
		j := i
		for j > 0 && a[j-1].x > x.x {
			a[j] = a[j-1]
			j--
		}
		a[j] = x
	}
}

// mergeIsects collapses intersections that share an X (shared vertices)
// so winding changes once and zero-length spans are skipped.
func mergeIsects(a []isect) []isect {
	if len(a) < 2 {
		return a
	}
	const eps = 1e-5
	n := 0
	for i := 0; i < len(a); i++ {
		if n > 0 && abs32(a[i].x-a[n-1].x) < eps {
			a[n-1].dir += a[i].dir
			continue
		}
		a[n] = a[i]
		n++
	}
	out := a[:n]
	w := 0
	for i := range out {
		if out[i].dir == 0 {
			continue
		}
		out[w] = out[i]
		w++
	}
	return out[:w]
}

func filled(winding, rule int) bool {
	if rule == FillEvenOdd {
		return winding&1 != 0
	}
	return winding != 0
}

func addSpan(cover []uint16, x0, x1 float32, weight, clipX0, clipX1 int) {
	if x1 <= x0 {
		return
	}
	if x1 <= float32(clipX0) || x0 >= float32(clipX1) {
		return
	}
	if x0 < float32(clipX0) {
		x0 = float32(clipX0)
	}
	if x1 > float32(clipX1) {
		x1 = float32(clipX1)
	}
	ix0 := floor32(x0)
	ix1 := floor32(x1)
	if ix0 == ix1 {
		if ix0 >= clipX0 && ix0 < clipX1 {
			cover[ix0] += uint16((x1 - x0) * float32(weight))
		}
		return
	}
	if ix0 >= clipX0 && ix0 < clipX1 {
		cover[ix0] += uint16((float32(ix0) + 1 - x0) * float32(weight))
	}
	start := ix0 + 1
	if start < clipX0 {
		start = clipX0
	}
	end := ix1
	if end > clipX1 {
		end = clipX1
	}
	for x := start; x < end; x++ {
		cover[x] += uint16(weight)
	}
	if ix1 >= clipX0 && ix1 < clipX1 {
		frac := x1 - float32(ix1)
		if frac > 0 {
			cover[ix1] += uint16(frac * float32(weight))
		}
	}
}

// RectCoverage writes analytical coverage of an axis-aligned rectangle into
// a single pixel row y. Used as the FillRect fast path.
func RectCoverage(cover []uint16, y int, x0, y0, x1, y1 float32, clipX0, clipX1 int) []uint16 {
	width := len(cover)
	if clipX0 < 0 {
		clipX0 = 0
	}
	if clipX1 > width {
		clipX1 = width
	}
	if clipX0 < clipX1 {
		clear(cover[clipX0:clipX1])
	}
	py0 := float32(y)
	py1 := py0 + 1
	if y1 <= py0 || y0 >= py1 || clipX0 >= clipX1 {
		return cover
	}
	fy := min32(py1, y1) - max32(py0, y0)
	if fy <= 0 {
		return cover
	}
	if x1 <= float32(clipX0) || x0 >= float32(clipX1) {
		return cover
	}
	if x0 < float32(clipX0) {
		x0 = float32(clipX0)
	}
	if x1 > float32(clipX1) {
		x1 = float32(clipX1)
	}
	ix0 := floor32(x0)
	ix1 := floor32(x1)
	w := fy * 255
	if ix0 == ix1 {
		if ix0 >= clipX0 && ix0 < clipX1 {
			cover[ix0] = uint16((x1 - x0) * w)
		}
		return cover
	}
	if ix0 >= clipX0 && ix0 < clipX1 {
		cover[ix0] = uint16((float32(ix0) + 1 - x0) * w)
	}
	start := ix0 + 1
	if start < clipX0 {
		start = clipX0
	}
	end := ix1
	if end > clipX1 {
		end = clipX1
	}
	full := uint16(w + 0.5)
	if full > 255 {
		full = 255
	}
	for x := start; x < end; x++ {
		cover[x] = full
	}
	if ix1 >= clipX0 && ix1 < clipX1 {
		frac := x1 - float32(ix1)
		if frac > 0 {
			cover[ix1] = uint16(frac * w)
		}
	}
	return cover
}
