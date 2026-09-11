package paintengine2d

import "testing"

func TestBitmapAtlasLabelPaintsPixels(t *testing.T) {
	atlas := NewBitmapAtlas(White)
	if _, ok := atlas.Cell(GlyphID('A')); !ok {
		t.Fatal("expected A in atlas")
	}
	img := NewImage(64, 16)
	ctx := NewContext(img)
	ctx.DrawLabel("OK", atlas, Pt(2, 2), Paint{Color: White, Filter: FilterNearest})
	if countOpaque(img, 200) < 8 {
		t.Fatalf("label should paint glyph pixels, n=%d", countOpaque(img, 200))
	}
	// Far right stays empty for a short string.
	assertAlpha(t, img, 60, 8, 0, 0, "beyond label")
}

func TestNullShaperSkipsMissing(t *testing.T) {
	atlas := NewBitmapAtlas(White)
	run := NullShaper{}.Shape("A~B", atlas) // '~' is not in the atlas
	if len(run.Glyphs) != 2 {
		t.Fatalf("glyphs %d", len(run.Glyphs))
	}
	if run.Glyphs[0].ID != GlyphID('A') || run.Glyphs[1].ID != GlyphID('B') {
		t.Fatalf("%+v", run.Glyphs)
	}
	if run.Glyphs[1].X <= run.Glyphs[0].X {
		t.Fatal("advance")
	}
}

func TestDrawGlyphsNilSafe(t *testing.T) {
	img := NewImage(8, 8)
	ctx := NewContext(img)
	ctx.DrawGlyphs(GlyphRun{}, Pt(0, 0), Fill(White))
	ctx.DrawLabel("HI", nil, Pt(0, 0), Fill(White))
	if countOpaque(img, 1) != 0 {
		t.Fatal("nil atlas should be a no-op")
	}
}

func TestBitmapAtlasAliasesLowercase(t *testing.T) {
	atlas := NewBitmapAtlas(White)
	if _, ok := atlas.Cell(GlyphID('s')); !ok {
		t.Fatal("lowercase should alias the A–Z cell")
	}
	img := NewImage(48, 16)
	ctx := NewContext(img)
	ctx.DrawLabel("Save", atlas, Pt(2, 2), Paint{Color: White, Filter: FilterNearest})
	if countOpaque(img, 200) < 12 {
		t.Fatalf("mixed-case label should paint, n=%d", countOpaque(img, 200))
	}
}

func TestGlyphRunBounds(t *testing.T) {
	atlas := NewBitmapAtlas(White)
	run := NullShaper{}.Shape("OK", atlas)
	b := run.Bounds(Pt(3, 4))
	if b.Empty() || b.Min.X != 3 || b.Min.Y != 4 {
		t.Fatalf("bounds %+v", b)
	}
	if b.Dx() < 10 || b.Dy() < 7 {
		t.Fatalf("expected two 6×8 cells, got %+v", b)
	}
	empty := NullShaper{}.Shape("", atlas)
	if !empty.Bounds(Pt(0, 0)).Empty() {
		t.Fatal("empty run")
	}
}

// stubShaper is a stand-in for a future HarfBuzz wrapper: it remaps runes
// and applies a tracking offset. Context / Device stay unchanged.
type stubShaper struct{ tracking float32 }

func (s stubShaper) Shape(text string, atlas *FontAtlas) GlyphRun {
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
		x += adv + s.tracking
	}
	return run
}

func TestCustomShaperHook(t *testing.T) {
	atlas := NewBitmapAtlas(White)
	var _ Shaper = stubShaper{}
	tight := NullShaper{}.Shape("AB", atlas)
	wide := stubShaper{tracking: 4}.Shape("AB", atlas)
	if len(wide.Glyphs) != 2 {
		t.Fatalf("glyphs %d", len(wide.Glyphs))
	}
	if wide.Glyphs[1].X <= tight.Glyphs[1].X {
		t.Fatalf("tracking should widen the run (%.1f vs %.1f)", wide.Glyphs[1].X, tight.Glyphs[1].X)
	}
	img := NewImage(40, 16)
	ctx := NewContext(img)
	ctx.DrawGlyphs(wide, Pt(1, 1), Paint{Color: White, Filter: FilterNearest})
	if countOpaque(img, 200) < 8 {
		t.Fatal("custom-shaped run should blit")
	}
}

func TestBitmapPunctuation(t *testing.T) {
	atlas := NewBitmapAtlas(White)
	for _, r := range []rune{'/', '_', '%', '(', ')', '='} {
		if _, ok := atlas.Cell(GlyphID(r)); !ok {
			t.Fatalf("missing %q", string(r))
		}
	}
	img := NewImage(48, 16)
	NewContext(img).DrawLabel("100%", atlas, Pt(2, 2), Paint{Color: White, Filter: FilterNearest})
	if countOpaque(img, 200) < 10 {
		t.Fatalf("punctuation label, n=%d", countOpaque(img, 200))
	}
}

func TestDrawGlyphsRGBTint(t *testing.T) {
	atlas := NewBitmapAtlas(White)
	img := NewImage(48, 16)
	ctx := NewContext(img)
	ctx.DrawLabel("A", atlas, Pt(2, 2), Paint{Color: RGB(0.2, 0.6, 1), Filter: FilterNearest})
	found := false
	for y := 0; y < img.Height && !found; y++ {
		for x := 0; x < img.Width; x++ {
			r, g, b, a := rgbAt(img, x, y)
			if a < 200 {
				continue
			}
			if r > 80 || g < 100 || b < 200 {
				t.Fatalf("tinted glyph %d,%d = %d %d %d %d", x, y, r, g, b, a)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatal("white atlas * accent should paint colored glyphs")
	}
}

func TestDrawGlyphsTintedVsWhite(t *testing.T) {
	atlas := NewBitmapAtlas(White)
	white := NewImage(32, 16)
	red := NewImage(32, 16)
	NewContext(white).DrawLabel("OK", atlas, Pt(2, 2), Paint{Color: White, Filter: FilterNearest})
	NewContext(red).DrawLabel("OK", atlas, Pt(2, 2), Paint{Color: Red, Filter: FilterNearest})
	same := true
	colored := false
	for y := 0; y < white.Height; y++ {
		for x := 0; x < white.Width; x++ {
			wr, wg, wb, wa := rgbAt(white, x, y)
			rr, rg, rb, ra := rgbAt(red, x, y)
			if wa != ra {
				t.Fatalf("tint must keep coverage, %d,%d a %d vs %d", x, y, wa, ra)
			}
			if wa < 200 {
				continue
			}
			if wr != rr || wg != rg || wb != rb {
				same = false
			}
			if rr > 200 && rg < 8 && rb < 8 {
				colored = true
			}
		}
	}
	if same || !colored {
		t.Fatal("red theme should recolor white atlas glyphs")
	}
}

func TestLabelDamage(t *testing.T) {
	img := NewImage(48, 16)
	ctx := NewContext(img)
	var d Damage
	ctx.SetDamage(&d)
	ctx.DrawLabel("UI", NewBitmapAtlas(White), Pt(1, 1), Paint{Color: White, Filter: FilterNearest})
	if d.Empty() {
		t.Fatal("glyphs should record damage")
	}
}
