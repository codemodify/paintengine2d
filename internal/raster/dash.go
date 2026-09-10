package raster

// Dash splits flattened contours into on/off pieces using an SVG-style
// dash array. Output contours are open (caps apply per dash). An empty
// or all-zero pattern returns the input unchanged.
func Dash(contours [][]Vec2, closed []bool, dashes []float32, offset float32) ([][]Vec2, []bool) {
	pat := normalizeDashes(dashes)
	if len(pat) == 0 {
		return contours, closed
	}
	out := make([][]Vec2, 0, len(contours)*4)
	outClosed := make([]bool, 0, len(contours)*4)
	for i, c := range contours {
		isClosed := i < len(closed) && closed[i]
		pts := dedupeContour(c, isClosed)
		if len(pts) < 2 {
			continue
		}
		pieces := dashContour(pts, isClosed, pat, offset)
		for _, p := range pieces {
			if len(p) >= 2 {
				out = append(out, p)
				outClosed = append(outClosed, false)
			}
		}
	}
	return out, outClosed
}

func normalizeDashes(dashes []float32) []float32 {
	if len(dashes) == 0 {
		return nil
	}
	out := make([]float32, 0, len(dashes)+1)
	sum := float32(0)
	for _, d := range dashes {
		if d < 0 {
			d = 0
		}
		out = append(out, d)
		sum += d
	}
	if sum < 1e-8 {
		return nil
	}
	// SVG: odd-length arrays are concatenated with themselves.
	if len(out)%2 == 1 {
		out = append(out, out...)
	}
	return out
}

func dashPeriod(pat []float32) float32 {
	var s float32
	for _, d := range pat {
		s += d
	}
	return s
}

func dashContour(pts []Vec2, closed bool, pat []float32, offset float32) [][]Vec2 {
	n := len(pts)
	if n < 2 {
		return nil
	}
	// Build segment list, including the close edge.
	type sg struct{ a, b Vec2 }
	segs := make([]sg, 0, n)
	lim := n - 1
	if closed {
		lim = n
	}
	var total float32
	for i := 0; i < lim; i++ {
		a := pts[i]
		b := pts[(i+1)%n]
		if a.sub(b).lenSq() < 1e-12 {
			continue
		}
		segs = append(segs, sg{a, b})
		total += b.sub(a).len()
	}
	if len(segs) == 0 || total < 1e-8 {
		return nil
	}

	period := dashPeriod(pat)
	// Positive offset advances into the pattern (SVG).
	off := offset
	if period > 0 {
		off = off - float32(mathFloor32(off/period))*period
		if off < 0 {
			off += period
		}
	}
	idx := 0
	remain := pat[0]
	on := true
	for off > 0 && period > 0 {
		if off >= remain {
			off -= remain
			idx = (idx + 1) % len(pat)
			remain = pat[idx]
			on = !on
		} else {
			remain -= off
			off = 0
		}
	}

	var out [][]Vec2
	var cur []Vec2
	flush := func() {
		if len(cur) >= 2 {
			out = append(out, cur)
		}
		cur = nil
	}

	for _, s := range segs {
		segLen := s.b.sub(s.a).len()
		pos := float32(0)
		for pos < segLen-1e-7 {
			if remain <= 1e-8 {
				idx = (idx + 1) % len(pat)
				remain = pat[idx]
				on = !on
				if !on {
					flush()
				}
				if remain <= 1e-8 {
					continue
				}
			}
			take := remain
			if take > segLen-pos {
				take = segLen - pos
			}
			t0 := pos / segLen
			t1 := (pos + take) / segLen
			p0 := s.a.add(s.b.sub(s.a).mul(t0))
			p1 := s.a.add(s.b.sub(s.a).mul(t1))
			if on {
				if len(cur) == 0 {
					cur = append(cur, p0)
				}
				cur = append(cur, p1)
			}
			pos += take
			remain -= take
		}
	}
	flush()
	return out
}
