package paintengine2d_test

import (
	"fmt"

	"github.com/codemodify/paintengine2d"
)

func Example() {
	img := paintengine2d.NewImage(128, 128)
	ctx := paintengine2d.NewContext(img)
	ctx.Clear(paintengine2d.RGB(0.10, 0.11, 0.14))
	ctx.DrawCircle(paintengine2d.Pt(64, 64), 40, paintengine2d.Fill(paintengine2d.RGB(0.2, 0.5, 0.95)))
	ctx.DrawCircle(paintengine2d.Pt(64, 64), 28, paintengine2d.StrokePaint(paintengine2d.White, 4))

	fmt.Println(img.Width, img.Height, paintengine2d.Version)
	// Output: 128 128 0.7.1
}
