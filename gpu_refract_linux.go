//go:build linux && cgo

package paintengine2d

/*
#cgo linux pkg-config: egl glesv2
#include <GLES2/gl2.h>
#include <stdlib.h>

static const char *pe_refr_vs =
	"attribute vec2 a_pos;\n"
	"attribute vec2 a_uv;\n"
	"uniform vec2 u_vp;\n"
	"varying vec2 v_pos;\n"
	"void main() {\n"
	"  v_pos = a_pos;\n"
	"  gl_Position = vec4(a_pos.x * 2.0 / u_vp.x - 1.0, 1.0 - a_pos.y * 2.0 / u_vp.y, 0.0, 1.0);\n"
	"}\n";

// The edge of a pane of glass: inside the rounded rect, each fragment is
// taken from where that edge would have bent it in from. The middle is
// discarded rather than written back unchanged, so the flat part of the
// pane costs nothing and keeps the pixels the blur left.
//
// Distances are in device pixels, top-down, which is what the vertex
// shader hands over in v_pos; the source texture holds the region
// bottom-up, as glCopyTexSubImage2D leaves it, so only the v of the
// sample is flipped.
static const char *pe_refr_fs =
	"precision highp float;\n"
	"varying vec2 v_pos;\n"
	"uniform sampler2D u_src;\n"
	"uniform vec4 u_rect;\n"    // x0, y0, x1, y1
	"uniform float u_corner;\n"
	"uniform float u_depth;\n"
	"uniform vec4 u_reg;\n"     // sx0, sy0, w, h
	"uniform vec2 u_texsz;\n"   // the scratch texture's size
	"void main() {\n"
	"  vec2 c  = (u_rect.xy + u_rect.zw) * 0.5;\n"
	"  vec2 hs = (u_rect.zw - u_rect.xy) * 0.5;\n"
	"  float r = min(u_corner, min(hs.x, hs.y));\n"
	"  vec2 p = v_pos - c;\n"
	"  vec2 q = abs(p) - hs + vec2(r);\n"
	"  float sdf = length(max(q, 0.0)) + min(max(q.x, q.y), 0.0) - r;\n"
	"  float din = -sdf;\n"
	"  if (din <= 0.0) discard;\n"
	"  float t = 1.0 - clamp(din / u_depth, 0.0, 1.0);\n"
	"  if (t <= 0.0) discard;\n"
	"  float off = u_depth * (1.0 - sqrt(max(1.0 - t * t, 0.0)));\n"
	"  vec2 m = max(q, 0.0);\n"
	"  vec2 n;\n"
	"  if (m.x + m.y > 0.0) {\n"
	"    n = normalize(m) * sign(p);\n"
	"  } else if (q.x > q.y) {\n"
	"    n = vec2(sign(p.x), 0.0);\n"
	"  } else {\n"
	"    n = vec2(0.0, sign(p.y));\n"
	"  }\n"
	"  vec2 sp = v_pos + n * off;\n"
	"  vec2 uv = vec2((sp.x - u_reg.x) / u_texsz.x,\n"
	"                 (u_reg.w - (sp.y - u_reg.y)) / u_texsz.y);\n"
	"  gl_FragColor = texture2D(u_src, uv);\n"
	"}\n";

static GLuint pe_refr_compile(void) {
	GLuint vs = glCreateShader(GL_VERTEX_SHADER);
	glShaderSource(vs, 1, &pe_refr_vs, NULL);
	glCompileShader(vs);
	GLint ok = 0;
	glGetShaderiv(vs, GL_COMPILE_STATUS, &ok);
	if (!ok) { glDeleteShader(vs); return 0; }
	GLuint fs = glCreateShader(GL_FRAGMENT_SHADER);
	glShaderSource(fs, 1, &pe_refr_fs, NULL);
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

import "math"

// gpuRefract holds the backdrop refraction's program. It borrows the
// blur's scratch textures: the two never run at once, and a second pair
// of full-region textures for an effect that is usually asked for
// straight after a blur of the same region would be the same memory
// twice.
type gpuRefract struct {
	prog               C.GLuint
	failed             bool
	locVP, locSrc      C.GLint
	locRect, locCorner C.GLint
	locDepth, locReg   C.GLint
	locTexSz           C.GLint
}

func (d *GPUDevice) refractInit() bool {
	f := &d.refract
	if f.prog != 0 {
		return true
	}
	if f.failed {
		return false
	}
	f.prog = C.pe_refr_compile()
	if f.prog == 0 {
		f.failed = true
		return false
	}
	f.locVP = loc(f.prog, "u_vp")
	f.locSrc = loc(f.prog, "u_src")
	f.locRect = loc(f.prog, "u_rect")
	f.locCorner = loc(f.prog, "u_corner")
	f.locDepth = loc(f.prog, "u_depth")
	f.locReg = loc(f.prog, "u_reg")
	f.locTexSz = loc(f.prog, "u_texsz")
	return true
}

// BackdropRefract implements [BackdropRefractor] on the GPU: the region
// goes into the blur's scratch texture and one pass writes it back bent,
// discarding everything the pane's flat middle would have left alone.
func (d *GPUDevice) BackdropRefract(r Rect, corner float32, xform Matrix, depth float32, clip Clip) {
	if d == nil || d.closed || depth <= 0 || !xform.Finite() {
		return
	}
	if err := d.makeCurrent(); err != nil || !d.refractInit() {
		return
	}
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
	x0, y0, x1, y1 := clampPixelBounds(dst, d.w, d.h)
	if x0 >= x1 || y0 >= y1 {
		return
	}
	// The samples reach a bevel outside the pane, so the copy must too.
	reach := int(math.Ceil(float64(devDepth))) + 1
	sx0, sy0 := max(x0-reach, 0), max(y0-reach, 0)
	sx1, sy1 := min(x1+reach, d.w), min(y1+reach, d.h)
	w, h := sx1-sx0, sy1-sy0
	if w < 1 || h < 1 || !d.blurTargets(w, h) {
		return
	}
	b := &d.blur
	f := &d.refract

	// The region, out of the (resolved) frame into A. GL rows run
	// bottom-up, which the shader's v accounts for.
	d.resolve()
	C.glBindFramebuffer(C.GL_FRAMEBUFFER, d.fbo)
	C.glBindTexture(C.GL_TEXTURE_2D, b.texA)
	C.glCopyTexSubImage2D(C.GL_TEXTURE_2D, 0, 0, 0, C.GLint(sx0), C.GLint(d.h-sy1), C.GLsizei(w), C.GLsizei(h))

	C.glUseProgram(f.prog)
	C.glUniform1i(f.locSrc, 0)
	C.glUniform2f(f.locVP, C.GLfloat(d.w), C.GLfloat(d.h))
	C.glUniform4f(f.locRect, C.GLfloat(dev.Min.X), C.GLfloat(dev.Min.Y), C.GLfloat(dev.Max.X), C.GLfloat(dev.Max.Y))
	C.glUniform1f(f.locCorner, C.GLfloat(devCorner))
	C.glUniform1f(f.locDepth, C.GLfloat(devDepth))
	C.glUniform4f(f.locReg, C.GLfloat(sx0), C.GLfloat(sy0), C.GLfloat(w), C.GLfloat(h))
	C.glUniform2f(f.locTexSz, C.GLfloat(b.tw), C.GLfloat(b.th))

	C.glBindFramebuffer(C.GL_FRAMEBUFFER, d.drawFBO())
	C.glViewport(0, 0, C.GLsizei(d.w), C.GLsizei(d.h))
	C.glDisable(C.GL_STENCIL_TEST)
	C.glEnable(C.GL_SCISSOR_TEST)
	C.glScissor(C.GLint(x0), C.GLint(d.h-y1), C.GLsizei(x1-x0), C.GLsizei(y1-y0))
	C.glColorMask(C.GL_TRUE, C.GL_TRUE, C.GL_TRUE, C.GL_TRUE)
	// The pass replaces what it keeps and discards the rest, so blending
	// it would mix the bent pixels with the ones they came from.
	C.glDisable(C.GL_BLEND)
	C.glActiveTexture(C.GL_TEXTURE0)
	C.glBindTexture(C.GL_TEXTURE_2D, b.texA)

	X0, Y0, X1, Y1 := float32(x0), float32(y0), float32(x1), float32(y1)
	d.quad = append(d.quad[:0],
		X0, Y0, 0, 0,
		X1, Y0, 0, 0,
		X1, Y1, 0, 0,
		X0, Y0, 0, 0,
		X1, Y1, 0, 0,
		X0, Y1, 0, 0,
	)
	d.drawTris(d.quad)

	C.glEnable(C.GL_BLEND)
	C.glBlendFunc(C.GL_ONE, C.GL_ONE_MINUS_SRC_ALPHA)
	C.glEnable(C.GL_STENCIL_TEST)
	C.glDisable(C.GL_SCISSOR_TEST)
	C.glUseProgram(d.prog)
	d.markDrawn()
}

var _ BackdropRefractor = (*GPUDevice)(nil)
