package paintengine2d

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestGoldenImages(t *testing.T) {
	scenes := []struct {
		name string
		draw func(*Context)
		w, h int
	}{
		{"fill_rects", sceneFillRects, 120, 80},
		{"circle_aa", sceneCircleAA, 96, 96},
		{"cubic_blob", sceneCubic, 160, 120},
		{"stroke_caps", sceneCaps, 220, 80},
		{"stroke_joins", sceneJoins, 220, 90},
		{"gradient", sceneGradient, 160, 80},
		{"clip_and_xform", sceneClipXform, 140, 140},
		{"evenodd_star", sceneStar, 100, 100},
		{"image_blit", sceneBlit, 80, 80},
		{"gradient_tiles", sceneGradientTiles, 160, 72},
		{"winding_pair", sceneWindingPair, 140, 70},
		{"clip_stack", sceneClipStack, 100, 100},
		{"alpha_over", sceneAlphaOver, 80, 48},
		{"thin_stroke", sceneThinStroke, 120, 40},
		{"radial", sceneRadial, 96, 96},
		{"dashes", sceneDashes, 200, 80},
		{"image_filter", sceneImageFilter, 96, 48},
		{"ui_label", sceneUILabel, 120, 48},
		{"roundrect_ui", sceneRoundRectUI, 140, 64},
		{"stroke_scaled", sceneStrokeScaled, 96, 96},
		{"diagonal_aa", sceneDiagonalAA, 80, 80},
		{"glyph_clip", sceneGlyphClip, 80, 32},
		{"arc_stroke", sceneArcStroke, 96, 96},
		{"evenodd_clip", sceneEvenOddClip, 80, 80},
		{"ui_button", sceneUIButton, 140, 48},
		{"dash_offset", sceneDashOffset, 200, 40},
		{"rotated_blit", sceneRotatedBlit, 80, 80},
		{"radial_tiles", sceneRadialTiles, 160, 72},
		{"ui_focus_ring", sceneUIFocusRing, 140, 48},
		{"ui_scroll_thumb", sceneUIScrollThumb, 28, 96},
		{"ui_overlap_damage", sceneUIOverlapDamage, 160, 64},
		{"aa_thin_diag", sceneAAThinDiag, 80, 80},
		{"aa_tiny_glyphs", sceneAATinyGlyphs, 120, 32},
		{"clip_xform_edge", sceneClipXformEdge, 96, 96},
		{"wrap_shm_pad", sceneWrapShmPad, 80, 48},
		{"image_tint", sceneImageTint, 128, 48},
		{"glyph_tint", sceneGlyphTint, 160, 48},
	}

	dir := filepath.Join("testdata", "golden")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, sc := range scenes {
		sc := sc
		t.Run(sc.name, func(t *testing.T) {
			img := NewImage(sc.w, sc.h)
			ctx := NewContext(img)
			sc.draw(ctx)
			path := filepath.Join(dir, sc.name+".png")
			if os.Getenv("UPDATE_GOLDENS") == "1" {
				if err := img.WritePNGFile(path); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := DecodePNGFile(path)
			if err != nil {
				t.Fatalf("missing golden %s (run UPDATE_GOLDENS=1 go test): %v", path, err)
			}
			if err := compareImages(want, img, 3, 0.75); err != nil {
				// Write the actual for debugging.
				_ = img.WritePNGFile(filepath.Join(t.TempDir(), sc.name+"_got.png"))
				t.Fatal(err)
			}
		})
	}
}

func sceneFillRects(ctx *Context) {
	ctx.Clear(RGB(0.08, 0.09, 0.11))
	ctx.DrawRect(XYWH(8, 8, 40, 24), Fill(RGB(0.2, 0.55, 0.95)))
	ctx.DrawRect(XYWH(56.4, 12.6, 36.2, 28.8), Fill(RGBA(0.95, 0.35, 0.2, 0.85)))
	ctx.DrawRoundRect(XYWH(12, 44, 90, 28), 10, 10, Fill(RGB(0.3, 0.75, 0.45)))
}

func sceneCircleAA(ctx *Context) {
	ctx.Clear(Transparent)
	ctx.DrawCircle(Pt(48, 48), 28, Fill(RGB(0.15, 0.55, 0.95)))
	ctx.DrawCircle(Pt(48, 48), 18, Paint{
		Color:  RGB(1, 0.85, 0.2),
		Style:  StyleStroke,
		Stroke: Stroke{Width: 5, Cap: CapRound, Join: JoinRound, MiterLimit: 4},
	})
}

func sceneCubic(ctx *Context) {
	ctx.Clear(RGB(0.12, 0.12, 0.14))
	p := NewPath()
	p.MoveTo(16, 90)
	p.CubicTo(20, 10, 70, 10, 80, 60)
	p.CubicTo(90, 110, 140, 20, 148, 90)
	p.LineTo(16, 90)
	p.Close()
	ctx.DrawPath(p, Fill(RGBA(0.95, 0.45, 0.2, 0.9)))
	ctx.DrawPath(p, Paint{
		Color:  White,
		Style:  StyleStroke,
		Stroke: Stroke{Width: 2.5, Cap: CapRound, Join: JoinRound, MiterLimit: 4},
	})
}

func sceneCaps(ctx *Context) {
	ctx.Clear(RGB(0.1, 0.1, 0.12))
	y := float32(20)
	for _, cap := range []Cap{CapButt, CapRound, CapSquare} {
		ctx.DrawLine(Pt(30, y), Pt(190, y), Paint{
			Color:  RGB(0.95, 0.85, 0.35),
			Style:  StyleStroke,
			Stroke: Stroke{Width: 14, Cap: cap, Join: JoinMiter, MiterLimit: 4},
		})
		y += 22
	}
}

func sceneJoins(ctx *Context) {
	ctx.Clear(RGB(0.1, 0.1, 0.12))
	x := float32(16)
	for _, join := range []Join{JoinMiter, JoinRound, JoinBevel} {
		p := NewPath()
		p.MoveTo(x, 70)
		p.LineTo(x+28, 18)
		p.LineTo(x+56, 70)
		ctx.DrawPath(p, Paint{
			Color:  RGB(0.4, 0.8, 0.95),
			Style:  StyleStroke,
			Stroke: Stroke{Width: 12, Cap: CapButt, Join: join, MiterLimit: 4},
		})
		x += 70
	}
}

func sceneGradient(ctx *Context) {
	ctx.Clear(Gray(0.12))
	ctx.DrawRect(XYWH(8, 8, 144, 64), Linear(LinearGradient{
		Start: Pt(8, 8),
		End:   Pt(152, 8),
		Stops: []GradientStop{
			{0, RGB(0.15, 0.35, 0.95)},
			{0.5, RGB(0.2, 0.85, 0.55)},
			{1, RGB(0.95, 0.85, 0.2)},
		},
	}))
}

func sceneClipXform(ctx *Context) {
	ctx.Clear(RGB(0.08, 0.08, 0.1))
	ctx.Save()
	ctx.ClipPath(CirclePath(Pt(70, 70), 52))
	ctx.Translate(70, 70)
	ctx.Rotate(0.35)
	ctx.Translate(-40, -40)
	ctx.DrawRect(XYWH(0, 0, 80, 80), Fill(RGB(0.9, 0.25, 0.35)))
	ctx.DrawRect(XYWH(10, 10, 80, 80), Fill(RGBA(0.2, 0.55, 0.95, 0.75)))
	ctx.Restore()
}

func sceneStar(ctx *Context) {
	ctx.Clear(RGB(0.08, 0.09, 0.12))
	ctx.DrawPath(pentagram(Pt(50, 50), 42), Paint{
		Color:    RGB(1, 0.82, 0.25),
		FillRule: FillEvenOdd,
	})
}

func sceneGradientTiles(ctx *Context) {
	ctx.Clear(Gray(0.1))
	stops := []GradientStop{
		{Offset: 0, Color: RGB(0.9, 0.2, 0.2)},
		{Offset: 1, Color: RGB(0.15, 0.35, 0.95)},
	}
	for i, tile := range []TileMode{TileClamp, TileRepeat, TileMirror} {
		y := float32(6 + i*22)
		ctx.DrawRect(XYWH(8, y, 144, 18), Linear(LinearGradient{
			Start: Pt(8, y),
			End:   Pt(48, y),
			Stops: stops,
			Tile:  tile,
		}))
	}
}

func sceneWindingPair(ctx *Context) {
	ctx.Clear(RGB(0.08, 0.09, 0.11))
	p := overlappingCircles(Pt(34, 35), Pt(54, 35), 22, 22)
	ctx.DrawPath(p, Paint{Color: RGB(0.95, 0.75, 0.2), FillRule: FillNonZero})
	ctx.Translate(70, 0)
	ctx.DrawPath(p, Paint{Color: RGB(0.3, 0.75, 0.95), FillRule: FillEvenOdd})
}

func sceneClipStack(ctx *Context) {
	ctx.Clear(RGB(0.08, 0.08, 0.1))
	ctx.ClipRect(XYWH(10, 10, 80, 80))
	ctx.ClipPath(CirclePath(Pt(50, 50), 38))
	ctx.DrawRect(XYWH(0, 0, 100, 100), Fill(RGB(0.9, 0.35, 0.4)))
	ctx.DrawRect(XYWH(50, 0, 50, 100), Fill(RGBA(0.2, 0.45, 0.95, 0.7)))
}

func sceneAlphaOver(ctx *Context) {
	ctx.Clear(RGB(0.12, 0.12, 0.14))
	ctx.DrawRect(XYWH(8, 8, 40, 32), Fill(RGBA(0.95, 0.25, 0.2, 0.55)))
	ctx.DrawRect(XYWH(28, 8, 40, 32), Fill(RGBA(0.2, 0.45, 0.95, 0.55)))
}

func sceneThinStroke(ctx *Context) {
	ctx.Clear(RGB(0.08, 0.08, 0.1))
	ctx.DrawLine(Pt(8, 12), Pt(112, 12), Paint{
		Color:  RGB(0.95, 0.85, 0.4),
		Style:  StyleStroke,
		Stroke: Stroke{Width: 0.6, Cap: CapRound, Join: JoinRound, MiterLimit: 4},
	})
	p := NewPath()
	p.MoveTo(10, 28)
	p.CubicTo(40, 8, 80, 48, 110, 28)
	ctx.DrawPath(p, Paint{
		Color:  RGB(0.4, 0.85, 1),
		Style:  StyleStroke,
		Stroke: Stroke{Width: 1.25, Cap: CapRound, Join: JoinRound, MiterLimit: 4},
	})
}

func sceneRadial(ctx *Context) {
	ctx.Clear(RGB(0.08, 0.08, 0.1))
	ctx.DrawCircle(Pt(48, 48), 40, Radial(RadialGradient{
		Center: Pt(40, 40),
		Radius: 48,
		Stops: []GradientStop{
			{Offset: 0, Color: RGB(1, 0.85, 0.35)},
			{Offset: 0.55, Color: RGB(0.95, 0.35, 0.2)},
			{Offset: 1, Color: RGB(0.15, 0.12, 0.35)},
		},
	}))
}

func sceneDashes(ctx *Context) {
	ctx.Clear(RGB(0.08, 0.08, 0.1))
	ctx.DrawLine(Pt(12, 18), Pt(188, 18), Paint{
		Color: RGB(0.95, 0.8, 0.35),
		Style: StyleStroke,
		Stroke: Stroke{Width: 6, Cap: CapButt, Join: JoinMiter, MiterLimit: 4,
			Dash: []float32{14, 8}},
	})
	p := NewPath()
	p.MoveTo(16, 60)
	p.CubicTo(70, 20, 130, 90, 184, 50)
	ctx.DrawPath(p, Paint{
		Color: RGB(0.4, 0.85, 1),
		Style: StyleStroke,
		Stroke: Stroke{Width: 4, Cap: CapRound, Join: JoinRound, MiterLimit: 4,
			Dash: []float32{10, 6, 2, 6}},
	})
}

func sceneImageFilter(ctx *Context) {
	ctx.Clear(Gray(0.12))
	src := NewImage(4, 4)
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			if (x+y)&1 == 0 {
				src.SetColor(x, y, RGB(0.95, 0.3, 0.25))
			} else {
				src.SetColor(x, y, RGB(0.2, 0.45, 0.95))
			}
		}
	}
	ctx.DrawImageRectPaint(src, XYWH(0, 0, 4, 4), XYWH(8, 8, 36, 32), Paint{Color: White, Filter: FilterNearest})
	ctx.DrawImageRectPaint(src, XYWH(0, 0, 4, 4), XYWH(52, 8, 36, 32), Paint{Color: White, Filter: FilterBilinear})
}

func sceneUILabel(ctx *Context) {
	ctx.Clear(RGB(0.10, 0.11, 0.14))
	ctx.DrawRoundRect(XYWH(8, 10, 104, 28), 6, 6, Fill(RGB(0.23, 0.51, 0.93)))
	ctx.DrawLabel("SAVE", NewBitmapAtlas(White), Pt(34, 16), Paint{Color: White, Filter: FilterNearest})
}

func sceneRoundRectUI(ctx *Context) {
	ctx.Clear(RGB(0.10, 0.11, 0.14))
	ctx.DrawRoundRect(XYWH(8, 8, 52, 48), 10, 10, Fill(RGB(0.22, 0.48, 0.90)))
	ctx.DrawRoundRect(XYWH(8, 8, 52, 48), 10, 10, StrokePaint(White, 2))
	ctx.DrawRoundRect(XYWH(72, 12, 56, 20), 4, 4, Fill(RGB(0.18, 0.72, 0.42)))
	ctx.DrawRoundRect(XYWH(72, 36, 56, 16), 8, 8, Paint{
		Color:  RGB(0.95, 0.35, 0.28),
		Style:  StyleStroke,
		Stroke: Stroke{Width: 3, Cap: CapRound, Join: JoinRound, MiterLimit: 4},
	})
}

func sceneStrokeScaled(ctx *Context) {
	ctx.Clear(Gray(0.12))
	ctx.Save()
	ctx.Translate(16, 16)
	ctx.Scale(3, 3)
	ctx.DrawCircle(Pt(10, 10), 7, StrokePaint(RGB(0.95, 0.85, 0.35), 1.1))
	ctx.Restore()
	ctx.DrawCircle(Pt(72, 48), 18, StrokePaint(RGB(0.4, 0.75, 1), 3))
}

func sceneDiagonalAA(ctx *Context) {
	ctx.Clear(Gray(0.10))
	ctx.DrawLine(Pt(6, 6), Pt(74, 74), StrokePaint(White, 1.25))
	ctx.DrawLine(Pt(8, 70), Pt(72, 10), Paint{
		Color:  RGB(0.3, 0.75, 0.95),
		Style:  StyleStroke,
		Stroke: Stroke{Width: 2, Cap: CapRound, Join: JoinRound, MiterLimit: 4},
	})
}

func sceneGlyphClip(ctx *Context) {
	ctx.Clear(RGB(0.10, 0.11, 0.14))
	ctx.ClipRect(XYWH(6, 4, 40, 24))
	ctx.DrawRoundRect(XYWH(4, 2, 72, 28), 4, 4, Fill(RGB(0.20, 0.32, 0.48)))
	ctx.DrawLabel("CLIPPED", NewBitmapAtlas(White), Pt(8, 10), Paint{Color: White, Filter: FilterNearest})
}

func sceneArcStroke(ctx *Context) {
	ctx.Clear(RGB(0.10, 0.11, 0.14))
	ctx.DrawCircle(Pt(48, 48), 28, StrokePaint(RGB(0.22, 0.26, 0.32), 6))
	ctx.DrawArc(Pt(48, 48), 28, 28, -1.5708, 2.2, Paint{
		Color:  RGB(0.25, 0.78, 0.52),
		Style:  StyleStroke,
		Stroke: Stroke{Width: 6, Cap: CapRound, Join: JoinRound, MiterLimit: 4},
	})
}

func sceneEvenOddClip(ctx *Context) {
	ctx.Clear(RGB(0.08, 0.09, 0.12))
	ctx.ClipPathRule(pentagram(Pt(40, 40), 34), FillEvenOdd)
	ctx.DrawRect(XYWH(0, 0, 80, 80), Fill(RGB(0.95, 0.82, 0.28)))
}

func sceneUIButton(ctx *Context) {
	ctx.Clear(RGB(0.10, 0.11, 0.14))
	box := XYWH(12, 10, 116, 28)
	ctx.DrawRoundRect(box, 6, 6, Fill(RGB(0.23, 0.51, 0.93)))
	ctx.DrawRoundRect(box, 6, 6, StrokePaint(RGB(0.45, 0.70, 1.0), 1.25))
	ctx.DrawLabel("Save", NewBitmapAtlas(White), Pt(48, 16), Paint{Color: White, Filter: FilterNearest})
}

func sceneDashOffset(ctx *Context) {
	ctx.Clear(RGB(0.08, 0.08, 0.10))
	for i, off := range []float32{0, 6, 12} {
		y := float32(10 + i*12)
		ctx.DrawLine(Pt(8, y), Pt(192, y), Paint{
			Color: RGB(0.95, 0.80, 0.35),
			Style: StyleStroke,
			Stroke: Stroke{Width: 5, Cap: CapButt, Join: JoinMiter, MiterLimit: 4,
				Dash: []float32{12, 8}, DashOffset: off},
		})
	}
}

func sceneRotatedBlit(ctx *Context) {
	ctx.Clear(Gray(0.12))
	src := NewImage(8, 8)
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			if x < 4 {
				src.SetColor(x, y, RGB(0.95, 0.3, 0.25))
			} else {
				src.SetColor(x, y, RGB(0.2, 0.5, 0.95))
			}
		}
	}
	ctx.Translate(40, 40)
	ctx.Rotate(0.35)
	ctx.DrawImageRectPaint(src, XYWH(0, 0, 8, 8), XYWH(-20, -20, 40, 40), Paint{Color: White, Filter: FilterNearest})
}

func sceneRadialTiles(ctx *Context) {
	ctx.Clear(Gray(0.10))
	stops := []GradientStop{
		{Offset: 0, Color: RGB(0.95, 0.85, 0.30)},
		{Offset: 1, Color: RGB(0.20, 0.25, 0.70)},
	}
	for i, tile := range []TileMode{TileClamp, TileRepeat, TileMirror} {
		x := float32(8 + i*50)
		ctx.DrawCircle(Pt(x+20, 36), 22, Radial(RadialGradient{
			Center: Pt(x+20, 36), Radius: 10,
			Stops: stops, Tile: tile,
		}))
	}
}

func sceneUIFocusRing(ctx *Context) {
	ctx.Clear(RGB(0.10, 0.11, 0.14))
	box := XYWH(16, 10, 108, 28)
	ctx.DrawRoundRect(box.Inset(-3), 8, 8, Paint{
		Color:  RGB(0.40, 0.72, 1.0),
		Style:  StyleStroke,
		Stroke: Stroke{Width: 2, Cap: CapRound, Join: JoinRound, MiterLimit: 4},
	})
	ctx.DrawRoundRect(box, 6, 6, Fill(RGB(0.23, 0.51, 0.93)))
	ctx.DrawLabel("Open", NewBitmapAtlas(White), Pt(50, 16), Paint{Color: White, Filter: FilterNearest})
}

func sceneUIScrollThumb(ctx *Context) {
	ctx.Clear(RGB(0.10, 0.11, 0.14))
	ctx.DrawRoundRect(XYWH(8, 6, 12, 84), 6, 6, Fill(RGB(0.16, 0.17, 0.20)))
	ctx.DrawRoundRect(XYWH(9, 22, 10, 28), 5, 5, Fill(RGB(0.48, 0.50, 0.58)))
}

func sceneUIOverlapDamage(ctx *Context) {
	ctx.Clear(RGB(0.10, 0.11, 0.14))
	a := XYWH(12, 10, 72, 36)
	b := XYWH(56, 18, 88, 32)
	ctx.DrawRoundRect(a, 6, 6, Fill(RGBA(0.95, 0.38, 0.30, 0.88)))
	ctx.DrawRoundRect(b, 6, 6, Fill(RGBA(0.25, 0.55, 0.95, 0.80)))
	ctx.DrawRoundRect(a.Union(b).Inset(-1), 0, 0, Paint{
		Color:  RGB(0.95, 0.85, 0.35),
		Style:  StyleStroke,
		Stroke: Stroke{Width: 1, Cap: CapButt, Join: JoinMiter, MiterLimit: 4},
	})
}

func sceneAAThinDiag(ctx *Context) {
	ctx.Clear(Gray(0.10))
	ctx.DrawLine(Pt(4, 6), Pt(76, 22), Paint{
		Color:  White,
		Style:  StyleStroke,
		Stroke: Stroke{Width: 0.6, Cap: CapButt, Join: JoinMiter, MiterLimit: 4},
	})
	ctx.DrawLine(Pt(6, 74), Pt(74, 8), Paint{
		Color:  RGB(0.35, 0.80, 1.0),
		Style:  StyleStroke,
		Stroke: Stroke{Width: 0.85, Cap: CapRound, Join: JoinRound, MiterLimit: 4},
	})
	ctx.DrawLine(Pt(8, 40), Pt(72, 52), Paint{
		Color:  RGB(0.95, 0.75, 0.30),
		Style:  StyleStroke,
		Stroke: Stroke{Width: 1.0, Cap: CapButt, Join: JoinMiter, MiterLimit: 4},
	})
}

func sceneAATinyGlyphs(ctx *Context) {
	ctx.Clear(RGB(0.10, 0.11, 0.14))
	atlas := NewBitmapAtlas(White)
	ctx.DrawLabel("ok /_%", atlas, Pt(6, 6), Paint{Color: White, Filter: FilterNearest})
	ctx.DrawLabel("i=1", atlas, Pt(6, 18), Paint{Color: RGB(0.7, 0.85, 1), Filter: FilterNearest})
	ctx.DrawLabel("xy", atlas, Pt(70.4, 10.6), Paint{Color: RGB(0.95, 0.8, 0.4), Filter: FilterNearest})
}

func sceneClipXformEdge(ctx *Context) {
	ctx.Clear(RGB(0.08, 0.09, 0.12))
	ctx.Save()
	ctx.Translate(48, 48)
	ctx.Rotate(0.55)
	ctx.ClipRoundRect(XYWH(-22, -18, 44, 36), 8, 8)
	ctx.Scale(1.15, 0.85)
	ctx.DrawRect(XYWH(-30, -24, 60, 48), Fill(RGB(0.90, 0.32, 0.38)))
	ctx.DrawRect(XYWH(-8, -24, 16, 48), Fill(RGBA(0.2, 0.55, 0.95, 0.75)))
	ctx.Restore()
}

func sceneWrapShmPad(ctx *Context) {
	const w, h, pad = 80, 48, 24
	stride := w*4 + pad
	buf := make([]byte, h*stride)
	for i := range buf {
		buf[i] = 0x4A
	}
	shm := WrapImage(buf, w, h, stride)
	tmp := NewContext(shm)
	tmp.Clear(RGB(0.10, 0.11, 0.14))
	tmp.DrawRoundRect(XYWH(8, 8, 48, 20), 5, 5, Fill(RGB(0.23, 0.51, 0.93)))
	tmp.DrawRoundRect(XYWH(8, 8, 48, 20), 5, 5, StrokePaint(RGB(0.45, 0.70, 1.0), 1.2))
	tmp.DrawLabel("SHM", NewBitmapAtlas(White), Pt(18, 13), Paint{Color: White, Filter: FilterNearest})
	tmp.DrawRoundRect(XYWH(64, 6, 10, 36), 4, 4, Fill(RGB(0.16, 0.17, 0.20)))
	tmp.DrawRoundRect(XYWH(65, 14, 8, 14), 3, 3, Fill(RGB(0.48, 0.50, 0.58)))
	ctx.DrawImageRectPaint(shm, XYWH(0, 0, float32(w), float32(h)), XYWH(0, 0, float32(w), float32(h)), Paint{Color: White, Filter: FilterNearest})
}

func sceneImageTint(ctx *Context) {
	ctx.Clear(RGB(0.10, 0.11, 0.14))
	src := NewImage(8, 8)
	src.Clear(White)
	colors := []Color{
		RGB(0.95, 0.32, 0.28),
		RGB(0.25, 0.78, 0.52),
		RGB(0.35, 0.60, 1.0),
		RGB(0.95, 0.82, 0.28),
	}
	for i, c := range colors {
		x := float32(8 + i*30)
		ctx.DrawImageRectPaint(src, XYWH(0, 0, 8, 8), XYWH(x, 8, 8, 8), Paint{Color: c, Filter: FilterNearest})
		ctx.DrawImageRectPaint(src, XYWH(0, 0, 8, 8), XYWH(x, 22, 20, 16), Paint{Color: c, Filter: FilterBilinear})
	}
}

func sceneGlyphTint(ctx *Context) {
	ctx.Clear(RGB(0.10, 0.11, 0.14))
	atlas := NewBitmapAtlas(White)
	ctx.DrawLabel("THEME", atlas, Pt(8, 8), Paint{Color: RGB(0.95, 0.38, 0.32), Filter: FilterNearest})
	ctx.DrawLabel("THEME", atlas, Pt(8, 20), Paint{Color: RGB(0.30, 0.80, 0.55), Filter: FilterNearest})
	ctx.DrawLabel("THEME", atlas, Pt(8, 32), Paint{Color: RGB(0.40, 0.65, 1.0), Filter: FilterNearest})
	ctx.DrawLabel("OK 50%", atlas, Pt(80, 8), Paint{Color: RGBA(1, 0.85, 0.3, 0.5), Filter: FilterNearest})
}

func sceneBlit(ctx *Context) {
	ctx.Clear(Gray(0.15))
	src := NewImage(16, 16)
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			if (x+y)&1 == 0 {
				src.SetColor(x, y, RGB(0.2, 0.6, 1))
			} else {
				src.SetColor(x, y, RGB(1, 0.4, 0.2))
			}
		}
	}
	ctx.DrawImageRect(src, XYWH(0, 0, 16, 16), XYWH(8, 8, 64, 64))
}

func compareImages(want, got *Image, maxAbs int, minMatchFrac float64) error {
	if want.Width != got.Width || want.Height != got.Height {
		return fmt.Errorf("size %dx%d vs %dx%d", want.Width, want.Height, got.Width, got.Height)
	}
	n := want.Width * want.Height
	mismatch := 0
	var maxd int
	for y := 0; y < want.Height; y++ {
		for x := 0; x < want.Width; x++ {
			wr, wg, wb, wa := want.PremulAt(x, y)
			gr, gg, gb, ga := got.PremulAt(x, y)
			d := absDiff(wr, gr)
			d = maxInt(d, absDiff(wg, gg))
			d = maxInt(d, absDiff(wb, gb))
			d = maxInt(d, absDiff(wa, ga))
			if d > maxd {
				maxd = d
			}
			if d > maxAbs {
				mismatch++
			}
		}
	}
	match := 1 - float64(mismatch)/float64(n)
	if match < minMatchFrac {
		return fmt.Errorf("golden mismatch: maxΔ=%d  match=%.4f (%d/%d pixels over ±%d)", maxd, match, mismatch, n, maxAbs)
	}
	return nil
}

func absDiff(a, b uint8) int {
	d := int(a) - int(b)
	if d < 0 {
		return -d
	}
	return d
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
