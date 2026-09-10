// Command hello draws a small anti-aliased scene and writes hello.png.
package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/codemodify/paintengine2d"
)

func main() {
	out := flag.String("o", "hello.png", "output PNG path")
	flag.Parse()

	img := paintengine2d.NewImage(640, 360)
	ctx := paintengine2d.NewContext(img)
	ctx.Clear(paintengine2d.RGB(0.10, 0.11, 0.14))

	ctx.DrawRoundRect(
		paintengine2d.XYWH(36, 36, 260, 150),
		28, 28,
		paintengine2d.Fill(paintengine2d.RGB(0.23, 0.51, 0.93)),
	)

	p := paintengine2d.NewPath()
	p.MoveTo(360, 70)
	p.CubicTo(430, 10, 520, 210, 600, 70)
	p.LineTo(600, 280)
	p.CubicTo(520, 230, 430, 320, 360, 280)
	p.Close()
	ctx.DrawPath(p, paintengine2d.Paint{
		Color: paintengine2d.RGBA(0.96, 0.56, 0.18, 0.92),
		Style: paintengine2d.StyleFill,
	})

	ctx.DrawCircle(paintengine2d.Pt(166, 268), 52, paintengine2d.Paint{
		Color:  paintengine2d.White,
		Style:  paintengine2d.StyleStroke,
		Stroke: paintengine2d.Stroke{Width: 10, Cap: paintengine2d.CapRound, Join: paintengine2d.JoinRound, MiterLimit: 4},
	})

	if err := img.WritePNGFile(*out); err != nil {
		log.Fatal(err)
	}
	fmt.Println("wrote", *out)
}
