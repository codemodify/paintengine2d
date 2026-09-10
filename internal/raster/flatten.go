package raster

// Verb mirrors the public path command set so this package stays independent.
type Verb uint8

const (
	Move Verb = iota
	Line
	Quad
	Cubic
	Close
)

// Flatten walks a path and appends device-space polylines (one slice per
// contour). Closed contours have their first point repeated at the end and
// closed[i] == true.
//
// tol is the flattening tolerance in the same space as pts (device pixels).
func Flatten(verbs []Verb, pts []Vec2, tol float32, contours *[][]Vec2, closed *[]bool) {
	if tol < 0.01 {
		tol = 0.01
	}
	*contours = (*contours)[:0]
	*closed = (*closed)[:0]

	var cur []Vec2
	var start Vec2
	var hasStart, hasCur bool
	pi := 0

	flush := func(isClosed bool) {
		if len(cur) >= 2 {
			if isClosed && !cur[0].near(cur[len(cur)-1], 1e-4) {
				cur = append(cur, cur[0])
			}
			*contours = append(*contours, cur)
			*closed = append(*closed, isClosed)
		}
		cur = nil
		hasStart, hasCur = false, false
	}

	for _, v := range verbs {
		switch v {
		case Move:
			flush(false)
			p := pts[pi]
			pi++
			start, hasStart, hasCur = p, true, true
			cur = append(cur, p)
		case Line:
			p := pts[pi]
			pi++
			if !hasCur {
				start, hasStart, hasCur = p, true, true
				cur = append(cur, p)
				continue
			}
			cur = append(cur, p)
		case Quad:
			c := pts[pi]
			p := pts[pi+1]
			pi += 2
			if !hasCur {
				start, hasStart, hasCur = c, true, true
				cur = append(cur, c)
			}
			from := cur[len(cur)-1]
			cur = flattenQuad(cur, from, c, p, tol, 0)
		case Cubic:
			c1 := pts[pi]
			c2 := pts[pi+1]
			p := pts[pi+2]
			pi += 3
			if !hasCur {
				start, hasStart, hasCur = c1, true, true
				cur = append(cur, c1)
			}
			from := cur[len(cur)-1]
			cur = flattenCubic(cur, from, c1, c2, p, tol, 0)
		case Close:
			if hasStart && hasCur {
				if !cur[len(cur)-1].near(start, 1e-4) {
					cur = append(cur, start)
				}
				flush(true)
			} else {
				flush(false)
			}
		}
	}
	flush(false)
}

func (p Vec2) near(q Vec2, eps float32) bool {
	dx, dy := p.X-q.X, p.Y-q.Y
	return dx*dx+dy*dy <= eps*eps
}

func flattenQuad(out []Vec2, p0, p1, p2 Vec2, tol float32, depth int) []Vec2 {
	if depth > 16 || distPointToLine(p1, p0, p2) <= tol {
		return append(out, p2)
	}
	// de Casteljau
	p01 := p0.add(p1).mul(0.5)
	p12 := p1.add(p2).mul(0.5)
	pm := p01.add(p12).mul(0.5)
	out = flattenQuad(out, p0, p01, pm, tol, depth+1)
	out = flattenQuad(out, pm, p12, p2, tol, depth+1)
	return out
}

func flattenCubic(out []Vec2, p0, p1, p2, p3 Vec2, tol float32, depth int) []Vec2 {
	d1 := distPointToLine(p1, p0, p3)
	d2 := distPointToLine(p2, p0, p3)
	if depth > 16 || d1+d2 <= tol {
		return append(out, p3)
	}
	p01 := p0.add(p1).mul(0.5)
	p12 := p1.add(p2).mul(0.5)
	p23 := p2.add(p3).mul(0.5)
	p012 := p01.add(p12).mul(0.5)
	p123 := p12.add(p23).mul(0.5)
	pm := p012.add(p123).mul(0.5)
	out = flattenCubic(out, p0, p01, p012, pm, tol, depth+1)
	out = flattenCubic(out, pm, p123, p23, p3, tol, depth+1)
	return out
}
