package paintengine2d

import (
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
)

// Image is an owned pixmap in premultiplied 8-bit sRGB RGBA.
//
// Layout: row-major, 4 bytes per pixel (R, G, B, A), stride = Width * 4.
// Channels are premultiplied: a fully transparent pixel is 0,0,0,0; a 50%
// red pixel is approximately 128,0,0,128. This is the format the CPU
// backend blends into.
//
// Image implements [image.Image] using straight-alpha [color.NRGBA] so
// standard encoders (PNG) see conventional colors.
type Image struct {
	Width  int
	Height int
	Pix    []byte
}

// NewImage allocates a transparent w×h pixmap.
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
		Pix:    make([]byte, w*h*4),
	}
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

func (im *Image) pixIndex(x, y int) int { return (y*im.Width + x) * 4 }

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
}

// Clear fills the entire pixmap with c (premultiplied).
func (im *Image) Clear(c Color) {
	if im == nil || len(im.Pix) == 0 {
		return
	}
	r, g, b, a := c.Premul8()
	pix := im.Pix
	for i := 0; i < len(pix); i += 4 {
		pix[i+0] = r
		pix[i+1] = g
		pix[i+2] = b
		pix[i+3] = a
	}
}

// Clone returns a deep copy of the pixmap.
func (im *Image) Clone() *Image {
	if im == nil {
		return nil
	}
	out := NewImage(im.Width, im.Height)
	copy(out.Pix, im.Pix)
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
