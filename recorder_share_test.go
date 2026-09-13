package paintengine2d

import "testing"

// Regression: the Recorder deep-copied the clip coverage mask for every
// recorded op, so one label under a rounded clip allocated megabytes per
// frame. Masks handed to a Device are immutable, so they are shared.
func TestRecorderSharesClipMasks(t *testing.T) {
	atlas := NewBitmapAtlas(White)
	rec := NewRecorder(400, 300)
	ctx := NewContextDevice(rec)
	ctx.Save()
	ctx.ClipRoundRect(XYWH(0, 0, 300, 200), 12, 12)

	allocs := testing.AllocsPerRun(3, func() {
		ctx.DrawLabel("HELLO WORLD 0123456789", atlas, Pt(10, 100), Fill(White))
	})
	ctx.Restore()
	// 22 glyphs: a per-op mask copy would be 22 × 300×200 bytes.
	if allocs > 40 {
		t.Fatalf("recording a label under a clip mask allocated %v times per run", allocs)
	}
}

// Once a mask has been handed to a Device it must never be rewritten in
// place, or the recorded scene would change under the recorder.
func TestClipMaskIsImmutableOnceExported(t *testing.T) {
	rec := NewRecorder(64, 64)
	ctx := NewContextDevice(rec)
	ctx.ClipRoundRect(XYWH(0, 0, 32, 32), 8, 8)
	ctx.DrawRect(XYWH(0, 0, 32, 32), Fill(Black))
	first := rec.root.Children[0].(*drawOp).clip
	if first.Mask == nil {
		t.Fatal("expected a recorded clip mask")
	}
	snapshot := append([]byte(nil), first.Mask...)

	// A second, different clip must not scribble on the recorded bytes.
	ctx.ClipRoundRect(XYWH(0, 0, 16, 16), 4, 4)
	ctx.DrawRect(XYWH(0, 0, 16, 16), Fill(Black))
	for i := range snapshot {
		if first.Mask[i] != snapshot[i] {
			t.Fatalf("recorded clip mask mutated at %d", i)
		}
	}
}

// Identical shapes recorded repeatedly share one Path.
func TestRecorderInternsPaths(t *testing.T) {
	rec := NewRecorder(100, 100)
	ctx := NewContextDevice(rec)
	// A list translates the canvas per row and draws the same local shape:
	// the geometry is identical, only the op transform differs.
	for i := 0; i < 8; i++ {
		ctx.Save()
		ctx.Translate(0, float32(i)*10)
		ctx.DrawRect(XYWH(0, 0, 40, 8), Fill(Black))
		ctx.Restore()
	}
	ctx.DrawRect(XYWH(0, 0, 41, 8), Fill(Black)) // a different shape
	seen := map[*Path]int{}
	for _, ch := range rec.root.Children {
		op, ok := ch.(*drawOp)
		if !ok || op.path == nil {
			continue
		}
		seen[op.path]++
	}
	if len(seen) != 2 {
		t.Fatalf("expected 2 distinct interned paths, got %d", len(seen))
	}
}
