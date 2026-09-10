package raster

import "math"

// Cap / Join match the public paintengine2d enumerations.
const (
	CapButt = iota
	CapRound
	CapSquare
)

const (
	JoinMiter = iota
	JoinRound
	JoinBevel
)

// StrokeOpts is the stroker configuration (user-space units).
type StrokeOpts struct {
	Width      float32
	Cap        int
	Join       int
	MiterLimit float32
}

type seg struct {
	a, b, dir, nor Vec2
}

// ExpandStroke offsets flattened contours by Width/2 and returns closed
// outline contours suitable for non-zero fill.
func ExpandStroke(contours [][]Vec2, closed []bool, opt StrokeOpts) [][]Vec2 {
	if opt.Width <= 0 {
		return nil
	}
	if opt.MiterLimit < 1 {
		opt.MiterLimit = 4
	}
	half := opt.Width * 0.5
	out := make([][]Vec2, 0, len(contours))
	for i, c := range contours {
		if len(c) < 2 {
			continue
		}
		isClosed := i < len(closed) && closed[i]
		pts := dedupeContour(c, isClosed)
		if len(pts) < 2 {
			continue
		}
		if isClosed && len(pts) < 3 {
			isClosed = false
		}
		if outline := strokeContour(pts, isClosed, half, opt); len(outline) >= 3 {
			out = append(out, outline)
		}
	}
	return out
}

func dedupeContour(c []Vec2, closed bool) []Vec2 {
	pts := make([]Vec2, 0, len(c))
	for _, p := range c {
		if len(pts) > 0 && p.near(pts[len(pts)-1], 1e-5) {
			continue
		}
		pts = append(pts, p)
	}
	if closed && len(pts) >= 2 && pts[0].near(pts[len(pts)-1], 1e-4) {
		pts = pts[:len(pts)-1]
	}
	return pts
}

func strokeContour(pts []Vec2, closed bool, half float32, opt StrokeOpts) []Vec2 {
	n := len(pts)
	segs := make([]seg, 0, n)
	lim := n - 1
	if closed {
		lim = n
	}
	for i := 0; i < lim; i++ {
		a := pts[i]
		b := pts[(i+1)%n]
		d := b.sub(a)
		if d.lenSq() < 1e-12 {
			continue
		}
		dir := d.norm()
		segs = append(segs, seg{a: a, b: b, dir: dir, nor: dir.perp()})
	}
	if len(segs) == 0 {
		return nil
	}

	// Left side travels a→b; right side travels b→a.
	left := make([]Vec2, 0, len(segs)*4)
	right := make([]Vec2, 0, len(segs)*4)

	emitJoin := func(dst *[]Vec2, p, nIn, nOut, dIn, dOut Vec2, leftSide bool) {
		inOff := p.add(nIn.mul(half))
		outOff := p.add(nOut.mul(half))
		cr := dIn.cross(dOut)
		// Left outline is the exterior on a right turn (cr < 0).
		outer := cr < -1e-8
		if !leftSide {
			outer = cr > 1e-8
		}
		if abs32(cr) < 1e-8 {
			*dst = append(*dst, outOff)
			return
		}
		if !outer {
			// Interior of the turn: a short bevel keeps the outline simple
			// without self-crossing at sharp corners.
			*dst = append(*dst, inOff, outOff)
			return
		}
		switch opt.Join {
		case JoinBevel:
			*dst = append(*dst, inOff, outOff)
		case JoinRound:
			*dst = append(*dst, inOff)
			appendWedge(dst, p, inOff, outOff, half, leftSide)
			*dst = append(*dst, outOff)
		default:
			hit, ok := lineIntersect(inOff, dIn, outOff, dOut)
			if !ok {
				*dst = append(*dst, inOff, outOff)
				return
			}
			if hit.sub(p).len() > opt.MiterLimit*half {
				*dst = append(*dst, inOff, outOff)
				return
			}
			*dst = append(*dst, inOff, hit, outOff)
		}
	}

	if !closed {
		// Start cap, then each body + join, then end cap, then reverse bodies.
		s0 := segs[0]
		left = append(left, s0.a.add(s0.nor.mul(half)))
		right = append(right, s0.a.sub(s0.nor.mul(half)))
		// We'll add the start cap after both sides are built (at the seam).

		for i := 0; i < len(segs); i++ {
			s := segs[i]
			l1 := s.b.add(s.nor.mul(half))
			r1 := s.b.sub(s.nor.mul(half))
			if i+1 < len(segs) {
				n := segs[i+1]
				emitJoin(&left, s.b, s.nor, n.nor, s.dir, n.dir, true)
				emitJoin(&right, s.b, s.nor.mul(-1), n.nor.mul(-1), s.dir, n.dir, false)
			} else {
				left = append(left, l1)
				right = append(right, r1)
			}
		}

		outline := make([]Vec2, 0, len(left)+len(right)+16)
		// Start cap from right[0] to left[0].
		outline = appendCap(outline, segs[0].a, right[0], left[0], segs[0].dir.mul(-1), half, opt.Cap)
		outline = append(outline, left...)
		last := segs[len(segs)-1]
		outline = appendCap(outline, last.b, left[len(left)-1], right[len(right)-1], last.dir, half, opt.Cap)
		for i := len(right) - 1; i >= 0; i-- {
			outline = append(outline, right[i])
		}
		return outline
	}

	// Closed: walk all joins on both sides, then concatenate left + reversed right.
	for i := 0; i < len(segs); i++ {
		s := segs[i]
		n := segs[(i+1)%len(segs)]
		// Leading offset of this segment (join from previous is emitted at prev).
		if i == 0 {
			left = append(left, s.a.add(s.nor.mul(half)))
			right = append(right, s.a.sub(s.nor.mul(half)))
		}
		emitJoin(&left, s.b, s.nor, n.nor, s.dir, n.dir, true)
		emitJoin(&right, s.b, s.nor.mul(-1), n.nor.mul(-1), s.dir, n.dir, false)
	}
	outline := make([]Vec2, 0, len(left)+len(right)+1)
	outline = append(outline, left...)
	for i := len(right) - 1; i >= 0; i-- {
		outline = append(outline, right[i])
	}
	if len(outline) > 0 {
		outline = append(outline, outline[0])
	}
	return outline
}

func appendCap(out []Vec2, center, from, to, outward Vec2, half float32, cap int) []Vec2 {
	switch cap {
	case CapSquare:
		out = append(out, from.add(outward.mul(half)), to.add(outward.mul(half)))
	case CapRound:
		out = append(out, from)
		// Semicircle on the outward side: from → outward*half → to.
		mid := center.add(outward.mul(half))
		appendWedge(&out, center, from, mid, half, true)
		appendWedge(&out, center, mid, to, half, true)
		out = append(out, to)
	default: // butt
		out = append(out, from, to)
	}
	return out
}

// appendWedge appends points of a circular arc from `from` to `to` around c.
func appendWedge(dst *[]Vec2, c, from, to Vec2, radius float32, _ bool) {
	v0 := from.sub(c)
	v1 := to.sub(c)
	if v0.lenSq() < 1e-12 || v1.lenSq() < 1e-12 {
		return
	}
	a0 := math.Atan2(float64(v0.Y), float64(v0.X))
	a1 := math.Atan2(float64(v1.Y), float64(v1.X))
	// Choose the shorter signed sweep.
	sweep := a1 - a0
	for sweep > math.Pi {
		sweep -= 2 * math.Pi
	}
	for sweep < -math.Pi {
		sweep += 2 * math.Pi
	}
	// Chord error ~0.35 px: more steps on thick UI strokes, not a fixed 22.5°.
	step := 0.35 / math.Max(float64(radius), 0.35)
	if step > math.Pi/6 {
		step = math.Pi / 6
	}
	if step < math.Pi/32 {
		step = math.Pi / 32
	}
	steps := int(math.Ceil(math.Abs(sweep) / step))
	if steps < 1 {
		steps = 1
	}
	if steps > 64 {
		steps = 64
	}
	for i := 1; i < steps; i++ {
		a := a0 + sweep*float64(i)/float64(steps)
		*dst = append(*dst, Vec2{
			X: c.X + radius*float32(math.Cos(a)),
			Y: c.Y + radius*float32(math.Sin(a)),
		})
	}
}

func lineIntersect(p, dp, q, dq Vec2) (Vec2, bool) {
	den := dp.cross(dq)
	if abs32(den) < 1e-10 {
		return Vec2{}, false
	}
	t := q.sub(p).cross(dq) / den
	return p.add(dp.mul(t)), true
}
