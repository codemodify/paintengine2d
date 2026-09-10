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
