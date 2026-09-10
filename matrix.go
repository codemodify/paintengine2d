package paintengine2d

import "math"

// Matrix is a 2D affine transform:
//
//	| A C E |
//	| B D F |
//	| 0 0 1 |
//
// A point (x, y) maps to (A*x + C*y + E, B*x + D*y + F).
type Matrix struct {
	A, B, C, D, E, F float32
}

// Identity returns the identity matrix.
func Identity() Matrix { return Matrix{A: 1, D: 1} }

// Translation returns a translation matrix.
func Translation(x, y float32) Matrix { return Matrix{A: 1, D: 1, E: x, F: y} }

// Scaling returns a scale about the origin.
func Scaling(sx, sy float32) Matrix { return Matrix{A: sx, D: sy} }

// Rotation returns a rotation about the origin, in radians (positive is clockwise
// because +Y points down).
func Rotation(radians float32) Matrix {
	s, c := math.Sincos(float64(radians))
	return Matrix{A: float32(c), B: float32(s), C: float32(-s), D: float32(c)}
}

// Mul returns the composition m * n (apply n first, then m).
func (m Matrix) Mul(n Matrix) Matrix {
	return Matrix{
		A: m.A*n.A + m.C*n.B,
		B: m.B*n.A + m.D*n.B,
		C: m.A*n.C + m.C*n.D,
		D: m.B*n.C + m.D*n.D,
		E: m.A*n.E + m.C*n.F + m.E,
		F: m.B*n.E + m.D*n.F + m.F,
	}
}

// Transform maps p by m.
func (m Matrix) Transform(p Point) Point {
	return Point{
		X: m.A*p.X + m.C*p.Y + m.E,
		Y: m.B*p.X + m.D*p.Y + m.F,
	}
}

// TransformRect maps the four corners of r and returns their axis-aligned bounds.
func (m Matrix) TransformRect(r Rect) Rect {
	if r.Empty() {
		return Rect{}
	}
	p0 := m.Transform(r.Min)
	p1 := m.Transform(Point{r.Max.X, r.Min.Y})
	p2 := m.Transform(r.Max)
	p3 := m.Transform(Point{r.Min.X, r.Max.Y})
	minX := min32(min32(p0.X, p1.X), min32(p2.X, p3.X))
	minY := min32(min32(p0.Y, p1.Y), min32(p2.Y, p3.Y))
	maxX := max32(max32(p0.X, p1.X), max32(p2.X, p3.X))
	maxY := max32(max32(p0.Y, p1.Y), max32(p2.Y, p3.Y))
	return Rect{Min: Point{minX, minY}, Max: Point{maxX, maxY}}
}

// Invert returns the inverse transform. ok is false if m is singular.
func (m Matrix) Invert() (inv Matrix, ok bool) {
	det := m.A*m.D - m.C*m.B
	if det > -1e-20 && det < 1e-20 {
		return Matrix{}, false
	}
	invDet := 1 / det
	inv = Matrix{
		A: m.D * invDet,
		B: -m.B * invDet,
		C: -m.C * invDet,
		D: m.A * invDet,
	}
	inv.E = -(inv.A*m.E + inv.C*m.F)
	inv.F = -(inv.B*m.E + inv.D*m.F)
	return inv, true
}

// IsIdentity reports whether m is the identity within a small epsilon.
func (m Matrix) IsIdentity() bool {
	return nearEq(m.A, 1) && nearEq(m.D, 1) &&
		nearEq(m.B, 0) && nearEq(m.C, 0) &&
		nearEq(m.E, 0) && nearEq(m.F, 0)
}

// IsTranslation reports whether m is a pure translation (possibly identity).
func (m Matrix) IsTranslation() bool {
	return nearEq(m.A, 1) && nearEq(m.D, 1) && nearEq(m.B, 0) && nearEq(m.C, 0)
}

// IsAxisAligned reports whether m maps axis-aligned rects to axis-aligned rects
// (uniform/non-uniform scale, translation, 90° flips — no general rotation).
func (m Matrix) IsAxisAligned() bool {
	return (nearEq(m.B, 0) && nearEq(m.C, 0)) || (nearEq(m.A, 0) && nearEq(m.D, 0))
}

// ApproxScale returns a representative scale factor (average of axis lengths).
// Useful for flattening tolerance and conservative stroke estimates.
func (m Matrix) ApproxScale() float32 {
	sx := float32(math.Hypot(float64(m.A), float64(m.B)))
	sy := float32(math.Hypot(float64(m.C), float64(m.D)))
	return (sx + sy) * 0.5
}

// Finite reports whether every coefficient is a finite number.
func (m Matrix) Finite() bool {
	return finite32(m.A) && finite32(m.B) && finite32(m.C) &&
		finite32(m.D) && finite32(m.E) && finite32(m.F)
}

func nearEq(a, b float32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 1e-6
}
