// Command gallery renders a richer path / gradient / clip / stroke sheet.
package main

import (
	"flag"
	"fmt"
	"log"
	"math"

	"github.com/codemodify/paintengine2d"
)

func main() {
	out := flag.String("o", "gallery.png", "output PNG path")
	flag.Parse()

	const W, H = 960, 540
	img := paintengine2d.NewImage(W, H)
	ctx := paintengine2d.NewContext(img)
	ctx.Clear(paintengine2d.RGB(0.07, 0.08, 0.10))

	// Header band
	ctx.DrawRect(paintengine2d.XYWH(0, 0, W, 72), paintengine2d.Linear(paintengine2d.LinearGradient{
		Start: paintengine2d.Pt(0, 0),
		End:   paintengine2d.Pt(W, 0),
		Stops: []paintengine2d.GradientStop{
			{Offset: 0, Color: paintengine2d.RGB(0.12, 0.22, 0.48)},
			{Offset: 1, Color: paintengine2d.RGB(0.08, 0.42, 0.38)},
		},
	}))

	// Rounded cards
	card(ctx, 24, 96, 280, 200, paintengine2d.RGB(0.23, 0.51, 0.93))
	card(ctx, 328, 96, 280, 200, paintengine2d.RGB(0.93, 0.42, 0.22))
	card(ctx, 632, 96, 304, 200, paintengine2d.RGB(0.22, 0.72, 0.48))

	// Cubic blob
	p := paintengine2d.NewPath()
	p.MoveTo(60, 250)
	p.CubicTo(70, 110, 200, 100, 230, 180)
	p.CubicTo(260, 260, 160, 280, 60, 250)
	p.Close()
	ctx.DrawPath(p, paintengine2d.Fill(paintengine2d.RGBA(1, 1, 1, 0.18)))

	// Stroke showcase
	ctx.Save()
	ctx.Translate(360, 160)
	zig := paintengine2d.NewPath()
	zig.MoveTo(0, 80)
	zig.LineTo(40, 10)
	zig.LineTo(80, 80)
	zig.LineTo(120, 10)
	zig.LineTo(160, 80)
	ctx.DrawPath(zig, paintengine2d.Paint{
		Color:  paintengine2d.RGB(1, 0.92, 0.55),
		Style:  paintengine2d.StyleStroke,
		Stroke: paintengine2d.Stroke{Width: 10, Cap: paintengine2d.CapRound, Join: paintengine2d.JoinRound, MiterLimit: 4},
	})
	ctx.Restore()

	// Clipped rotating squares
	ctx.Save()
	ctx.Translate(784, 196)
	ctx.ClipPath(paintengine2d.CirclePath(paintengine2d.Pt(0, 0), 78))
	ctx.Rotate(0.4)
	ctx.DrawRect(paintengine2d.XYWH(-70, -70, 140, 140), paintengine2d.Fill(paintengine2d.RGB(0.98, 0.82, 0.25)))
	ctx.Rotate(0.5)
	ctx.DrawRect(paintengine2d.XYWH(-50, -50, 140, 140), paintengine2d.Fill(paintengine2d.RGBA(0.15, 0.2, 0.35, 0.65)))
	ctx.Restore()

	// Bottom: even-odd star + concentric ellipses + dashed-looking ticks
	ctx.DrawPath(pentagram(paintengine2d.Pt(160, 420), 90), paintengine2d.Paint{
		Color:    paintengine2d.RGB(0.98, 0.78, 0.22),
		FillRule: paintengine2d.FillEvenOdd,
	})

	ctx.Save()
	ctx.Translate(480, 420)
	for i := 0; i < 6; i++ {
		r := float32(28 + i*14)
		ctx.DrawOval(paintengine2d.XYWH(-r*1.4, -r, r*2.8, r*2), paintengine2d.Paint{
			Color:  paintengine2d.RGBA(0.55, 0.8, 1, 0.35),
			Style:  paintengine2d.StyleStroke,
			Stroke: paintengine2d.Stroke{Width: 3, Cap: paintengine2d.CapRound, Join: paintengine2d.JoinRound, MiterLimit: 4},
		})
	}
	ctx.Restore()

	// Gradient sweep wheel (filled wedges approximated by pies)
	ctx.Save()
	ctx.Translate(800, 420)
	for i := 0; i < 12; i++ {
		a0 := float32(i) * float32(math.Pi*2) / 12
		p := paintengine2d.NewPath()
		p.MoveTo(0, 0)
		p.AddArc(paintengine2d.Pt(0, 0), 88, 88, a0, float32(math.Pi*2)/12)
		p.Close()
		t := float32(i) / 11
		ctx.DrawPath(p, paintengine2d.Fill(paintengine2d.RGB(0.2+0.7*t, 0.35+0.2*(1-t), 0.85-0.6*t)))
	}
	ctx.Restore()

	if err := img.WritePNGFile(*out); err != nil {
		log.Fatal(err)
	}
	fmt.Println("wrote", *out)
}

func card(ctx *paintengine2d.Context, x, y, w, h float32, c paintengine2d.Color) {
	ctx.DrawRoundRect(paintengine2d.XYWH(x, y, w, h), 18, 18, paintengine2d.Fill(c.WithAlpha(0.22)))
	ctx.DrawRoundRect(paintengine2d.XYWH(x, y, w, h), 18, 18, paintengine2d.Paint{
		Color:  c,
		Style:  paintengine2d.StyleStroke,
		Stroke: paintengine2d.Stroke{Width: 2, Cap: paintengine2d.CapRound, Join: paintengine2d.JoinRound, MiterLimit: 4},
	})
}

func pentagram(c paintengine2d.Point, r float32) *paintengine2d.Path {
	p := paintengine2d.NewPath()
	for i := 0; i < 5; i++ {
		ang := -math.Pi/2 + float64(i)*4*math.Pi/5
		x := c.X + r*float32(math.Cos(ang))
		y := c.Y + r*float32(math.Sin(ang))
		if i == 0 {
			p.MoveTo(x, y)
		} else {
			p.LineTo(x, y)
		}
	}
	p.Close()
	return p
}
