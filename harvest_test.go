package paintengine2d

import "testing"

// Additional regression scenarios harvested from public engine *themes*
// (AGG demos, Skia canvas contracts, Cairo clips, JUCE nested xforms,
// Serenity painter lines). No upstream source is vendored.

func TestSaveDepthTwenty(t *testing.T) {
	img := NewImage(32, 32)
	ctx := NewContext(img)
	ctx.SetColor(White)
	for i := 0; i < 20; i++ {
		ctx.Save()
		ctx.Translate(0.4, 0)
	}
	ctx.FillRect(XYWH(2, 2, 6, 6))
	for i := 0; i < 20; i++ {
		ctx.Restore()
	}
	ctx.SetColor(RGB(1, 0, 0))
	ctx.FillRect(XYWH(20, 20, 6, 6))
	assertAlpha(t, img, 12, 4, 200, 255, "deep save still draws")
	r, _, _, a := rgbAt(img, 22, 22)
	if a < 250 || r < 250 {
		t.Fatalf("restored color r=%d a=%d", r, a)
	}
}

func TestCollinearPathFills(t *testing.T) {
	img := NewImage(24, 24)
	ctx := NewContext(img)
	p := NewPath()
	p.MoveTo(4, 4)
	p.LineTo(12, 4)
	p.LineTo(20, 4)
	p.LineTo(20, 20)
	p.LineTo(4, 20)
	p.Close()
	ctx.DrawPath(p, Fill(White))
	assertAlpha(t, img, 12, 12, 250, 255, "collinear top edge")
}

func TestOpenVsClosedStroke(t *testing.T) {
	open := NewImage(32, 24)
	closed := NewImage(32, 24)
	p := NewPath()
	p.MoveTo(6, 6)
	p.LineTo(26, 6)
	p.LineTo(26, 18)
	p.LineTo(6, 18)
	co := p.Clone()
	co.Close()
	NewContext(open).DrawPath(p, StrokePaint(White, 3))
	NewContext(closed).DrawPath(co, StrokePaint(White, 3))
	// Closed should paint the left edge; open should not.
	if alphaAt(closed, 6, 12) < 150 {
		t.Fatalf("closed left edge a=%d", alphaAt(closed, 6, 12))
	}
	if alphaAt(open, 6, 12) > 40 {
		t.Fatalf("open should lack the closing edge, a=%d", alphaAt(open, 6, 12))
	}
}

func TestClipAfterRotateThenIdentityDraw(t *testing.T) {
	img := NewImage(48, 48)
	ctx := NewContext(img)
	ctx.Translate(24, 24)
	ctx.Rotate(0.7)
	ctx.ClipRect(XYWH(-10, -6, 20, 12))
	ctx.SetMatrix(Identity())
	ctx.SetColor(White)
	ctx.FillRect(XYWH(0, 0, 48, 48))
	if countOpaque(img, 200) == 0 {
		t.Fatal("rotated clip should leave some coverage")
	}
	if countOpaque(img, 200) == 48*48 {
		t.Fatal("rotated clip should not be the full canvas")
	}
}

func TestPartialSrcRectBlit(t *testing.T) {
	src := NewImage(8, 8)
	src.Clear(RGB(1, 0, 0))
	for y := 0; y < 8; y++ {
		for x := 4; x < 8; x++ {
			src.SetColor(x, y, RGB(0, 0, 1))
		}
	}
	dst := NewImage(16, 16)
	ctx := NewContext(dst)
	ctx.DrawImageRect(src, XYWH(4, 0, 4, 8), XYWH(2, 2, 8, 8))
	_, _, b, a := rgbAt(dst, 4, 6)
	if a < 200 || b < 200 {
		t.Fatalf("partial src should be blue, b=%d a=%d", b, a)
	}
}

func TestSinglePointAndEmptyArc(t *testing.T) {
	img := NewImage(12, 12)
	ctx := NewContext(img)
	p := NewPath()
	p.MoveTo(3, 3)
	p.AddArc(Pt(6, 6), 0, 4, 0, 1) // rx=0 → no-op
	ctx.DrawPath(p, Fill(White))
	if countOpaque(img, 1) != 0 {
		t.Fatal("point + empty arc")
	}
}
