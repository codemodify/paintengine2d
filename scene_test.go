package paintengine2d

import "testing"

func TestRecorderDrawSceneMatchesImmediate(t *testing.T) {
	w, h := 80, 48
	rec := NewRecorder(w, h)
	ctx := NewContextDevice(rec)
	paintScene(ctx)
	s := rec.Finish()
	if s == nil || s.Nodes < 3 {
		t.Fatalf("scene %+v", s)
	}
	if s.Reused != 0 {
		t.Fatalf("fresh recording reused=%d", s.Reused)
	}

	got := NewImage(w, h)
	DrawScene(s, NewCPUDevice(got))

	want := NewImage(w, h)
	paintScene(NewContext(want))

	if !imagesClose(got, want, 0) {
		t.Fatal("DrawScene CPU replay diverged from immediate paint")
	}
}

func paintScene(ctx *Context) {
	ctx.Clear(RGB(0.10, 0.11, 0.14))
	ctx.DrawRect(XYWH(8, 8, 24, 16), Fill(RGB(0.9, 0.2, 0.2)))
	ctx.DrawCircle(Pt(56, 24), 12, Fill(RGB(0.2, 0.6, 0.9)))
	ctx.DrawRoundRect(XYWH(10, 30, 28, 12), 3, 3, Paint{
		Color:  RGB(0.95, 0.85, 0.2),
		Style:  StyleStroke,
		Stroke: Stroke{Width: 2, Cap: CapRound, Join: JoinRound, MiterLimit: 4},
	})
}

func TestRecorderAttachReused(t *testing.T) {
	rec := NewRecorder(64, 64)
	g := rec.BeginGroup(1, Identity())
	ctx := NewContextDevice(rec)
	ctx.DrawRect(XYWH(0, 0, 8, 8), Fill(White))
	rec.EndGroup()
	if g == nil || len(g.Children) == 0 {
		t.Fatal("group should contain the rect")
	}
	rec2 := NewRecorder(64, 64)
	rec2.Attach(g)
	rec2.Attach(&GroupNode{ID: 1, Xform: Translation(10, 0), Children: g.Children})
	s := rec2.Finish()
	if s.Reused < 2 {
		t.Fatalf("reused=%d", s.Reused)
	}
	if s.Nodes < 2 {
		t.Fatalf("nodes=%d", s.Nodes)
	}

	img := NewImage(64, 64)
	DrawScene(s, NewCPUDevice(img))
	if alphaAt(img, 2, 2) < 200 {
		t.Fatalf("attached group a=%d", alphaAt(img, 2, 2))
	}
	if alphaAt(img, 12, 2) < 200 {
		t.Fatalf("translated attach a=%d", alphaAt(img, 12, 2))
	}
}

func TestRecorderGroupXform(t *testing.T) {
	rec := NewRecorder(40, 20)
	rec.BeginGroup(7, Translation(12, 0))
	ctx := NewContextDevice(rec)
	ctx.DrawRect(XYWH(0, 0, 8, 8), Fill(White))
	rec.EndGroup()
	img := NewImage(40, 20)
	DrawScene(rec.Finish(), NewCPUDevice(img))
	if alphaAt(img, 1, 1) > 10 {
		t.Fatalf("unshifted origin should stay empty a=%d", alphaAt(img, 1, 1))
	}
	if alphaAt(img, 14, 2) < 200 {
		t.Fatalf("group translation missing a=%d", alphaAt(img, 14, 2))
	}
}

func TestRecorderScratchPathStable(t *testing.T) {
	rec := NewRecorder(32, 32)
	ctx := NewContextDevice(rec)
	ctx.DrawRect(XYWH(2, 2, 6, 6), Fill(Red))
	ctx.DrawRect(XYWH(16, 16, 8, 8), Fill(Blue))
	img := NewImage(32, 32)
	DrawScene(rec.Finish(), NewCPUDevice(img))
	r, _, _, a := img.PremulAt(4, 4)
	if a < 200 || r < 200 {
		t.Fatalf("first scratch rect rgba=%d a=%d", r, a)
	}
	_, _, b, a := img.PremulAt(18, 18)
	if a < 200 || b < 200 {
		t.Fatalf("second scratch rect must be cloned, b=%d a=%d", b, a)
	}
}

func TestDrawSceneCPUKeepsWorking(t *testing.T) {
	rec := NewRecorder(24, 24)
	ctx := NewContextDevice(rec)
	ctx.Clear(Black)
	ctx.FillRect(XYWH(4, 4, 8, 8))
	img := NewImage(24, 24)
	DrawScene(rec.Finish(), NewCPUDevice(img))
	_, _, _, a := img.PremulAt(6, 6)
	if a < 200 {
		t.Fatalf("cpu device replay a=%d", a)
	}
}

func TestOpaqueAxisAlignedRectDetect(t *testing.T) {
	p := RectPath(XYWH(0, 0, 10, 10))
	if !opaqueAxisAlignedRect(p, Identity(), Fill(White), Clip{}) {
		t.Fatal("solid white rect")
	}
	if opaqueAxisAlignedRect(p, Identity(), Fill(White.WithAlpha(0.5)), Clip{}) {
		t.Fatal("translucent should not batch")
	}
	if opaqueAxisAlignedRect(CirclePath(Pt(4, 4), 3), Identity(), Fill(White), Clip{}) {
		t.Fatal("circle is not a rect")
	}
}

func imagesClose(a, b *Image, slop int) bool {
	if a == nil || b == nil || a.Width != b.Width || a.Height != b.Height {
		return false
	}
	for y := 0; y < a.Height; y++ {
		for x := 0; x < a.Width; x++ {
			ar, ag, ab, aa := a.PremulAt(x, y)
			br, bg, bb, ba := b.PremulAt(x, y)
			if absDiff(ar, br) > slop || absDiff(ag, bg) > slop ||
				absDiff(ab, bb) > slop || absDiff(aa, ba) > slop {
				return false
			}
		}
	}
	return true
}
