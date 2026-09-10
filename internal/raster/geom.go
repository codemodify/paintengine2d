// Package raster is the CPU scanline anti-aliased rasterizer.
//
// Algorithm (v0): flatten curves to polylines in device space, then for each
// pixel row take N vertical sample lines, compute exact X intersections,
// walk winding (non-zero or even-odd), and accumulate analytical horizontal
// coverage. This is the classic AGG/FreeType-inspired “scanline AA” family:
// cheap, predictable, and visibly anti-aliased.
package raster

import "math"

// Vec2 is a 2D point used by the rasterizer.
type Vec2 struct{ X, Y float32 }

func (p Vec2) add(q Vec2) Vec2      { return Vec2{p.X + q.X, p.Y + q.Y} }
func (p Vec2) sub(q Vec2) Vec2      { return Vec2{p.X - q.X, p.Y - q.Y} }
func (p Vec2) mul(s float32) Vec2   { return Vec2{p.X * s, p.Y * s} }
func (p Vec2) dot(q Vec2) float32   { return p.X*q.X + p.Y*q.Y }
func (p Vec2) cross(q Vec2) float32 { return p.X*q.Y - p.Y*q.X }
func (p Vec2) len() float32         { return float32(math.Hypot(float64(p.X), float64(p.Y))) }
func (p Vec2) lenSq() float32       { return p.X*p.X + p.Y*p.Y }

func (p Vec2) norm() Vec2 {
	n := p.len()
	if n < 1e-12 {
		return Vec2{}
	}
	return Vec2{p.X / n, p.Y / n}
}

func (p Vec2) perp() Vec2 { return Vec2{-p.Y, p.X} }

func (p Vec2) finite() bool {
	return !math.IsNaN(float64(p.X)) && !math.IsInf(float64(p.X), 0) &&
		!math.IsNaN(float64(p.Y)) && !math.IsInf(float64(p.Y), 0)
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
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

func floor32(v float32) int {
	i := int(v)
	if v < 0 && float32(i) != v {
		return i - 1
	}
	return i
}

// DistPointToSeg is the distance from p to the line through a–b.
func distPointToLine(p, a, b Vec2) float32 {
	d := b.sub(a)
	lenSq := d.lenSq()
	if lenSq < 1e-20 {
		return p.sub(a).len()
	}
	return abs32(p.sub(a).cross(d)) / float32(math.Sqrt(float64(lenSq)))
}
