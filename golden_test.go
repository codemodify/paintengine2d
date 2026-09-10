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
