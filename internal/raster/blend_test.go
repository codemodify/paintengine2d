package raster

import "testing"

func TestBlendSrcOverOpaqueAndPartial(t *testing.T) {
	dst := []byte{0, 0, 0, 0, 10, 10, 10, 10}
	BlendSrcOver(dst, 0, 255, 0, 0, 255, 255)
	if dst[0] != 255 || dst[3] != 255 {
		t.Fatalf("opaque %v", dst[:4])
	}
	BlendSrcOver(dst, 4, 255, 255, 255, 255, 128)
	if dst[4] <= 10 || dst[7] <= 10 {
		t.Fatalf("partial over %v", dst[4:8])
	}
}

func TestTintPremulRGBWhiteAtlas(t *testing.T) {
	r, g, b, a := TintPremulRGB(255, 255, 255, 255, 255, 0, 0)
	if r != 255 || g != 0 || b != 0 || a != 255 {
		t.Fatalf("white * red = %d %d %d %d", r, g, b, a)
	}
	r, g, b, a = TintPremulRGB(128, 128, 128, 128, 255, 0, 0)
	if r != 128 || g != 0 || b != 0 || a != 128 {
		t.Fatalf("premul white * red = %d %d %d %d", r, g, b, a)
	}
	r, g, b, a = TintPremulRGB(200, 80, 40, 200, 255, 255, 255)
	if r != 200 || g != 80 || b != 40 || a != 200 {
		t.Fatalf("white tint is a no-op: %d %d %d %d", r, g, b, a)
	}
}

func TestSampleBilinearCenterAndOutside(t *testing.T) {
	pix := []byte{
		255, 0, 0, 255, 0, 255, 0, 255,
		0, 0, 255, 255, 255, 255, 255, 255,
	}
	r, g, b, a := SampleBilinearPremul(pix, 2, 2, 8, 0, 0)
	if r < 200 || a < 200 || g != 0 {
		t.Fatalf("at (0,0) %d %d %d %d", r, g, b, a)
	}
	r, g, b, a = SampleBilinearPremul(pix, 2, 2, 8, -2, -2)
	if r|g|b|a != 0 {
		t.Fatalf("outside should be transparent %d %d %d %d", r, g, b, a)
	}
}
