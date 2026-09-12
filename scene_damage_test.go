package paintengine2d

import "testing"

func menuScene(w, h int) *Scene {
	rec := NewRecorder(w, h)
	ctx := NewContextDevice(rec)
	ctx.Clear(RGB(0.10, 0.11, 0.14))
	for i := 0; i < 16; i++ {
		y := float32(8 + i*28)
		ctx.DrawRect(XYWH(8, y, 200, 24), Fill(RGB(0.16, 0.17, 0.20)))
		ctx.DrawRect(XYWH(12, y+4, 8, 16), Fill(RGB(0.3, 0.5, 0.8)))
	}
	for i := 0; i < 10; i++ {
		y := float32(40 + i*26)
		ctx.DrawRect(XYWH(240, y, 280, 24), Fill(RGB(0.18, 0.18, 0.22)))
	}
	ctx.DrawRect(XYWH(240, 40+3*26, 280, 24), Fill(RGB(0.23, 0.51, 0.93)))
	return rec.Finish()
}

func TestDrawSceneDamageMatchesFullInDirtyRegion(t *testing.T) {
	const w, h = 640, 360
	s := menuScene(w, h)
	full := NewImage(w, h)
	DrawScene(s, NewCPUDevice(full))

	dirty := NewImage(w, h)
	NewContext(dirty).Clear(RGB(0.10, 0.11, 0.14))
	row := XYWH(240, 40+3*26, 280, 24)
	var d Damage
	d.Add(row.Inset(-2))
	DrawSceneDamage(s, NewCPUDevice(dirty), &d)

	x0, y0 := int(row.Min.X), int(row.Min.Y)
	x1, y1 := int(row.Max.X), int(row.Max.Y)
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			fr, fg, fb, fa := full.PremulAt(x, y)
			dr, dg, db, da := dirty.PremulAt(x, y)
			if absDiff(fr, dr) > 0 || absDiff(fg, dg) > 0 || absDiff(fb, db) > 0 || absDiff(fa, da) > 0 {
				t.Fatalf("dirty %d,%d full=%d,%d,%d,%d got=%d,%d,%d,%d", x, y, fr, fg, fb, fa, dr, dg, db, da)
			}
		}
	}
}

func TestDrawSceneDamageLeavesOutsideUntouched(t *testing.T) {
	const w, h = 160, 80
	rec := NewRecorder(w, h)
	ctx := NewContextDevice(rec)
	ctx.Clear(RGB(0.10, 0.11, 0.14))
	ctx.DrawRect(XYWH(8, 8, 40, 20), Fill(RGB(0.9, 0.2, 0.2)))
	ctx.DrawRect(XYWH(80, 8, 40, 20), Fill(RGB(0.2, 0.8, 0.3)))
	s := rec.Finish()

	img := NewImage(w, h)
	img.Clear(RGB(0.05, 0.06, 0.07))
	br, bg, bb, ba := img.PremulAt(90, 12)
	var d Damage
	d.Add(XYWH(8, 8, 40, 20))
	DrawSceneDamage(s, NewCPUDevice(img), &d)

	r, _, _, a := img.PremulAt(12, 12)
	if a < 200 || r < 200 {
		t.Fatalf("dirty widget r=%d a=%d", r, a)
	}
	r, g, b, a := img.PremulAt(90, 12)
	if r != br || g != bg || b != bb || a != ba {
		t.Fatalf("clean sibling clobbered %d,%d,%d,%d want %d,%d,%d,%d", r, g, b, a, br, bg, bb, ba)
	}
}

func TestDrawSceneDamageEmptyNoop(t *testing.T) {
	rec := NewRecorder(32, 32)
	NewContextDevice(rec).DrawRect(XYWH(4, 4, 8, 8), Fill(White))
	img := NewImage(32, 32)
	var d Damage
	DrawSceneDamage(rec.Finish(), NewCPUDevice(img), &d)
	_, _, _, a := img.PremulAt(6, 6)
	if a != 0 {
		t.Fatalf("empty dirty should not paint, a=%d", a)
	}
}

func TestDrawSceneRectsClipsReplay(t *testing.T) {
	rec := NewRecorder(64, 32)
	ctx := NewContextDevice(rec)
	ctx.Clear(RGB(0.1, 0.1, 0.1))
	ctx.DrawRect(XYWH(2, 2, 12, 12), Fill(Red))
	ctx.DrawRect(XYWH(40, 2, 12, 12), Fill(Blue))
	img := NewImage(64, 32)
	img.Clear(RGB(0.1, 0.1, 0.1))
	DrawSceneRects(rec.Finish(), NewCPUDevice(img), []Rect{XYWH(2, 2, 12, 12)})
	r, _, _, a := img.PremulAt(6, 6)
	if a < 200 || r < 200 {
		t.Fatalf("first rect r=%d a=%d", r, a)
	}
	_, _, b, a := img.PremulAt(44, 6)
	if b > 40 {
		t.Fatalf("second rect should be skipped, b=%d a=%d", b, a)
	}
}

func TestDrawSceneDamageSkipsCleanFills(t *testing.T) {
	rec := NewRecorder(200, 80)
	ctx := NewContextDevice(rec)
	ctx.Clear(RGB(0.1, 0.1, 0.12))
	for i := 0; i < 8; i++ {
		ctx.DrawRect(XYWH(float32(8+i*24), 8, 20, 20), Fill(RGB(0.2, 0.4, 0.8)))
	}
	s := rec.Finish()
	img := NewImage(200, 80)
	dev := &countDev{CPUDevice: NewCPUDevice(img)}
	var d Damage
	d.Add(XYWH(8, 8, 20, 20))
	DrawSceneDamage(s, dev, &d)
	if dev.fills != 1 {
		t.Fatalf("fills=%d, want 1 dirty widget (ClearRect is not Fill)", dev.fills)
	}
	if dev.clears != 0 {
		t.Fatalf("clears=%d, want 0 (ClearRect path)", dev.clears)
	}
}

func TestBakeGroupReplayMatchesWalk(t *testing.T) {
	rec := NewRecorder(80, 40)
	g := rec.BeginGroup(1, Translation(10, 4))
	ctx := NewContextDevice(rec)
	ctx.DrawRect(XYWH(0, 0, 24, 16), Fill(RGB(0.9, 0.2, 0.2)))
	ctx.DrawRect(XYWH(4, 4, 8, 8), Fill(RGB(0.2, 0.8, 0.3)))
	rec.EndGroup()
	s := rec.Finish()

	want := NewImage(80, 40)
	DrawScene(s, NewCPUDevice(want))

	if BakeGroup(g) == nil || !g.HasLayer() {
		t.Fatal("BakeGroup")
	}
	got := NewImage(80, 40)
	DrawScene(s, NewCPUDevice(got))
	if !imagesClose(got, want, 1) {
		t.Fatal("baked layer blit diverged from live walk")
	}
}

func TestBakeGroupTranslationIsBlit(t *testing.T) {
	rec := NewRecorder(120, 40)
	g := rec.BeginGroup(2, Identity())
	ctx := NewContextDevice(rec)
	for i := 0; i < 12; i++ {
		ctx.DrawRect(XYWH(float32(i*8), 4, 6, 20), Fill(RGB(0.3, 0.5, 0.9)))
	}
	rec.EndGroup()
	if BakeGroup(g) == nil {
		t.Fatal("bake")
	}
	g.Xform = Translation(16, 0)
	img := NewImage(120, 40)
	dev := &countDev{CPUDevice: NewCPUDevice(img)}
	DrawScene(rec.Finish(), dev)
	if dev.fills != 0 {
		t.Fatalf("baked group should not re-fill, fills=%d", dev.fills)
	}
	if dev.blits != 1 {
		t.Fatalf("blits=%d, want 1 layer", dev.blits)
	}
	_, _, b, a := img.PremulAt(18, 10)
	if a < 200 || b < 180 {
		t.Fatalf("translated layer pixel b=%d a=%d", b, a)
	}
}

func TestSplitterTwoPaneLayerReplay(t *testing.T) {
	rec := NewRecorder(200, 80)
	left := rec.BeginGroup(1, Identity())
	lctx := NewContextDevice(rec)
	lctx.DrawRect(XYWH(0, 0, 90, 80), Fill(RGB(0.2, 0.3, 0.8)))
	lctx.DrawRect(XYWH(8, 8, 40, 16), Fill(White))
	rec.EndGroup()
	right := rec.BeginGroup(2, Translation(100, 0))
	rctx := NewContextDevice(rec)
	rctx.DrawRect(XYWH(0, 0, 100, 80), Fill(RGB(0.8, 0.3, 0.2)))
	rctx.DrawRect(XYWH(8, 8, 40, 16), Fill(White))
	rec.EndGroup()
	if BakeGroup(left) == nil || BakeGroup(right) == nil {
		t.Fatal("bake panes")
	}
	s := rec.Finish()

	img := NewImage(200, 80)
	DrawScene(s, NewCPUDevice(img))
	_, _, b, a := img.PremulAt(10, 4)
	if a < 200 || b < 180 {
		t.Fatalf("left pane b=%d a=%d", b, a)
	}
	r, _, _, a := img.PremulAt(120, 4)
	if a < 200 || r < 180 {
		t.Fatalf("right pane r=%d a=%d", r, a)
	}

	// Drag: only Xform changes; re-blit, no re-raster of children.
	right.Xform = Translation(120, 0)
	img2 := NewImage(200, 80)
	dev := &countDev{CPUDevice: NewCPUDevice(img2)}
	var d Damage
	d.Add(XYWH(90, 0, 110, 80))
	DrawSceneDamage(s, dev, &d)
	if dev.fills != 0 || dev.blits == 0 || dev.blits > 2 {
		t.Fatalf("drag should blit baked panes, fills=%d blits=%d", dev.fills, dev.blits)
	}
	r, _, _, a = img2.PremulAt(140, 4)
	if a < 200 || r < 180 {
		t.Fatalf("dragged right pane r=%d a=%d", r, a)
	}
}

func TestBakeGroupMarksOpaquePane(t *testing.T) {
	rec := NewRecorder(40, 20)
	g := rec.BeginGroup(1, Identity())
	NewContextDevice(rec).DrawRect(XYWH(0, 0, 40, 20), Fill(Red))
	rec.EndGroup()
	if BakeGroup(g) == nil || !g.LayerOpaque {
		t.Fatalf("solid pane layer opaque=%v", g.HasLayer() && g.LayerOpaque)
	}
}

func TestGroupInvalidateLayerWalksAgain(t *testing.T) {
	rec := NewRecorder(32, 32)
	g := rec.BeginGroup(1, Identity())
	NewContextDevice(rec).DrawRect(XYWH(2, 2, 8, 8), Fill(White))
	rec.EndGroup()
	if BakeGroup(g) == nil {
		t.Fatal("bake")
	}
	g.InvalidateLayer()
	if g.HasLayer() {
		t.Fatal("layer should be gone")
	}
	img := NewImage(32, 32)
	dev := &countDev{CPUDevice: NewCPUDevice(img)}
	DrawScene(rec.Finish(), dev)
	if dev.fills != 1 {
		t.Fatalf("after invalidate, want live fill, fills=%d", dev.fills)
	}
}

type countDev struct {
	*CPUDevice
	fills, strokes, blits, clears int
}

func (d *countDev) Clear(c Color) {
	d.clears++
	d.CPUDevice.Clear(c)
}

func (d *countDev) Fill(path *Path, xform Matrix, paint Paint, clip Clip) {
	d.fills++
	d.CPUDevice.Fill(path, xform, paint, clip)
}

func (d *countDev) Stroke(path *Path, xform Matrix, paint Paint, clip Clip) {
	d.strokes++
	d.CPUDevice.Stroke(path, xform, paint, clip)
}

func (d *countDev) Blit(src *Image, srcR, dstR Rect, xform Matrix, paint Paint, clip Clip) {
	d.blits++
	d.CPUDevice.Blit(src, srcR, dstR, xform, paint, clip)
}

func BenchmarkDrawSceneFull(b *testing.B) {
	s := menuScene(1920, 1080)
	img := NewImage(1920, 1080)
	dev := NewCPUDevice(img)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DrawScene(s, dev)
	}
}

func BenchmarkDrawSceneDamageHover(b *testing.B) {
	s := menuScene(1920, 1080)
	img := NewImage(1920, 1080)
	DrawScene(s, NewCPUDevice(img))
	dev := NewCPUDevice(img)
	var d Damage
	d.Add(XYWH(240, 40+3*26, 280, 24).Inset(-2))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DrawSceneDamage(s, dev, &d)
	}
}

func BenchmarkSplitterReraster(b *testing.B) {
	s, _, _ := splitterScene(false)
	img := NewImage(800, 400)
	dev := NewCPUDevice(img)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DrawScene(s, dev)
	}
}

func BenchmarkSplitterLayerBlit(b *testing.B) {
	s, left, right := splitterScene(true)
	img := NewImage(800, 400)
	dev := NewCPUDevice(img)
	var d Damage
	d.Add(XYWH(0, 0, 800, 400))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		right.Xform = Translation(float32(400+i%8), 0)
		_ = left
		DrawSceneDamage(s, dev, &d)
	}
}

func splitterScene(bake bool) (*Scene, *GroupNode, *GroupNode) {
	rec := NewRecorder(800, 400)
	left := rec.BeginGroup(1, Identity())
	paintPane(NewContextDevice(rec), RGB(0.2, 0.35, 0.75), RGB(0.45, 0.7, 0.95))
	rec.EndGroup()
	right := rec.BeginGroup(2, Translation(400, 0))
	paintPane(NewContextDevice(rec), RGB(0.75, 0.3, 0.2), RGB(0.95, 0.6, 0.4))
	rec.EndGroup()
	if bake {
		BakeGroup(left)
		BakeGroup(right)
	}
	return rec.Finish(), left, right
}

func paintPane(ctx *Context, panel, accent Color) {
	ctx.DrawRect(XYWH(0, 0, 400, 400), Fill(panel))
	aa := Fill(RGBA(accent.R, accent.G, accent.B, 0.45))
	for i := 0; i < 24; i++ {
		y := float32(8 + i*16)
		ctx.DrawCircle(Pt(28, y+6), 7, aa)
		ctx.DrawRoundRect(XYWH(44, y, 340, 12), 3, 3, aa)
	}
}
