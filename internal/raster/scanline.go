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
//
// Edges are kept in an active-edge list: [Rasterizer.ResetEdges] sorts edge
// indices by top Y once, and each [Rasterizer.CoverageRow] admits the edges
// that start on the row and retires the ones that ended. Sequential rows
// therefore cost O(active) instead of O(all edges); a random-access row
// rebuilds the active list from the sorted order.
type Rasterizer struct {
	edges  []Edge
	slope  []float32 // dx/dy per edge (0 for horizontal-ish)
	order  []int32   // edge indices sorted by Y0
	active []int32
	isects []isect
	cover  []uint16
	// run holds the whole pixels of a row's spans as a difference array:
	// +weight where a span's interior starts, -weight where it ends,
	// summed once per row rather than added pixel by pixel for each of
	// the SamplesY sub-scanlines.
	run []int32

	cursor  int // next index into order not yet admitted
	nextRow int // row the active list is primed for
	primed  bool
}

// ResetEdges replaces the active edge list. The slice is borrowed until the
// next ResetEdges; CoverageRow only reads it.
func (r *Rasterizer) ResetEdges(edges []Edge) {
	r.edges = edges
	r.primed = false
	r.cursor = 0
	r.active = r.active[:0]

	n := len(edges)
	if cap(r.slope) < n {
		r.slope = make([]float32, n)
	} else {
		r.slope = r.slope[:n]
	}
	if cap(r.order) < n {
		r.order = make([]int32, n)
	} else {
		r.order = r.order[:n]
	}
	for i := range edges {
		e := &edges[i]
		dy := e.Y1 - e.Y0
		if abs32(dy) < 1e-20 {
			r.slope[i] = 0
		} else {
			r.slope[i] = (e.X1 - e.X0) / dy
		}
		r.order[i] = int32(i)
	}
	sortByTopY(r.order, edges)
}

// sortByTopY sorts edge indices by Edge.Y0. Hand-rolled so the hot path
// stays allocation-free (sort.Slice escapes a closure and a swapper).
func sortByTopY(order []int32, edges []Edge) {
	for len(order) > 12 {
		// Median-of-three pivot, Hoare partition.
		lo, mid, hi := 0, len(order)/2, len(order)-1
		if edges[order[mid]].Y0 < edges[order[lo]].Y0 {
			order[mid], order[lo] = order[lo], order[mid]
		}
		if edges[order[hi]].Y0 < edges[order[mid]].Y0 {
			order[hi], order[mid] = order[mid], order[hi]
			if edges[order[mid]].Y0 < edges[order[lo]].Y0 {
				order[mid], order[lo] = order[lo], order[mid]
			}
		}
		pivot := edges[order[mid]].Y0
		i, j := 0, len(order)-1
		for i <= j {
			for edges[order[i]].Y0 < pivot {
				i++
			}
			for edges[order[j]].Y0 > pivot {
				j--
			}
			if i <= j {
				order[i], order[j] = order[j], order[i]
				i++
				j--
			}
		}
		// Recurse into the smaller side, loop on the larger.
		if j+1 < len(order)-i {
			sortByTopY(order[:j+1], edges)
			order = order[i:]
		} else {
			sortByTopY(order[i:], edges)
			order = order[:j+1]
		}
	}
	for i := 1; i < len(order); i++ {
		v := order[i]
		key := edges[v].Y0
		j := i
		for j > 0 && edges[order[j-1]].Y0 > key {
			order[j] = order[j-1]
			j--
		}
		order[j] = v
	}
}

// xAtFast is [Edge.xAt] using the precomputed slope.
func (r *Rasterizer) xAtFast(i int32, y float32) float32 {
	e := &r.edges[i]
	x := e.X0 + (y-e.Y0)*r.slope[i]
	if x != x { // NaN
		return e.X0
	}
	return x
}

// syncActive makes the active list valid for pixel row y.
func (r *Rasterizer) syncActive(y int) {
	if !r.primed || y < r.nextRow {
		r.cursor = 0
		r.active = r.active[:0]
		r.primed = true
	} else if y > r.nextRow {
		// Skipped rows: retire everything that ended before y.
		r.retire(float32(y))
	}
	r.nextRow = y + 1

	y0 := float32(y)
	y1 := y0 + 1
	// Admit edges whose top is above the bottom of this row.
	for r.cursor < len(r.order) {
		i := r.order[r.cursor]
		if r.edges[i].Y0 >= y1 {
			break
		}
		r.cursor++
		if r.edges[i].Y1 <= y0 {
			continue // already finished before this row
		}
		r.active = append(r.active, i)
	}
	r.retire(y0)
}

// retire drops active edges whose bottom is at or above y.
func (r *Rasterizer) retire(y float32) {
	n := 0
	for _, i := range r.active {
		if r.edges[i].Y1 > y {
			r.active[n] = i
			n++
		}
	}
	r.active = r.active[:n]
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
	if cap(r.run) < width+1 {
		r.run = make([]int32, width+1)
	} else {
		r.run = r.run[:width+1]
	}
	clear(r.run[clipX0 : clipX1+1])

	r.syncActive(y)
	if len(r.active) == 0 {
		return r.cover
	}

	y0 := float32(y)
	const weight = 256 / SamplesY // 32
	for s := 0; s < SamplesY; s++ {
		yy := y0 + (float32(s)+0.5)/SamplesY
		r.isects = r.isects[:0]
		for _, i := range r.active {
			e := &r.edges[i]
			if yy < e.Y0 || yy >= e.Y1 {
				continue
			}
			r.isects = append(r.isects, isect{x: r.xAtFast(i, yy), dir: e.Dir})
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
			addSpan(r.cover, r.run, x0, x1, weight, clipX0, clipX1)
		}
	}
	// Fold the spans' whole pixels in, and clamp to 255 inside the clip
	// span only.
	var acc int32
	for i := clipX0; i < clipX1; i++ {
		acc += r.run[i]
		c := int32(r.cover[i]) + acc
		if c > 255 {
			c = 255
		}
		r.cover[i] = uint16(c)
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

// addCov adds v to cover[i], saturating instead of wrapping. Spans inside a
// single sample line are disjoint so the sum is bounded in practice, but a
// pathological self-overlapping path must not wrap to a dark pixel.
func addCov(cover []uint16, i int, v uint16) {
	c := cover[i] + v
	if c < cover[i] {
		c = 0xFFFF
	}
	cover[i] = c
}

func addSpan(cover []uint16, run []int32, x0, x1 float32, weight, clipX0, clipX1 int) {
	if !(x1 > x0) { // NaN-safe
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
			addCov(cover, ix0, uint16((x1-x0)*float32(weight)))
		}
		return
	}
	if ix0 >= clipX0 && ix0 < clipX1 {
		addCov(cover, ix0, uint16((float32(ix0)+1-x0)*float32(weight)))
	}
	start := ix0 + 1
	if start < clipX0 {
		start = clipX0
	}
	end := ix1
	if end > clipX1 {
		end = clipX1
	}
	if end > start {
		run[start] += int32(weight)
		run[end] -= int32(weight)
	}
	if ix1 >= clipX0 && ix1 < clipX1 {
		frac := x1 - float32(ix1)
		if frac > 0 {
			addCov(cover, ix1, uint16(frac*float32(weight)))
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
