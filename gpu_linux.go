//go:build linux && cgo

package paintengine2d

/*
#cgo linux pkg-config: egl glesv2
#define EGL_EGLEXT_PROTOTYPES
#include <EGL/egl.h>
#include <EGL/eglext.h>
#include <GLES2/gl2.h>
#include <GLES2/gl2ext.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

static const char *pe_vs =
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

static const char *pe_fs =
	"precision mediump float;\n"
	"varying vec2 v_pos;\n"
	"varying vec2 v_uv;\n"
	"uniform int u_mode;\n"
	"uniform vec4 u_color;\n"
	"uniform sampler2D u_tex;\n"
	"uniform sampler2D u_ramp;\n"
	"uniform sampler2D u_mask;\n"
	"uniform int u_useMask;\n"
	"uniform vec4 u_maskRect;\n"
	"uniform vec2 u_g0;\n"
	"uniform vec2 u_g1;\n"
	"uniform vec2 u_center;\n"
	"uniform float u_radius;\n"
	"uniform float u_inner;\n"
	"uniform int u_tile;\n"
	"uniform mat3 u_inv;\n"
	"uniform vec4 u_tint;\n"
	"float tileT(float t) {\n"
	"  if (u_tile == 1) {\n"
	"    t = t - floor(t);\n"
	"    if (t < 0.0) t += 1.0;\n"
	"    return t;\n"
	"  }\n"
	"  if (u_tile == 2) {\n"
	"    t = t - floor(t * 0.5) * 2.0;\n"
	"    if (t < 0.0) t += 2.0;\n"
	"    if (t > 1.0) t = 2.0 - t;\n"
	"    return t;\n"
	"  }\n"
	"  return clamp(t, 0.0, 1.0);\n"
	"}\n"
	"vec2 toUser(vec2 p) {\n"
	"  return (u_inv * vec3(p, 1.0)).xy;\n"
	"}\n"
	"void main() {\n"
	"  vec4 c = u_color;\n"
	"  if (u_mode == 1) {\n"
	"    vec2 p = toUser(v_pos);\n"
	"    vec2 d = u_g1 - u_g0;\n"
	"    float len2 = dot(d, d);\n"
	"    float t = 0.0;\n"
	"    if (len2 > 1e-12) t = dot(p - u_g0, d) / len2;\n"
	"    c = texture2D(u_ramp, vec2(tileT(t), 0.5));\n"
	"  } else if (u_mode == 2) {\n"
	"    vec2 p = toUser(v_pos);\n"
	"    float dist = length(p - u_center);\n"
	"    float span = u_radius - u_inner;\n"
	"    float t = 0.0;\n"
	"    if (span < 1e-8) t = dist <= u_radius ? 0.0 : 1.0;\n"
	"    else t = (dist - u_inner) / span;\n"
	"    c = texture2D(u_ramp, vec2(tileT(t), 0.5));\n"
	"  } else if (u_mode == 3) {\n"
	"    c = texture2D(u_tex, v_uv);\n"
	"    c.rgb *= u_tint.rgb;\n"
	"    c *= u_tint.a;\n"
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

static EGLDisplay pe_get_display(void *native, EGLenum platform) {
	if (platform != 0) {
		PFNEGLGETPLATFORMDISPLAYEXTPROC fn =
			(PFNEGLGETPLATFORMDISPLAYEXTPROC)eglGetProcAddress("eglGetPlatformDisplayEXT");
		if (fn) {
			EGLDisplay d = fn(platform, native ? native : EGL_DEFAULT_DISPLAY, NULL);
			if (d != EGL_NO_DISPLAY) return d;
		}
	}
	return eglGetDisplay(native ? (EGLNativeDisplayType)native : EGL_DEFAULT_DISPLAY);
}

static EGLSurface pe_window_surface(EGLDisplay dpy, EGLConfig cfg, uintptr_t win) {
	return eglCreateWindowSurface(dpy, cfg, (EGLNativeWindowType)win, NULL);
}

typedef EGLBoolean (*pe_swap_damage_fn)(EGLDisplay, EGLSurface, const EGLint *, EGLint);
static pe_swap_damage_fn pe_swap_damage;

static void pe_load_ext(void) {
	pe_swap_damage = (pe_swap_damage_fn)eglGetProcAddress("eglSwapBuffersWithDamageKHR");
	if (!pe_swap_damage) {
		pe_swap_damage = (pe_swap_damage_fn)eglGetProcAddress("eglSwapBuffersWithDamageEXT");
	}
}

static int pe_swap_with_damage(EGLDisplay dpy, EGLSurface surf, EGLint *rects, EGLint n) {
	if (pe_swap_damage && rects && n > 0) {
		return pe_swap_damage(dpy, surf, rects, n) == EGL_TRUE;
	}
	return eglSwapBuffers(dpy, surf) == EGL_TRUE;
}

static int pe_compile(GLuint *outProg) {
	GLuint vs = glCreateShader(GL_VERTEX_SHADER);
	glShaderSource(vs, 1, &pe_vs, NULL);
	glCompileShader(vs);
	GLint ok = 0;
	glGetShaderiv(vs, GL_COMPILE_STATUS, &ok);
	if (!ok) { glDeleteShader(vs); return 0; }
	GLuint fs = glCreateShader(GL_FRAGMENT_SHADER);
	glShaderSource(fs, 1, &pe_fs, NULL);
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
	*outProg = p;
	return 1;
}
*/
import "C"

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/codemodify/paintengine2d/internal/raster"
)

// GPUDevice is the Linux EGL / OpenGL ES 2 paint backend.
// It implements [Device] with stencil-and-cover path fill, stroke expansion
// reused from the CPU stroker, linear/radial ramps, and textured blits.
// Flattened contours and triangle fans are cached per path content + xform;
// glyph/icon atlases are re-uploaded when [Image.Epoch] changes.
type GPUDevice struct {
	w, h int

	dpy  C.EGLDisplay
	ctx  C.EGLContext
	surf C.EGLSurface
	cfg  C.EGLConfig

	fbo, color, stencil C.GLuint
	prog                C.GLuint
	vbo                 C.GLuint
	rampTex             C.GLuint
	maskTex             C.GLuint

	locVP, locMode, locColor          C.GLint
	locTex, locRamp, locMask, locUseM C.GLint
	locMaskR, locG0, locG1, locCenter C.GLint
	locRad, locInner, locTile, locInv C.GLint
	locTint                           C.GLint

	window bool
	ownEGL bool
	closed bool
	info   string

	pixels    *Image
	readDirty bool

	verbs     []raster.Verb
	pts       []raster.Vec2
	contours  [][]raster.Vec2
	closedC   []bool
	outline   [][]raster.Vec2
	pool      raster.StrokePool
	verts     []float32
	quad      []float32
	maskPix   []byte
	eglDamage []C.EGLint

	texCache map[uintptr]gpuTex
	gradRamp [256 * 4]byte
	tess     *tessCache
}

type gpuTex struct {
	id     C.GLuint
	w, h   int
	nbytes int
	epoch  uint64
}

type gpuSurface struct {
	dev *GPUDevice
}

var (
	gpuAvailOnce bool
	gpuAvailOK   bool
	gpuInfoCache string
)

func gpuAvailable() bool {
	if gpuAvailOnce {
		return gpuAvailOK
	}
	gpuAvailOnce = true
	d, err := newGPUDevice(8, 8)
	if err != nil {
		return false
	}
	gpuInfoCache = d.info
	gpuAvailOK = true
	_ = d.Close()
	return true
}

func gpuInfo() string {
	if !gpuAvailOnce {
		_ = gpuAvailable()
	}
	return gpuInfoCache
}

func newGPUSurface(w, h int) (Surface, error) {
	d, err := newGPUDevice(w, h)
	if err != nil {
		return nil, err
	}
	return &gpuSurface{dev: d}, nil
}

func (s *gpuSurface) Size() (w, h int)            { return s.dev.Size() }
func (s *gpuSurface) Device() Device              { return s.dev }
func (s *gpuSurface) Image() *Image               { return s.dev.Snapshot() }
func (s *gpuSurface) Kind() BackendKind           { return BackendGPU }
func (s *gpuSurface) Close() error                { return s.dev.Close() }
func (s *gpuSurface) Present() error              { return s.dev.Present() }
func (s *gpuSurface) PresentRects(r []Rect) error { return s.dev.PresentRects(r) }

func (s *gpuSurface) Resize(w, h int) error { return s.dev.Resize(w, h) }

func newGPUDevice(w, h int) (*GPUDevice, error) {
	return initGPU(EGLNative{Width: w, Height: h, Platform: EGLPlatformSurfaceless}, false)
}

func newGPUDeviceEGL(n EGLNative) (*GPUDevice, error) {
	if n.Width < 1 {
		n.Width = 1
	}
	if n.Height < 1 {
		n.Height = 1
	}
	return initGPU(n, true)
}

func initGPU(n EGLNative, window bool) (*GPUDevice, error) {
	runtime.LockOSThread()
	d := &GPUDevice{
		w:        n.Width,
		h:        n.Height,
		window:   window,
		ownEGL:   true,
		texCache: make(map[uintptr]gpuTex),
		quad:     make([]float32, 16),
		tess:     newTessCache(),
	}
	if d.w < 1 {
		d.w = 1
	}
	if d.h < 1 {
		d.h = 1
	}

	var native unsafe.Pointer
	if n.Display != 0 {
		native = unsafe.Pointer(n.Display)
	}
	plat := C.EGLenum(n.Platform)
	if !window && n.Platform == 0 {
		plat = C.EGLenum(EGLPlatformSurfaceless)
	}
	d.dpy = C.pe_get_display(native, plat)
	if d.dpy == C.EGLDisplay(C.EGL_NO_DISPLAY) && plat != 0 {
		d.dpy = C.pe_get_display(native, 0)
	}
	if d.dpy == C.EGLDisplay(C.EGL_NO_DISPLAY) {
		return nil, fmt.Errorf("%w: eglGetDisplay", ErrGPUUnavailable)
	}
	var maj, min C.EGLint
	if C.eglInitialize(d.dpy, &maj, &min) == C.EGL_FALSE {
		return nil, fmt.Errorf("%w: eglInitialize 0x%x", ErrGPUUnavailable, C.eglGetError())
	}
	C.pe_load_ext()
	C.eglBindAPI(C.EGL_OPENGL_ES_API)

	// Opaque window configs (ALPHA 0) avoid the transparent-window bug.
	// Do not OR pbuffer+window: Mesa surfaceless has pbuffer configs only.
	type cfgTry struct{ attr []C.EGLint }
	var tries []cfgTry
	if window {
		tries = []cfgTry{
			{[]C.EGLint{C.EGL_RENDERABLE_TYPE, C.EGL_OPENGL_ES2_BIT, C.EGL_SURFACE_TYPE, C.EGL_WINDOW_BIT, C.EGL_RED_SIZE, 8, C.EGL_GREEN_SIZE, 8, C.EGL_BLUE_SIZE, 8, C.EGL_ALPHA_SIZE, 0, C.EGL_STENCIL_SIZE, 8, C.EGL_NONE}},
			{[]C.EGLint{C.EGL_RENDERABLE_TYPE, C.EGL_OPENGL_ES2_BIT, C.EGL_SURFACE_TYPE, C.EGL_WINDOW_BIT, C.EGL_RED_SIZE, 8, C.EGL_GREEN_SIZE, 8, C.EGL_BLUE_SIZE, 8, C.EGL_STENCIL_SIZE, 8, C.EGL_NONE}},
			{[]C.EGLint{C.EGL_RENDERABLE_TYPE, C.EGL_OPENGL_ES2_BIT, C.EGL_SURFACE_TYPE, C.EGL_WINDOW_BIT, C.EGL_RED_SIZE, 8, C.EGL_GREEN_SIZE, 8, C.EGL_BLUE_SIZE, 8, C.EGL_NONE}},
		}
	} else {
		tries = []cfgTry{
			{[]C.EGLint{C.EGL_RENDERABLE_TYPE, C.EGL_OPENGL_ES2_BIT, C.EGL_SURFACE_TYPE, C.EGL_PBUFFER_BIT, C.EGL_RED_SIZE, 8, C.EGL_GREEN_SIZE, 8, C.EGL_BLUE_SIZE, 8, C.EGL_ALPHA_SIZE, 8, C.EGL_STENCIL_SIZE, 8, C.EGL_NONE}},
			{[]C.EGLint{C.EGL_RENDERABLE_TYPE, C.EGL_OPENGL_ES2_BIT, C.EGL_RED_SIZE, 8, C.EGL_GREEN_SIZE, 8, C.EGL_BLUE_SIZE, 8, C.EGL_STENCIL_SIZE, 8, C.EGL_NONE}},
			{[]C.EGLint{C.EGL_RENDERABLE_TYPE, C.EGL_OPENGL_ES2_BIT, C.EGL_RED_SIZE, 8, C.EGL_GREEN_SIZE, 8, C.EGL_BLUE_SIZE, 8, C.EGL_NONE}},
			{[]C.EGLint{C.EGL_RENDERABLE_TYPE, C.EGL_OPENGL_ES2_BIT, C.EGL_NONE}},
		}
	}
	var ncfg C.EGLint
	okCfg := false
	for _, t := range tries {
		ncfg = 0
		if C.eglChooseConfig(d.dpy, &t.attr[0], &d.cfg, 1, &ncfg) != C.EGL_FALSE && ncfg >= 1 {
			okCfg = true
			break
		}
	}
	if !okCfg {
		d.destroyEGL()
		return nil, fmt.Errorf("%w: eglChooseConfig 0x%x", ErrGPUUnavailable, C.eglGetError())
	}
	ctxAttr := []C.EGLint{C.EGL_CONTEXT_CLIENT_VERSION, 2, C.EGL_NONE}
	d.ctx = C.eglCreateContext(d.dpy, d.cfg, C.EGLContext(C.EGL_NO_CONTEXT), &ctxAttr[0])
	if d.ctx == C.EGLContext(C.EGL_NO_CONTEXT) {
		d.destroyEGL()
		return nil, fmt.Errorf("%w: eglCreateContext 0x%x", ErrGPUUnavailable, C.eglGetError())
	}

	if window && n.Window != 0 {
		d.surf = C.pe_window_surface(d.dpy, d.cfg, C.uintptr_t(n.Window))
		if d.surf == C.EGLSurface(C.EGL_NO_SURFACE) {
			d.destroyEGL()
			return nil, fmt.Errorf("%w: eglCreateWindowSurface 0x%x", ErrGPUUnavailable, C.eglGetError())
		}
		if C.eglMakeCurrent(d.dpy, d.surf, d.surf, d.ctx) == C.EGL_FALSE {
			d.destroyEGL()
			return nil, fmt.Errorf("%w: eglMakeCurrent window 0x%x", ErrGPUUnavailable, C.eglGetError())
		}
	} else {
		pb := []C.EGLint{C.EGL_WIDTH, C.EGLint(d.w), C.EGL_HEIGHT, C.EGLint(d.h), C.EGL_NONE}
		d.surf = C.eglCreatePbufferSurface(d.dpy, d.cfg, &pb[0])
		if d.surf == C.EGLSurface(C.EGL_NO_SURFACE) {
			if C.eglMakeCurrent(d.dpy, C.EGLSurface(C.EGL_NO_SURFACE), C.EGLSurface(C.EGL_NO_SURFACE), d.ctx) == C.EGL_FALSE {
				d.destroyEGL()
				return nil, fmt.Errorf("%w: eglMakeCurrent surfaceless 0x%x", ErrGPUUnavailable, C.eglGetError())
			}
		} else if C.eglMakeCurrent(d.dpy, d.surf, d.surf, d.ctx) == C.EGL_FALSE {
			d.destroyEGL()
			return nil, fmt.Errorf("%w: eglMakeCurrent pbuffer 0x%x", ErrGPUUnavailable, C.eglGetError())
		}
	}

	if rend := C.glGetString(C.GL_RENDERER); rend != nil {
		d.info = C.GoString((*C.char)(unsafe.Pointer(rend)))
	}

	if C.pe_compile(&d.prog) == 0 {
		d.destroyEGL()
		return nil, fmt.Errorf("%w: shader compile/link", ErrGPUUnavailable)
	}
	d.bindLocs()
	C.glGenBuffers(1, &d.vbo)
	C.glGenTextures(1, &d.rampTex)
	C.glGenTextures(1, &d.maskTex)
	C.glPixelStorei(C.GL_UNPACK_ALIGNMENT, 1)
	C.glPixelStorei(C.GL_PACK_ALIGNMENT, 1)
	if err := d.allocTarget(); err != nil {
		d.destroyGL()
		d.destroyEGL()
		return nil, err
	}
	C.glDisable(C.GL_DEPTH_TEST)
	C.glEnable(C.GL_BLEND)
	C.glBlendFunc(C.GL_ONE, C.GL_ONE_MINUS_SRC_ALPHA)
	C.glEnable(C.GL_STENCIL_TEST)
	d.Clear(Transparent)
	return d, nil
}

func loc(p C.GLuint, name string) C.GLint {
	cs := C.CString(name)
	defer C.free(unsafe.Pointer(cs))
	return C.glGetUniformLocation(p, cs)
}

func (d *GPUDevice) bindLocs() {
	p := d.prog
	d.locVP = loc(p, "u_vp")
	d.locMode = loc(p, "u_mode")
	d.locColor = loc(p, "u_color")
	d.locTex = loc(p, "u_tex")
	d.locRamp = loc(p, "u_ramp")
	d.locMask = loc(p, "u_mask")
	d.locUseM = loc(p, "u_useMask")
	d.locMaskR = loc(p, "u_maskRect")
	d.locG0 = loc(p, "u_g0")
	d.locG1 = loc(p, "u_g1")
	d.locCenter = loc(p, "u_center")
	d.locRad = loc(p, "u_radius")
	d.locInner = loc(p, "u_inner")
	d.locTile = loc(p, "u_tile")
	d.locInv = loc(p, "u_inv")
	d.locTint = loc(p, "u_tint")
}

func (d *GPUDevice) allocTarget() error {
	d.freeTarget()
	C.glGenFramebuffers(1, &d.fbo)
	C.glGenTextures(1, &d.color)
	C.glBindTexture(C.GL_TEXTURE_2D, d.color)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_MIN_FILTER, C.GL_NEAREST)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_MAG_FILTER, C.GL_NEAREST)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_WRAP_S, C.GL_CLAMP_TO_EDGE)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_WRAP_T, C.GL_CLAMP_TO_EDGE)
	C.glTexImage2D(C.GL_TEXTURE_2D, 0, C.GL_RGBA, C.GLsizei(d.w), C.GLsizei(d.h), 0, C.GL_RGBA, C.GL_UNSIGNED_BYTE, nil)
	C.glGenRenderbuffers(1, &d.stencil)
	C.glBindRenderbuffer(C.GL_RENDERBUFFER, d.stencil)
	C.glRenderbufferStorage(C.GL_RENDERBUFFER, C.GL_STENCIL_INDEX8, C.GLsizei(d.w), C.GLsizei(d.h))
	C.glBindFramebuffer(C.GL_FRAMEBUFFER, d.fbo)
	C.glFramebufferTexture2D(C.GL_FRAMEBUFFER, C.GL_COLOR_ATTACHMENT0, C.GL_TEXTURE_2D, d.color, 0)
	C.glFramebufferRenderbuffer(C.GL_FRAMEBUFFER, C.GL_STENCIL_ATTACHMENT, C.GL_RENDERBUFFER, d.stencil)
	if C.glCheckFramebufferStatus(C.GL_FRAMEBUFFER) != C.GL_FRAMEBUFFER_COMPLETE {
		return fmt.Errorf("%w: incomplete FBO 0x%x", ErrGPUUnavailable, C.glCheckFramebufferStatus(C.GL_FRAMEBUFFER))
	}
	C.glViewport(0, 0, C.GLsizei(d.w), C.GLsizei(d.h))
	d.pixels = NewImage(d.w, d.h)
	d.readDirty = true
	return nil
}

func (d *GPUDevice) freeTarget() {
	if d.fbo != 0 {
		C.glDeleteFramebuffers(1, &d.fbo)
		d.fbo = 0
	}
	if d.color != 0 {
		C.glDeleteTextures(1, &d.color)
		d.color = 0
	}
	if d.stencil != 0 {
		C.glDeleteRenderbuffers(1, &d.stencil)
		d.stencil = 0
	}
}

func (d *GPUDevice) destroyGL() {
	d.freeTarget()
	if d.vbo != 0 {
		C.glDeleteBuffers(1, &d.vbo)
		d.vbo = 0
	}
	if d.rampTex != 0 {
		C.glDeleteTextures(1, &d.rampTex)
		d.rampTex = 0
	}
	if d.maskTex != 0 {
		C.glDeleteTextures(1, &d.maskTex)
		d.maskTex = 0
	}
	if d.prog != 0 {
		C.glDeleteProgram(d.prog)
		d.prog = 0
	}
	for k, t := range d.texCache {
		id := t.id
		C.glDeleteTextures(1, &id)
		delete(d.texCache, k)
	}
}

func (d *GPUDevice) destroyEGL() {
	if d.dpy != C.EGLDisplay(C.EGL_NO_DISPLAY) {
		C.eglMakeCurrent(d.dpy, C.EGLSurface(C.EGL_NO_SURFACE), C.EGLSurface(C.EGL_NO_SURFACE), C.EGLContext(C.EGL_NO_CONTEXT))
		if d.ctx != C.EGLContext(C.EGL_NO_CONTEXT) {
			C.eglDestroyContext(d.dpy, d.ctx)
			d.ctx = C.EGLContext(C.EGL_NO_CONTEXT)
		}
		if d.surf != nil && d.surf != C.EGLSurface(C.EGL_NO_SURFACE) {
			C.eglDestroySurface(d.dpy, d.surf)
			d.surf = C.EGLSurface(C.EGL_NO_SURFACE)
		}
		if d.ownEGL {
			C.eglTerminate(d.dpy)
		}
		d.dpy = C.EGLDisplay(C.EGL_NO_DISPLAY)
	}
}

func (d *GPUDevice) MakeCurrent() error {
	if d == nil || d.closed {
		return ErrGPUUnavailable
	}
	runtime.LockOSThread()
	draw := d.surf
	if draw == nil || draw == C.EGLSurface(C.EGL_NO_SURFACE) {
		draw = C.EGLSurface(C.EGL_NO_SURFACE)
	}
	if C.eglMakeCurrent(d.dpy, draw, draw, d.ctx) == C.EGL_FALSE {
		return fmt.Errorf("%w: eglMakeCurrent 0x%x", ErrGPUUnavailable, C.eglGetError())
	}
	C.glBindFramebuffer(C.GL_FRAMEBUFFER, d.fbo)
	C.glViewport(0, 0, C.GLsizei(d.w), C.GLsizei(d.h))
	return nil
}

func (d *GPUDevice) Size() (w, h int)  { return d.w, d.h }
func (d *GPUDevice) Kind() BackendKind { return BackendGPU }

func (d *GPUDevice) Resize(w, h int) error {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	if d.w == w && d.h == h {
		return nil
	}
	if err := d.MakeCurrent(); err != nil {
		return err
	}
	d.w, d.h = w, h
	return d.allocTarget()
}

func (d *GPUDevice) Close() error {
	if d == nil || d.closed {
		return nil
	}
	_ = d.MakeCurrent()
	d.destroyGL()
	d.destroyEGL()
	d.closed = true
	return nil
}

func (d *GPUDevice) Clear(c Color) {
	if d == nil || d.closed {
		return
	}
	if err := d.MakeCurrent(); err != nil {
		return
	}
	r, g, b, a := c.Premul8()
	C.glDisable(C.GL_SCISSOR_TEST)
	C.glColorMask(C.GL_TRUE, C.GL_TRUE, C.GL_TRUE, C.GL_TRUE)
	C.glClearColor(C.GLfloat(r)/255, C.GLfloat(g)/255, C.GLfloat(b)/255, C.GLfloat(a)/255)
	C.glClearStencil(0)
	C.glClear(C.GL_COLOR_BUFFER_BIT | C.GL_STENCIL_BUFFER_BIT)
	d.readDirty = true
}

// ClearRect overwrites the device-space box r (scissored glClear).
func (d *GPUDevice) ClearRect(r Rect, c Color) {
	if d == nil || d.closed {
		return
	}
	if err := d.MakeCurrent(); err != nil {
		return
	}
	if !d.applyClip(Clip{HasScissor: true, Scissor: r}, r) {
		return
	}
	cr, cg, cb, ca := c.Premul8()
	C.glColorMask(C.GL_TRUE, C.GL_TRUE, C.GL_TRUE, C.GL_TRUE)
	C.glClearColor(C.GLfloat(cr)/255, C.GLfloat(cg)/255, C.GLfloat(cb)/255, C.GLfloat(ca)/255)
	C.glClearStencil(0)
	C.glClear(C.GL_COLOR_BUFFER_BIT | C.GL_STENCIL_BUFFER_BIT)
	C.glDisable(C.GL_SCISSOR_TEST)
	d.readDirty = true
}

func (d *GPUDevice) Fill(path *Path, xform Matrix, paint Paint, clip Clip) {
	if path == nil || path.Empty() || d.w == 0 || d.h == 0 || !xform.Finite() {
		return
	}
	if err := d.MakeCurrent(); err != nil {
		return
	}
	if d.fillAxisAligned(path, xform, paint, clip) {
		return
	}
	e := d.tess.lookupFill(path, xform, paint.FillRule)
	if e == nil {
		return
	}
	d.stencilAndCover(e.contours, paint, xform, clip, int(paint.FillRule), e.verts, e.box)
}

func (d *GPUDevice) Stroke(path *Path, xform Matrix, paint Paint, clip Clip) {
	if path == nil || path.Empty() || d.w == 0 || d.h == 0 || !xform.Finite() {
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
	if err := d.MakeCurrent(); err != nil {
		return
	}
	e := d.tess.lookupStroke(path, xform, st)
	if e == nil {
		return
	}
	d.stencilAndCover(e.contours, paint, xform, clip, raster.FillNonZero, e.verts, e.box)
}

func (d *GPUDevice) Blit(src *Image, srcRect, dstRect Rect, xform Matrix, paint Paint, clip Clip) {
	if src == nil || src.Width == 0 || src.Height == 0 || dstRect.Empty() || !xform.Finite() {
		return
	}
	if err := d.MakeCurrent(); err != nil {
		return
	}
	srcRect = srcRect.Canon()
	dstRect = dstRect.Canon()
	if srcRect.Empty() {
		srcRect = XYWH(0, 0, float32(src.Width), float32(src.Height))
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
	tex := d.uploadImage(src, paint.Filter)
	if tex == 0 {
		C.glDisable(C.GL_SCISSOR_TEST)
		return
	}
	d.bindProgram(3, paint, xform, clip)
	C.glUniform4f(d.locTint, C.GLfloat(tr)/255, C.GLfloat(tg)/255, C.GLfloat(tb)/255, C.GLfloat(ta)/255)
	C.glActiveTexture(C.GL_TEXTURE0)
	C.glBindTexture(C.GL_TEXTURE_2D, tex)
	C.glUniform1i(d.locTex, 0)

	p0 := xform.Transform(dstRect.Min)
	p1 := xform.Transform(Point{dstRect.Max.X, dstRect.Min.Y})
	p2 := xform.Transform(dstRect.Max)
	p3 := xform.Transform(Point{dstRect.Min.X, dstRect.Max.Y})
	su0 := srcRect.Min.X / float32(src.Width)
	sv0 := srcRect.Min.Y / float32(src.Height)
	su1 := srcRect.Max.X / float32(src.Width)
	sv1 := srcRect.Max.Y / float32(src.Height)
	d.quad = append(d.quad[:0],
		p0.X, p0.Y, su0, sv0,
		p1.X, p1.Y, su1, sv0,
		p2.X, p2.Y, su1, sv1,
		p0.X, p0.Y, su0, sv0,
		p2.X, p2.Y, su1, sv1,
		p3.X, p3.Y, su0, sv1,
	)
	C.glColorMask(C.GL_TRUE, C.GL_TRUE, C.GL_TRUE, C.GL_TRUE)
	C.glStencilFunc(C.GL_ALWAYS, 0, 0xFF)
	C.glStencilOp(C.GL_KEEP, C.GL_KEEP, C.GL_KEEP)
	d.drawTris(d.quad)
	C.glDisable(C.GL_SCISSOR_TEST)
	d.readDirty = true
}

func (d *GPUDevice) Image() *Image { return d.Snapshot() }
func (d *GPUDevice) Snapshot() *Image {
	if d == nil || d.closed {
		return nil
	}
	if !d.readDirty && d.pixels != nil {
		return d.pixels
	}
	if err := d.MakeCurrent(); err != nil {
		return d.pixels
	}
	if d.pixels == nil || d.pixels.Width != d.w || d.pixels.Height != d.h {
		d.pixels = NewImage(d.w, d.h)
	}
	C.glBindFramebuffer(C.GL_FRAMEBUFFER, d.fbo)
	C.glReadPixels(0, 0, C.GLsizei(d.w), C.GLsizei(d.h), C.GL_RGBA, C.GL_UNSIGNED_BYTE, unsafe.Pointer(&d.pixels.Pix[0]))
	// GL origin is bottom-left; flip to +Y down.
	row := d.w * 4
	tmp := make([]byte, row)
	for y := 0; y < d.h/2; y++ {
		a := y * row
		b := (d.h - 1 - y) * row
		copy(tmp, d.pixels.Pix[a:a+row])
		copy(d.pixels.Pix[a:a+row], d.pixels.Pix[b:b+row])
		copy(d.pixels.Pix[b:b+row], tmp)
	}
	d.readDirty = false
	return d.pixels
}

func (d *GPUDevice) Present() error { return d.PresentRects(nil) }

// PresentRects blits the FBO to the window and swaps. A nil/empty list is a
// full-surface present. Non-empty rects are passed to
// eglSwapBuffersWithDamageKHR/EXT when available so the compositor can
// skip clean tiles (KDE/Qt partial update). The FBO→window blit stays
// full-frame because EGL back buffers are not preserved after swap.
func (d *GPUDevice) PresentRects(rects []Rect) error {
	if d == nil || d.closed {
		return ErrGPUUnavailable
	}
	if err := d.MakeCurrent(); err != nil {
		return err
	}
	if !d.window || d.surf == nil || d.surf == C.EGLSurface(C.EGL_NO_SURFACE) {
		C.glFlush()
		return nil
	}
	// Blit FBO to the window (opaque: force alpha in the default FB).
	C.glBindFramebuffer(C.GL_FRAMEBUFFER, 0)
	C.glViewport(0, 0, C.GLsizei(d.w), C.GLsizei(d.h))
	C.glDisable(C.GL_STENCIL_TEST)
	C.glDisable(C.GL_BLEND)
	C.glDisable(C.GL_SCISSOR_TEST)
	C.glUseProgram(d.prog)
	C.glUniform2f(d.locVP, C.GLfloat(d.w), C.GLfloat(d.h))
	C.glUniform1i(d.locMode, 3)
	C.glUniform1i(d.locUseM, 0)
	C.glUniform4f(d.locTint, 1, 1, 1, 1)
	C.glActiveTexture(C.GL_TEXTURE0)
	C.glBindTexture(C.GL_TEXTURE_2D, d.color)
	C.glUniform1i(d.locTex, 0)
	d.quad = append(d.quad[:0],
		0, 0, 0, 1,
		float32(d.w), 0, 1, 1,
		float32(d.w), float32(d.h), 1, 0,
		0, 0, 0, 1,
		float32(d.w), float32(d.h), 1, 0,
		0, float32(d.h), 0, 0,
	)
	d.drawTris(d.quad)
	C.glEnable(C.GL_BLEND)
	C.glEnable(C.GL_STENCIL_TEST)
	if !d.swap(rects) {
		return fmt.Errorf("paintengine2d: eglSwapBuffers 0x%x", C.eglGetError())
	}
	C.glBindFramebuffer(C.GL_FRAMEBUFFER, d.fbo)
	return nil
}

func (d *GPUDevice) swap(rects []Rect) bool {
	n := 0
	if len(rects) > 0 {
		need := len(rects) * 4
		if cap(d.eglDamage) < need {
			d.eglDamage = make([]C.EGLint, need)
		} else {
			d.eglDamage = d.eglDamage[:need]
		}
		for _, r := range rects {
			x0, y0, x1, y1 := clampPixelBounds(r, d.w, d.h)
			if x0 >= x1 || y0 >= y1 {
				continue
			}
			// EGL damage origin is bottom-left.
			d.eglDamage[n+0] = C.EGLint(x0)
			d.eglDamage[n+1] = C.EGLint(d.h - y1)
			d.eglDamage[n+2] = C.EGLint(x1 - x0)
			d.eglDamage[n+3] = C.EGLint(y1 - y0)
			n += 4
		}
	}
	if n == 0 {
		return C.eglSwapBuffers(d.dpy, d.surf) != C.EGL_FALSE
	}
	return C.pe_swap_with_damage(d.dpy, d.surf, &d.eglDamage[0], C.EGLint(n/4)) != 0
}

func (d *GPUDevice) fillAxisAligned(path *Path, xform Matrix, paint Paint, clip Clip) bool {
	if paint.FillRule == FillEvenOdd || !isClosedRectPath(path) || !xform.IsAxisAligned() {
		return false
	}
	r := xform.TransformRect(path.Bounds())
	if r.Empty() {
		return true
	}
	if paint.Shader == nil {
		_, _, _, a := paint.Color.Premul8()
		if a == 255 && clip.Mask == nil {
			d.fillOpaqueRects([]Rect{r}, paint.Color, clip)
			return true
		}
	}
	if !d.applyClip(clip, r) {
		return true
	}
	C.glDisable(C.GL_STENCIL_TEST)
	C.glColorMask(C.GL_TRUE, C.GL_TRUE, C.GL_TRUE, C.GL_TRUE)
	C.glStencilFunc(C.GL_ALWAYS, 0, 0xFF)
	C.glStencilOp(C.GL_KEEP, C.GL_KEEP, C.GL_KEEP)
	if paint.Shader == nil {
		_, _, _, a := paint.Color.Premul8()
		if a == 255 {
			C.glDisable(C.GL_BLEND)
		}
	}
	d.bindProgram(d.shaderMode(paint), paint, xform, clip)
	d.coverQuad(r)
	C.glEnable(C.GL_BLEND)
	C.glEnable(C.GL_STENCIL_TEST)
	C.glDisable(C.GL_SCISSOR_TEST)
	d.readDirty = true
	return true
}

func (d *GPUDevice) packPath(path *Path) {
	d.verbs = d.verbs[:0]
	d.pts = d.pts[:0]
	for _, v := range path.verbs {
		d.verbs = append(d.verbs, raster.Verb(v))
	}
	for _, p := range path.pts {
		d.pts = append(d.pts, raster.Vec2{X: p.X, Y: p.Y})
	}
}

func (d *GPUDevice) preparePath(path *Path, xform Matrix) {
	d.packPath(path)
	if !xform.IsIdentity() {
		for i := range d.pts {
			q := xform.Transform(Point{d.pts[i].X, d.pts[i].Y})
			d.pts[i] = raster.Vec2{X: q.X, Y: q.Y}
		}
	}
	raster.Flatten(d.verbs, d.pts, 0.2, &d.contours, &d.closedC)
}

func (d *GPUDevice) contourBounds() Rect {
	var minX, minY, maxX, maxY float32
	n := 0
	for _, c := range d.contours {
		for _, p := range c {
			if n == 0 {
				minX, minY, maxX, maxY = p.X, p.Y, p.X, p.Y
			} else {
				minX = min32(minX, p.X)
				minY = min32(minY, p.Y)
				maxX = max32(maxX, p.X)
				maxY = max32(maxY, p.Y)
			}
			n++
		}
	}
	if n == 0 {
		return Rect{}
	}
	return Rect{Min: Point{minX, minY}, Max: Point{maxX, maxY}}.Inset(-1)
}

func (d *GPUDevice) applyClip(clip Clip, bounds Rect) bool {
	scissor := XYWH(0, 0, float32(d.w), float32(d.h))
	if clip.HasScissor {
		scissor = scissor.Intersect(clip.Scissor)
	}
	scissor = scissor.Intersect(bounds)
	if scissor.Empty() {
		return false
	}
	x0, y0, x1, y1 := clampPixelBounds(scissor, d.w, d.h)
	if x0 >= x1 || y0 >= y1 {
		return false
	}
	// GL scissor origin is bottom-left.
	C.glEnable(C.GL_SCISSOR_TEST)
	C.glScissor(C.GLint(x0), C.GLint(d.h-y1), C.GLsizei(x1-x0), C.GLsizei(y1-y0))
	return true
}

func (d *GPUDevice) stencilAndCover(contours [][]raster.Vec2, paint Paint, xform Matrix, clip Clip, rule int, verts []float32, box Rect) {
	if box.Empty() {
		box = contourBoundsOf(contours)
	}
	if !d.applyClip(clip, box) {
		return
	}
	C.glUseProgram(d.prog)
	C.glUniform2f(d.locVP, C.GLfloat(d.w), C.GLfloat(d.h))
	C.glColorMask(C.GL_FALSE, C.GL_FALSE, C.GL_FALSE, C.GL_FALSE)
	C.glStencilFunc(C.GL_ALWAYS, 0, 0xFF)
	C.glClearStencil(0)
	C.glClear(C.GL_STENCIL_BUFFER_BIT)
	if rule == int(FillEvenOdd) {
		C.glStencilOp(C.GL_KEEP, C.GL_KEEP, C.GL_INVERT)
	} else {
		C.glStencilOpSeparate(C.GL_FRONT, C.GL_KEEP, C.GL_KEEP, C.GL_INCR_WRAP)
		C.glStencilOpSeparate(C.GL_BACK, C.GL_KEEP, C.GL_KEEP, C.GL_DECR_WRAP)
	}
	if len(verts) >= 12 {
		d.drawTris(verts)
	} else {
		for _, c := range contours {
			d.drawFan(c)
		}
	}
	C.glColorMask(C.GL_TRUE, C.GL_TRUE, C.GL_TRUE, C.GL_TRUE)
	if rule == int(FillEvenOdd) {
		C.glStencilFunc(C.GL_NOTEQUAL, 0, 0x01)
	} else {
		C.glStencilFunc(C.GL_NOTEQUAL, 0, 0xFF)
	}
	C.glStencilOp(C.GL_KEEP, C.GL_KEEP, C.GL_KEEP)
	d.bindProgram(d.shaderMode(paint), paint, xform, clip)
	d.coverQuad(box)
	C.glStencilFunc(C.GL_ALWAYS, 0, 0xFF)
	C.glDisable(C.GL_SCISSOR_TEST)
	d.readDirty = true
}

func (d *GPUDevice) shaderMode(paint Paint) int {
	switch paint.Shader.(type) {
	case LinearGradient:
		return 1
	case RadialGradient:
		return 2
	default:
		return 0
	}
}

func (d *GPUDevice) bindProgram(mode int, paint Paint, xform Matrix, clip Clip) {
	C.glUseProgram(d.prog)
	C.glUniform2f(d.locVP, C.GLfloat(d.w), C.GLfloat(d.h))
	C.glUniform1i(d.locMode, C.GLint(mode))
	r, g, b, a := paint.Color.Premul8()
	C.glUniform4f(d.locColor, C.GLfloat(r)/255, C.GLfloat(g)/255, C.GLfloat(b)/255, C.GLfloat(a)/255)
	inv, ok := xform.Invert()
	if !ok {
		inv = Identity()
	}
	m := [9]C.GLfloat{
		C.GLfloat(inv.A), C.GLfloat(inv.B), 0,
		C.GLfloat(inv.C), C.GLfloat(inv.D), 0,
		C.GLfloat(inv.E), C.GLfloat(inv.F), 1,
	}
	C.glUniformMatrix3fv(d.locInv, 1, C.GL_FALSE, &m[0])
	C.glUniform4f(d.locTint, 1, 1, 1, 1)

	useMask := 0
	if clip.Mask != nil && clip.MaskW > 0 && clip.MaskH > 0 {
		useMask = 1
		d.uploadMask(clip)
		C.glUniform4f(d.locMaskR, C.GLfloat(clip.MaskX), C.GLfloat(clip.MaskY), C.GLfloat(clip.MaskW), C.GLfloat(clip.MaskH))
		C.glActiveTexture(C.GL_TEXTURE2)
		C.glBindTexture(C.GL_TEXTURE_2D, d.maskTex)
		C.glUniform1i(d.locMask, 2)
	}
	C.glUniform1i(d.locUseM, C.GLint(useMask))

	switch g := paint.Shader.(type) {
	case LinearGradient:
		d.bakeStops(g.Stops)
		C.glUniform2f(d.locG0, C.GLfloat(g.Start.X), C.GLfloat(g.Start.Y))
		C.glUniform2f(d.locG1, C.GLfloat(g.End.X), C.GLfloat(g.End.Y))
		C.glUniform1i(d.locTile, C.GLint(g.Tile))
		C.glActiveTexture(C.GL_TEXTURE1)
		C.glBindTexture(C.GL_TEXTURE_2D, d.rampTex)
		C.glUniform1i(d.locRamp, 1)
	case RadialGradient:
		d.bakeStops(g.Stops)
		origin := g.Center
		if g.Focal != (Point{}) && (g.Focal.X != g.Center.X || g.Focal.Y != g.Center.Y) {
			origin = g.Focal
		}
		C.glUniform2f(d.locCenter, C.GLfloat(origin.X), C.GLfloat(origin.Y))
		rad, inner := g.Radius, g.Inner
		if rad < 0 {
			rad = -rad
		}
		if inner < 0 {
			inner = -inner
		}
		C.glUniform1f(d.locRad, C.GLfloat(rad))
		C.glUniform1f(d.locInner, C.GLfloat(inner))
		C.glUniform1i(d.locTile, C.GLint(g.Tile))
		C.glActiveTexture(C.GL_TEXTURE1)
		C.glBindTexture(C.GL_TEXTURE_2D, d.rampTex)
		C.glUniform1i(d.locRamp, 1)
	}
}

func (d *GPUDevice) bakeStops(stops []GradientStop) {
	for i := 0; i < 256; i++ {
		c := sampleStops(stops, float32(i)/255)
		r, g, b, a := c.Premul8()
		d.gradRamp[i*4+0] = r
		d.gradRamp[i*4+1] = g
		d.gradRamp[i*4+2] = b
		d.gradRamp[i*4+3] = a
	}
	C.glBindTexture(C.GL_TEXTURE_2D, d.rampTex)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_MIN_FILTER, C.GL_LINEAR)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_MAG_FILTER, C.GL_LINEAR)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_WRAP_S, C.GL_CLAMP_TO_EDGE)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_WRAP_T, C.GL_CLAMP_TO_EDGE)
	C.glTexImage2D(C.GL_TEXTURE_2D, 0, C.GL_RGBA, 256, 1, 0, C.GL_RGBA, C.GL_UNSIGNED_BYTE, unsafe.Pointer(&d.gradRamp[0]))
}

func (d *GPUDevice) uploadMask(clip Clip) {
	need := clip.MaskW * clip.MaskH * 4
	if need <= 0 {
		return
	}
	if cap(d.maskPix) < need {
		d.maskPix = make([]byte, need)
	} else {
		d.maskPix = d.maskPix[:need]
	}
	alpha := d.maskPix
	for i, v := range clip.Mask {
		if i*4+3 >= len(alpha) {
			break
		}
		alpha[i*4+0] = v
		alpha[i*4+1] = v
		alpha[i*4+2] = v
		alpha[i*4+3] = v
	}
	C.glBindTexture(C.GL_TEXTURE_2D, d.maskTex)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_MIN_FILTER, C.GL_NEAREST)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_MAG_FILTER, C.GL_NEAREST)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_WRAP_S, C.GL_CLAMP_TO_EDGE)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_WRAP_T, C.GL_CLAMP_TO_EDGE)
	C.glTexImage2D(C.GL_TEXTURE_2D, 0, C.GL_RGBA, C.GLsizei(clip.MaskW), C.GLsizei(clip.MaskH), 0, C.GL_RGBA, C.GL_UNSIGNED_BYTE, unsafe.Pointer(&alpha[0]))
}

func (d *GPUDevice) uploadImage(src *Image, filter FilterMode) C.GLuint {
	key := uintptr(unsafe.Pointer(src))
	if e, ok := d.texCache[key]; ok && e.w == src.Width && e.h == src.Height && e.nbytes == len(src.Pix) && e.epoch == src.Epoch {
		return e.id
	}
	if e, ok := d.texCache[key]; ok {
		id := e.id
		C.glDeleteTextures(1, &id)
		delete(d.texCache, key)
	}
	var id C.GLuint
	C.glGenTextures(1, &id)
	C.glBindTexture(C.GL_TEXTURE_2D, id)
	minF := C.GLint(C.GL_LINEAR)
	if filter == FilterNearest {
		minF = C.GL_NEAREST
	}
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_MIN_FILTER, minF)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_MAG_FILTER, minF)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_WRAP_S, C.GL_CLAMP_TO_EDGE)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_WRAP_T, C.GL_CLAMP_TO_EDGE)
	// Packed upload: copy if stride is padded.
	pix := src.Pix
	if src.RowStride() != src.Width*4 {
		pack := make([]byte, src.Width*src.Height*4)
		row := src.Width * 4
		for y := 0; y < src.Height; y++ {
			copy(pack[y*row:(y+1)*row], src.Pix[y*src.RowStride():y*src.RowStride()+row])
		}
		pix = pack
	}
	C.glTexImage2D(C.GL_TEXTURE_2D, 0, C.GL_RGBA, C.GLsizei(src.Width), C.GLsizei(src.Height), 0, C.GL_RGBA, C.GL_UNSIGNED_BYTE, unsafe.Pointer(&pix[0]))
	d.texCache[key] = gpuTex{id: id, w: src.Width, h: src.Height, nbytes: len(src.Pix), epoch: src.Epoch}
	return id
}

func (d *GPUDevice) drawFan(c []raster.Vec2) {
	if len(c) < 3 {
		return
	}
	d.verts = d.verts[:0]
	for i := 1; i+1 < len(c); i++ {
		d.verts = append(d.verts,
			c[0].X, c[0].Y, 0, 0,
			c[i].X, c[i].Y, 0, 0,
			c[i+1].X, c[i+1].Y, 0, 0,
		)
	}
	if len(d.verts) == 0 {
		return
	}
	d.drawTris(d.verts)
}

func (d *GPUDevice) coverQuad(r Rect) {
	d.quad = append(d.quad[:0],
		r.Min.X, r.Min.Y, 0, 0,
		r.Max.X, r.Min.Y, 1, 0,
		r.Max.X, r.Max.Y, 1, 1,
		r.Min.X, r.Min.Y, 0, 0,
		r.Max.X, r.Max.Y, 1, 1,
		r.Min.X, r.Max.Y, 0, 1,
	)
	d.drawTris(d.quad)
}

func (d *GPUDevice) drawTris(verts []float32) {
	if len(verts) < 12 {
		return
	}
	C.glBindBuffer(C.GL_ARRAY_BUFFER, d.vbo)
	C.glBufferData(C.GL_ARRAY_BUFFER, C.GLsizeiptr(len(verts)*4), unsafe.Pointer(&verts[0]), C.GL_STREAM_DRAW)
	C.glEnableVertexAttribArray(0)
	C.glEnableVertexAttribArray(1)
	C.glVertexAttribPointer(0, 2, C.GL_FLOAT, C.GL_FALSE, 16, unsafe.Pointer(uintptr(0)))
	C.glVertexAttribPointer(1, 2, C.GL_FLOAT, C.GL_FALSE, 16, unsafe.Pointer(uintptr(8)))
	C.glDrawArrays(C.GL_TRIANGLES, 0, C.GLsizei(len(verts)/4))
	C.glDisableVertexAttribArray(0)
	C.glDisableVertexAttribArray(1)
}

func (d *GPUDevice) fillOpaqueRects(rects []Rect, color Color, clip Clip) {
	if d == nil || d.closed || len(rects) == 0 {
		return
	}
	if err := d.MakeCurrent(); err != nil {
		return
	}
	box := rects[0]
	for _, r := range rects[1:] {
		box = box.Union(r)
	}
	if !d.applyClip(clip, box) {
		return
	}
	cr, cg, cb, ca := color.Premul8()
	if ca == 0 {
		C.glDisable(C.GL_SCISSOR_TEST)
		return
	}
	C.glUseProgram(d.prog)
	C.glUniform2f(d.locVP, C.GLfloat(d.w), C.GLfloat(d.h))
	C.glUniform1i(d.locMode, 0)
	C.glUniform4f(d.locColor, C.GLfloat(cr)/255, C.GLfloat(cg)/255, C.GLfloat(cb)/255, C.GLfloat(ca)/255)
	C.glUniform1i(d.locUseM, 0)
	C.glColorMask(C.GL_TRUE, C.GL_TRUE, C.GL_TRUE, C.GL_TRUE)
	C.glStencilFunc(C.GL_ALWAYS, 0, 0xFF)
	C.glStencilOp(C.GL_KEEP, C.GL_KEEP, C.GL_KEEP)
	C.glDisable(C.GL_STENCIL_TEST)
	if ca == 255 {
		C.glDisable(C.GL_BLEND)
	}
	d.verts = d.verts[:0]
	for _, r := range rects {
		d.verts = append(d.verts,
			r.Min.X, r.Min.Y, 0, 0,
			r.Max.X, r.Min.Y, 1, 0,
			r.Max.X, r.Max.Y, 1, 1,
			r.Min.X, r.Min.Y, 0, 0,
			r.Max.X, r.Max.Y, 1, 1,
			r.Min.X, r.Max.Y, 0, 1,
		)
	}
	d.drawTris(d.verts)
	C.glEnable(C.GL_BLEND)
	C.glEnable(C.GL_STENCIL_TEST)
	C.glDisable(C.GL_SCISSOR_TEST)
	d.readDirty = true
}

var _ Device = (*GPUDevice)(nil)
