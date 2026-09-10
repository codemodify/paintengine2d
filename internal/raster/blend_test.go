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

func TestSampleBilinearCenterAndOutside(t *testing.T) {
	pix := []byte{
		255, 0, 0, 255, 0, 255, 0, 255,
		0, 0, 255, 255, 255, 255, 255, 255,
	}
	r, g, b, a := SampleBilinearPremul(pix, 2, 2, 0, 0)
	if r < 200 || a < 200 || g != 0 {
		t.Fatalf("at (0,0) %d %d %d %d", r, g, b, a)
	}
	r, g, b, a = SampleBilinearPremul(pix, 2, 2, -2, -2)
	if r|g|b|a != 0 {
		t.Fatalf("outside should be transparent %d %d %d %d", r, g, b, a)
	}
}
