package paintengine2d

import "errors"

// A picture this engine did not draw.
//
// [Device.Blit] takes an [Image], which is CPU memory by definition — a
// []byte of pixels. For a video frame that is three copies and a full texture
// upload every frame: a decoder rasterizes into its own buffer, the
// application copies that into Go memory, the engine copies it into a texture.
// At 1280x720 RGBA and 60 frames a second that is about 210 MB/s of upload,
// and it is why an application that plays video caps its render size.
//
// A [ForeignTexture] is the other way round: the caller renders *into* a GPU
// texture this engine owns the blending of, and the engine draws it with the
// same transform, clip and tint as anything else. Nothing is copied.

// ErrForeignUnsupported is returned by [NewForeignTarget] for a device that
// cannot take a foreign texture — the CPU backend, a recorder, or a window
// whose GPU never started. Ask [SupportsForeign] first.
var ErrForeignUnsupported = errors.New("paintengine2d: this device cannot draw a foreign texture")

// ForeignTexture is a GPU texture the caller draws into itself — a video
// frame from a decoder, a scene from another renderer — that a [Device] can
// blit like an [Image].
//
// Get one from [GPUDevice.NewForeignTarget], which creates the texture and a
// framebuffer bound to it, or from [GPUDevice.AdoptTexture] to wrap a texture
// the caller already has. Both belong to the device's GL context, so:
//
//   - **render into it on the device's thread, with the device's context
//     current.** [GPUDevice.MakeCurrent] does that, and the device refuses
//     calls from another OS thread anyway. Commands issued on one context are
//     ordered, so a blit after the caller's draws sees them — no fence, and
//     none is offered, because a fence is only needed for a *second* context.
//   - **it does not outlive the device.** A window's paint device is closed
//     and made again when the window changes visual, when a resize fails, and
//     when the GPU is lost and painting falls back to the CPU. A toolkit
//     should say when that happens; the texture names are meaningless
//     afterwards and blitting one is refused.
//
// The zero value is not usable.
type ForeignTexture struct {
	// Texture is the GL texture name (glGenTextures).
	Texture uint32
	// FBO is a framebuffer whose colour attachment is Texture, for the caller
	// to render into — what libmpv's MPV_RENDER_PARAM_OPENGL_FBO wants. Zero
	// when the caller brought only a texture ([GPUDevice.AdoptTexture]).
	FBO uint32
	// Width and Height are the texture's size in pixels.
	Width, Height int
	// Flipped says the texture's rows run bottom-up, which is GL's own
	// convention and what a renderer that was written for a window gives.
	// The blit flips it back; a caller that would rather flip at the source
	// (libmpv's MPV_RENDER_PARAM_FLIP_Y) leaves this false.
	Flipped bool
	// mine records that this engine created the names and will delete them.
	mine bool
	// dead is set when the device that owns the names has gone, so a blit is
	// refused rather than sampling a recycled texture name.
	dead bool
}

// Valid reports whether t can still be blitted: it names a texture, and the
// device that owns it has not been closed.
func (t *ForeignTexture) Valid() bool {
	return t != nil && !t.dead && t.Texture != 0 && t.Width > 0 && t.Height > 0
}

// foreignBlitter is the optional [Device] method. It is not on the Device
// interface — a backend that cannot do it stays source-compatible, which is
// the same rule ClearRect, Present and Scroll follow.
type foreignBlitter interface {
	BlitForeign(t *ForeignTexture, srcRect, dstRect Rect, xform Matrix, paint Paint, clip Clip)
}

// foreignTargeter is the other half: making the texture.
//
// Drawing one and allocating one are different capabilities and used to
// be one interface, so a device had to do both or neither. That left a
// [Recorder] out — and a recorder is exactly what a retained painting
// path hands a widget, so the feature was unreachable from the place it
// was built for.
type foreignTargeter interface {
	NewForeignTarget(w, h int) (*ForeignTexture, error)
}

// SupportsForeign reports whether dev can draw a [ForeignTexture].
//
// Ask *before* setting a decoder up, not per frame: a renderer has to be
// built one way or the other at the start, and the answer is a property of
// the window's paint device. It is false for the CPU backend and for a
// window whose GPU never started or was lost — so an application that
// offers a GPU path still needs its CPU one, and this is how it chooses.
//
// It is **true for a [Recorder]**, which carries the draw into the scene
// and performs it when that scene is replayed on a device that can.
// Retained painting is the default, so a widget asking this of the
// context it was handed is asking the right question.
func SupportsForeign(dev Device) bool {
	if dev == nil {
		return false
	}
	_, ok := dev.(foreignBlitter)
	return ok
}

// NewForeignTarget makes a texture and a framebuffer on dev's GL context for
// the caller to render into, or reports why it could not.
//
// It is [GPUDevice.NewForeignTarget] through the [Device] interface, so a
// caller holding whatever the toolkit gave it does not have to type-assert.
func NewForeignTarget(dev Device, w, h int) (*ForeignTexture, error) {
	fb, ok := dev.(foreignTargeter)
	if !ok {
		// A recorder can carry a foreign draw but cannot make the
		// texture: that needs the GL context a window owns.
		return nil, ErrForeignUnsupported
	}
	return fb.NewForeignTarget(w, h)
}

// DrawForeign blits t into dst (user space), with the context's transform,
// clip and global alpha, and reports whether it drew.
//
// False means this device cannot take a foreign texture at all
// ([SupportsForeign]) or t is no longer valid — in both cases the caller
// falls back to whatever it does on the CPU.
func (c *Context) DrawForeign(t *ForeignTexture, src, dst Rect) bool {
	return c.DrawForeignPaint(t, src, dst, Paint{Color: White})
}

// DrawForeignPaint is [Context.DrawForeign] with a tint: paint.Color RGB
// multiplies the texture's premultiplied samples and Color.A modulates
// coverage, exactly as for [Context.DrawImageRectPaint].
func (c *Context) DrawForeignPaint(t *ForeignTexture, src, dst Rect, paint Paint) bool {
	if c == nil || !t.Valid() {
		return false
	}
	fb, ok := c.dev.(foreignBlitter)
	if !ok {
		return false
	}
	p, ok := c.faded(paint)
	if !ok {
		// Fully transparent: nothing to draw, and the caller has nothing to
		// fall back to either.
		return true
	}
	if src.Empty() {
		src = XYWH(0, 0, float32(t.Width), float32(t.Height))
	}
	fb.BlitForeign(t, src, dst, c.cur.xform, p, c.clip())
	c.markDirtyUser(dst)
	return true
}
