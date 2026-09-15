//go:build linux && cgo

package paintengine2d

/*
#cgo linux pkg-config: egl glesv2
#include <GLES2/gl2.h>
#include <stdlib.h>

static const char *pe_blur_vs =
	"attribute vec2 a_pos;\n"
	"attribute vec2 a_uv;\n"
	"uniform vec2 u_vp;\n"
	"varying vec2 v_pos;\n"
	"varying vec2 v_uv;\n"
	"void main() {\n"
	"  v_pos = a_pos;\n"
	"  v_uv = a_uv;\n"
	"  gl_Position = vec4(a_pos.x * 2.0 / u_vp.x - 1.0, 1.0 - a_pos.y * 2.0 / u_vp.y, 0.0, 1.0);\n"
	"}\n";

// One pass of a separable Gaussian: the centre tap and up to 16 symmetric
// pairs along u_dir (texels in uv), weights normalised on the CPU. The clip
// mask, when there is one, scales the result so a rounded clip stays soft.
static const char *pe_blur_fs =
	"precision mediump float;\n"
	"varying vec2 v_pos;\n"
	"varying vec2 v_uv;\n"
	"uniform sampler2D u_src;\n"
	"uniform vec2 u_dir;\n"
	"uniform float u_w[17];\n"
	"uniform float u_off[17];\n"
	"uniform int u_taps;\n"
	"uniform int u_useMask;\n"
	"uniform sampler2D u_mask;\n"
	"uniform vec4 u_maskRect;\n"
	"void main() {\n"
	"  vec4 c = texture2D(u_src, v_uv) * u_w[0];\n"
	"  for (int i = 1; i < 17; i++) {\n"
	"    if (i >= u_taps) break;\n"
	"    vec2 o = u_dir * u_off[i];\n"
	"    c += (texture2D(u_src, v_uv + o) + texture2D(u_src, v_uv - o)) * u_w[i];\n"
	"  }\n"
	"  if (u_useMask == 1) {\n"
	"    vec2 uv = (v_pos - u_maskRect.xy) / u_maskRect.zw;\n"
	"    float m = 0.0;\n"
	"    if (uv.x >= 0.0 && uv.y >= 0.0 && uv.x < 1.0 && uv.y < 1.0)\n"
	"      m = texture2D(u_mask, uv).a;\n"
	"    c *= m;\n"
	"  }\n"
	"  gl_FragColor = c;\n"
	"}\n";

static GLuint pe_blur_compile(void) {
	GLuint vs = glCreateShader(GL_VERTEX_SHADER);
	glShaderSource(vs, 1, &pe_blur_vs, NULL);
	glCompileShader(vs);
	GLint ok = 0;
	glGetShaderiv(vs, GL_COMPILE_STATUS, &ok);
	if (!ok) { glDeleteShader(vs); return 0; }
	GLuint fs = glCreateShader(GL_FRAGMENT_SHADER);
	glShaderSource(fs, 1, &pe_blur_fs, NULL);
	glCompileShader(fs);
	glGetShaderiv(fs, GL_COMPILE_STATUS, &ok);
	if (!ok) { glDeleteShader(vs); glDeleteShader(fs); return 0; }
	GLuint p = glCreateProgram();
	glAttachShader(p, vs);
	glAttachShader(p, fs);
	glBindAttribLocation(p, 0, "a_pos");
	glBindAttribLocation(p, 1, "a_uv");
	glLinkProgram(p);
	glDeleteShader(vs);
	glDeleteShader(fs);
	glGetProgramiv(p, GL_LINK_STATUS, &ok);
	if (!ok) { glDeleteProgram(p); return 0; }
	return p;
}
*/
import "C"

import (
	"math"
	"unsafe"
)

// blurTaps is the most tap pairs one blur pass samples (plus the centre).
const blurTaps = 17

// gpuBlur holds the backdrop blur's program and scratch targets.
type gpuBlur struct {
	prog             C.GLuint
	failed           bool
	locVP, locSrc    C.GLint
	locDir, locW     C.GLint
	locOff, locTaps  C.GLint
	locUseM, locMask C.GLint
	locMaskR         C.GLint
	texA, texB, fboB C.GLuint
	tw, th           int
	weights, offsets [blurTaps]float32
}

func (d *GPUDevice) blurInit() bool {
	b := &d.blur
	if b.prog != 0 {
		return true
	}
	if b.failed {
		return false
	}
	b.prog = C.pe_blur_compile()
	if b.prog == 0 {
		b.failed = true
		return false
	}
	b.locVP = loc(b.prog, "u_vp")
	b.locSrc = loc(b.prog, "u_src")
	b.locDir = loc(b.prog, "u_dir")
	b.locW = loc(b.prog, "u_w")
	b.locOff = loc(b.prog, "u_off")
	b.locTaps = loc(b.prog, "u_taps")
	b.locUseM = loc(b.prog, "u_useMask")
	b.locMask = loc(b.prog, "u_mask")
	b.locMaskR = loc(b.prog, "u_maskRect")
	return true
}

// blurTargets grows the two scratch textures (and B's framebuffer) to hold
// a w×h region.
func (d *GPUDevice) blurTargets(w, h int) bool {
	b := &d.blur
	if b.texA != 0 && b.tw >= w && b.th >= h {
		return true
	}
	d.blurFreeTargets()
	tw, th := max(w, 64), max(h, 64)
	mk := func() C.GLuint {
		var t C.GLuint
		C.glGenTextures(1, &t)
		C.glBindTexture(C.GL_TEXTURE_2D, t)
		C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_MIN_FILTER, C.GL_LINEAR)
		C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_MAG_FILTER, C.GL_LINEAR)
		C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_WRAP_S, C.GL_CLAMP_TO_EDGE)
		C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_WRAP_T, C.GL_CLAMP_TO_EDGE)
		C.glTexImage2D(C.GL_TEXTURE_2D, 0, C.GL_RGBA, C.GLsizei(tw), C.GLsizei(th), 0, C.GL_RGBA, C.GL_UNSIGNED_BYTE, nil)
		return t
	}
	b.texA, b.texB = mk(), mk()
	C.glGenFramebuffers(1, &b.fboB)
	C.glBindFramebuffer(C.GL_FRAMEBUFFER, b.fboB)
	C.glFramebufferTexture2D(C.GL_FRAMEBUFFER, C.GL_COLOR_ATTACHMENT0, C.GL_TEXTURE_2D, b.texB, 0)
	ok := C.glCheckFramebufferStatus(C.GL_FRAMEBUFFER) == C.GL_FRAMEBUFFER_COMPLETE
	C.glBindFramebuffer(C.GL_FRAMEBUFFER, d.drawFBO())
	if !ok {
		d.blurFreeTargets()
		return false
	}
	b.tw, b.th = tw, th
	return true
}

func (d *GPUDevice) blurFreeTargets() {
	b := &d.blur
	if b.fboB != 0 {
		C.glDeleteFramebuffers(1, &b.fboB)
		b.fboB = 0
	}
	for _, t := range []*C.GLuint{&b.texA, &b.texB} {
		if *t != 0 {
			C.glDeleteTextures(1, t)
			*t = 0
		}
	}
	b.tw, b.th = 0, 0
}

// freeBlur releases the blur's GL objects (with the context current).
func (d *GPUDevice) freeBlur() {
	d.blurFreeTargets()
	if d.blur.prog != 0 {
		C.glDeleteProgram(d.blur.prog)
		d.blur.prog = 0
	}
}

// blurKernel fills the pass weights for a Gaussian of standard deviation
// sigma: up to blurTaps−1 symmetric pairs spread over three sigmas (sparse
// beyond that, the linear filter fills in), weights normalised to one.
func (b *gpuBlur) blurKernel(sigma float32) int {
	reach := float64(sigma) * 3
	step := math.Max(1, reach/float64(blurTaps-1))
	taps := int(math.Ceil(reach/step)) + 1
	if taps > blurTaps {
		taps = blurTaps
	}
	sum := 0.0
	for i := 0; i < taps; i++ {
		o := float64(i) * step
		w := math.Exp(-o * o / (2 * float64(sigma) * float64(sigma)))
		b.offsets[i], b.weights[i] = float32(o), float32(w)
		if i == 0 {
			sum += w
		} else {
			sum += 2 * w
		}
	}
	for i := 0; i < taps; i++ {
		b.weights[i] = float32(float64(b.weights[i]) / sum)
	}
	return taps
}

// BackdropBlur implements [BackdropBlurrer] on the GPU: the region is
// copied out of the frame, blurred across into a scratch target, and
// blurred down back into the frame inside r and the clip.
func (d *GPUDevice) BackdropBlur(r Rect, xform Matrix, radius float32, clip Clip) {
	if d == nil || d.closed || radius <= 0 || !xform.Finite() {
		return
	}
	if err := d.makeCurrent(); err != nil || !d.blurInit() {
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
	x0, y0, x1, y1 := clampPixelBounds(dst, d.w, d.h)
	if x0 >= x1 || y0 >= y1 {
		return
	}
	reach := blurReach(sigma)
	sx0, sy0 := max(x0-reach, 0), max(y0-reach, 0)
	sx1, sy1 := min(x1+reach, d.w), min(y1+reach, d.h)
	w, h := sx1-sx0, sy1-sy0
	if !d.blurTargets(w, h) {
		return
	}
	b := &d.blur
	taps := b.blurKernel(sigma)

	// The region, out of the (resolved) frame into A. GL rows run bottom-up.
	d.resolve()
	C.glBindFramebuffer(C.GL_FRAMEBUFFER, d.fbo)
	C.glBindTexture(C.GL_TEXTURE_2D, b.texA)
	C.glCopyTexSubImage2D(C.GL_TEXTURE_2D, 0, 0, 0, C.GLint(sx0), C.GLint(d.h-sy1), C.GLsizei(w), C.GLsizei(h))

	C.glUseProgram(b.prog)
	C.glUniform1i(b.locSrc, 0)
	C.glUniform1i(b.locTaps, C.GLint(taps))
	C.glUniform1fv(b.locW, blurTaps, (*C.GLfloat)(unsafe.Pointer(&b.weights[0])))
	C.glUniform1fv(b.locOff, blurTaps, (*C.GLfloat)(unsafe.Pointer(&b.offsets[0])))
	C.glDisable(C.GL_STENCIL_TEST)
	C.glDisable(C.GL_SCISSOR_TEST)
	C.glColorMask(C.GL_TRUE, C.GL_TRUE, C.GL_TRUE, C.GL_TRUE)
	u1, v1 := float32(w)/float32(b.tw), float32(h)/float32(b.th)

	// Across: A → B over the region, same orientation.
	C.glBindFramebuffer(C.GL_FRAMEBUFFER, b.fboB)
	C.glViewport(0, 0, C.GLsizei(b.tw), C.GLsizei(b.th))
	C.glDisable(C.GL_BLEND)
	C.glUniform2f(b.locVP, C.GLfloat(b.tw), C.GLfloat(b.th))
	C.glUniform2f(b.locDir, C.GLfloat(1/float32(b.tw)), 0)
	C.glUniform1i(b.locUseM, 0)
	C.glActiveTexture(C.GL_TEXTURE0)
	C.glBindTexture(C.GL_TEXTURE_2D, b.texA)
	fw, fh := float32(w), float32(h)
	// Target rows are drawn top-down in the quad; B keeps A's bottom-up
	// rows, so the quad's top samples A's top row (v1).
	d.quad = append(d.quad[:0],
		0, float32(b.th)-fh, 0, v1,
		fw, float32(b.th)-fh, u1, v1,
		fw, float32(b.th), u1, 0,
		0, float32(b.th)-fh, 0, v1,
		fw, float32(b.th), u1, 0,
		0, float32(b.th), 0, 0,
	)
	d.drawTris(d.quad)

	// Down: B → the frame, inside r and the clip.
	C.glBindFramebuffer(C.GL_FRAMEBUFFER, d.drawFBO())
	C.glViewport(0, 0, C.GLsizei(d.w), C.GLsizei(d.h))
	C.glEnable(C.GL_SCISSOR_TEST)
	C.glScissor(C.GLint(x0), C.GLint(d.h-y1), C.GLsizei(x1-x0), C.GLsizei(y1-y0))
	C.glUniform2f(b.locVP, C.GLfloat(d.w), C.GLfloat(d.h))
	C.glUniform2f(b.locDir, 0, C.GLfloat(1/float32(b.th)))
	if clip.Mask != nil && clip.MaskW > 0 && clip.MaskH > 0 {
		// The mask weights the blur in: a rounded clip stays soft.
		d.uploadMask(clip)
		C.glUseProgram(b.prog)
		C.glUniform1i(b.locUseM, 1)
		C.glUniform4f(b.locMaskR, C.GLfloat(clip.MaskX), C.GLfloat(clip.MaskY), C.GLfloat(clip.MaskW), C.GLfloat(clip.MaskH))
		C.glActiveTexture(C.GL_TEXTURE2)
		C.glBindTexture(C.GL_TEXTURE_2D, d.maskTex)
		C.glUniform1i(b.locMask, 2)
		C.glEnable(C.GL_BLEND)
	} else {
		C.glUniform1i(b.locUseM, 0)
		C.glDisable(C.GL_BLEND)
	}
	C.glActiveTexture(C.GL_TEXTURE0)
	C.glBindTexture(C.GL_TEXTURE_2D, b.texB)
	// Device (x, y) is region texel (x−sx0, y−sy0), whose row in B counts
	// from the bottom of the region.
	tu := func(x int) float32 { return float32(x-sx0) / float32(b.tw) }
	tv := func(y int) float32 { return float32(h-(y-sy0)) / float32(b.th) }
	X0, Y0, X1, Y1 := float32(x0), float32(y0), float32(x1), float32(y1)
	d.quad = append(d.quad[:0],
		X0, Y0, tu(x0), tv(y0),
		X1, Y0, tu(x1), tv(y0),
		X1, Y1, tu(x1), tv(y1),
		X0, Y0, tu(x0), tv(y0),
		X1, Y1, tu(x1), tv(y1),
		X0, Y1, tu(x0), tv(y1),
	)
	d.drawTris(d.quad)

	C.glEnable(C.GL_BLEND)
	C.glBlendFunc(C.GL_ONE, C.GL_ONE_MINUS_SRC_ALPHA)
	C.glEnable(C.GL_STENCIL_TEST)
	C.glDisable(C.GL_SCISSOR_TEST)
	C.glUseProgram(d.prog)
	d.markDrawn()
}

var _ BackdropBlurrer = (*GPUDevice)(nil)
