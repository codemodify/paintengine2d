//go:build linux && cgo

package paintengine2d

/*
#cgo pkg-config: egl glesv2
#include <EGL/egl.h>
#include <GLES2/gl2.h>
#include <GLES2/gl2ext.h>

// pe_make_target makes a texture of w x h and a framebuffer whose colour
// attachment is that texture, for a caller to render into. It reports 0 and
// leaves nothing behind when the framebuffer is not complete, which is the
// one failure a driver will actually give for a size it dislikes.
static int pe_make_target(int w, int h, GLuint* tex, GLuint* fbo) {
	GLint oldFBO = 0;
	glGetIntegerv(GL_FRAMEBUFFER_BINDING, &oldFBO);
	GLuint t = 0, f = 0;
	glGenTextures(1, &t);
	glBindTexture(GL_TEXTURE_2D, t);
	glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MIN_FILTER, GL_LINEAR);
	glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MAG_FILTER, GL_LINEAR);
	glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_S, GL_CLAMP_TO_EDGE);
	glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_T, GL_CLAMP_TO_EDGE);
	glTexImage2D(GL_TEXTURE_2D, 0, GL_RGBA, w, h, 0, GL_RGBA, GL_UNSIGNED_BYTE, NULL);
	glGenFramebuffers(1, &f);
	glBindFramebuffer(GL_FRAMEBUFFER, f);
	glFramebufferTexture2D(GL_FRAMEBUFFER, GL_COLOR_ATTACHMENT0, GL_TEXTURE_2D, t, 0);
	GLenum st = glCheckFramebufferStatus(GL_FRAMEBUFFER);
	// The target is left as it was: the caller is mid-frame on its own FBO.
	glBindFramebuffer(GL_FRAMEBUFFER, (GLuint)oldFBO);
	if (st != GL_FRAMEBUFFER_COMPLETE) {
		glDeleteFramebuffers(1, &f);
		glDeleteTextures(1, &t);
		return 0;
	}
	*tex = t;
	*fbo = f;
	return 1;
}

static void pe_free_target(GLuint tex, GLuint fbo) {
	if (fbo) glDeleteFramebuffers(1, &fbo);
	if (tex) glDeleteTextures(1, &tex);
}

// pe_clear_target fills a target with one colour. The caller's own
// framebuffer binding is put back, because it is mid-frame on its own.
static void pe_clear_target(GLuint fbo, int w, int h, float r, float g, float b, float a) {
	GLint oldFBO = 0;
	glGetIntegerv(GL_FRAMEBUFFER_BINDING, &oldFBO);
	glBindFramebuffer(GL_FRAMEBUFFER, fbo);
	glDisable(GL_SCISSOR_TEST);
	glViewport(0, 0, w, h);
	glClearColor(r, g, b, a);
	glClear(GL_COLOR_BUFFER_BIT);
	glBindFramebuffer(GL_FRAMEBUFFER, (GLuint)oldFBO);
}
*/
import "C"

import "fmt"

// NewForeignTarget makes a texture and a framebuffer on this device's GL
// context, of w x h pixels, for the caller to render into.
//
// The caller renders with the device's context current on the device's thread
// ([GPUDevice.MakeCurrent]), binding [ForeignTexture.FBO] — which is what
// libmpv's MPV_RENDER_PARAM_OPENGL_FBO takes — and then draws it with
// [Context.DrawForeign] like any other picture. Nothing is copied, and the
// blending, the transform, the clip and the tint are the engine's own.
//
// The device owns the names and deletes them when it is closed or when
// [GPUDevice.ReleaseForeign] is called. The caller must not use them
// afterwards; [ForeignTexture.Valid] says so.
//
// It is cleared, because a fresh texture holds whatever the driver had.
func (d *GPUDevice) NewForeignTarget(w, h int) (*ForeignTexture, error) {
	if d == nil || d.closed {
		return nil, ErrGPUUnavailable
	}
	if w < 1 || h < 1 {
		return nil, fmt.Errorf("paintengine2d: foreign target %dx%d", w, h)
	}
	if err := d.makeCurrent(); err != nil {
		return nil, err
	}
	var tex, fbo C.GLuint
	if C.pe_make_target(C.int(w), C.int(h), &tex, &fbo) == 0 {
		return nil, fmt.Errorf("paintengine2d: the driver would not give a %dx%d render target", w, h)
	}
	C.pe_clear_target(fbo, C.int(w), C.int(h), 0, 0, 0, 0)
	t := &ForeignTexture{
		Texture: uint32(tex), FBO: uint32(fbo),
		Width: w, Height: h, mine: true,
	}
	d.foreign = append(d.foreign, t)
	return t, nil
}

// AdoptTexture wraps a texture the caller already created on this device's
// context, so it can be blitted without the engine owning it.
//
// Nothing is allocated and nothing is deleted: the caller made the texture and
// the caller deletes it. It is for a library that hands out a texture name
// rather than taking a framebuffer. The engine still marks it dead when the
// device closes, because the name means nothing on another context.
func (d *GPUDevice) AdoptTexture(texture uint32, w, h int) (*ForeignTexture, error) {
	if d == nil || d.closed {
		return nil, ErrGPUUnavailable
	}
	if texture == 0 || w < 1 || h < 1 {
		return nil, fmt.Errorf("paintengine2d: adopt texture %d of %dx%d", texture, w, h)
	}
	t := &ForeignTexture{Texture: texture, Width: w, Height: h}
	d.foreign = append(d.foreign, t)
	return t, nil
}

// ClearForeign fills a target with one colour, on the device's context.
//
// For the caller that has to blank a frame — between one video and the next,
// or while a decoder has nothing ready — without building a shader of its own
// to do it. A target made by [GPUDevice.NewForeignTarget] is cleared already;
// this is for the times after the first.
//
// It is refused for an adopted texture, which has no framebuffer of ours to
// render through, and for one whose device has gone.
func (d *GPUDevice) ClearForeign(t *ForeignTexture, c Color) bool {
	if d == nil || d.closed || !t.Valid() || t.FBO == 0 {
		return false
	}
	if err := d.makeCurrent(); err != nil {
		return false
	}
	C.pe_clear_target(C.GLuint(t.FBO), C.int(t.Width), C.int(t.Height),
		C.float(c.R), C.float(c.G), C.float(c.B), C.float(c.A))
	return true
}

// ReleaseForeign gives up a target: the names this engine made are deleted,
// and t is no longer valid. A texture that was only adopted is left alone.
func (d *GPUDevice) ReleaseForeign(t *ForeignTexture) {
	if d == nil || t == nil {
		return
	}
	for i, o := range d.foreign {
		if o == t {
			d.foreign = append(d.foreign[:i], d.foreign[i+1:]...)
			break
		}
	}
	if t.mine && !t.dead && !d.closed {
		if err := d.makeCurrent(); err == nil {
			C.pe_free_target(C.GLuint(t.Texture), C.GLuint(t.FBO))
		}
	}
	t.dead = true
	t.Texture, t.FBO = 0, 0
}

// dropForeign marks every target dead as the device goes. The GL names die
// with the context, so a blit afterwards would sample whatever got the name
// next — and a caller that is told nothing would go on drawing it.
func (d *GPUDevice) dropForeign() {
	for _, t := range d.foreign {
		if t.mine {
			C.pe_free_target(C.GLuint(t.Texture), C.GLuint(t.FBO))
		}
		t.dead = true
		t.Texture, t.FBO = 0, 0
	}
	d.foreign = nil
}

// BlitForeign draws t into dstRect, taking the same transform, clip, tint and
// filter as [GPUDevice.Blit] — it is the same textured quad, so a video frame
// composites with the interface around it rather than beside it.
//
// A texture whose device has gone is refused rather than sampled.
func (d *GPUDevice) BlitForeign(t *ForeignTexture, srcRect, dstRect Rect, xform Matrix, paint Paint, clip Clip) {
	if d == nil || d.closed || !t.Valid() || dstRect.Empty() || !xform.Finite() {
		return
	}
	if err := d.makeCurrent(); err != nil {
		return
	}
	srcRect, dstRect = srcRect.Canon(), dstRect.Canon()
	if srcRect.Empty() {
		srcRect = XYWH(0, 0, float32(t.Width), float32(t.Height))
	}
	dev := xform.TransformRect(dstRect)
	if !d.applyClip(clip, dev) {
		return
	}
	tr, tg, tb, ta := blitTint(paint)
	if ta == 0 {
		C.glDisable(C.GL_SCISSOR_TEST)
		return
	}
	d.drawTexQuad(C.GLuint(t.Texture), t.Width, t.Height, false, t.Flipped,
		srcRect, dstRect, xform, paint, clip, tr, tg, tb, ta)
}
