package paintengine2d

import "testing"

// A group clip lives in the parent's space, so re-attaching the group with a
// different Xform scrolls the content under a viewport that stays put.
// Before v0.11 the only clip available was the per-op clip, which moved with
// the content and let it paint outside the viewport.
func TestGroupClipIsParentSpace(t *testing.T) {
	content := &GroupNode{Xform: Identity()}
	rec := NewRecorder(100, 100)
	ctx := NewContextDevice(rec)
	g := rec.BeginGroup(7, Identity())
	ctx.DrawRect(XYWH(0, 0, 50, 200), Fill(Black)) // tall content
	rec.EndGroup()
	content.Children = g.Children

	draw := func(dy float32) *Image {
		r2 := NewRecorder(100, 100)
		c2 := NewContextDevice(r2)
		c2.Clear(White)
		r2.Attach(&GroupNode{
			ID:       7,
			Xform:    Translation(0, dy),
			Clip:     XYWH(0, 0, 50, 50), // fixed viewport
			HasClip:  true,
			Children: content.Children,
		})
		img := NewImage(100, 100)
		DrawScene(r2.Finish(), NewCPUDevice(img))
		return img
	}

	img := draw(30)
	if _, _, _, a := img.PremulAt(10, 65); a != 255 {
		t.Fatalf("scrolled content must not paint past the viewport (alpha %d)", a)
	}
	if r, _, _, _ := img.PremulAt(10, 65); r != 255 {
		t.Fatalf("pixel below the viewport should stay white, got R=%d", r)
	}
	if r, _, _, _ := img.PremulAt(10, 40); r != 0 {
		t.Fatal("content inside the viewport should paint")
	}
	if r, _, _, _ := img.PremulAt(10, 10); r != 255 {
		t.Fatal("content scrolled down leaves the top of the viewport clear")
	}

	img = draw(-30)
	if r, _, _, _ := img.PremulAt(10, 35); r != 0 {
		t.Fatalf("content scrolled up must still fill the viewport, got R=%d", r)
	}
	if r, _, _, _ := img.PremulAt(10, 60); r != 255 {
		t.Fatal("below the viewport stays clear")
	}
}

// SetClipPath gives a group a rounded/circular viewport.
func TestGroupClipPath(t *testing.T) {
	rec := NewRecorder(60, 60)
	ctx := NewContextDevice(rec)
	ctx.Clear(White)
	g := rec.BeginGroup(1, Identity())
	ctx.DrawRect(XYWH(0, 0, 60, 60), Fill(Black))
	rec.EndGroup()
	g.SetClipPath(CirclePath(Pt(30, 30), 20), FillNonZero)

	img := NewImage(60, 60)
	DrawScene(rec.Finish(), NewCPUDevice(img))
	if r, _, _, _ := img.PremulAt(30, 30); r != 0 {
		t.Fatal("circle interior should be painted")
	}
	if r, _, _, _ := img.PremulAt(2, 2); r != 255 {
		t.Fatalf("outside the circle clip must stay clear, got R=%d", r)
	}
}

// A group clip must also bound a baked layer blit.
func TestGroupClipAppliesToBakedLayer(t *testing.T) {
	rec := NewRecorder(80, 80)
	ctx := NewContextDevice(rec)
	g := rec.BeginGroup(1, Identity())
	ctx.DrawRect(XYWH(0, 0, 60, 60), Fill(Black))
	rec.EndGroup()
	scene := rec.Finish()
	if BakeGroup(g) == nil {
		t.Fatal("bake")
	}
	g.SetClipRect(XYWH(0, 0, 20, 20))

	img := NewImage(80, 80)
	img.Clear(White)
	DrawScene(scene, NewCPUDevice(img))
	if r, _, _, _ := img.PremulAt(5, 5); r != 0 {
		t.Fatal("layer inside the clip should blit")
	}
	if r, _, _, _ := img.PremulAt(40, 40); r != 255 {
		t.Fatalf("layer outside the group clip must not blit, got R=%d", r)
	}
}

// Regression: a dirty replay used to scissor every op to the union of the
// dirty boxes while erasing only the boxes, so a translucent op that
// straddled two boxes was composited again over the un-erased gap.
func TestDamageReplayDoesNotDoubleBlend(t *testing.T) {
	rec := NewRecorder(200, 50)
	ctx := NewContextDevice(rec)
	ctx.Clear(White)
	ctx.DrawRect(XYWH(0, 0, 200, 50), Fill(RGBA(0, 0, 0, 0.5)))
	scene := rec.Finish()

	img := NewImage(200, 50)
	dev := NewCPUDevice(img)
	DrawScene(scene, dev)
	want, _, _, _ := img.PremulAt(100, 25)

	var d Damage
	d.Add(XYWH(0, 0, 10, 50))
	d.Add(XYWH(190, 0, 10, 50))
	if d.Count() != 2 {
		t.Fatalf("expected two disjoint dirty boxes, got %d", d.Count())
	}
	DrawSceneDamage(scene, dev, &d)
	if got, _, _, _ := img.PremulAt(100, 25); got != want {
		t.Fatalf("pixel between the dirty boxes changed: %d → %d", want, got)
	}
	if got, _, _, _ := img.PremulAt(2, 25); got != want {
		t.Fatalf("pixel inside a dirty box should re-render identically: %d vs %d", got, want)
	}
}

// Regression: the root group's bounds are the union of its draw ops, so a
// dirty box covering only a removed widget missed every op, the group was
// skipped, and the recorded Clear never ran — leaving the old pixels.
func TestDamageReplayRestoresBackgroundWhereOpsWereRemoved(t *testing.T) {
	build := func(withTip bool) *Scene {
		rec := NewRecorder(100, 50)
		ctx := NewContextDevice(rec)
		ctx.Clear(White)
		ctx.DrawRect(XYWH(5, 5, 20, 20), Fill(Blue))
		if withTip {
			ctx.DrawRect(XYWH(60, 10, 30, 20), Fill(Red))
		}
		return rec.Finish()
	}
	img := NewImage(100, 50)
	dev := NewCPUDevice(img)
	DrawScene(build(true), dev)
	if r, _, _, _ := img.PremulAt(70, 20); r == 0 {
		t.Fatal("tooltip should be painted")
	}
	DrawSceneRects(build(false), dev, []Rect{XYWH(60, 10, 30, 20)})
	r, g, b, _ := img.PremulAt(70, 20)
	if r != 255 || g != 255 || b != 255 {
		t.Fatalf("removed widget left stale pixels: %d/%d/%d", r, g, b)
	}
	if r, _, _, _ := img.PremulAt(10, 15); r != 0 {
		t.Fatal("the surviving widget outside the dirty box must not be erased")
	}
}

// Regression: blit bounds were not padded, so a fractional destination edge
// fell outside an outward-rounded dirty rect and the blit was skipped even
// though the rect had erased its pixels.
func TestDamageReplayKeepsFractionalBlits(t *testing.T) {
	icon := NewImage(10, 10)
	icon.Clear(Red)
	rec := NewRecorder(40, 20)
	ctx := NewContextDevice(rec)
	ctx.Clear(White)
	ctx.DrawImageRectPaint(icon, XYWH(0, 0, 10, 10), XYWH(10.5, 5, 10, 10), Paint{Filter: FilterNearest})
	scene := rec.Finish()

	img := NewImage(40, 20)
	dev := NewCPUDevice(img)
	DrawScene(scene, dev)
	wr, wg, wb, _ := img.PremulAt(10, 8)
	DrawSceneRects(scene, dev, []Rect{XYWH(0, 0, 10.5, 20)})
	r, g, b, _ := img.PremulAt(10, 8)
	if r != wr || g != wg || b != wb {
		t.Fatalf("fractional blit dropped by dirty replay: want %d/%d/%d got %d/%d/%d", wr, wg, wb, r, g, b)
	}
}

// A recorded clip mask must survive a fractional or scaled group transform.
func TestMapClipResamplesMask(t *testing.T) {
	rec := NewRecorder(100, 100)
	ctx := NewContextDevice(rec)
	g := rec.BeginGroup(1, Identity())
	ctx.Save()
	ctx.ClipRoundRect(XYWH(0, 0, 20, 20), 10, 10) // a circle
	ctx.DrawRect(XYWH(0, 0, 20, 20), Fill(Black))
	ctx.Restore()
	rec.EndGroup()
	scene := rec.Finish()
	g.Xform = Scaling(2, 2)

	img := NewImage(100, 100)
	img.Clear(White)
	DrawScene(scene, NewCPUDevice(img))
	if r, _, _, _ := img.PremulAt(20, 20); r != 0 {
		t.Fatal("scaled circle centre should paint")
	}
	if r, _, _, _ := img.PremulAt(2, 2); r != 255 {
		t.Fatalf("scaled group dropped its clip mask and filled the corner (R=%d)", r)
	}
}
