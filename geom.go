package paintengine2d

import "math"

// Point is a 2D point or vector in user or device space.
type Point struct {
	X, Y float32
}

// Pt is a shorthand constructor for [Point].
func Pt(x, y float32) Point { return Point{X: x, Y: y} }

// Add returns p + q.
func (p Point) Add(q Point) Point { return Point{p.X + q.X, p.Y + q.Y} }

// Sub returns p - q.
func (p Point) Sub(q Point) Point { return Point{p.X - q.X, p.Y - q.Y} }

// Mul returns p scaled by s.
func (p Point) Mul(s float32) Point { return Point{p.X * s, p.Y * s} }

// Dot returns the dot product.
func (p Point) Dot(q Point) float32 { return p.X*q.X + p.Y*q.Y }

// Cross returns the 2D cross product p.X*q.Y - p.Y*q.X.
func (p Point) Cross(q Point) float32 { return p.X*q.Y - p.Y*q.X }

// Len returns the Euclidean length.
func (p Point) Len() float32 { return float32(math.Hypot(float64(p.X), float64(p.Y))) }

// LenSq returns X*X + Y*Y.
func (p Point) LenSq() float32 { return p.X*p.X + p.Y*p.Y }

// Normalize returns a unit-length vector, or (0,0) if p is too small.
func (p Point) Normalize() Point {
	n := p.Len()
	if n < 1e-12 {
		return Point{}
	}
	return Point{p.X / n, p.Y / n}
}

// Perp returns the left-hand perpendicular (-Y, X).
func (p Point) Perp() Point { return Point{-p.Y, p.X} }

// Lerp returns the linear interpolation from p to q at t.
func (p Point) Lerp(q Point, t float32) Point {
	return Point{p.X + (q.X-p.X)*t, p.Y + (q.Y-p.Y)*t}
}

// Near reports whether p and q are within eps in both axes.
func (p Point) Near(q Point, eps float32) bool {
	dx, dy := p.X-q.X, p.Y-q.Y
	return dx*dx+dy*dy <= eps*eps
}

// Rect is an axis-aligned rectangle in a 2D plane.
// It is stored as min/max corners. A rect is empty when Max is not
// strictly greater than Min on both axes.
type Rect struct {
	Min, Max Point
}

// XYWH constructs a rectangle from origin and size. Negative sizes are
// normalized so Min is the lesser corner.
func XYWH(x, y, w, h float32) Rect {
	r := Rect{Min: Point{x, y}, Max: Point{x + w, y + h}}
	return r.Canon()
}

// RectPts constructs a rectangle from two corners (order-independent).
func RectPts(a, b Point) Rect {
	return Rect{Min: a, Max: b}.Canon()
}

// Canon returns a rectangle with Min <= Max on both axes.
func (r Rect) Canon() Rect {
	if r.Min.X > r.Max.X {
		r.Min.X, r.Max.X = r.Max.X, r.Min.X
	}
	if r.Min.Y > r.Max.Y {
		r.Min.Y, r.Max.Y = r.Max.Y, r.Min.Y
	}
	return r
}

// Empty reports whether r has zero or negative area.
func (r Rect) Empty() bool { return r.Max.X <= r.Min.X || r.Max.Y <= r.Min.Y }

// Dx returns the width.
func (r Rect) Dx() float32 { return r.Max.X - r.Min.X }

// Dy returns the height.
func (r Rect) Dy() float32 { return r.Max.Y - r.Min.Y }

// Size returns (width, height) as a point.
func (r Rect) Size() Point { return Point{r.Dx(), r.Dy()} }

// Center returns the midpoint.
func (r Rect) Center() Point {
	return Point{(r.Min.X + r.Max.X) * 0.5, (r.Min.Y + r.Max.Y) * 0.5}
}

// Inset returns r shrunk by n on every side (or expanded if n is negative).
func (r Rect) Inset(n float32) Rect {
	return Rect{
		Min: Point{r.Min.X + n, r.Min.Y + n},
		Max: Point{r.Max.X - n, r.Max.Y - n},
	}
}

// Translate returns r moved by p.
func (r Rect) Translate(p Point) Rect {
	return Rect{Min: r.Min.Add(p), Max: r.Max.Add(p)}
}

// Contains reports whether p is inside r (inclusive min, exclusive max
// on the right/bottom — a half-open convention matching pixel coverage).
func (r Rect) Contains(p Point) bool {
	return p.X >= r.Min.X && p.X < r.Max.X && p.Y >= r.Min.Y && p.Y < r.Max.Y
}

// Intersect returns the overlapping rectangle, or an empty rect.
func (r Rect) Intersect(s Rect) Rect {
	out := Rect{
		Min: Point{max32(r.Min.X, s.Min.X), max32(r.Min.Y, s.Min.Y)},
		Max: Point{min32(r.Max.X, s.Max.X), min32(r.Max.Y, s.Max.Y)},
	}
	if out.Empty() {
		return Rect{}
	}
	return out
}

// Union returns the smallest rectangle covering both r and s.
// Empty inputs are ignored.
func (r Rect) Union(s Rect) Rect {
	switch {
	case r.Empty():
		return s
	case s.Empty():
		return r
	}
	return Rect{
		Min: Point{min32(r.Min.X, s.Min.X), min32(r.Min.Y, s.Min.Y)},
		Max: Point{max32(r.Max.X, s.Max.X), max32(r.Max.Y, s.Max.Y)},
	}
}

// Overlaps reports whether r and s share any interior area.
func (r Rect) Overlaps(s Rect) bool {
	return r.Min.X < s.Max.X && s.Min.X < r.Max.X && r.Min.Y < s.Max.Y && s.Min.Y < r.Max.Y
}

// IntBounds returns the integer pixel bounds that fully cover r,
// suitable for allocating a clip mask or iterating pixels.
func (r Rect) IntBounds() (x0, y0, x1, y1 int) {
	if r.Empty() {
		return 0, 0, 0, 0
	}
	x0 = int(math.Floor(float64(r.Min.X)))
	y0 = int(math.Floor(float64(r.Min.Y)))
	x1 = int(math.Ceil(float64(r.Max.X)))
	y1 = int(math.Ceil(float64(r.Max.Y)))
	return
}

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func max32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func clamp32(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
