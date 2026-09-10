package paintengine2d

// Damage is a stub for a future Evas-style dirty-rectangle tracker.
//
// A retained-mode or windowing layer would accumulate rectangles that
// changed since the last present and only re-rasterize those tiles.
// v0 is an immediate-mode canvas: every draw composites now, and Damage
// records nothing unless you call [Damage.Add] yourself.
//
// The type exists so a later release can grow the API without inventing
// a new package-level name.
type Damage struct {
	// Rects holds dirty regions in device pixels. The engine does not
	// populate this in v0.
	Rects []Rect
}

// Add marks r dirty.
func (d *Damage) Add(r Rect) {
	if d == nil || r.Empty() {
		return
	}
	d.Rects = append(d.Rects, r)
}

// Reset clears recorded rectangles.
func (d *Damage) Reset() {
	if d == nil {
		return
	}
	d.Rects = d.Rects[:0]
}

// Bounds returns the union of recorded rectangles, or an empty rect.
func (d *Damage) Bounds() Rect {
	if d == nil || len(d.Rects) == 0 {
		return Rect{}
	}
	u := d.Rects[0]
	for _, r := range d.Rects[1:] {
		u = u.Union(r)
	}
	return u
}
