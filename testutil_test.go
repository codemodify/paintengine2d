package paintengine2d

import "testing"

func alphaAt(img *Image, x, y int) uint8 {
	_, _, _, a := img.PremulAt(x, y)
	return a
}

func rgbAt(img *Image, x, y int) (r, g, b, a uint8) {
	return img.PremulAt(x, y)
}

func countPartial(img *Image, lo, hi uint8) int {
	n := 0
	for y := 0; y < img.Height; y++ {
		for x := 0; x < img.Width; x++ {
			a := alphaAt(img, x, y)
			if a > lo && a < hi {
				n++
			}
		}
	}
	return n
}

func countOpaque(img *Image, minA uint8) int {
	n := 0
	for y := 0; y < img.Height; y++ {
		for x := 0; x < img.Width; x++ {
			if alphaAt(img, x, y) >= minA {
				n++
			}
		}
	}
	return n
}

func assertAlpha(t *testing.T, img *Image, x, y int, minA, maxA uint8, msg string) {
	t.Helper()
	a := alphaAt(img, x, y)
	if a < minA || a > maxA {
		t.Fatalf("%s: (%d,%d) alpha=%d want [%d,%d]", msg, x, y, a, minA, maxA)
	}
}

func overlappingCircles(a, b Point, ra, rb float32) *Path {
	p := NewPath()
	p.AddCircle(a, ra)
	p.AddCircle(b, rb)
	return p
}
