package paintengine2d

import (
	"github.com/codemodify/paintengine2d/internal/raster"
)

// CPUDevice is the software raster backend. It owns (or wraps) an [Image]
// and implements [Device] with scanline anti-aliasing.
//
// Scratch buffers are reused across draw calls to keep the hot path lean.
type CPUDevice struct {
	img *Image

	ras      raster.Rasterizer
	edges    []raster.Edge
	contours [][]raster.Vec2
	closed   []bool
	verbs    []raster.Verb
	pts      []raster.Vec2
	cover    []uint16
	// outlineStore reuses transformed stroke outlines across draw calls.
	outlineStore [][]raster.Vec2
}

// NewCPUDevice wraps img. The image must outlive the device. Passing nil
// panics — use [NewImage] first.
func NewCPUDevice(img *Image) *CPUDevice {
	if img == nil {
		panic("paintengine2d: NewCPUDevice(nil)")
	}
	return &CPUDevice{img: img}
}

// Image returns the target pixmap.
func (d *CPUDevice) Image() *Image { return d.img }

// Size implements [Device].
func (d *CPUDevice) Size() (w, h int) { return d.img.Width, d.img.Height }

// Clear implements [Device].
func (d *CPUDevice) Clear(c Color) { d.img.Clear(c) }

// Fill implements [Device].
func (d *CPUDevice) Fill(path *Path, xform Matrix, paint Paint, clip Clip) {
	if path == nil || path.Empty() || d.img.Width == 0 || d.img.Height == 0 || !xform.Finite() {
		return
	}
	if d.fillAxisAlignedRect(path, xform, paint, clip) {
		return
	}
	d.preparePath(path, xform)
	if len(d.contours) == 0 {
		return
	}
	d.edges = raster.BuildEdges(d.contours, d.edges)
	d.ras.ResetEdges(d.edges)
	d.rasterFill(paint, xform, clip, int(paint.FillRule))
}

// Stroke implements [Device].
func (d *CPUDevice) Stroke(path *Path, xform Matrix, paint Paint, clip Clip) {
	if path == nil || path.Empty() || d.img.Width == 0 || d.img.Height == 0 || !xform.Finite() {
		return
	}
	st := paint.Stroke.normalized()
	if st.Width <= 0 {
		return
	}
	scale := xform.ApproxScale()
	if !finite32(scale) || scale < 1e-8 {
		return
	}
	// Flatten in user space at a tolerance that is ~0.2 px after xform.
	tol := flattenTol() / scale
	// Expand in user space, then transform the outline (stroke width follows
	// the current matrix, matching Skia / SVG).
	d.packPath(path)
	raster.Flatten(d.verbs, d.pts, tol, &d.contours, &d.closed)
	userContours, userClosed := d.contours, d.closed
	if len(st.Dash) > 0 {
		userContours, userClosed = raster.Dash(userContours, userClosed, st.Dash, st.DashOffset)
	}
	opt := raster.StrokeOpts{
		Width:      st.Width,
		Cap:        int(st.Cap),
		Join:       int(st.Join),
		MiterLimit: st.MiterLimit,
	}
	outlines := raster.ExpandStroke(userContours, userClosed, opt)
	if len(outlines) == 0 {
		return
	}
	d.storeTransformed(outlines, xform)
	d.edges = raster.BuildEdges(d.contours, d.edges)
	d.ras.ResetEdges(d.edges)
	d.rasterFill(paint, xform, clip, raster.FillNonZero)
}

func (d *CPUDevice) storeTransformed(outlines [][]raster.Vec2, xform Matrix) {
	for len(d.outlineStore) < len(outlines) {
		d.outlineStore = append(d.outlineStore, nil)
	}
	d.contours = d.contours[:0]
	d.closed = d.closed[:0]
	for i, c := range outlines {
		tc := d.outlineStore[i]
		if cap(tc) < len(c) {
			tc = make([]raster.Vec2, len(c))
		} else {
			tc = tc[:len(c)]
		}
		for j, p := range c {
			q := xform.Transform(Point{p.X, p.Y})
			tc[j] = raster.Vec2{X: q.X, Y: q.Y}
		}
		d.outlineStore[i] = tc
		d.contours = append(d.contours, tc)
		d.closed = append(d.closed, true)
	}
}

// Blit implements [Device].
func (d *CPUDevice) Blit(src *Image, srcRect, dstRect Rect, xform Matrix, paint Paint, clip Clip) {
	if src == nil || src.Width == 0 || src.Height == 0 || dstRect.Empty() || !xform.Finite() {
		return
	}
	srcRect = srcRect.Canon()
	dstRect = dstRect.Canon()
	if srcRect.Empty() {
		srcRect = XYWH(0, 0, float32(src.Width), float32(src.Height))
	}

	dev := xform.TransformRect(dstRect)
	scissor := XYWH(0, 0, float32(d.img.Width), float32(d.img.Height))
	if clip.HasScissor {
		scissor = scissor.Intersect(clip.Scissor)
	}
	dev = dev.Intersect(scissor)
	if dev.Empty() {
		return
	}
	x0, y0, x1, y1 := clampPixelBounds(dev, d.img.Width, d.img.Height)
	if x0 >= x1 || y0 >= y1 {
		return
	}

	inv, ok := xform.Invert()
	if !ok {
		return
	}
	sx := srcRect.Dx() / dstRect.Dx()
	sy := srcRect.Dy() / dstRect.Dy()
	modA := paint.Color.A
	if paint.Shader == nil && (paint.Color == (Color{}) || modA == 0) {
		// Zero-value paint means "unmodulated blit".
		modA = 1
	}
	if modA <= 0 {
		return
	}
	mod := uint8(clamp32(modA, 0, 1)*255 + 0.5)

	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			m := clip.maskAt(x, y)
			if m == 0 {
				continue
			}
			u := inv.Transform(Pt(float32(x)+0.5, float32(y)+0.5))
			if u.X < dstRect.Min.X || u.X >= dstRect.Max.X || u.Y < dstRect.Min.Y || u.Y >= dstRect.Max.Y {
				continue
			}
			tx := (u.X-dstRect.Min.X)*sx + srcRect.Min.X
			ty := (u.Y-dstRect.Min.Y)*sy + srcRect.Min.Y
			if tx < srcRect.Min.X || tx >= srcRect.Max.X || ty < srcRect.Min.Y || ty >= srcRect.Max.Y {
				continue
			}
			var sr, sg, sb, sa uint8
			if paint.Filter == FilterNearest {
				sr, sg, sb, sa = raster.SampleNearestPremul(src.Pix, src.Width, src.Height, src.RowStride(), tx, ty)
			} else {
				sr, sg, sb, sa = raster.SampleBilinearPremul(src.Pix, src.Width, src.Height, src.RowStride(), tx-0.5, ty-0.5)
			}
			if sa == 0 {
				continue
			}
			cover := m
			if mod != 255 {
				cover = uint8((uint16(cover)*uint16(mod) + 127) / 255)
			}
			i := d.img.pixIndex(x, y)
			raster.BlendSrcOver(d.img.Pix, i, sr, sg, sb, sa, cover)
		}
	}
}

func (d *CPUDevice) packPath(path *Path) {
	d.verbs = d.verbs[:0]
	d.pts = d.pts[:0]
	for _, v := range path.verbs {
		d.verbs = append(d.verbs, raster.Verb(v))
	}
	for _, p := range path.pts {
		d.pts = append(d.pts, raster.Vec2{X: p.X, Y: p.Y})
	}
}

func (d *CPUDevice) preparePath(path *Path, xform Matrix) {
	d.packPath(path)
	if !xform.IsIdentity() {
		for i := range d.pts {
			q := xform.Transform(Point{d.pts[i].X, d.pts[i].Y})
			d.pts[i] = raster.Vec2{X: q.X, Y: q.Y}
		}
	}
	raster.Flatten(d.verbs, d.pts, flattenTol(), &d.contours, &d.closed)
}

func flattenTol() float32 {
	// 0.2 device pixels — tight enough for visible AA, cheap to flatten.
	return 0.2
}

func (d *CPUDevice) rasterFill(paint Paint, xform Matrix, clip Clip, rule int) {
	x0, y0, x1, y1, ok := d.clipBounds(clip)
	if !ok {
		return
	}
	solid := paint.Shader == nil
	var sr, sg, sb, sa uint8
	if solid {
		sr, sg, sb, sa = paint.Color.Premul8()
		if sa == 0 {
			return
		}
	}
	w := d.img.Width
	for y := y0; y < y1; y++ {
		cover := d.ras.CoverageRow(y, w, rule, x0, x1)
		d.blendRow(y, x0, x1, cover, clip, solid, sr, sg, sb, sa, paint, xform)
	}
}

func (d *CPUDevice) blendRow(y, x0, x1 int, cover []uint16, clip Clip, solid bool, sr, sg, sb, sa uint8, paint Paint, xform Matrix) {
	for x := x0; x < x1; x++ {
		c := cover[x]
		if c == 0 {
			continue
		}
		m := clip.maskAt(x, y)
		if m == 0 {
			continue
		}
		if m != 255 {
			c = uint16(c) * uint16(m) / 255
		}
		if c == 0 {
			continue
		}
		if c > 255 {
			c = 255
		}
		i := d.img.pixIndex(x, y)
		if solid {
			raster.BlendSrcOver(d.img.Pix, i, sr, sg, sb, sa, uint8(c))
			continue
		}
		col := paint.Shader.Shade(float32(x)+0.5, float32(y)+0.5, xform)
		rr, gg, bb, aa := col.Premul8()
		if aa == 0 {
			continue
		}
		raster.BlendSrcOver(d.img.Pix, i, rr, gg, bb, aa, uint8(c))
	}
}

func (d *CPUDevice) clipBounds(clip Clip) (x0, y0, x1, y1 int, ok bool) {
	w, h := d.img.Width, d.img.Height
	scissor := XYWH(0, 0, float32(w), float32(h))
	if clip.HasScissor {
		scissor = scissor.Intersect(clip.Scissor)
	}
	if scissor.Empty() {
		return 0, 0, 0, 0, false
	}
	x0, y0, x1, y1 = clampPixelBounds(scissor, w, h)
	return x0, y0, x1, y1, x0 < x1 && y0 < y1
}

func (d *CPUDevice) ensureCover(n int) []uint16 {
	if cap(d.cover) < n {
		d.cover = make([]uint16, n)
	} else {
		d.cover = d.cover[:n]
	}
	return d.cover
}

func (d *CPUDevice) fillAxisAlignedRect(path *Path, xform Matrix, paint Paint, clip Clip) bool {
	if paint.Shader != nil || !isClosedRectPath(path) || !xform.IsAxisAligned() {
		return false
	}
	r := xform.TransformRect(path.Bounds())
	if r.Empty() {
		return true
	}
	x0, y0, x1, y1, ok := d.clipBounds(clip)
	if !ok {
		return true
	}
	sr, sg, sb, sa := paint.Color.Premul8()
	if sa == 0 {
		return true
	}
	// Opaque, integer-aligned rect with a scissor-only clip: tight fill.
	if sa == 255 && clip.Mask == nil &&
		r.Min.X == float32(int(r.Min.X)) && r.Min.Y == float32(int(r.Min.Y)) &&
		r.Max.X == float32(int(r.Max.X)) && r.Max.Y == float32(int(r.Max.Y)) {
		ix0, iy0, ix1, iy1 := clampPixelBounds(r.Intersect(XYWH(float32(x0), float32(y0), float32(x1-x0), float32(y1-y0))), d.img.Width, d.img.Height)
		pix := d.img.Pix
		for y := iy0; y < iy1; y++ {
			i := d.img.pixIndex(ix0, y)
			for x := ix0; x < ix1; x++ {
				pix[i+0] = sr
				pix[i+1] = sg
				pix[i+2] = sb
				pix[i+3] = sa
				i += 4
			}
		}
		return true
	}
	cover := d.ensureCover(d.img.Width)
	for y := y0; y < y1; y++ {
		raster.RectCoverage(cover, y, r.Min.X, r.Min.Y, r.Max.X, r.Max.Y, x0, x1)
		d.blendRow(y, x0, x1, cover, clip, true, sr, sg, sb, sa, paint, xform)
	}
	return true
}

func isClosedRectPath(p *Path) bool {
	if p == nil || len(p.pts) < 4 {
		return false
	}
	hasClose := false
	lines := 0
	moves := 0
	for _, v := range p.verbs {
		switch v {
		case VerbMove:
			moves++
		case VerbLine:
			lines++
		case VerbClose:
			hasClose = true
		default:
			return false
		}
	}
	if moves != 1 || !hasClose || lines < 3 || lines > 4 {
		return false
	}
	var xs, ys [4]float32
	nx, ny := 0, 0
	for _, q := range p.pts {
		seenX := false
		for i := 0; i < nx; i++ {
			if abs32(xs[i]-q.X) < 1e-4 {
				seenX = true
				break
			}
		}
		if !seenX {
			if nx == 4 {
				return false
			}
			xs[nx] = q.X
			nx++
		}
		seenY := false
		for i := 0; i < ny; i++ {
			if abs32(ys[i]-q.Y) < 1e-4 {
				seenY = true
				break
			}
		}
		if !seenY {
			if ny == 4 {
				return false
			}
			ys[ny] = q.Y
			ny++
		}
	}
	return nx == 2 && ny == 2
}

func clampPixelBounds(r Rect, w, h int) (x0, y0, x1, y1 int) {
	x0, y0, x1, y1 = r.IntBounds()
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > w {
		x1 = w
	}
	if y1 > h {
		y1 = h
	}
	return
}
