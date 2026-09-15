package paintengine2d

import "math"

// BackdropBlurrer is a [Device] that can blur, in place, what it has
// already drawn under a rectangle: the frosted material of acrylic menus,
// glass captions and vibrant sidebars (CSS backdrop-filter: blur()).
// [Context.BackdropBlur] is a no-op on a device without it.
//
// r is in user space and xform maps it to the device; radius is the
// Gaussian's standard deviation in user units (CSS blur() semantics); clip
// is in device space and bounds what is written.
type BackdropBlurrer interface {
	BackdropBlur(r Rect, xform Matrix, radius float32, clip Clip)
}

// BackdropBlur blurs what is already drawn under r with a Gaussian of
// standard deviation radius (user units), within the current clip: paint a
// translucent tint over it next for acrylic or glass. Devices that cannot
// blur ignore it, so a look must still read without the blur.
func (c *Context) BackdropBlur(r Rect, radius float32) {
	if radius <= 0 || r.Empty() {
		return
	}
	b, ok := c.dev.(BackdropBlurrer)
	if !ok || c.QuickReject(r) {
		return
	}
	b.BackdropBlur(r, c.cur.xform, radius, c.clip())
	c.markDirtyUser(r)
}

// blurReach is how far a Gaussian of standard deviation sigma samples.
func blurReach(sigma float32) int { return int(math.Ceil(float64(sigma) * 3)) }

// boxesForGauss is the three box widths whose repeated average matches a
// Gaussian of standard deviation sigma (odd widths, as a sliding window
// needs a centre).
func boxesForGauss(sigma float64) [3]int {
	const n = 3
	wIdeal := math.Sqrt(12*sigma*sigma/n + 1)
	wl := int(math.Floor(wIdeal))
	if wl%2 == 0 {
		wl--
	}
	if wl < 1 {
		wl = 1
	}
	wu := wl + 2
	mIdeal := (12*sigma*sigma - n*float64(wl*wl) - 4*n*float64(wl) - 3*n) / (-4*float64(wl) - 4)
	m := int(math.Round(mIdeal))
	var out [3]int
	for i := range out {
		if i < m {
			out[i] = wl
		} else {
			out[i] = wu
		}
	}
	return out
}

// BackdropBlur implements [BackdropBlurrer] on the CPU: three box passes
// per axis over the pixels under r (and as far round it as the blur
// reaches), written back inside r and the clip, weighted by a clip mask.
func (d *CPUDevice) BackdropBlur(r Rect, xform Matrix, radius float32, clip Clip) {
	img := d.img
	if img == nil || radius <= 0 || !xform.Finite() {
		return
	}
	sigma := radius * xform.ApproxScale()
	if sigma < 0.5 {
		return
	}
	dst := xform.TransformRect(r)
	if clip.HasScissor {
		dst = dst.Intersect(clip.Scissor)
	}
	x0, y0, x1, y1 := clampPixelBounds(dst, img.Width, img.Height)
	if x0 >= x1 || y0 >= y1 {
		return
	}
	reach := blurReach(sigma)
	sx0, sy0 := max(x0-reach, 0), max(y0-reach, 0)
	sx1, sy1 := min(x1+reach, img.Width), min(y1+reach, img.Height)
	w, h := sx1-sx0, sy1-sy0
	// Work on a copy of the sampled area, channel-interleaved.
	buf := make([]uint8, w*h*4)
	for y := 0; y < h; y++ {
		i := img.pixIndex(sx0, sy0+y)
		copy(buf[y*w*4:(y+1)*w*4], img.Pix[i:i+w*4])
	}
	tmp := make([]uint8, len(buf))
	for _, box := range boxesForGauss(float64(sigma)) {
		rad := (box - 1) / 2
		boxBlurH(buf, tmp, w, h, rad)
		boxBlurV(tmp, buf, w, h, rad)
	}
	for y := y0; y < y1; y++ {
		di := img.pixIndex(x0, y)
		si := ((y-sy0)*w + (x0 - sx0)) * 4
		for x := x0; x < x1; x, di, si = x+1, di+4, si+4 {
			m := clip.maskAt(x, y)
			if m == 0 {
				continue
			}
			if m == 255 {
				copy(img.Pix[di:di+4], buf[si:si+4])
				continue
			}
			for k := 0; k < 4; k++ {
				o, n := uint32(img.Pix[di+k]), uint32(buf[si+k])
				img.Pix[di+k] = uint8((o*(255-uint32(m)) + n*uint32(m) + 127) / 255)
			}
		}
	}
	img.TouchRect(XYWH(float32(x0), float32(y0), float32(x1-x0), float32(y1-y0)))
}

// boxBlurH averages each row of src over a window of 2·rad+1 pixels into
// dst, extending the edge pixels.
func boxBlurH(src, dst []uint8, w, h, rad int) {
	if rad <= 0 {
		copy(dst, src)
		return
	}
	span := uint32(2*rad + 1)
	for y := 0; y < h; y++ {
		row := y * w * 4
		for c := 0; c < 4; c++ {
			at := func(x int) uint32 {
				if x < 0 {
					x = 0
				} else if x >= w {
					x = w - 1
				}
				return uint32(src[row+x*4+c])
			}
			var sum uint32
			for x := -rad; x <= rad; x++ {
				sum += at(x)
			}
			for x := 0; x < w; x++ {
				dst[row+x*4+c] = uint8((sum + span/2) / span)
				sum += at(x+rad+1) - at(x-rad)
			}
		}
	}
}

// boxBlurV is [boxBlurH] down the columns.
func boxBlurV(src, dst []uint8, w, h, rad int) {
	if rad <= 0 {
		copy(dst, src)
		return
	}
	span := uint32(2*rad + 1)
	stride := w * 4
	for x := 0; x < w; x++ {
		for c := 0; c < 4; c++ {
			col := x*4 + c
			at := func(y int) uint32 {
				if y < 0 {
					y = 0
				} else if y >= h {
					y = h - 1
				}
				return uint32(src[y*stride+col])
			}
			var sum uint32
			for y := -rad; y <= rad; y++ {
				sum += at(y)
			}
			for y := 0; y < h; y++ {
				dst[y*stride+col] = uint8((sum + span/2) / span)
				sum += at(y+rad+1) - at(y-rad)
			}
		}
	}
}

var _ BackdropBlurrer = (*CPUDevice)(nil)
