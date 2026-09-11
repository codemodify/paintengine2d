package paintengine2d

// Text hooks for a UI layer. This package does **not** ship OpenType shaping,
// bidi, or HarfBuzz. IME, a11y, and line breaking live above this API.
//
// The contract:
//
//   - A [FontAtlas] is a bitmap / packed glyph sheet the app (or a future
//     shaper) owns.
//   - A [GlyphRun] is already-positioned glyphs (user space).
//   - [Context.DrawGlyphs] blits atlas cells with the current transform/clip.
//   - [Shaper] is the extension point; [NullShaper] maps runes → atlas IDs
//     for bitmap fonts and debug labels.
//
// A later HarfBuzz-quality implementation can satisfy [Shaper] without
// changing [Context] or [Device].

// GlyphID identifies a cell in a [FontAtlas]. Bitmap fonts typically use
// the rune value; a shaper may assign packed IDs.
type GlyphID uint32

// Glyph is one positioned glyph relative to the run origin.
type Glyph struct {
	ID   GlyphID
	X, Y float32
}

// AtlasCell is one glyph's rectangle in atlas image pixels plus layout.
type AtlasCell struct {
	Src     Rect    // pixel box in [FontAtlas.Image]
	Bearing Point   // offset from glyph origin to the dest top-left
	Advance float32 // x-advance after the glyph (user units); 0 → Src.Dx()+1
}

// FontAtlas is a GPU/CPU glyph sheet. The image is premul RGBA like [Image].
// After rewriting Image.Pix in place, call [Image.Touch] so the GPU atlas
// epoch invalidates the cached texture.
type FontAtlas struct {
	Image *Image
	Cells map[GlyphID]AtlasCell
}

// Epoch is the atlas image epoch (0 if Image is nil).
func (a *FontAtlas) Epoch() uint64 {
	if a == nil || a.Image == nil {
		return 0
	}
	return a.Image.Epoch
}

// Cell returns the atlas cell for id, or false if missing.
func (a *FontAtlas) Cell(id GlyphID) (AtlasCell, bool) {
	if a == nil || a.Cells == nil {
		return AtlasCell{}, false
	}
	c, ok := a.Cells[id]
	return c, ok
}

// GlyphRun is a shaped (or stub-shaped) sequence sharing one atlas.
type GlyphRun struct {
	Atlas  *FontAtlas
	Glyphs []Glyph
}

// Shaper turns UTF-8 into a [GlyphRun]. paintengine2d ships only [NullShaper].
type Shaper interface {
	Shape(text string, atlas *FontAtlas) GlyphRun
}

// NullShaper maps each rune to GlyphID(rune) and places glyphs on a single
// baseline using [AtlasCell.Advance]. Missing cells are skipped. This is
// enough for bitmap UI labels, not for Arabic, ligatures, or kerning.
type NullShaper struct{}

// Shape implements [Shaper].
func (NullShaper) Shape(text string, atlas *FontAtlas) GlyphRun {
	run := GlyphRun{Atlas: atlas}
	if atlas == nil {
		return run
	}
	var x float32
	for _, r := range text {
		id := GlyphID(r)
		cell, ok := atlas.Cell(id)
		if !ok {
			continue
		}
		run.Glyphs = append(run.Glyphs, Glyph{ID: id, X: x, Y: 0})
		adv := cell.Advance
		if adv <= 0 {
			adv = cell.Src.Dx() + 1
		}
		x += adv
	}
	return run
}

// Bounds is the user-space AABB of every atlas cell in the run, placed at
// origin. Empty if there is nothing to draw. A UI layer uses this for
// hit-testing and dirty-rect inflation of labels.
func (run GlyphRun) Bounds(origin Point) Rect {
	if run.Atlas == nil || len(run.Glyphs) == 0 {
		return Rect{}
	}
	var dirty Rect
	for _, g := range run.Glyphs {
		cell, ok := run.Atlas.Cell(g.ID)
		if !ok || cell.Src.Empty() {
			continue
		}
		dst := XYWH(
			origin.X+g.X+cell.Bearing.X,
			origin.Y+g.Y+cell.Bearing.Y,
			cell.Src.Dx(),
			cell.Src.Dy(),
		)
		if dirty.Empty() {
			dirty = dst
		} else {
			dirty = dirty.Union(dst)
		}
	}
	return dirty
}

// DrawGlyphs blits each atlas cell in run at origin (user space).
// paint.Color RGB multiplies premul atlas samples (theme a white sheet);
// Color.A modulates coverage. paint.Filter selects nearest/bilinear
// (nearest is the usual choice for pixel fonts).
func (c *Context) DrawGlyphs(run GlyphRun, origin Point, paint Paint) {
	if run.Atlas == nil || run.Atlas.Image == nil || len(run.Glyphs) == 0 {
		return
	}
	if paint.Color == (Color{}) {
		paint.Color = White
	}
	var dirty Rect
	for _, g := range run.Glyphs {
		cell, ok := run.Atlas.Cell(g.ID)
		if !ok || cell.Src.Empty() {
			continue
		}
		dst := XYWH(
			origin.X+g.X+cell.Bearing.X,
			origin.Y+g.Y+cell.Bearing.Y,
			cell.Src.Dx(),
			cell.Src.Dy(),
		)
		c.dev.Blit(run.Atlas.Image, cell.Src, dst, c.cur.xform, paint, c.clip())
		if dirty.Empty() {
			dirty = dst
		} else {
			dirty = dirty.Union(dst)
		}
	}
	c.markDirtyUser(dirty)
}

// DrawLabel shapes text with [NullShaper] and draws it. For production UI
// text, call a real [Shaper] and [Context.DrawGlyphs].
func (c *Context) DrawLabel(text string, atlas *FontAtlas, origin Point, paint Paint) {
	c.DrawGlyphs(NullShaper{}.Shape(text, atlas), origin, paint)
}
