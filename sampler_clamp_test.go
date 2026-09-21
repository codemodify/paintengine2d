package paintengine2d

import "testing"

// The bilinear sampler clamps to the image's edge.
//
// It used to return transparent past the edge, so an image drawn larger than
// itself faded over its outermost half texel — and a nine-slice, whose middle
// is always stretched, showed a pale seam down each join with its fixed
// edges. These pin the clamp on the CPU; gpu_sampler_clamp_linux_test.go
// pins the same pictures on the GPU, which has always clamped.

// clampSource is a 3×3 opaque image, a different colour per column, so a
// fade at either edge or a bleed between columns shows as a wrong colour.
func clampSource() *Image {
	img := NewImage(3, 3)
	cols := [3]Color{RGB(0.8, 0.1, 0.1), RGB(0.1, 0.7, 0.2), RGB(0.1, 0.2, 0.9)}
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			img.SetColor(x, y, cols[x])
		}
	}
	return img
}

// clampScene draws the source scaled up, and a three-piece "nine-slice" row
// whose middle is one texel stretched across twenty pixels, each piece its
// own image as a skin cuts them.
func clampScene(ctx *Context) {
	ctx.Clear(Transparent)
	src := clampSource()
	ctx.DrawImageRectPaint(src, XYWH(0, 0, 3, 3), XYWH(2, 2, 24, 24), Paint{Filter: FilterBilinear})
	left, mid, right := src.SubImage(0, 0, 1, 3), src.SubImage(1, 0, 2, 3), src.SubImage(2, 0, 3, 3)
	y := float32(30)
	ctx.DrawImageRectPaint(left, XYWH(0, 0, 1, 3), XYWH(2, y, 2, 6), Paint{Filter: FilterBilinear})
	ctx.DrawImageRectPaint(mid, XYWH(0, 0, 1, 3), XYWH(4, y, 20, 6), Paint{Filter: FilterBilinear})
	ctx.DrawImageRectPaint(right, XYWH(0, 0, 1, 3), XYWH(24, y, 2, 6), Paint{Filter: FilterBilinear})
}

// checkClamped asserts every pixel the scene covers is opaque and the
// scaled image's outermost pixels are its edge texels' colours.
func checkClamped(t *testing.T, img *Image, where string) {
	t.Helper()
	for _, r := range []Rect{XYWH(2, 2, 24, 24), XYWH(2, 30, 24, 6)} {
		for y := int(r.Min.Y); y < int(r.Max.Y); y++ {
			for x := int(r.Min.X); x < int(r.Max.X); x++ {
				if _, _, _, a := img.PremulAt(x, y); a != 255 {
					t.Fatalf("%s: pixel (%d, %d) has alpha %d; a scaled image must be opaque to its edge", where, x, y, a)
				}
			}
		}
	}
	for _, p := range [][2]int{{2, 2}, {2, 25}, {25, 2}, {25, 25}} {
		want := clampSource()
		sx := 0
		if p[0] > 10 {
			sx = 2
		}
		wr, wg, wb, _ := want.PremulAt(sx, 0)
		r, g, b, _ := img.PremulAt(p[0], p[1])
		if absDiff(r, wr) > 3 || absDiff(g, wg) > 3 || absDiff(b, wb) > 3 {
			t.Fatalf("%s: corner (%d, %d) is %d,%d,%d, want the edge texel %d,%d,%d", where, p[0], p[1], r, g, b, wr, wg, wb)
		}
	}
	// The stretched middle is one texel: every pixel of it is that texel.
	mr, mg, mb, _ := clampSource().PremulAt(1, 0)
	for x := 4; x < 24; x++ {
		r, g, b, _ := img.PremulAt(x, 32)
		if absDiff(r, mr) > 3 || absDiff(g, mg) > 3 || absDiff(b, mb) > 3 {
			t.Fatalf("%s: stretched middle at x=%d is %d,%d,%d, want %d,%d,%d (a seam)", where, x, r, g, b, mr, mg, mb)
		}
	}
}

func TestBilinearBlitClampsToEdgeCPU(t *testing.T) {
	img := NewImage(28, 40)
	ctx := NewContext(img)
	clampScene(ctx)
	checkClamped(t, img, "cpu")
}

func TestBilinearA8BlitClampsToEdgeCPU(t *testing.T) {
	mask := NewImageA8(2, 2)
	for i := range mask.Pix {
		mask.Pix[i] = 255
	}
	mask.Touch()
	img := NewImage(20, 20)
	ctx := NewContext(img)
	ctx.Clear(Transparent)
	ctx.DrawImageRectPaint(mask, XYWH(0, 0, 2, 2), XYWH(0, 0, 20, 20), Paint{Color: White, Filter: FilterBilinear})
	for _, p := range [][2]int{{0, 0}, {19, 0}, {0, 19}, {19, 19}, {10, 0}} {
		if _, _, _, a := img.PremulAt(p[0], p[1]); a != 255 {
			t.Fatalf("a full mask scaled up is full to its edge; (%d, %d) has alpha %d", p[0], p[1], a)
		}
	}
}
