package paintengine2d

import "math"

// BackdropRefractor is a [Device] that can refract, in place, what it has
// already drawn under a rounded rectangle: the lens a pane of glass makes
// of what is behind it near its edge.
//
// Blur alone gives frosted material — acrylic, vibrancy. What separates
// *glass* from frosting is that a pane has thickness, so what lies behind
// it is bent where the pane curves away at the edge and left alone in the
// middle. [Context.BackdropRefract] is a no-op on a device without it, so
// a look must still read without it.
//
// r and corner are in user space and xform maps them to the device; depth
// is how far in from the edge the bending reaches, in user units; clip is
// in device space and bounds what is written.
type BackdropRefractor interface {
	BackdropRefract(r Rect, corner float32, xform Matrix, depth float32, clip Clip)
}

// BackdropRefract bends what is already drawn under the rounded rect
// (r, corner) the way the edge of a pane of glass bends what is behind
// it: nothing through the middle, and most in the last few pixels before
// the edge, where the content is squeezed and drawn inwards.
//
// depth is the thickness of that bevel in user units — how far in from
// the edge the bending reaches. It is the one number that decides whether
// the glass reads as a thin sheet or a thick slab, and past about a
// quarter of the shape's smaller side there is no flat middle left and the
// whole pane looks like a bubble.
//
// Blur first and refract after, which is the order the eye expects: a
// pane frosts what is behind it *and* bends it, and bending an already
// frosted image is what a real pane does to light that has already
// scattered. Devices that cannot refract ignore it.
func (c *Context) BackdropRefract(r Rect, corner, depth float32) {
	if depth <= 0 || r.Empty() {
		return
	}
	b, ok := c.dev.(BackdropRefractor)
	if !ok || c.QuickReject(r) {
		return
	}
	b.BackdropRefract(r, corner, c.cur.xform, depth, c.clip())
	c.markDirtyUser(r)
}

// roundRectSDF is the signed distance from p to the boundary of the
// rounded rect (r, corner): positive inside, negative outside, and the
// gradient is the unit normal almost everywhere.
//
// It is the standard rounded-box distance — fold the point into one
// quadrant by symmetry, and the problem becomes the distance to a corner
// circle — written here rather than taken from the path machinery because
// a resample needs it per pixel and a path would have to be rasterised to
// answer at all.
func roundRectSDF(px, py float32, r Rect, corner float32) float32 {
	hw, hh := r.Dx()/2, r.Dy()/2
	corner = min(corner, hw, hh)
	cx, cy := r.Min.X+hw, r.Min.Y+hh
	// Into the first quadrant.
	qx := absF32(px-cx) - (hw - corner)
	qy := absF32(py-cy) - (hh - corner)
	switch {
	case qx <= 0 && qy <= 0:
		// Inside the straight part: the nearest edge is the nearer axis.
		return corner + min(-qx, -qy)
	case qx > 0 && qy > 0:
		// In a corner: the distance to the corner circle.
		return corner - float32(math.Hypot(float64(qx), float64(qy)))
	case qx > 0:
		return corner - qx
	default:
		return corner - qy
	}
}

// roundRectNormal is the outward unit normal of (r, corner) at p: the
// direction the glass's edge faces, which is the direction content is
// pulled in from.
func roundRectNormal(px, py float32, r Rect, corner float32) (nx, ny float32) {
	hw, hh := r.Dx()/2, r.Dy()/2
	corner = min(corner, hw, hh)
	cx, cy := r.Min.X+hw, r.Min.Y+hh
	dx, dy := px-cx, py-cy
	qx := absF32(dx) - (hw - corner)
	qy := absF32(dy) - (hh - corner)
	switch {
	case qx > 0 && qy > 0:
		n := float32(math.Hypot(float64(qx), float64(qy)))
		if n < 1e-6 {
			return 0, 0
		}
		return signF32(dx) * qx / n, signF32(dy) * qy / n
	case qx <= 0 && qy <= 0:
		// The straight part: whichever edge is nearer.
		if -qx < -qy {
			return signF32(dx), 0
		}
		return 0, signF32(dy)
	case qx > 0:
		return signF32(dx), 0
	default:
		return 0, signF32(dy)
	}
}

// refractOffset is how far the sample is pulled out along the normal at a
// point d inside the edge of a bevel depth deep.
//
// The profile is a circular edge: a pane whose last depth units curve away
// to a quarter round. At the edge (d = 0) the sample comes from a whole
// depth outside; in the flat middle (d >= depth) it comes from where it
// is. 1 - sqrt(1 - t²) rather than t² because the eye reads the circular
// falloff as thickness and the quadratic one as a smudge.
func refractOffset(d, depth float32) float32 {
	if d >= depth || depth <= 0 {
		return 0
	}
	if d < 0 {
		d = 0
	}
	t := 1 - d/depth // 0 in the middle, 1 at the edge
	return depth * (1 - float32(math.Sqrt(float64(1-t*t))))
}

func absF32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func signF32(v float32) float32 {
	if v < 0 {
		return -1
	}
	return 1
}

// BackdropRefract implements [BackdropRefractor] on the CPU: the region
// is copied, then each pixel inside the rounded rect is resampled from
// where the edge of a pane of that thickness would have bent it in from.
//
// It samples a copy rather than working in place for the reason every
// displacement does: a pixel already rewritten is no longer the backdrop,
// and bending in place smears along whichever direction the loop happens
// to run.
func (d *CPUDevice) BackdropRefract(r Rect, corner float32, xform Matrix, depth float32, clip Clip) {
	img := d.img
	if img == nil || depth <= 0 || !xform.Finite() {
		return
	}
	// Everything in device pixels: the shape, the bevel and the sampling
	// all scale with the window.
	scale := xform.ApproxScale()
	dev := xform.TransformRect(r)
	devCorner, devDepth := corner*scale, depth*scale
	if devDepth < 0.5 || dev.Empty() {
		return
	}
	dst := dev
	if clip.HasScissor {
		dst = dst.Intersect(clip.Scissor)
	}
	x0, y0, x1, y1 := clampPixelBounds(dst, img.Width, img.Height)
	if x0 >= x1 || y0 >= y1 {
		return
	}
	// The samples come from up to a bevel outside the shape, so the copy
	// has to reach that far.
	reach := int(math.Ceil(float64(devDepth))) + 1
	sx0, sy0 := max(x0-reach, 0), max(y0-reach, 0)
	sx1, sy1 := min(x1+reach, img.Width), min(y1+reach, img.Height)
	w, h := sx1-sx0, sy1-sy0
	if w < 1 || h < 1 {
		return
	}
	buf := make([]uint8, w*h*4)
	for y := 0; y < h; y++ {
		i := img.pixIndex(sx0, sy0+y)
		copy(buf[y*w*4:(y+1)*w*4], img.Pix[i:i+w*4])
	}

	for y := y0; y < y1; y++ {
		di := img.pixIndex(x0, y)
		for x := x0; x < x1; x, di = x+1, di+4 {
			m := clip.maskAt(x, y)
			if m == 0 {
				continue
			}
			px, py := float32(x)+0.5, float32(y)+0.5
			sd := roundRectSDF(px, py, dev, devCorner)
			if sd <= 0 {
				continue // outside the pane: not glass, not refracted
			}
			off := refractOffset(sd, devDepth)
			if off <= 0 {
				continue // the flat middle: what is behind shows straight through
			}
			nx, ny := roundRectNormal(px, py, dev, devCorner)
			sampleBilinear(buf, w, h, px+nx*off-float32(sx0), py+ny*off-float32(sy0), img.Pix[di:di+4], m)
		}
	}
	img.TouchRect(XYWH(float32(x0), float32(y0), float32(x1-x0), float32(y1-y0)))
}

// sampleBilinear reads buf at (sx, sy) and writes it over dst at coverage
// m. Coordinates are in buf's own pixel space and are clamped to it, so a
// sample that would come from beyond the copied region takes the edge
// pixel rather than nothing.
func sampleBilinear(buf []uint8, w, h int, sx, sy float32, dst []uint8, m uint8) {
	sx, sy = sx-0.5, sy-0.5
	x0 := int(math.Floor(float64(sx)))
	y0 := int(math.Floor(float64(sy)))
	fx, fy := sx-float32(x0), sy-float32(y0)
	at := func(x, y int) []uint8 {
		x = clampInt(x, 0, w-1)
		y = clampInt(y, 0, h-1)
		i := (y*w + x) * 4
		return buf[i : i+4]
	}
	p00, p10 := at(x0, y0), at(x0+1, y0)
	p01, p11 := at(x0, y0+1), at(x0+1, y0+1)
	for k := 0; k < 4; k++ {
		top := float32(p00[k]) + (float32(p10[k])-float32(p00[k]))*fx
		bot := float32(p01[k]) + (float32(p11[k])-float32(p01[k]))*fx
		v := top + (bot-top)*fy
		n := uint32(v + 0.5)
		if n > 255 {
			n = 255
		}
		if m == 255 {
			dst[k] = uint8(n)
			continue
		}
		o := uint32(dst[k])
		dst[k] = uint8((o*(255-uint32(m)) + n*uint32(m) + 127) / 255)
	}
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
