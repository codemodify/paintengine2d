package paintengine2d

import (
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
)

// Image is a pixmap in premultiplied 8-bit sRGB RGBA.
//
// Layout: row-major, 4 bytes per pixel (R, G, B, A). [Image.Stride] is the
// byte distance between rows (default Width*4). A UI toolkit or compositor
// can [WrapImage] an existing buffer (swapchain, X11 shm, Wayland shm)
// without copying. This package does not talk to those platforms.
//
// Channels are premultiplied: a fully transparent pixel is 0,0,0,0; a 50%
// red pixel is approximately 128,0,0,128. This is the format the CPU
// backend blends into.
//
// Image implements [image.Image] using straight-alpha [color.NRGBA] so
// standard encoders (PNG) see conventional colors.
type Image struct {
	Width  int
	Height int
	// Stride is bytes per row. Zero means packed (Width * 4).
	Stride int
	Pix    []byte
	// Epoch increments when pixels change ([Image.Clear], [Image.SetColor],
	// [Image.Bump] / [Image.Touch] / [Image.TouchRect]). [GPUDevice] keys
	// its texture cache on this so a reused glyph/icon atlas is re-uploaded
	// after an in-place bake. [Image.Dirty] is the union of in-place writes
	// since the last GPU upload (empty means “whole image”).
	Epoch uint64
	Dirty Rect
}

// NewImage allocates a transparent packed w×h pixmap (stride = width*4).
func NewImage(w, h int) *Image {
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	return &Image{
		Width:  w,
		Height: h,
		Stride: w * 4,
		Pix:    make([]byte, w*h*4),
	}
}

// WrapImage attaches an existing premul RGBA buffer. The caller owns pix;
// it must remain valid and unchanged in length for the Image's lifetime.
//
// stride is bytes per row and must be >= width*4. A stride of 0 means packed
// (width*4). Returns nil if the buffer is too small or the size is invalid.
//
// Typical callers: a UI framework painting into a swapchain/shm buffer, or
// a window manager painting decorations into an X11/Wayland surface.
func WrapImage(pix []byte, w, h, stride int) *Image {
	if w <= 0 || h <= 0 || pix == nil {
		return nil
	}
	if stride <= 0 {
		stride = w * 4
	}
	if stride < w*4 {
		return nil
	}
	need := (h-1)*stride + w*4
	if len(pix) < need {
		return nil
	}
	return &Image{Width: w, Height: h, Stride: stride, Pix: pix}
}

// RowStride returns the byte stride (never 0 for a non-empty image).
func (im *Image) RowStride() int {
	if im == nil {
		return 0
	}
	if im.Stride > 0 {
		return im.Stride
	}
	return im.Width * 4
}

// NewImageFromNRGBA copies an [image.NRGBA] (or converts any image.Image)
// into a premultiplied pixmap.
func NewImageFromNRGBA(src image.Image) *Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	im := NewImage(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			n := color.NRGBAModel.Convert(src.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
			c := FromNRGBA(n)
			r, g, bl, a := c.Premul8()
			i := im.pixIndex(x, y)
			im.Pix[i+0] = r
			im.Pix[i+1] = g
			im.Pix[i+2] = bl
			im.Pix[i+3] = a
		}
	}
	return im
}

func (im *Image) pixIndex(x, y int) int { return y*im.RowStride() + x*4 }

// ColorModel implements [image.Image].
func (im *Image) ColorModel() color.Model { return color.NRGBAModel }

// Bounds implements [image.Image].
func (im *Image) Bounds() image.Rectangle {
	if im == nil {
		return image.Rectangle{}
	}
	return image.Rect(0, 0, im.Width, im.Height)
}

// At implements [image.Image] (straight-alpha NRGBA).
func (im *Image) At(x, y int) color.Color { return im.NRGBAAt(x, y) }

// NRGBAAt returns the straight-alpha 8-bit color at a pixel.
func (im *Image) NRGBAAt(x, y int) color.NRGBA {
	if im == nil || x < 0 || y < 0 || x >= im.Width || y >= im.Height {
		return color.NRGBA{}
	}
	i := im.pixIndex(x, y)
	return FromPremul8(im.Pix[i+0], im.Pix[i+1], im.Pix[i+2], im.Pix[i+3]).NRGBA()
}

// PremulAt returns the raw premultiplied 8-bit pixel.
func (im *Image) PremulAt(x, y int) (r, g, b, a uint8) {
	if im == nil || x < 0 || y < 0 || x >= im.Width || y >= im.Height {
		return
	}
	i := im.pixIndex(x, y)
	return im.Pix[i+0], im.Pix[i+1], im.Pix[i+2], im.Pix[i+3]
}

// SetColor writes a straight-alpha color (converted to premul).
func (im *Image) SetColor(x, y int, c Color) {
	if im == nil || x < 0 || y < 0 || x >= im.Width || y >= im.Height {
		return
	}
	r, g, b, a := c.Premul8()
	i := im.pixIndex(x, y)
	im.Pix[i+0] = r
	im.Pix[i+1] = g
	im.Pix[i+2] = b
	im.Pix[i+3] = a
	im.TouchRect(XYWH(float32(x), float32(y), 1, 1))
}

// Touch records that Pix was mutated in place (atlas rebake, shm rewrite).
// Call this after writing Image.Pix directly so [GPUDevice] drops the stale
// texture. [Image.Clear] and [Image.SetColor] already bump [Image.Epoch].
func (im *Image) Touch() {
	if im == nil {
		return
	}
	im.Epoch++
	im.Dirty = XYWH(0, 0, float32(im.Width), float32(im.Height))
}

// TouchRect records an in-place write to r (glyph/icon pack). [GPUDevice]
// can glTexSubImage2D just this box instead of re-uploading the atlas.
func (im *Image) TouchRect(r Rect) {
	if im == nil {
		return
	}
	im.Epoch++
	r = r.Canon().Intersect(XYWH(0, 0, float32(im.Width), float32(im.Height)))
	if im.Dirty.Empty() {
		im.Dirty = r
	} else {
		im.Dirty = im.Dirty.Union(r)
	}
}

// Bump is [Image.Touch]. uitoolkit calls this after packing a glyph into Pix.
// Prefer [Image.TouchRect] when only one cell changed (rapid scroll bake).
func (im *Image) Bump() { im.Touch() }

// BumpRect is [Image.TouchRect].
func (im *Image) BumpRect(r Rect) { im.TouchRect(r) }

// Clear fills the entire pixmap with c (premultiplied). Padding bytes
// beyond each row's pixels are left untouched (surface stride).
func (im *Image) Clear(c Color) {
	if im == nil || len(im.Pix) == 0 || im.Width <= 0 || im.Height <= 0 {
		return
	}
	r, g, b, a := c.Premul8()
	rowBytes := im.Width * 4
	stride := im.RowStride()
	pix := im.Pix
	fillRGBA(pix, 0, rowBytes, r, g, b, a)
	row := pix[0:rowBytes]
	for y := 1; y < im.Height; y++ {
		copy(pix[y*stride:y*stride+rowBytes], row)
	}
	im.Epoch++
	im.Dirty = XYWH(0, 0, float32(im.Width), float32(im.Height))
}

// ClearRect fills the pixel box covering r with c (premultiplied).
// Pixels outside r ∩ image bounds, and stride padding, are not written.
func (im *Image) ClearRect(r Rect, c Color) {
	if im == nil || len(im.Pix) == 0 || im.Width <= 0 || im.Height <= 0 {
		return
	}
	r = r.Canon()
	if r.Empty() || !r.Finite() {
		return
	}
	x0, y0, x1, y1 := clampPixelBounds(r, im.Width, im.Height)
	if x0 >= x1 || y0 >= y1 {
		return
	}
	pr, pg, pb, pa := c.Premul8()
	stride := im.RowStride()
	pix := im.Pix
	rowBytes := (x1 - x0) * 4
	i0 := y0*stride + x0*4
	fillRGBA(pix, i0, rowBytes, pr, pg, pb, pa)
	src := pix[i0 : i0+rowBytes]
	for y := y0 + 1; y < y1; y++ {
		copy(pix[y*stride+x0*4:y*stride+x0*4+rowBytes], src)
	}
	box := XYWH(float32(x0), float32(y0), float32(x1-x0), float32(y1-y0))
	if im.Dirty.Empty() {
		im.Dirty = box
	} else {
		im.Dirty = im.Dirty.Union(box)
	}
	im.Epoch++
}

// Clone returns a packed deep copy of the pixmap.
func (im *Image) Clone() *Image {
	if im == nil {
		return nil
	}
	out := NewImage(im.Width, im.Height)
	if im.RowStride() == out.RowStride() {
		copy(out.Pix, im.Pix)
		return out
	}
	row := im.Width * 4
	for y := 0; y < im.Height; y++ {
		copy(out.Pix[y*row:(y+1)*row], im.Pix[y*im.RowStride():y*im.RowStride()+row])
	}
	return out
}

// WritePNG encodes a straight-alpha PNG to w.
func (im *Image) WritePNG(w io.Writer) error {
	return png.Encode(w, im)
}

// WritePNGFile creates/truncates path and writes a PNG.
func (im *Image) WritePNGFile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return im.WritePNG(f)
}

// DecodePNG reads a PNG (any stdlib-supported color type) into a premul Image.
func DecodePNG(r io.Reader) (*Image, error) {
	src, err := png.Decode(r)
	if err != nil {
		return nil, err
	}
	return NewImageFromNRGBA(src), nil
}

// DecodePNGFile opens path and decodes a PNG.
func DecodePNGFile(path string) (*Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return DecodePNG(f)
}

// SubImage returns a copied crop of r intersected with the image bounds.
func (im *Image) SubImage(x0, y0, x1, y1 int) *Image {
	if im == nil {
		return NewImage(0, 0)
	}
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > im.Width {
		x1 = im.Width
	}
	if y1 > im.Height {
		y1 = im.Height
	}
	if x1 <= x0 || y1 <= y0 {
		return NewImage(0, 0)
	}
	out := NewImage(x1-x0, y1-y0)
	for y := y0; y < y1; y++ {
		si := im.pixIndex(x0, y)
		di := out.pixIndex(0, y-y0)
		copy(out.Pix[di:di+out.Width*4], im.Pix[si:si+out.Width*4])
	}
	return out
}

// fillRGBA writes n bytes (multiple of 4) of a solid premul pixel starting
// at pix[start], by writing one pixel and doubling. Used for large Clear
// and opaque FillRect.
func fillRGBA(pix []byte, start, n int, r, g, b, a uint8) {
	if n < 4 || start < 0 || start+n > len(pix) {
		return
	}
	pix[start+0] = r
	pix[start+1] = g
	pix[start+2] = b
	pix[start+3] = a
	filled := 4
	end := start + n
	for filled < n {
		copied := copy(pix[start+filled:end], pix[start:start+filled])
		filled += copied
	}
}

// Scroll moves pixels inside r by (dx, dy) device pixels (memmove).
// The destination is clipped to r ∩ image. Vacated pixels are left as-is;
// the caller [Image.ClearRect]s the exposed strip. Overlap is safe.
func (im *Image) Scroll(dx, dy int, r Rect) {
	if im == nil || (dx == 0 && dy == 0) {
		return
	}
	x0, y0, x1, y1 := clampPixelBounds(r.Canon(), im.Width, im.Height)
	if x0 >= x1 || y0 >= y1 {
		return
	}
	dstX0, dstY0, dstX1, dstY1 := x0, y0, x1, y1
	if dx > 0 {
		dstX0 += dx
	} else {
		dstX1 += dx
	}
	if dy > 0 {
		dstY0 += dy
	} else {
		dstY1 += dy
	}
	if dstX0 < x0 {
		dstX0 = x0
	}
	if dstY0 < y0 {
		dstY0 = y0
	}
	if dstX1 > x1 {
		dstX1 = x1
	}
	if dstY1 > y1 {
		dstY1 = y1
	}
	if dstX0 >= dstX1 || dstY0 >= dstY1 {
		return
	}
	w := dstX1 - dstX0
	h := dstY1 - dstY0
	srcX0 := dstX0 - dx
	srcY0 := dstY0 - dy
	stride := im.RowStride()
	rowBytes := w * 4
	pix := im.Pix
	if dy > 0 {
		for y := h - 1; y >= 0; y-- {
			si := (srcY0+y)*stride + srcX0*4
			di := (dstY0+y)*stride + dstX0*4
			copy(pix[di:di+rowBytes], pix[si:si+rowBytes])
		}
	} else {
		for y := 0; y < h; y++ {
			si := (srcY0+y)*stride + srcX0*4
			di := (dstY0+y)*stride + dstX0*4
			copy(pix[di:di+rowBytes], pix[si:si+rowBytes])
		}
	}
}

// CopyFrom copies srcRect from src to (destX, destY) using SRC (not src-over).
// Used to stamp a cached layer. Overlap with src==im is safe via [Image.Scroll].
func (im *Image) CopyFrom(src *Image, srcRect Rect, destX, destY int) {
	if im == nil || src == nil {
		return
	}
	if src == im {
		sx, sy, _, _ := clampPixelBounds(srcRect.Canon(), src.Width, src.Height)
		im.Scroll(destX-sx, destY-sy, srcRect)
		return
	}
	sx0, sy0, sx1, sy1 := clampPixelBounds(srcRect.Canon(), src.Width, src.Height)
	if sx0 >= sx1 || sy0 >= sy1 {
		return
	}
	if destX < 0 {
		sx0 -= destX
		destX = 0
	}
	if destY < 0 {
		sy0 -= destY
		destY = 0
	}
	w := sx1 - sx0
	h := sy1 - sy0
	if destX+w > im.Width {
		w = im.Width - destX
	}
	if destY+h > im.Height {
		h = im.Height - destY
	}
	if w <= 0 || h <= 0 {
		return
	}
	sstride, dstride := src.RowStride(), im.RowStride()
	rowBytes := w * 4
	for y := 0; y < h; y++ {
		si := (sy0+y)*sstride + sx0*4
		di := (destY+y)*dstride + destX*4
		copy(im.Pix[di:di+rowBytes], src.Pix[si:si+rowBytes])
	}
}
