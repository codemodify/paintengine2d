package paintengine2d

// Clip is the resolved clip state passed to a [Device].
//
// Scissor is a device-space axis-aligned box (half-open). If HasScissor is
// false the device should use its full bounds.
//
// Mask, when non-nil, is an 8-bit coverage buffer in device space with
// origin (MaskX, MaskY) and size MaskW×MaskH. Coverage is multiplied into
// the path's AA coverage. A future GPU backend can ignore the CPU bytes and
// keep an equivalent stencil/texture behind its own Device implementation.
type Clip struct {
	Scissor    Rect
	HasScissor bool
	Mask       []byte
	MaskX      int
	MaskY      int
	MaskW      int
	MaskH      int
}

// maskAt returns the 8-bit clip coverage at a device pixel, or 255 if none.
func (c Clip) maskAt(x, y int) uint8 {
	if c.Mask == nil {
		return 255
	}
	ix := x - c.MaskX
	iy := y - c.MaskY
	if ix < 0 || iy < 0 || ix >= c.MaskW || iy >= c.MaskH {
		return 0
	}
	return c.Mask[iy*c.MaskW+ix]
}

func (c Clip) intersectScissor(r Rect) Rect {
	if !c.HasScissor {
		return r
	}
	return r.Intersect(c.Scissor)
}

// deviceClip is the mutable clip stack entry stored by Context.
type deviceClip struct {
	scissor                    Rect
	hasScissor                 bool
	mask                       []byte
	maskX, maskY, maskW, maskH int
}

func (d deviceClip) export() Clip {
	return Clip{
		Scissor:    d.scissor,
		HasScissor: d.hasScissor,
		Mask:       d.mask,
		MaskX:      d.maskX,
		MaskY:      d.maskY,
		MaskW:      d.maskW,
		MaskH:      d.maskH,
	}
}

func (d deviceClip) clone() deviceClip {
	out := d
	if d.mask != nil {
		out.mask = append([]byte(nil), d.mask...)
	}
	return out
}
