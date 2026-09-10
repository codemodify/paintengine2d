package paintengine2d

// Damage records dirty rectangles for UI-style partial redraw (Evas-like).
//
// The paint engine is still immediate-mode: draws composite now. A UI
// framework or window manager attaches a Damage via [Context.SetDamage],
// skips clean regions with [Damage.Overlaps] (widgets or decoration
// strips), and presents [Damage.Bounds] / [Damage.Rects] to the OS.
//
// Rects are in device pixels. Overlapping or nearly adjacent boxes are merged
// so the list stays small.
type Damage struct {
	Rects []Rect
	// Pad is the extra slack (device px) when deciding two rects should merge.
	// Zero means they must overlap or touch.
	Pad float32
	// MaxRects collapses the list to a single union when exceeded (default 32).
	MaxRects int
}

// Add marks r dirty and coalesces when needed.
func (d *Damage) Add(r Rect) {
	if d == nil {
		return
	}
	r = r.Canon()
	if r.Empty() {
		return
	}
	d.Rects = append(d.Rects, r)
	d.coalesce()
}

// Reset clears recorded rectangles.
func (d *Damage) Reset() {
	if d == nil {
		return
	}
	d.Rects = d.Rects[:0]
}

// Empty reports whether nothing is dirty.
func (d *Damage) Empty() bool {
	return d == nil || len(d.Rects) == 0
}

// Bounds returns the union of recorded rectangles, or an empty rect.
func (d *Damage) Bounds() Rect {
	if d == nil || len(d.Rects) == 0 {
		return Rect{}
	}
	return unionRects(d.Rects)
}

func unionRects(rs []Rect) Rect {
	if len(rs) == 0 {
		return Rect{}
	}
	u := rs[0]
	for _, r := range rs[1:] {
		u = u.Union(r)
	}
	return u
}

// Overlaps reports whether any recorded dirty box intersects r (device pixels).
// A UI layer uses this to decide whether a widget needs to paint.
func (d *Damage) Overlaps(r Rect) bool {
	if d == nil || r.Empty() {
		return false
	}
	r = r.Canon()
	for _, b := range d.Rects {
		if b.Overlaps(r) {
			return true
		}
	}
	return false
}

// ClipTo intersects every rect with bounds and drops empties.
func (d *Damage) ClipTo(bounds Rect) {
	if d == nil {
		return
	}
	n := 0
	for _, r := range d.Rects {
		r = r.Intersect(bounds)
		if r.Empty() {
			continue
		}
		d.Rects[n] = r
		n++
	}
	d.Rects = d.Rects[:n]
}

func (d *Damage) maxN() int {
	if d.MaxRects <= 0 {
		return 32
	}
	return d.MaxRects
}

func (d *Damage) coalesce() {
	if d == nil || len(d.Rects) < 2 {
		return
	}
	if len(d.Rects) > d.maxN() {
		u := unionRects(d.Rects)
		d.Rects = d.Rects[:1]
		d.Rects[0] = u
		return
	}
	pad := d.Pad
	changed := true
	for changed {
		changed = false
		out := d.Rects[:0]
		for _, r := range d.Rects {
			merged := false
			for i := range out {
				if rectsNear(out[i], r, pad) {
					out[i] = out[i].Union(r)
					merged = true
					changed = true
					break
				}
			}
			if !merged {
				out = append(out, r)
			}
		}
		d.Rects = out
	}
}

func rectsNear(a, b Rect, pad float32) bool {
	if a.Empty() || b.Empty() {
		return false
	}
	a = a.Inset(-pad)
	return a.Overlaps(b) ||
		(a.Max.X >= b.Min.X && a.Min.X <= b.Max.X && (a.Max.Y == b.Min.Y || b.Max.Y == a.Min.Y)) ||
		(a.Max.Y >= b.Min.Y && a.Min.Y <= b.Max.Y && (a.Max.X == b.Min.X || b.Max.X == a.Min.X))
}
