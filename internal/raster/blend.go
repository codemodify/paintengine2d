package raster

// BlendSrcOver blends a premultiplied source (sr,sg,sb,sa) with coverage
// (0–255) onto a premultiplied destination pixel.
func BlendSrcOver(dst []byte, i int, sr, sg, sb, sa, cover uint8) {
	if cover == 0 {
		return
	}
	if cover != 255 {
		sr = mul255(sr, cover)
		sg = mul255(sg, cover)
		sb = mul255(sb, cover)
		sa = mul255(sa, cover)
	}
	if sa == 255 {
		dst[i+0] = sr
		dst[i+1] = sg
		dst[i+2] = sb
		dst[i+3] = 255
		return
	}
	inv := uint8(255 - sa)
	dst[i+0] = sr + mul255(dst[i+0], inv)
	dst[i+1] = sg + mul255(dst[i+1], inv)
	dst[i+2] = sb + mul255(dst[i+2], inv)
	dst[i+3] = sa + mul255(dst[i+3], inv)
}

func mul255(a, b uint8) uint8 {
	return uint8((uint16(a)*uint16(b) + 127) / 255)
}

// SampleBilinearPremul samples a premul RGBA buffer with bilinear filtering.
// Pixels outside [0,w)×[0,h) are treated as transparent.
func SampleBilinearPremul(pix []byte, w, h int, x, y float32) (r, g, b, a uint8) {
	if w <= 0 || h <= 0 {
		return
	}
	x0 := int(mathFloor32(x))
	y0 := int(mathFloor32(y))
	fx := x - float32(x0)
	fy := y - float32(y0)
	if fx < 0 {
		fx += 1
		x0--
	}
	if fy < 0 {
		fy += 1
		y0--
	}
	var sr, sg, sb, sa float32
	for oy := 0; oy < 2; oy++ {
		wy := (1 - fy)
		if oy == 1 {
			wy = fy
		}
		yy := y0 + oy
		for ox := 0; ox < 2; ox++ {
			wx := (1 - fx)
			if ox == 1 {
				wx = fx
			}
			xx := x0 + ox
			wt := wx * wy
			if wt == 0 {
				continue
			}
			pr, pg, pb, pa := pixelAt(pix, w, h, xx, yy)
			sr += float32(pr) * wt
			sg += float32(pg) * wt
			sb += float32(pb) * wt
			sa += float32(pa) * wt
		}
	}
	return uint8(sr + 0.5), uint8(sg + 0.5), uint8(sb + 0.5), uint8(sa + 0.5)
}

// SampleNearestPremul returns the premul pixel covering (x, y) in pixel
// space (pixel i covers [i, i+1)). Outside is transparent.
func SampleNearestPremul(pix []byte, w, h int, x, y float32) (r, g, b, a uint8) {
	if w <= 0 || h <= 0 {
		return
	}
	return pixelAt(pix, w, h, mathFloor32(x), mathFloor32(y))
}

func pixelAt(pix []byte, w, h, x, y int) (r, g, b, a uint8) {
	if x < 0 || y < 0 || x >= w || y >= h {
		return
	}
	i := (y*w + x) * 4
	return pix[i+0], pix[i+1], pix[i+2], pix[i+3]
}

func mathFloor32(v float32) int {
	i := int(v)
	if float32(i) > v {
		return i - 1
	}
	return i
}
