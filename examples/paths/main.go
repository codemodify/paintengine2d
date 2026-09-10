// Command paths draws a more complex flattened outline (spiral + blobs).
package main

import (
	"flag"
	"fmt"
	"log"
	"math"

	"github.com/codemodify/paintengine2d"
)

func main() {
	out := flag.String("o", "paths.png", "output PNG path")
	flag.Parse()

	img := paintengine2d.NewImage(720, 480)
	ctx := paintengine2d.NewContext(img)
	ctx.Clear(paintengine2d.RGB(0.05, 0.06, 0.08))

	spiral := paintengine2d.NewPath()
	const turns = 5
	for i := 0; i <= 360*turns; i++ {
		a := float64(i) * math.Pi / 180
		r := 8 + float32(i)*0.12
		x := 240 + r*float32(math.Cos(a))
		y := 240 + r*float32(math.Sin(a))
		if i == 0 {
			spiral.MoveTo(x, y)
		} else {
			spiral.LineTo(x, y)
		}
	}
	ctx.DrawPath(spiral, paintengine2d.Paint{
		Color:  paintengine2d.RGB(0.45, 0.85, 1),
		Style:  paintengine2d.StyleStroke,
		Stroke: paintengine2d.Stroke{Width: 3.5, Cap: paintengine2d.CapRound, Join: paintengine2d.JoinRound, MiterLimit: 4},
	})

	blob := paintengine2d.NewPath()
	blob.MoveTo(500, 120)
	blob.CubicTo(620, 40, 700, 180, 640, 260)
	blob.CubicTo(580, 340, 700, 400, 560, 430)
	blob.CubicTo(420, 460, 400, 280, 460, 220)
	blob.CubicTo(400, 160, 430, 80, 500, 120)
	blob.Close()
	ctx.DrawPath(blob, paintengine2d.Linear(paintengine2d.LinearGradient{
		Start: paintengine2d.Pt(420, 80),
		End:   paintengine2d.Pt(700, 440),
		Stops: []paintengine2d.GradientStop{
			{Offset: 0, Color: paintengine2d.RGB(0.95, 0.35, 0.45)},
			{Offset: 1, Color: paintengine2d.RGB(0.95, 0.8, 0.25)},
		},
	}))
	ctx.DrawPath(blob, paintengine2d.Paint{
		Color:  paintengine2d.White,
		Style:  paintengine2d.StyleStroke,
		Stroke: paintengine2d.Stroke{Width: 2, Join: paintengine2d.JoinRound, MiterLimit: 4},
	})

	if err := img.WritePNGFile(*out); err != nil {
		log.Fatal(err)
	}
	fmt.Println("wrote", *out)
}
