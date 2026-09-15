package paintengine2d

import (
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"sync/atomic"
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
//
// An image may instead be a [FormatA8] coverage mask, one byte per pixel,
// made by [NewImageA8]: a pixel of coverage a reads as premultiplied white
// (a, a, a, a) wherever the image is sampled or read, so a glyph or icon
// mask tinted by [Paint.Color] draws as its white RGBA twin would, in a
// quarter of the memory.
type Image struct {
	Width  int
	Height int
	// Stride is bytes per row. Zero means packed (Width × bytes per pixel).
	Stride int
	Pix    []byte
	// Format is how Pix stores a pixel; the zero value is [FormatRGBA8].
	Format PixelFormat
	// Epoch increments when pixels change ([Image.Clear], [Image.SetColor],
	// [Image.Bump] / [Image.Touch] / [Image.TouchRect], [Image.Scroll],
	// [Image.CopyFrom], [Image.ClearRect]). [GPUDevice] keys its texture
	// cache on this so a reused glyph/icon atlas is re-uploaded after an
	// in-place bake. [Image.Dirty] is the union of in-place writes since
	// the last GPU upload (empty means “whole image”).
	Epoch uint64
	Dirty Rect
	// ID is a process-unique identity assigned by [NewImage] / [WrapImage].
	// [GPUDevice] keys its texture cache on it: a raw pointer key could be
	// recycled by the allocator and serve another image's texture. Images
	// built as struct literals get one on the first [Image.UID] call.
	// Copying an Image value copies its ID — use [Image.Clone].
	ID uint64
}

// PixelFormat is how an [Image] stores its pixels.
type PixelFormat uint8

const (
	// FormatRGBA8 is premultiplied 8-bit RGBA, 4 bytes per pixel.
	FormatRGBA8 PixelFormat = iota
	// FormatA8 is 8-bit coverage, 1 byte per pixel, read as premultiplied
	// white: a glyph or icon mask that is tinted when drawn.
	FormatA8
)

// BytesPerPixel is 4 for [FormatRGBA8] and 1 for [FormatA8].
func (im *Image) BytesPerPixel() int {
	if im != nil && im.Format == FormatA8 {
		return 1
	}
	return 4
}

// imageIDs hands out [Image.ID] values. Zero is reserved for "unassigned".
var imageIDs atomic.Uint64

func nextImageID() uint64 { return imageIDs.Add(1) }

// UID returns the image's process-unique identity, assigning one if the
// Image was built as a struct literal. Safe for concurrent use.
func (im *Image) UID() uint64 {
	if im == nil {
		return 0
	}
	if id := atomic.LoadUint64(&im.ID); id != 0 {
		return id
	}
	id := nextImageID()
	if atomic.CompareAndSwapUint64(&im.ID, 0, id) {
		return id
	}
	return atomic.LoadUint64(&im.ID)
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
		ID:     nextImageID(),
	}
}

// NewImageA8 allocates a transparent packed w×h coverage mask
// ([FormatA8], stride = width): glyph sheets and icon masks, which are
// tinted when drawn, in a quarter of an RGBA image's memory.
func NewImageA8(w, h int) *Image {
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	return &Image{
		Width:  w,
		Height: h,
		Stride: w,
		Pix:    make([]byte, w*h),
		Format: FormatA8,
		ID:     nextImageID(),
	}
}

// newImageLike allocates a transparent packed w×h image in im's format.
func (im *Image) newImageLike(w, h int) *Image {
	if im != nil && im.Format == FormatA8 {
		return NewImageA8(w, h)
	}
	return NewImage(w, h)
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
	return &Image{Width: w, Height: h, Stride: stride, Pix: pix, ID: nextImageID()}
}

// RowStride returns the byte stride (never 0 for a non-empty image).
func (im *Image) RowStride() int {
	if im == nil {
		return 0
	}
	if im.Stride > 0 {
		return im.Stride
	}
	return im.Width * im.BytesPerPixel()
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

func (im *Image) pixIndex(x, y int) int { return y*im.RowStride() + x*im.BytesPerPixel() }

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
	return FromPremul8(im.PremulAt(x, y)).NRGBA()
}

// PremulAt returns the raw premultiplied 8-bit pixel ([FormatA8]: white
// at the pixel's coverage).
func (im *Image) PremulAt(x, y int) (r, g, b, a uint8) {
	if im == nil || x < 0 || y < 0 || x >= im.Width || y >= im.Height {
		return
	}
	i := im.pixIndex(x, y)
	if im.Format == FormatA8 {
		a = im.Pix[i]
		return a, a, a, a
	}
	return im.Pix[i+0], im.Pix[i+1], im.Pix[i+2], im.Pix[i+3]
}

// SetColor writes a straight-alpha color (converted to premul; a
// [FormatA8] mask keeps its alpha).
func (im *Image) SetColor(x, y int, c Color) {
	if im == nil || x < 0 || y < 0 || x >= im.Width || y >= im.Height {
		return
	}
	r, g, b, a := c.Premul8()
	i := im.pixIndex(x, y)
	if im.Format == FormatA8 {
		im.Pix[i] = a
	} else {
		im.Pix[i+0] = r
		im.Pix[i+1] = g
		im.Pix[i+2] = b
		im.Pix[i+3] = a
	}
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
	rowBytes := im.Width * im.BytesPerPixel()
	stride := im.RowStride()
	pix := im.Pix
	if im.Format == FormatA8 {
		fillBytes(pix[0:rowBytes], a)
	} else {
		fillRGBA(pix, 0, rowBytes, r, g, b, a)
	}
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
	bpp := im.BytesPerPixel()
	rowBytes := (x1 - x0) * bpp
	i0 := y0*stride + x0*bpp
	if im.Format == FormatA8 {
		fillBytes(pix[i0:i0+rowBytes], pa)
	} else {
		fillRGBA(pix, i0, rowBytes, pr, pg, pb, pa)
	}
	src := pix[i0 : i0+rowBytes]
	for y := y0 + 1; y < y1; y++ {
		copy(pix[y*stride+x0*bpp:y*stride+x0*bpp+rowBytes], src)
	}
	box := XYWH(float32(x0), float32(y0), float32(x1-x0), float32(y1-y0))
	if im.Dirty.Empty() {
		im.Dirty = box
	} else {
		im.Dirty = im.Dirty.Union(box)
	}
	im.Epoch++
}

// Clone returns a packed deep copy of the pixmap, in its format.
func (im *Image) Clone() *Image {
	if im == nil {
		return nil
	}
	out := im.newImageLike(im.Width, im.Height)
	if im.RowStride() == out.RowStride() {
		copy(out.Pix, im.Pix)
		return out
	}
	row := im.Width * im.BytesPerPixel()
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
		return im.newImageLike(0, 0)
	}
	out := im.newImageLike(x1-x0, y1-y0)
	n := out.Width * out.BytesPerPixel()
	for y := y0; y < y1; y++ {
		si := im.pixIndex(x0, y)
		di := out.pixIndex(0, y-y0)
		copy(out.Pix[di:di+n], im.Pix[si:si+n])
	}
	return out
}

// fillBytes sets every byte of b to v, by doubling.
func fillBytes(b []byte, v byte) {
	if len(b) == 0 {
		return
	}
	b[0] = v
	for filled := 1; filled < len(b); filled *= 2 {
		copy(b[filled:], b[:filled])
	}
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
	bpp := im.BytesPerPixel()
	rowBytes := w * bpp
	pix := im.Pix
	if dy > 0 {
		for y := h - 1; y >= 0; y-- {
			si := (srcY0+y)*stride + srcX0*bpp
			di := (dstY0+y)*stride + dstX0*bpp
			copy(pix[di:di+rowBytes], pix[si:si+rowBytes])
		}
	} else {
		for y := 0; y < h; y++ {
			si := (srcY0+y)*stride + srcX0*bpp
			di := (dstY0+y)*stride + dstX0*bpp
			copy(pix[di:di+rowBytes], pix[si:si+rowBytes])
		}
	}
	// Pixels moved in place: a GPU texture cached for this image is stale.
	im.TouchRect(XYWH(float32(dstX0), float32(dstY0), float32(w), float32(h)))
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
	if src.Format != im.Format {
		// Between formats: a mask copies in as white at its coverage, an
		// RGBA image copies into a mask as its alpha.
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				r, g, b, a := src.PremulAt(sx0+x, sy0+y)
				i := im.pixIndex(destX+x, destY+y)
				if im.Format == FormatA8 {
					im.Pix[i] = a
				} else {
					im.Pix[i+0], im.Pix[i+1], im.Pix[i+2], im.Pix[i+3] = r, g, b, a
				}
			}
		}
		im.TouchRect(XYWH(float32(destX), float32(destY), float32(w), float32(h)))
		return
	}
	sstride, dstride := src.RowStride(), im.RowStride()
	bpp := im.BytesPerPixel()
	rowBytes := w * bpp
	for y := 0; y < h; y++ {
		si := (sy0+y)*sstride + sx0*bpp
		di := (destY+y)*dstride + destX*bpp
		copy(im.Pix[di:di+rowBytes], src.Pix[si:si+rowBytes])
	}
	im.TouchRect(XYWH(float32(destX), float32(destY), float32(w), float32(h)))
}
