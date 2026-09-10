package paintengine2d

import "math"

// Verb is a path command. Points consumed by each verb:
//
//	Move, Line, Close: 1, 1, 0
//	Quad: 2 (control, end)
//	Cubic: 3 (c1, c2, end)
type Verb uint8

// Path verbs.
const (
	VerbMove Verb = iota
	VerbLine
	VerbQuad
	VerbCubic
	VerbClose
)

// Path is a vector outline: a sequence of contours built from lines and
// Bézier curves. Paths are resolution-independent; the rasterizer flattens
// them in device space.
//
// A Path is not safe for concurrent mutation.
type Path struct {
	verbs []Verb
	pts   []Point
	// start is the first point of the current open contour.
	start      Point
	hasStart   bool
	hasCurrent bool
}

// NewPath returns an empty path.
func NewPath() *Path { return &Path{} }

// Reset clears the path but keeps allocated capacity.
func (p *Path) Reset() {
	p.verbs = p.verbs[:0]
	p.pts = p.pts[:0]
	p.start = Point{}
	p.hasStart = false
	p.hasCurrent = false
}

// Empty reports whether the path has no verbs.
func (p *Path) Empty() bool { return len(p.verbs) == 0 }

// Clone returns a deep copy.
func (p *Path) Clone() *Path {
	if p == nil {
		return NewPath()
	}
	c := &Path{
		verbs:      append([]Verb(nil), p.verbs...),
		pts:        append([]Point(nil), p.pts...),
		start:      p.start,
		hasStart:   p.hasStart,
		hasCurrent: p.hasCurrent,
	}
	return c
}

// Verbs returns the command stream. The slice must not be mutated.
func (p *Path) Verbs() []Verb { return p.verbs }

// Points returns the packed point stream. The slice must not be mutated.
func (p *Path) Points() []Point { return p.pts }

// MoveTo begins a new contour at (x, y).
func (p *Path) MoveTo(x, y float32) {
	p.verbs = append(p.verbs, VerbMove)
	p.pts = append(p.pts, Point{x, y})
	p.start = Point{x, y}
	p.hasStart = true
	p.hasCurrent = true
}

// LineTo adds a straight line to (x, y). If there is no current point,
// this is equivalent to MoveTo.
func (p *Path) LineTo(x, y float32) {
	if !p.hasCurrent {
		p.MoveTo(x, y)
		return
	}
	p.verbs = append(p.verbs, VerbLine)
	p.pts = append(p.pts, Point{x, y})
}

// QuadTo adds a quadratic Bézier (control, end).
func (p *Path) QuadTo(cx, cy, x, y float32) {
	if !p.hasCurrent {
		p.MoveTo(cx, cy)
	}
	p.verbs = append(p.verbs, VerbQuad)
	p.pts = append(p.pts, Point{cx, cy}, Point{x, y})
}

// CubicTo adds a cubic Bézier (c1, c2, end).
func (p *Path) CubicTo(c1x, c1y, c2x, c2y, x, y float32) {
	if !p.hasCurrent {
		p.MoveTo(c1x, c1y)
	}
	p.verbs = append(p.verbs, VerbCubic)
	p.pts = append(p.pts, Point{c1x, c1y}, Point{c2x, c2y}, Point{x, y})
}

// Close closes the current contour with a straight line back to its start.
func (p *Path) Close() {
	if !p.hasStart {
		return
	}
	p.verbs = append(p.verbs, VerbClose)
	p.hasCurrent = false
	p.hasStart = false
}

// AddRect appends a closed rectangle contour (clockwise: +Y down).
func (p *Path) AddRect(r Rect) {
	r = r.Canon()
	if r.Empty() {
		return
	}
	p.MoveTo(r.Min.X, r.Min.Y)
	p.LineTo(r.Max.X, r.Min.Y)
	p.LineTo(r.Max.X, r.Max.Y)
	p.LineTo(r.Min.X, r.Max.Y)
	p.Close()
}

// AddRoundRect appends a closed rounded rectangle. Radii are clamped to
// half the corresponding side. Degenerate radii fall back to a sharp rect.
func (p *Path) AddRoundRect(r Rect, rx, ry float32) {
	r = r.Canon()
	if r.Empty() {
		return
	}
	if rx < 0 {
		rx = 0
	}
	if ry < 0 {
		ry = 0
	}
	hw, hh := r.Dx()*0.5, r.Dy()*0.5
	if rx > hw {
		rx = hw
	}
	if ry > hh {
		ry = hh
	}
	if rx < 1e-4 || ry < 1e-4 {
		p.AddRect(r)
		return
	}
	x0, y0, x1, y1 := r.Min.X, r.Min.Y, r.Max.X, r.Max.Y
	// Clockwise, starting at top edge after the top-left corner.
	p.MoveTo(x0+rx, y0)
	p.LineTo(x1-rx, y0)
	p.addCornerArc(Pt(x1-rx, y0+ry), rx, ry, -math.Pi/2, math.Pi/2)
	p.LineTo(x1, y1-ry)
	p.addCornerArc(Pt(x1-rx, y1-ry), rx, ry, 0, math.Pi/2)
	p.LineTo(x0+rx, y1)
	p.addCornerArc(Pt(x0+rx, y1-ry), rx, ry, math.Pi/2, math.Pi/2)
	p.LineTo(x0, y0+ry)
	p.addCornerArc(Pt(x0+rx, y0+ry), rx, ry, math.Pi, math.Pi/2)
	p.Close()
}

// AddEllipse appends a closed ellipse centered at c with radii rx, ry.
func (p *Path) AddEllipse(c Point, rx, ry float32) {
	if rx < 0 {
		rx = -rx
	}
	if ry < 0 {
		ry = -ry
	}
	if rx < 1e-8 || ry < 1e-8 {
		return
	}
	p.MoveTo(c.X+rx, c.Y)
	p.addCornerArc(c, rx, ry, 0, math.Pi/2)
	p.addCornerArc(c, rx, ry, math.Pi/2, math.Pi/2)
	p.addCornerArc(c, rx, ry, math.Pi, math.Pi/2)
	p.addCornerArc(c, rx, ry, 3*math.Pi/2, math.Pi/2)
	p.Close()
}

// AddCircle appends a closed circle.
func (p *Path) AddCircle(c Point, radius float32) {
	p.AddEllipse(c, radius, radius)
}

// AddArc appends an elliptical arc centered at c. start and sweep are
// radians (0 = +X, positive clockwise). The arc is a new contour unless
// the path already has a current point, in which case a line is drawn to
// the arc start (SVG-like continuation).
func (p *Path) AddArc(c Point, rx, ry, start, sweep float32) {
	if rx < 0 {
		rx = -rx
	}
	if ry < 0 {
		ry = -ry
	}
	if rx < 1e-8 || ry < 1e-8 || abs32(sweep) < 1e-8 {
		return
	}
	// Split into <= 90° cubics.
	const maxSweep = float32(math.Pi / 2)
	n := int(math.Ceil(float64(abs32(sweep) / maxSweep)))
	if n < 1 {
		n = 1
	}
	if n > 16 {
		n = 16
	}
	ds := sweep / float32(n)
	a0 := start
	x0 := c.X + rx*float32(math.Cos(float64(a0)))
	y0 := c.Y + ry*float32(math.Sin(float64(a0)))
	if !p.hasCurrent {
		p.MoveTo(x0, y0)
	} else {
		p.LineTo(x0, y0)
	}
	for i := 0; i < n; i++ {
		p.addCornerArc(c, rx, ry, a0, ds)
		a0 += ds
	}
}

func (p *Path) addCornerArc(c Point, rx, ry, start, sweep float32) {
	// Cubic approximation of an elliptical arc. κ = 4/3 tan(sweep/4).
	if abs32(sweep) < 1e-8 {
		return
	}
	k := float32(4.0/3.0) * float32(math.Tan(float64(sweep)/4))
	a0 := float64(start)
	a1 := float64(start + sweep)
	s0, c0 := math.Sincos(a0)
	s1, c1 := math.Sincos(a1)
	p0 := Point{c.X + rx*float32(c0), c.Y + ry*float32(s0)}
	p3 := Point{c.X + rx*float32(c1), c.Y + ry*float32(s1)}
	// Tangent at angle a is (-rx sin, ry cos).
	t0 := Point{-rx * float32(s0), ry * float32(c0)}
	t1 := Point{-rx * float32(s1), ry * float32(c1)}
	p1 := p0.Add(t0.Mul(k))
	p2 := p3.Sub(t1.Mul(k))
	if !p.hasCurrent {
		p.MoveTo(p0.X, p0.Y)
	}
	p.CubicTo(p1.X, p1.Y, p2.X, p2.Y, p3.X, p3.Y)
}

// Transform maps every stored point by m (in place).
func (p *Path) Transform(m Matrix) {
	for i := range p.pts {
		p.pts[i] = m.Transform(p.pts[i])
	}
	if p.hasStart {
		p.start = m.Transform(p.start)
	}
}

// Bounds returns a conservative axis-aligned box of all on-curve and
// control points. Empty if the path has no points.
func (p *Path) Bounds() Rect {
	if len(p.pts) == 0 {
		return Rect{}
	}
	minX, minY := p.pts[0].X, p.pts[0].Y
	maxX, maxY := minX, minY
	for _, q := range p.pts[1:] {
		if q.X < minX {
			minX = q.X
		}
		if q.Y < minY {
			minY = q.Y
		}
		if q.X > maxX {
			maxX = q.X
		}
		if q.Y > maxY {
			maxY = q.Y
		}
	}
	return Rect{Min: Point{minX, minY}, Max: Point{maxX, maxY}}
}

// RectPath returns a new path containing one closed rectangle.
func RectPath(r Rect) *Path {
	p := NewPath()
	p.AddRect(r)
	return p
}

// RoundRectPath returns a new rounded-rectangle path.
func RoundRectPath(r Rect, rx, ry float32) *Path {
	p := NewPath()
	p.AddRoundRect(r, rx, ry)
	return p
}

// EllipsePath returns a new ellipse path.
func EllipsePath(c Point, rx, ry float32) *Path {
	p := NewPath()
	p.AddEllipse(c, rx, ry)
	return p
}

// CirclePath returns a new circle path.
func CirclePath(c Point, radius float32) *Path {
	p := NewPath()
	p.AddCircle(c, radius)
	return p
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
