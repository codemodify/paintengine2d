//go:build linux && cgo

package paintengine2d

/*
#cgo linux pkg-config: egl glesv2
#define EGL_EGLEXT_PROTOTYPES
#include <EGL/egl.h>
#include <EGL/eglext.h>
#include <GLES2/gl2.h>
#include <GLES2/gl2ext.h>
#include <pthread.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

// GLES3 tokens we use through runtime-resolved entry points (the headers
// here are GLES2 only, but Mesa hands an ES2 request an ES3.x context).
#ifndef PE_GL_READ_FRAMEBUFFER
#define PE_GL_READ_FRAMEBUFFER 0x8CA8
#define PE_GL_DRAW_FRAMEBUFFER 0x8CA9
#define PE_GL_RGBA8            0x8058
#define PE_GL_MAX_SAMPLES      0x8D57
#endif

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
	"uniform int u_cov;\n"
	"uniform vec2 u_patOrigin;\n"
	"uniform vec2 u_patSize;\n"
	"uniform int u_texA8;\n"
	"vec4 texel(vec2 uv) {\n"
	"  vec4 t = texture2D(u_tex, uv);\n"
	"  return u_texA8 == 1 ? vec4(t.a) : t;\n"
	"}\n"
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
	"    c = texture2D(u_ramp, vec2(tileT(t), 0.5)) * u_tint.a;\n"
	"  } else if (u_mode == 2) {\n"
	"    vec2 p = toUser(v_pos);\n"
	"    float dist = length(p - u_center);\n"
	"    float span = u_radius - u_inner;\n"
	"    float t = 0.0;\n"
	"    if (span < 1e-8) t = dist <= u_radius ? 0.0 : 1.0;\n"
	"    else t = (dist - u_inner) / span;\n"
	"    c = texture2D(u_ramp, vec2(tileT(t), 0.5)) * u_tint.a;\n"
	"  } else if (u_mode == 4) {\n"
	"    vec2 t = mod(toUser(v_pos) - u_patOrigin, u_patSize) / u_patSize;\n"
	"    c = texel(t) * u_tint.a;\n"
	"  } else if (u_mode == 3) {\n"
	"    c = texel(v_uv);\n"
	"    c.rgb *= u_tint.rgb;\n"
	"    c *= u_tint.a;\n"
	"  }\n"
	"  if (u_cov == 1) {\n"
	"    c *= clamp(v_uv.x, 0.0, 1.0);\n"
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

typedef EGLBoolean (*pe_set_damage_fn)(EGLDisplay, EGLSurface, EGLint *, EGLint);
static pe_set_damage_fn pe_set_damage;

#ifndef EGL_BUFFER_AGE_KHR
#define EGL_BUFFER_AGE_KHR 0x313D
#endif

static void pe_load_ext(void) {
	pe_swap_damage = (pe_swap_damage_fn)eglGetProcAddress("eglSwapBuffersWithDamageKHR");
	if (!pe_swap_damage) {
		pe_swap_damage = (pe_swap_damage_fn)eglGetProcAddress("eglSwapBuffersWithDamageEXT");
	}
	pe_set_damage = (pe_set_damage_fn)eglGetProcAddress("eglSetDamageRegionKHR");
}

static int pe_swap_with_damage(EGLDisplay dpy, EGLSurface surf, EGLint *rects, EGLint n) {
	if (pe_swap_damage && rects && n > 0) {
		return pe_swap_damage(dpy, surf, rects, n) == EGL_TRUE;
	}
	return eglSwapBuffers(dpy, surf) == EGL_TRUE;
}

static EGLint pe_buffer_age(EGLDisplay dpy, EGLSurface surf) {
	EGLint age = 0;
	if (eglQuerySurface(dpy, surf, EGL_BUFFER_AGE_KHR, &age) == EGL_TRUE) {
		return age;
	}
	return 0;
}

static int pe_set_damage_region(EGLDisplay dpy, EGLSurface surf, EGLint *rects, EGLint n) {
	if (!pe_set_damage || !rects || n <= 0) {
		return 0;
	}
	return pe_set_damage(dpy, surf, rects, n) == EGL_TRUE;
}

// pe_attrib wires a vertex attribute without forming a Go unsafe.Pointer
// from an integer offset (which trips go vet and -race checkptr).
static void pe_attrib(GLuint index, GLsizei stride, size_t offset) {
	glVertexAttribPointer(index, 2, GL_FLOAT, GL_FALSE, stride, (const void *)offset);
}

static EGLDisplay pe_display_from_ptr(uintptr_t native, EGLenum platform) {
	return pe_get_display(native ? (void *)native : NULL, platform);
}

static uintptr_t pe_thread_self(void) { return (uintptr_t)pthread_self(); }

// Opaque EGL handles the caller owns arrive as integers; convert them in C
// so no Go unsafe.Pointer is ever built from a uintptr.
static EGLDisplay pe_to_display(uintptr_t v) { return (EGLDisplay)v; }
static EGLContext pe_to_context(uintptr_t v) { return (EGLContext)v; }
static EGLSurface pe_to_surface(uintptr_t v) { return (EGLSurface)v; }
static uintptr_t pe_from_display(EGLDisplay v) { return (uintptr_t)v; }
static uintptr_t pe_from_context(EGLContext v) { return (uintptr_t)v; }
static uintptr_t pe_from_surface(EGLSurface v) { return (uintptr_t)v; }

// --- GLES3 multisample resolve, resolved at runtime ---------------------
typedef void (*pe_rbsm_fn)(GLenum, GLsizei, GLenum, GLsizei, GLsizei);
typedef void (*pe_blitfb_fn)(GLint, GLint, GLint, GLint, GLint, GLint, GLint, GLint, GLbitfield, GLenum);
static pe_rbsm_fn pe_rbsm;
static pe_blitfb_fn pe_blitfb;

static int pe_load_msaa(void) {
	pe_rbsm = (pe_rbsm_fn)eglGetProcAddress("glRenderbufferStorageMultisample");
	pe_blitfb = (pe_blitfb_fn)eglGetProcAddress("glBlitFramebuffer");
	if (!pe_rbsm || !pe_blitfb) {
		return 0;
	}
	const char *v = (const char *)glGetString(GL_VERSION);
	// Mesa hands an ES2 request an ES3 context; only then are the
	// multisample entry points real.
	if (!v || !strstr(v, "OpenGL ES 3")) {
		return 0;
	}
	return 1;
}

static int pe_max_samples(void) {
	GLint n = 0;
	glGetIntegerv(PE_GL_MAX_SAMPLES, &n);
	return (int)n;
}

static void pe_rb_multisample(GLsizei samples, GLenum fmt, GLsizei w, GLsizei h) {
	if (pe_rbsm) {
		pe_rbsm(GL_RENDERBUFFER, samples, fmt, w, h);
	}
}

static void pe_resolve(GLuint src, GLuint dst, GLsizei w, GLsizei h) {
	if (!pe_blitfb) {
		return;
	}
	glBindFramebuffer(PE_GL_READ_FRAMEBUFFER, src);
	glBindFramebuffer(PE_GL_DRAW_FRAMEBUFFER, dst);
	pe_blitfb(0, 0, w, h, 0, 0, w, h, GL_COLOR_BUFFER_BIT, GL_NEAREST);
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
	"math"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
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
	msFBO, msColor      C.GLuint
	msStencil           C.GLuint
	msaa                bool
	msSamples           int
	msDirty             bool
	prog                C.GLuint
	vbo                 C.GLuint
	rampTex             C.GLuint
	maskTex             C.GLuint

	locVP, locMode, locColor          C.GLint
	locTex, locRamp, locMask, locUseM C.GLint
	locMaskR, locG0, locG1, locCenter C.GLint
	locRad, locInner, locTile, locInv C.GLint
	locTint, locCov                   C.GLint
	locPatO, locPatS                  C.GLint
	// locTexA8 is u_texA8: the bound image is a FormatA8 mask, uploaded as
	// GL_ALPHA and read as premultiplied white.
	locTexA8 C.GLint

	window bool
	// blend is the Porter-Duff operator glBlendFunc is set to now, so a
	// draw only changes the GL state when the paint's operator differs.
	blend              BlendMode
	ownEGL             bool
	ownCtx             bool
	closed             bool
	thread             C.uintptr_t
	frameDepth         int
	err                error
	preserve           bool
	partial            bool
	bufferAge          bool
	presentSet         bool
	presentSkip        bool
	presentRects       []Rect
	info               string
	scrollTex          C.GLuint
	scrollTW, scrollTH int
	blur               gpuBlur // backdrop blur program and scratch targets

	// pixels is the CPU copy Snapshot reads the frame into, made by the
	// first Snapshot: a device-size image (12.5 MB at 2240×1400) that only
	// screenshots and a fallback to the CPU need.
	pixels    *Image
	readDirty bool

	// ageRing remembers the damage of recent frames. With
	// EGL_KHR_partial_update and a buffer age of N, the back buffer holds
	// the frame from N swaps ago, so the blit must repaint this frame's
	// damage plus the previous N-1 frames'.
	// ageFull marks slots whose frame repainted the whole surface (a
	// full present, or unknown history after a resize): a back buffer
	// older than such a frame can only be repaired by a full blit.
	ageRing  [maxBufferAge][]Rect
	ageFull  [maxBufferAge]bool
	ageReset bool // next recorded frame follows a reset: count it as full
	ageIdx   int
	ageScrat []Rect

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

	texCache  map[uint64]*gpuTex
	texBytes  int
	texBudget int
	texClock  uint64
	gradRamp  [256 * 4]byte
	rampHash  uint64
	rampSet   bool
	maskHash  uint64
	maskSet   bool
	tess      *tessCache
}

// maxBufferAge bounds the swap-damage history kept for partial presents.
const maxBufferAge = 4

// defaultTexBudget caps the bytes of cached image textures before the least
// recently used entries are evicted.
const defaultTexBudget = 64 << 20

type gpuTex struct {
	id     C.GLuint
	w, h   int
	nbytes int
	// bytes is the texture's size on the GPU (4 or 1 byte per pixel).
	bytes int
	epoch uint64
	used  uint64
}

// eglDisplayRef refcounts one native EGL display. eglTerminate tears down
// every context and surface on the display, and eglGetDisplay returns the
// same handle for the same native display — so a per-device Close() used to
// kill the other windows' devices (and the availability probe killed the
// caller's device mid-frame).
type eglDisplayRef struct {
	dpy  C.EGLDisplay
	refs int
	ext  string
}

var (
	eglMu    sync.Mutex
	eglRefs  = map[C.EGLDisplay]*eglDisplayRef{}
	curDev   atomic.Pointer[GPUDevice]
	probeMu  sync.Mutex
	probeRun bool
	probeOK  bool
)

// acquireDisplay returns an initialized EGLDisplay for the native display,
// bumping its reference count.
func acquireDisplay(native uintptr, platform C.EGLenum) (C.EGLDisplay, string, error) {
	dpy := C.pe_display_from_ptr(C.uintptr_t(native), platform)
	if dpy == C.EGLDisplay(C.EGL_NO_DISPLAY) && platform != 0 {
		dpy = C.pe_display_from_ptr(C.uintptr_t(native), 0)
	}
	if dpy == C.EGLDisplay(C.EGL_NO_DISPLAY) {
		return dpy, "", fmt.Errorf("%w: eglGetDisplay", ErrGPUUnavailable)
	}
	eglMu.Lock()
	defer eglMu.Unlock()
	if r, ok := eglRefs[dpy]; ok {
		r.refs++
		return dpy, r.ext, nil
	}
	var maj, min C.EGLint
	if C.eglInitialize(dpy, &maj, &min) == C.EGL_FALSE {
		return dpy, "", fmt.Errorf("%w: eglInitialize 0x%x", ErrGPUUnavailable, C.eglGetError())
	}
	var ext string
	if e := C.eglQueryString(dpy, C.EGL_EXTENSIONS); e != nil {
		ext = C.GoString(e)
	}
	eglRefs[dpy] = &eglDisplayRef{dpy: dpy, refs: 1, ext: ext}
	return dpy, ext, nil
}

// adoptDisplay registers a display this process did not initialize (the
// caller keeps ownership; we never terminate it).
func adoptDisplay(dpy C.EGLDisplay) string {
	eglMu.Lock()
	defer eglMu.Unlock()
	if r, ok := eglRefs[dpy]; ok {
		r.refs++
		return r.ext
	}
	var ext string
	if e := C.eglQueryString(dpy, C.EGL_EXTENSIONS); e != nil {
		ext = C.GoString(e)
	}
	// refs starts at 2 so the borrowed display is never terminated by us.
	eglRefs[dpy] = &eglDisplayRef{dpy: dpy, refs: 2, ext: ext}
	return ext
}

// releaseDisplay drops one reference and terminates the display only when
// this process holds the last one.
func releaseDisplay(dpy C.EGLDisplay) {
	if dpy == C.EGLDisplay(C.EGL_NO_DISPLAY) {
		return
	}
	eglMu.Lock()
	defer eglMu.Unlock()
	r, ok := eglRefs[dpy]
	if !ok {
		return
	}
	r.refs--
	if r.refs > 0 {
		return
	}
	delete(eglRefs, dpy)
	C.eglTerminate(dpy)
}

type gpuSurface struct {
	dev *GPUDevice
}

var gpuInfoCache string

// gpuAvailable probes once whether an offscreen device can be created. The
// probe device holds a reference on the shared EGL display for its lifetime
// and releases (not terminates) it on Close, so probing can never disturb a
// device the caller already owns.
func gpuAvailable() bool {
	probeMu.Lock()
	defer probeMu.Unlock()
	if probeRun {
		return probeOK
	}
	probeRun = true
	d, err := newGPUDevice(8, 8)
	if err != nil {
		return false
	}
	gpuInfoCache = d.info
	probeOK = true
	_ = d.Close()
	return true
}

// gpuUsable reports whether d is a live GPU device. [DrawSceneDamage] uses
// it instead of GPUAvailable(): the availability probe creates and destroys
// its own EGL device, which must never be a side effect of replaying a
// scene onto a device the caller already owns.
func gpuUsable(d *GPUDevice) bool { return d != nil && !d.closed }

func gpuInfo() string {
	probeMu.Lock()
	run := probeRun
	probeMu.Unlock()
	if !run {
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

// newGPUDeviceAdopt binds to an EGL display/context the caller owns.
func newGPUDeviceAdopt(a EGLAdopt) (*GPUDevice, error) {
	if a.Display == 0 || a.Context == 0 {
		return nil, fmt.Errorf("%w: AdoptEGL needs a Display and a Context", ErrGPUUnavailable)
	}
	if a.Width < 1 {
		a.Width = 1
	}
	if a.Height < 1 {
		a.Height = 1
	}
	runtime.LockOSThread()
	d := &GPUDevice{
		w: a.Width, h: a.Height,
		window:    a.Draw != 0,
		ownEGL:    false,
		ownCtx:    false,
		thread:    C.pe_thread_self(),
		texCache:  make(map[uint64]*gpuTex),
		texBudget: defaultTexBudget,
		quad:      make([]float32, 16),
		tess:      newTessCache(),
	}
	d.dpy = C.pe_to_display(C.uintptr_t(a.Display))
	d.ctx = C.pe_to_context(C.uintptr_t(a.Context))
	if a.Draw != 0 {
		d.surf = C.pe_to_surface(C.uintptr_t(a.Draw))
	} else {
		d.surf = C.EGLSurface(C.EGL_NO_SURFACE)
	}
	ext := adoptDisplay(d.dpy)
	d.partial = strings.Contains(ext, "EGL_KHR_partial_update")
	d.bufferAge = d.partial || strings.Contains(ext, "EGL_EXT_buffer_age")
	C.pe_load_ext()
	if C.eglMakeCurrent(d.dpy, d.surf, d.surf, d.ctx) == C.EGL_FALSE {
		releaseDisplay(d.dpy)
		return nil, fmt.Errorf("%w: eglMakeCurrent adopted 0x%x", ErrGPUUnavailable, C.eglGetError())
	}
	curDev.Store(d)
	if err := d.initGL(); err != nil {
		releaseDisplay(d.dpy)
		return nil, err
	}
	return d, nil
}

func initGPU(n EGLNative, window bool) (*GPUDevice, error) {
	runtime.LockOSThread()
	d := &GPUDevice{
		w:         n.Width,
		h:         n.Height,
		window:    window,
		ownEGL:    true,
		ownCtx:    true,
		thread:    C.pe_thread_self(),
		texCache:  make(map[uint64]*gpuTex),
		texBudget: defaultTexBudget,
		quad:      make([]float32, 16),
		tess:      newTessCache(),
	}
	if d.w < 1 {
		d.w = 1
	}
	if d.h < 1 {
		d.h = 1
	}

	plat := C.EGLenum(n.Platform)
	if !window && n.Platform == 0 {
		plat = C.EGLenum(EGLPlatformSurfaceless)
	}
	dpy, ext, err := acquireDisplay(n.Display, plat)
	if err != nil {
		return nil, err
	}
	d.dpy = dpy
	C.pe_load_ext()
	d.partial = strings.Contains(ext, "EGL_KHR_partial_update")
	d.bufferAge = d.partial || strings.Contains(ext, "EGL_EXT_buffer_age")
	C.eglBindAPI(C.EGL_OPENGL_ES_API)

	// Window configs are opaque (ALPHA 0) unless the caller asks for an
	// ARGB visual — a compositor painting translucent chrome needs the
	// alpha channel, a plain app window must not present see-through.
	alphaBits := C.EGLint(0)
	if n.Alpha {
		alphaBits = 8
	}
	type cfgTry struct{ attr []C.EGLint }
	var tries []cfgTry
	if window {
		preserved := C.EGLint(C.EGL_WINDOW_BIT | C.EGL_SWAP_BEHAVIOR_PRESERVED_BIT)
		tries = []cfgTry{
			{[]C.EGLint{C.EGL_RENDERABLE_TYPE, C.EGL_OPENGL_ES2_BIT, C.EGL_SURFACE_TYPE, preserved, C.EGL_RED_SIZE, 8, C.EGL_GREEN_SIZE, 8, C.EGL_BLUE_SIZE, 8, C.EGL_ALPHA_SIZE, alphaBits, C.EGL_STENCIL_SIZE, 8, C.EGL_NONE}},
			{[]C.EGLint{C.EGL_RENDERABLE_TYPE, C.EGL_OPENGL_ES2_BIT, C.EGL_SURFACE_TYPE, C.EGL_WINDOW_BIT, C.EGL_RED_SIZE, 8, C.EGL_GREEN_SIZE, 8, C.EGL_BLUE_SIZE, 8, C.EGL_ALPHA_SIZE, alphaBits, C.EGL_STENCIL_SIZE, 8, C.EGL_NONE}},
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
		e := fmt.Errorf("%w: eglChooseConfig 0x%x", ErrGPUUnavailable, C.eglGetError())
		d.destroyEGL()
		return nil, e
	}
	ctxAttr := []C.EGLint{C.EGL_CONTEXT_CLIENT_VERSION, 2, C.EGL_NONE}
	share := C.EGLContext(C.EGL_NO_CONTEXT)
	if n.Share != 0 {
		share = C.pe_to_context(C.uintptr_t(n.Share))
	}
	d.ctx = C.eglCreateContext(d.dpy, d.cfg, share, &ctxAttr[0])
	if d.ctx == C.EGLContext(C.EGL_NO_CONTEXT) {
		e := fmt.Errorf("%w: eglCreateContext 0x%x", ErrGPUUnavailable, C.eglGetError())
		d.destroyEGL()
		return nil, e
	}

	if window && n.Window != 0 {
		d.surf = C.pe_window_surface(d.dpy, d.cfg, C.uintptr_t(n.Window))
		if d.surf == C.EGLSurface(C.EGL_NO_SURFACE) {
			e := fmt.Errorf("%w: eglCreateWindowSurface 0x%x", ErrGPUUnavailable, C.eglGetError())
			d.destroyEGL()
			return nil, e
		}
		if C.eglMakeCurrent(d.dpy, d.surf, d.surf, d.ctx) == C.EGL_FALSE {
			e := fmt.Errorf("%w: eglMakeCurrent window 0x%x", ErrGPUUnavailable, C.eglGetError())
			d.destroyEGL()
			return nil, e
		}
		C.eglSurfaceAttrib(d.dpy, d.surf, C.EGL_SWAP_BEHAVIOR, C.EGL_BUFFER_PRESERVED)
		var beh C.EGLint
		if C.eglQuerySurface(d.dpy, d.surf, C.EGL_SWAP_BEHAVIOR, &beh) != C.EGL_FALSE {
			d.preserve = beh == C.EGL_BUFFER_PRESERVED
		}
	} else {
		pb := []C.EGLint{C.EGL_WIDTH, C.EGLint(d.w), C.EGL_HEIGHT, C.EGLint(d.h), C.EGL_NONE}
		d.surf = C.eglCreatePbufferSurface(d.dpy, d.cfg, &pb[0])
		if d.surf == C.EGLSurface(C.EGL_NO_SURFACE) {
			if C.eglMakeCurrent(d.dpy, C.EGLSurface(C.EGL_NO_SURFACE), C.EGLSurface(C.EGL_NO_SURFACE), d.ctx) == C.EGL_FALSE {
				e := fmt.Errorf("%w: eglMakeCurrent surfaceless 0x%x", ErrGPUUnavailable, C.eglGetError())
				d.destroyEGL()
				return nil, e
			}
		} else if C.eglMakeCurrent(d.dpy, d.surf, d.surf, d.ctx) == C.EGL_FALSE {
			e := fmt.Errorf("%w: eglMakeCurrent pbuffer 0x%x", ErrGPUUnavailable, C.eglGetError())
			d.destroyEGL()
			return nil, e
		}
	}
	curDev.Store(d)
	if err := d.initGL(); err != nil {
		d.destroyEGL()
		return nil, err
	}
	return d, nil
}

// initGL builds the program, buffers and render target on the current context.
func (d *GPUDevice) initGL() error {
	if rend := C.glGetString(C.GL_RENDERER); rend != nil {
		d.info = C.GoString((*C.char)(unsafe.Pointer(rend)))
	}
	if C.pe_compile(&d.prog) == 0 {
		return fmt.Errorf("%w: shader compile/link", ErrGPUUnavailable)
	}
	d.bindLocs()
	C.glGenBuffers(1, &d.vbo)
	C.glGenTextures(1, &d.rampTex)
	C.glGenTextures(1, &d.maskTex)
	C.glPixelStorei(C.GL_UNPACK_ALIGNMENT, 1)
	C.glPixelStorei(C.GL_PACK_ALIGNMENT, 1)
	if MSAAEnabled() && C.pe_load_msaa() != 0 {
		if n := int(C.pe_max_samples()); n >= 2 {
			d.msSamples = n
			if d.msSamples > 4 {
				d.msSamples = 4
			}
			d.msaa = true
		}
	}
	if err := d.allocTarget(); err != nil {
		d.destroyGL()
		return err
	}
	C.glDisable(C.GL_DEPTH_TEST)
	C.glEnable(C.GL_BLEND)
	d.blend = BlendSrcOver
	C.glBlendFunc(C.GL_ONE, C.GL_ONE_MINUS_SRC_ALPHA)
	C.glEnable(C.GL_STENCIL_TEST)
	d.Clear(Transparent)
	return nil
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
	d.locCov = loc(p, "u_cov")
	d.locPatO = loc(p, "u_patOrigin")
	d.locPatS = loc(p, "u_patSize")
	d.locTexA8 = loc(p, "u_texA8")
}

func (d *GPUDevice) allocTarget() error {
	d.freeTarget()
	d.resetDamageHistory()
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
	if d.msaa && !d.allocMSAA() {
		// Fall back to single-sample rendering; rect fills keep their
		// analytic fringe, paths lose MSAA.
		d.freeMSAA()
		d.msaa = false
	}
	C.glBindFramebuffer(C.GL_FRAMEBUFFER, d.drawFBO())
	C.glViewport(0, 0, C.GLsizei(d.w), C.GLsizei(d.h))
	d.pixels = nil // made at the new size by the next Snapshot
	d.readDirty = true
	d.msDirty = false
	return nil
}

// allocMSAA builds the multisampled draw target resolved into d.fbo.
func (d *GPUDevice) allocMSAA() bool {
	C.glGenFramebuffers(1, &d.msFBO)
	C.glGenRenderbuffers(1, &d.msColor)
	C.glBindRenderbuffer(C.GL_RENDERBUFFER, d.msColor)
	C.pe_rb_multisample(C.GLsizei(d.msSamples), C.PE_GL_RGBA8, C.GLsizei(d.w), C.GLsizei(d.h))
	C.glGenRenderbuffers(1, &d.msStencil)
	C.glBindRenderbuffer(C.GL_RENDERBUFFER, d.msStencil)
	C.pe_rb_multisample(C.GLsizei(d.msSamples), C.GL_STENCIL_INDEX8, C.GLsizei(d.w), C.GLsizei(d.h))
	C.glBindFramebuffer(C.GL_FRAMEBUFFER, d.msFBO)
	C.glFramebufferRenderbuffer(C.GL_FRAMEBUFFER, C.GL_COLOR_ATTACHMENT0, C.GL_RENDERBUFFER, d.msColor)
	C.glFramebufferRenderbuffer(C.GL_FRAMEBUFFER, C.GL_STENCIL_ATTACHMENT, C.GL_RENDERBUFFER, d.msStencil)
	return C.glCheckFramebufferStatus(C.GL_FRAMEBUFFER) == C.GL_FRAMEBUFFER_COMPLETE
}

func (d *GPUDevice) freeMSAA() {
	if d.msFBO != 0 {
		C.glDeleteFramebuffers(1, &d.msFBO)
		d.msFBO = 0
	}
	if d.msColor != 0 {
		C.glDeleteRenderbuffers(1, &d.msColor)
		d.msColor = 0
	}
	if d.msStencil != 0 {
		C.glDeleteRenderbuffers(1, &d.msStencil)
		d.msStencil = 0
	}
}

// drawFBO is the framebuffer draws go to (multisampled when available).
func (d *GPUDevice) drawFBO() C.GLuint {
	if d.msaa && d.msFBO != 0 {
		return d.msFBO
	}
	return d.fbo
}

// resolve flushes multisample results into the single-sample color texture
// that read-back and window presentation sample.
func (d *GPUDevice) resolve() {
	if !d.msaa || d.msFBO == 0 || !d.msDirty {
		return
	}
	C.pe_resolve(d.msFBO, d.fbo, C.GLsizei(d.w), C.GLsizei(d.h))
	C.glBindFramebuffer(C.GL_FRAMEBUFFER, d.drawFBO())
	d.msDirty = false
}

// Antialiased reports whether path fills get multisample coverage. Axis
// aligned rectangle fills are analytically anti-aliased either way.
func (d *GPUDevice) Antialiased() bool { return d != nil && d.msaa }

// Samples is the multisample count of the render target (1 when MSAA is
// unavailable).
func (d *GPUDevice) Samples() int {
	if d == nil || !d.msaa {
		return 1
	}
	return d.msSamples
}

func (d *GPUDevice) freeTarget() {
	d.freeMSAA()
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
	d.freeBlur()
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
	if d.scrollTex != 0 {
		C.glDeleteTextures(1, &d.scrollTex)
		d.scrollTex = 0
		d.scrollTW, d.scrollTH = 0, 0
	}
	for k := range d.texCache {
		d.dropTex(k)
	}
	d.texBytes = 0
}

// destroyEGL releases this device's EGL objects. The display is reference
// counted: eglTerminate tears down every context and surface on it, so it
// runs only when this device held the last reference.
func (d *GPUDevice) destroyEGL() {
	if d.dpy == C.EGLDisplay(C.EGL_NO_DISPLAY) {
		return
	}
	if curDev.Load() == d {
		curDev.Store(nil)
	}
	if d.ownCtx {
		C.eglMakeCurrent(d.dpy, C.EGLSurface(C.EGL_NO_SURFACE), C.EGLSurface(C.EGL_NO_SURFACE), C.EGLContext(C.EGL_NO_CONTEXT))
		if d.ctx != C.EGLContext(C.EGL_NO_CONTEXT) {
			C.eglDestroyContext(d.dpy, d.ctx)
			d.ctx = C.EGLContext(C.EGL_NO_CONTEXT)
		}
		if d.surf != nil && d.surf != C.EGLSurface(C.EGL_NO_SURFACE) {
			C.eglDestroySurface(d.dpy, d.surf)
			d.surf = C.EGLSurface(C.EGL_NO_SURFACE)
		}
	}
	releaseDisplay(d.dpy)
	d.dpy = C.EGLDisplay(C.EGL_NO_DISPLAY)
}

// MakeCurrent binds the device's EGL context to the calling OS thread.
// Every draw call does this implicitly; call it directly only when host code
// touched the GL state.
//
// A GPUDevice is bound to the thread it was created on (EGL contexts are).
// Calls from another thread fail instead of drawing into nothing.
func (d *GPUDevice) MakeCurrent() error {
	err := d.makeCurrentForce()
	if err != nil {
		d.setErr(err)
	}
	return err
}

// makeCurrent is the hot-path binding: inside a BeginFrame/EndFrame pair,
// and while this device is still the current one, it is a no-op.
func (d *GPUDevice) makeCurrent() error {
	if d == nil || d.closed {
		return ErrGPUUnavailable
	}
	if d.frameDepth > 0 && curDev.Load() == d {
		return nil
	}
	return d.MakeCurrent()
}

func (d *GPUDevice) makeCurrentForce() error {
	if d == nil || d.closed {
		return ErrGPUUnavailable
	}
	if t := C.pe_thread_self(); t != d.thread {
		return fmt.Errorf("%w: GPUDevice used from another OS thread (created on %#x, called on %#x); "+
			"an EGL context is thread-bound — keep the device on one goroutine with runtime.LockOSThread",
			ErrGPUUnavailable, uint64(d.thread), uint64(t))
	}
	runtime.LockOSThread()
	draw := d.surf
	if draw == nil || draw == C.EGLSurface(C.EGL_NO_SURFACE) {
		draw = C.EGLSurface(C.EGL_NO_SURFACE)
	}
	if C.eglMakeCurrent(d.dpy, draw, draw, d.ctx) == C.EGL_FALSE {
		return fmt.Errorf("%w: eglMakeCurrent 0x%x", ErrGPUUnavailable, C.eglGetError())
	}
	curDev.Store(d)
	C.glBindFramebuffer(C.GL_FRAMEBUFFER, d.drawFBO())
	C.glViewport(0, 0, C.GLsizei(d.w), C.GLsizei(d.h))
	return nil
}

// BeginFrame makes the context current once for a batch of draws. Draw calls
// between BeginFrame and [GPUDevice.EndFrame] skip the per-call
// eglMakeCurrent round-trip. Pairs may nest; an unpaired draw still works.
func (d *GPUDevice) BeginFrame() error {
	if d == nil || d.closed {
		return ErrGPUUnavailable
	}
	if err := d.MakeCurrent(); err != nil {
		return err
	}
	d.frameDepth++
	return nil
}

// EndFrame closes a [GPUDevice.BeginFrame] and returns the first error
// recorded during the frame (also available from [GPUDevice.Err]).
func (d *GPUDevice) EndFrame() error {
	if d == nil {
		return ErrGPUUnavailable
	}
	if d.frameDepth > 0 {
		d.frameDepth--
	}
	if d.frameDepth == 0 {
		d.checkGL("frame")
	}
	return d.err
}

// Err returns the first error the device recorded since [GPUDevice.ClearErr]
// (a lost context, a thread misuse, a failed swap, a GL error). Draw calls
// cannot return errors — the [Device] interface has no error results — so
// they record here instead of failing silently.
func (d *GPUDevice) Err() error {
	if d == nil {
		return ErrGPUUnavailable
	}
	return d.err
}

// ClearErr forgets the recorded error.
func (d *GPUDevice) ClearErr() {
	if d != nil {
		d.err = nil
	}
}

func (d *GPUDevice) setErr(err error) {
	if d != nil && err != nil && d.err == nil {
		d.err = err
	}
}

// checkGL records a pending GL error, if any.
func (d *GPUDevice) checkGL(op string) {
	if d == nil || d.err != nil {
		return
	}
	if e := C.glGetError(); e != C.GL_NO_ERROR {
		d.err = fmt.Errorf("paintengine2d: GL error 0x%x during %s", uint32(e), op)
	}
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
	d.presentSet = false
	d.presentSkip = false
	d.presentRects = d.presentRects[:0]
	return d.allocTarget()
}

// Close releases the device's GL objects and drops its reference on the EGL
// display. Other devices sharing the display keep working.
func (d *GPUDevice) Close() error {
	if d == nil || d.closed {
		return nil
	}
	if err := d.makeCurrentForce(); err == nil {
		d.destroyGL()
	}
	d.destroyEGL()
	d.frameDepth = 0
	d.closed = true
	return nil
}

func (d *GPUDevice) Clear(c Color) {
	if d == nil || d.closed {
		return
	}
	if err := d.makeCurrent(); err != nil {
		return
	}
	r, g, b, a := c.Premul8()
	C.glDisable(C.GL_SCISSOR_TEST)
	C.glColorMask(C.GL_TRUE, C.GL_TRUE, C.GL_TRUE, C.GL_TRUE)
	C.glClearColor(C.GLfloat(r)/255, C.GLfloat(g)/255, C.GLfloat(b)/255, C.GLfloat(a)/255)
	C.glClearStencil(0)
	C.glClear(C.GL_COLOR_BUFFER_BIT | C.GL_STENCIL_BUFFER_BIT)
	d.markDrawn()
}

// ClearRect overwrites the device-space box r (scissored glClear).
func (d *GPUDevice) ClearRect(r Rect, c Color) {
	if d == nil || d.closed {
		return
	}
	if err := d.makeCurrent(); err != nil {
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
	d.markDrawn()
}

func (d *GPUDevice) Fill(path *Path, xform Matrix, paint Paint, clip Clip) {
	if path == nil || path.Empty() || d.w == 0 || d.h == 0 || !xform.Finite() {
		return
	}
	if err := d.makeCurrent(); err != nil {
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
	if err := d.makeCurrent(); err != nil {
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
	if err := d.makeCurrent(); err != nil {
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
	if src.Format == FormatA8 {
		C.glUniform1i(d.locTexA8, 1)
	}
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
	d.markDrawn()
}

func (d *GPUDevice) Image() *Image { return d.Snapshot() }
func (d *GPUDevice) Snapshot() *Image {
	if d == nil || d.closed {
		return nil
	}
	if !d.readDirty && d.pixels != nil {
		return d.pixels
	}
	if err := d.makeCurrent(); err != nil {
		return d.pixels
	}
	if d.pixels == nil || d.pixels.Width != d.w || d.pixels.Height != d.h {
		d.pixels = NewImage(d.w, d.h)
	}
	d.resolve()
	C.glBindFramebuffer(C.GL_FRAMEBUFFER, d.fbo)
	C.glReadPixels(0, 0, C.GLsizei(d.w), C.GLsizei(d.h), C.GL_RGBA, C.GL_UNSIGNED_BYTE, unsafe.Pointer(&d.pixels.Pix[0]))
	C.glBindFramebuffer(C.GL_FRAMEBUFFER, d.drawFBO())
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

// SnapshotRect reads back a device-space box (full snapshot if r is empty).
func (d *GPUDevice) SnapshotRect(r Rect) *Image {
	img := d.Snapshot()
	if img == nil {
		return nil
	}
	r = r.Canon()
	if r.Empty() {
		return img
	}
	x0, y0, x1, y1 := clampPixelBounds(r, d.w, d.h)
	if x0 >= x1 || y0 >= y1 {
		return NewImage(0, 0)
	}
	return img.SubImage(x0, y0, x1, y1)
}

// Scroll copies the FBO region r by (dx, dy) via a scratch texture.
func (d *GPUDevice) Scroll(dx, dy int, r Rect) {
	if d == nil || d.closed || (dx == 0 && dy == 0) {
		return
	}
	if err := d.makeCurrent(); err != nil {
		return
	}
	x0, y0, x1, y1 := clampPixelBounds(r, d.w, d.h)
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
	sw, sh := dstX1-dstX0, dstY1-dstY0
	srcX0 := dstX0 - dx
	srcY0 := dstY0 - dy
	if !d.ensureScrollTex(sw, sh) {
		return
	}
	d.resolve()
	C.glBindFramebuffer(C.GL_FRAMEBUFFER, d.fbo)
	C.glBindTexture(C.GL_TEXTURE_2D, d.scrollTex)
	// FBO is GL bottom-left; our Y is top-down.
	C.glCopyTexSubImage2D(C.GL_TEXTURE_2D, 0, 0, 0, C.GLint(srcX0), C.GLint(d.h-srcY0-sh), C.GLsizei(sw), C.GLsizei(sh))
	C.glBindFramebuffer(C.GL_FRAMEBUFFER, d.drawFBO())
	C.glDisable(C.GL_STENCIL_TEST)
	C.glDisable(C.GL_BLEND)
	C.glEnable(C.GL_SCISSOR_TEST)
	C.glScissor(C.GLint(dstX0), C.GLint(d.h-dstY1), C.GLsizei(sw), C.GLsizei(sh))
	C.glUseProgram(d.prog)
	C.glUniform2f(d.locVP, C.GLfloat(d.w), C.GLfloat(d.h))
	C.glUniform1i(d.locMode, 3)
	C.glUniform1i(d.locUseM, 0)
	C.glUniform1i(d.locCov, 0)
	C.glUniform1i(d.locTexA8, 0)
	C.glUniform4f(d.locTint, 1, 1, 1, 1)
	C.glActiveTexture(C.GL_TEXTURE0)
	C.glBindTexture(C.GL_TEXTURE_2D, d.scrollTex)
	C.glUniform1i(d.locTex, 0)
	// Copied tex is GL-oriented (Y-up). Draw with v flipped relative to dest.
	u1 := float32(sw) / float32(d.scrollTW)
	v1 := float32(sh) / float32(d.scrollTH)
	d.quad = append(d.quad[:0],
		float32(dstX0), float32(dstY0), 0, v1,
		float32(dstX1), float32(dstY0), u1, v1,
		float32(dstX1), float32(dstY1), u1, 0,
		float32(dstX0), float32(dstY0), 0, v1,
		float32(dstX1), float32(dstY1), u1, 0,
		float32(dstX0), float32(dstY1), 0, 0,
	)
	C.glColorMask(C.GL_TRUE, C.GL_TRUE, C.GL_TRUE, C.GL_TRUE)
	d.drawTris(d.quad)
	C.glEnable(C.GL_BLEND)
	C.glEnable(C.GL_STENCIL_TEST)
	C.glDisable(C.GL_SCISSOR_TEST)
	d.markDrawn()
}

func (d *GPUDevice) ensureScrollTex(w, h int) bool {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	if d.scrollTex != 0 && d.scrollTW >= w && d.scrollTH >= h {
		return true
	}
	if d.scrollTex != 0 {
		C.glDeleteTextures(1, &d.scrollTex)
		d.scrollTex = 0
	}
	C.glGenTextures(1, &d.scrollTex)
	C.glBindTexture(C.GL_TEXTURE_2D, d.scrollTex)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_MIN_FILTER, C.GL_NEAREST)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_MAG_FILTER, C.GL_NEAREST)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_WRAP_S, C.GL_CLAMP_TO_EDGE)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_WRAP_T, C.GL_CLAMP_TO_EDGE)
	C.glTexImage2D(C.GL_TEXTURE_2D, 0, C.GL_RGBA, C.GLsizei(w), C.GLsizei(h), 0, C.GL_RGBA, C.GL_UNSIGNED_BYTE, nil)
	d.scrollTW, d.scrollTH = w, h
	return true
}

func (d *GPUDevice) Present() error {
	if d == nil || d.closed {
		return ErrGPUUnavailable
	}
	if d.presentSkip {
		d.presentSkip = false
		d.presentSet = false
		return nil
	}
	if d.presentSet {
		rects := d.presentRects
		d.presentSet = false
		return d.PresentRects(rects)
	}
	return d.PresentRects(nil)
}

// SetPresentDamage stores rects for the next [GPUDevice.Present].
// nil means a full-surface present. A non-nil empty list skips the swap
// ([DrawSceneDamage] with an empty dirty tracker).
func (d *GPUDevice) SetPresentDamage(rects []Rect) {
	if d == nil {
		return
	}
	d.presentSet = true
	d.presentSkip = rects != nil && len(rects) == 0
	d.presentRects = d.presentRects[:0]
	if len(rects) > 0 {
		d.presentRects = append(d.presentRects, rects...)
	}
}

// EGLHandles exposes the device's EGL display, context and draw surface as
// opaque integers (0 when there is none). A host that must interleave its
// own GL work — or create a second device sharing this context — needs them.
func (d *GPUDevice) EGLHandles() (display, context, draw uintptr) {
	if d == nil || d.closed {
		return 0, 0, 0
	}
	display = uintptr(C.pe_from_display(d.dpy))
	context = uintptr(C.pe_from_context(d.ctx))
	if d.surf != C.EGLSurface(C.EGL_NO_SURFACE) {
		draw = uintptr(C.pe_from_surface(d.surf))
	}
	return display, context, draw
}

// PartialUpdate reports EGL_KHR_partial_update (eglSetDamageRegionKHR).
func (d *GPUDevice) PartialUpdate() bool { return d != nil && d.partial }

// SwapPreserves reports that the window surface kept EGL_BUFFER_PRESERVED.
func (d *GPUDevice) SwapPreserves() bool { return d != nil && d.preserve }

// PresentRects blits the FBO to the window and swaps. A nil/empty list is a
// full-surface present. Non-empty rects are passed to
// eglSwapBuffersWithDamageKHR/EXT when available so the compositor can
// skip clean tiles (KDE/Qt partial update).
//
// The FBO→window blit is scissored to dirty boxes when either the
// surface preserved the back buffer (EGL_BUFFER_PRESERVED) or
// EGL_KHR_partial_update is present and buffer age is > 0. Otherwise
// the blit is full (undefined back buffer) but the swap still carries
// the damage hint.
func (d *GPUDevice) PresentRects(rects []Rect) error {
	if d == nil || d.closed {
		return ErrGPUUnavailable
	}
	d.presentSet = false
	d.presentSkip = false
	if err := d.MakeCurrent(); err != nil {
		return err
	}
	d.resolve()
	if !d.window || d.surf == nil || d.surf == C.EGLSurface(C.EGL_NO_SURFACE) {
		C.glFlush()
		d.recordFrameDamage(rects)
		return nil
	}
	// A partial blit is only sound when we know what the back buffer holds.
	// EGL_BUFFER_PRESERVED means it is the previous frame. With
	// EGL_KHR_partial_update it is the frame from `age` swaps ago, so the
	// blit must also repaint the damage of the intervening frames.
	blitRects := rects
	partialBlit := len(rects) > 0 && (d.preserve || d.partial)
	if partialBlit && !d.preserve {
		age := int(C.pe_buffer_age(d.dpy, d.surf))
		switch {
		case age <= 0 || age > maxBufferAge:
			// Unknown or older than our history: repaint everything.
			partialBlit = false
		case age > 1:
			var full bool
			if blitRects, full = d.damageForAge(rects, age); full {
				// The back buffer predates a full frame: nothing short of
				// a full blit brings it up to date.
				partialBlit = false
			}
		}
	}
	n := d.packEGLDamage(blitRects)
	if n == 0 {
		partialBlit = false
	}
	if partialBlit && d.partial {
		C.pe_set_damage_region(d.dpy, d.surf, &d.eglDamage[0], C.EGLint(n/4))
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
	C.glUniform1i(d.locCov, 0)
	C.glUniform1i(d.locTexA8, 0)
	C.glUniform4f(d.locTint, 1, 1, 1, 1)
	C.glActiveTexture(C.GL_TEXTURE0)
	C.glBindTexture(C.GL_TEXTURE_2D, d.color)
	C.glUniform1i(d.locTex, 0)
	if partialBlit {
		d.blitDamageRects(blitRects)
	} else {
		d.quad = append(d.quad[:0],
			0, 0, 0, 1,
			float32(d.w), 0, 1, 1,
			float32(d.w), float32(d.h), 1, 0,
			0, 0, 0, 1,
			float32(d.w), float32(d.h), 1, 0,
			0, float32(d.h), 0, 0,
		)
		d.drawTris(d.quad)
	}
	C.glEnable(C.GL_BLEND)
	d.blend = BlendSrcOver
	C.glBlendFunc(C.GL_ONE, C.GL_ONE_MINUS_SRC_ALPHA)
	C.glEnable(C.GL_STENCIL_TEST)
	// The swap hint always carries this frame's own damage.
	n = d.packEGLDamage(rects)
	swapped := d.swapPacked(n)
	d.recordFrameDamage(rects)
	C.glBindFramebuffer(C.GL_FRAMEBUFFER, d.drawFBO())
	d.msDirty = false
	if !swapped {
		err := fmt.Errorf("paintengine2d: eglSwapBuffers 0x%x", C.eglGetError())
		d.setErr(err)
		return err
	}
	return nil
}

// recordFrameDamage pushes this frame's damage onto the age ring. A nil or
// empty list is a full-surface present and is remembered as such — it used
// to be stored as "no damage", so the next partial present into an older
// back buffer repaired only its own rects and the rest of the window showed
// the frame from before the full repaint (menus vanishing, closed dialogs
// reappearing).
func (d *GPUDevice) recordFrameDamage(rects []Rect) {
	d.ageIdx = (d.ageIdx + 1) % maxBufferAge
	slot := d.ageRing[d.ageIdx][:0]
	slot = append(slot, rects...)
	d.ageRing[d.ageIdx] = slot
	d.ageFull[d.ageIdx] = len(rects) == 0 || d.ageReset
	d.ageReset = false
}

// resetDamageHistory forgets every recorded frame: after a resize or target
// reallocation no back buffer can be repaired from the ring.
func (d *GPUDevice) resetDamageHistory() {
	for i := range d.ageRing {
		d.ageRing[i] = d.ageRing[i][:0]
		d.ageFull[i] = true
	}
	// The first frame on a fresh target is a repaint of everything, whatever
	// rects the caller passed.
	d.ageReset = true
}

// damageForAge unions this frame's damage with the damage of the age-1
// previous frames, which the back buffer has not seen. full reports that
// one of those frames repainted everything, so only a full blit is sound.
func (d *GPUDevice) damageForAge(rects []Rect, age int) (out []Rect, full bool) {
	out = d.ageScrat[:0]
	out = append(out, rects...)
	for i := 1; i < age && i < maxBufferAge; i++ {
		idx := ((d.ageIdx-i+1)%maxBufferAge + maxBufferAge) % maxBufferAge
		if d.ageFull[idx] {
			d.ageScrat = out
			return nil, true
		}
		out = append(out, d.ageRing[idx]...)
	}
	d.ageScrat = out
	return out, false
}

// PresentDamageAge reports the buffer age EGL last returned for the window
// surface (0 when unknown or unsupported). A compositor can use it to decide
// how much history it must repaint itself.
func (d *GPUDevice) PresentDamageAge() int {
	if d == nil || d.closed || !d.window || d.surf == C.EGLSurface(C.EGL_NO_SURFACE) {
		return 0
	}
	if err := d.makeCurrent(); err != nil {
		return 0
	}
	return int(C.pe_buffer_age(d.dpy, d.surf))
}

func (d *GPUDevice) blitDamageRects(rects []Rect) {
	fw, fh := float32(d.w), float32(d.h)
	for _, r := range rects {
		x0, y0, x1, y1 := clampPixelBounds(r, d.w, d.h)
		if x0 >= x1 || y0 >= y1 {
			continue
		}
		C.glEnable(C.GL_SCISSOR_TEST)
		C.glScissor(C.GLint(x0), C.GLint(d.h-y1), C.GLsizei(x1-x0), C.GLsizei(y1-y0))
		u0, u1 := float32(x0)/fw, float32(x1)/fw
		v0, v1 := 1-float32(y0)/fh, 1-float32(y1)/fh
		d.quad = append(d.quad[:0],
			float32(x0), float32(y0), u0, v0,
			float32(x1), float32(y0), u1, v0,
			float32(x1), float32(y1), u1, v1,
			float32(x0), float32(y0), u0, v0,
			float32(x1), float32(y1), u1, v1,
			float32(x0), float32(y1), u0, v1,
		)
		d.drawTris(d.quad)
	}
	C.glDisable(C.GL_SCISSOR_TEST)
}

func (d *GPUDevice) packEGLDamage(rects []Rect) int {
	if len(rects) == 0 {
		return 0
	}
	need := len(rects) * 4
	if cap(d.eglDamage) < need {
		d.eglDamage = make([]C.EGLint, need)
	} else {
		d.eglDamage = d.eglDamage[:need]
	}
	n := 0
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
	d.eglDamage = d.eglDamage[:n]
	return n
}

func (d *GPUDevice) swapPacked(n int) bool {
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
	if paint.isOpaqueSolid() && clip.Mask == nil {
		d.fillOpaqueRects([]Rect{r}, paint.effectiveColor(), clip)
		return true
	}
	if !d.applyClip(clip, r) {
		return true
	}
	C.glDisable(C.GL_STENCIL_TEST)
	C.glColorMask(C.GL_TRUE, C.GL_TRUE, C.GL_TRUE, C.GL_TRUE)
	C.glStencilFunc(C.GL_ALWAYS, 0, 0xFF)
	C.glStencilOp(C.GL_KEEP, C.GL_KEEP, C.GL_KEEP)
	d.bindProgram(d.shaderMode(paint), paint, xform, clip)
	if isPixelAligned(r) {
		d.coverQuad(r)
	} else {
		// Analytic coverage on the fringe pixels.
		C.glEnable(C.GL_BLEND)
		C.glUniform1i(d.locCov, 1)
		d.verts = appendRectAA(d.verts[:0], r)
		d.drawTris(d.verts)
		C.glUniform1i(d.locCov, 0)
	}
	C.glEnable(C.GL_BLEND)
	C.glEnable(C.GL_STENCIL_TEST)
	C.glDisable(C.GL_SCISSOR_TEST)
	d.markDrawn()
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
	C.glUniform1i(d.locCov, 0)
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
	d.markDrawn()
}

func (d *GPUDevice) shaderMode(paint Paint) int {
	switch paint.Shader.(type) {
	case LinearGradient:
		return 1
	case RadialGradient:
		return 2
	case ImagePattern:
		return 4
	default:
		return 0
	}
}

// setBlend puts the operator on the blend unit: src-over (ONE,
// 1-SRC_ALPHA on premultiplied colour) or dest-out (ZERO, 1-SRC_ALPHA:
// the source erases what it covers).
func (d *GPUDevice) setBlend(mode BlendMode) {
	if mode != BlendDestOut {
		mode = BlendSrcOver
	}
	if d.blend == mode {
		return
	}
	d.blend = mode
	if mode == BlendDestOut {
		C.glBlendFunc(C.GL_ZERO, C.GL_ONE_MINUS_SRC_ALPHA)
		return
	}
	C.glBlendFunc(C.GL_ONE, C.GL_ONE_MINUS_SRC_ALPHA)
}

func (d *GPUDevice) bindProgram(mode int, paint Paint, xform Matrix, clip Clip) {
	d.setBlend(paint.Blend)
	C.glUseProgram(d.prog)
	C.glUniform2f(d.locVP, C.GLfloat(d.w), C.GLfloat(d.h))
	C.glUniform1i(d.locMode, C.GLint(mode))
	C.glUniform1i(d.locCov, 0)
	C.glUniform1i(d.locTexA8, 0)
	r, g, b, a := paint.effectiveColor().Premul8()
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
	// Gradients take the paint's layer alpha through the tint (solid colours
	// fold it into u_color; a blit sets its own tint after this).
	C.glUniform4f(d.locTint, 1, 1, 1, C.GLfloat(paint.LayerAlpha()))

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
	case ImagePattern:
		// The pattern tiles in the shader (mod), so a texture of any size
		// repeats under GLES2; nearest keeps dithers crisp.
		if g.Image != nil && g.Image.Width > 0 && g.Image.Height > 0 {
			tex := d.uploadImage(g.Image, FilterNearest)
			if g.Image.Format == FormatA8 {
				C.glUniform1i(d.locTexA8, 1)
			}
			s := g.scale()
			C.glUniform2f(d.locPatO, C.GLfloat(g.Origin.X), C.GLfloat(g.Origin.Y))
			C.glUniform2f(d.locPatS, C.GLfloat(float32(g.Image.Width)*s), C.GLfloat(float32(g.Image.Height)*s))
			C.glActiveTexture(C.GL_TEXTURE0)
			C.glBindTexture(C.GL_TEXTURE_2D, tex)
			C.glUniform1i(d.locTex, 0)
		}
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
	// Re-baking 256 samples and re-uploading the ramp on every draw showed
	// up in profiles; the stops rarely change between draws.
	h := hashStops(stops)
	if d.rampSet && d.rampHash == h {
		C.glBindTexture(C.GL_TEXTURE_2D, d.rampTex)
		return
	}
	d.rampHash, d.rampSet = h, true
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

// uploadMask uploads a clip coverage mask as a single-channel GL_ALPHA
// texture. It used to expand every mask to RGBA (4x the bytes) and upload it
// again for every op under the same clip; the hash skips the repeat.
func (d *GPUDevice) uploadMask(clip Clip) {
	need := clip.MaskW * clip.MaskH
	if need <= 0 || len(clip.Mask) < need {
		return
	}
	h := maskHash(clip)
	C.glBindTexture(C.GL_TEXTURE_2D, d.maskTex)
	if d.maskSet && d.maskHash == h {
		return
	}
	d.maskHash, d.maskSet = h, true
	if cap(d.maskPix) < need {
		d.maskPix = make([]byte, need)
	} else {
		d.maskPix = d.maskPix[:need]
	}
	copy(d.maskPix, clip.Mask[:need])
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_MIN_FILTER, C.GL_NEAREST)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_MAG_FILTER, C.GL_NEAREST)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_WRAP_S, C.GL_CLAMP_TO_EDGE)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_WRAP_T, C.GL_CLAMP_TO_EDGE)
	C.glTexImage2D(C.GL_TEXTURE_2D, 0, C.GL_ALPHA, C.GLsizei(clip.MaskW), C.GLsizei(clip.MaskH), 0, C.GL_ALPHA, C.GL_UNSIGNED_BYTE, unsafe.Pointer(&d.maskPix[0]))
}

// maskHash identifies a clip mask cheaply (size, origin and a sparse sample
// of the bytes — masks that differ in content differ in their samples).
func maskHash(clip Clip) uint64 {
	h := uint64(14695981039346656037)
	mix := func(v uint64) {
		h ^= v
		h *= 1099511628211
	}
	mix(uint64(clip.MaskW))
	mix(uint64(clip.MaskH))
	mix(uint64(uint32(clip.MaskX)))
	mix(uint64(uint32(clip.MaskY)))
	n := len(clip.Mask)
	mix(uint64(n))
	step := n/64 + 1
	for i := 0; i < n; i += step {
		mix(uint64(clip.Mask[i]))
	}
	return h
}

func hashStops(stops []GradientStop) uint64 {
	h := uint64(14695981039346656037)
	mix := func(v uint64) {
		h ^= v
		h *= 1099511628211
	}
	for _, s := range stops {
		mix(uint64(math.Float32bits(s.Offset)))
		mix(uint64(math.Float32bits(s.Color.R)))
		mix(uint64(math.Float32bits(s.Color.G)))
		mix(uint64(math.Float32bits(s.Color.B)))
		mix(uint64(math.Float32bits(s.Color.A)))
	}
	return h
}

// uploadImage returns a texture for src, uploading or patching it as
// needed. The cache is keyed on [Image.UID] — a pointer key could be
// recycled by the allocator and hand back another image's texture — and is
// bounded by a byte budget with least-recently-used eviction, so a
// compositor that blits a new pixmap every frame does not leak VRAM.
func (d *GPUDevice) uploadImage(src *Image, filter FilterMode) C.GLuint {
	key := src.UID()
	d.texClock++
	if e, ok := d.texCache[key]; ok {
		if e.w == src.Width && e.h == src.Height && e.nbytes == len(src.Pix) {
			e.used = d.texClock
			if e.epoch == src.Epoch {
				return e.id
			}
			if d.uploadDirty(src, e.id) {
				e.epoch = src.Epoch
				return e.id
			}
		}
		d.dropTex(key)
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
	bpp := src.BytesPerPixel()
	pix := src.Pix
	if src.RowStride() != src.Width*bpp {
		row := src.Width * bpp
		pack := make([]byte, row*src.Height)
		for y := 0; y < src.Height; y++ {
			copy(pack[y*row:(y+1)*row], src.Pix[y*src.RowStride():y*src.RowStride()+row])
		}
		pix = pack
	}
	if len(pix) == 0 {
		C.glDeleteTextures(1, &id)
		return 0
	}
	format := C.GLenum(C.GL_RGBA)
	if src.Format == FormatA8 {
		format = C.GL_ALPHA
	}
	C.glTexImage2D(C.GL_TEXTURE_2D, 0, C.GLint(format), C.GLsizei(src.Width), C.GLsizei(src.Height), 0, format, C.GL_UNSIGNED_BYTE, unsafe.Pointer(&pix[0]))
	bytes := src.Width * src.Height * bpp
	d.texCache[key] = &gpuTex{
		id: id, w: src.Width, h: src.Height,
		nbytes: len(src.Pix), bytes: bytes, epoch: src.Epoch, used: d.texClock,
	}
	d.texBytes += bytes
	d.evictTextures()
	return id
}

// dropTex deletes one cached texture.
func (d *GPUDevice) dropTex(key uint64) {
	e, ok := d.texCache[key]
	if !ok {
		return
	}
	id := e.id
	C.glDeleteTextures(1, &id)
	d.texBytes -= e.bytes
	if d.texBytes < 0 {
		d.texBytes = 0
	}
	delete(d.texCache, key)
}

// evictTextures drops least-recently-used entries until the cache fits.
func (d *GPUDevice) evictTextures() {
	budget := d.texBudget
	if budget <= 0 {
		budget = defaultTexBudget
	}
	for d.texBytes > budget && len(d.texCache) > 1 {
		var oldest uint64
		var oldKey uint64
		first := true
		for k, e := range d.texCache {
			if first || e.used < oldest {
				oldest, oldKey, first = e.used, k, false
			}
		}
		if first {
			return
		}
		d.dropTex(oldKey)
	}
}

// SetTextureBudget caps the bytes of cached image textures (default 64 MiB).
// A value <= 0 restores the default.
func (d *GPUDevice) SetTextureBudget(bytes int) {
	if d == nil {
		return
	}
	if bytes <= 0 {
		bytes = defaultTexBudget
	}
	d.texBudget = bytes
	if err := d.makeCurrent(); err != nil {
		return
	}
	d.evictTextures()
}

// TextureBytes is the size of the cached image textures.
func (d *GPUDevice) TextureBytes() int {
	if d == nil {
		return 0
	}
	return d.texBytes
}

// ReleaseImage drops any GPU texture cached for img. Call it when a pixmap
// (a window snapshot, a video frame, a discarded layer) will not be drawn
// again; otherwise the texture lives until the budget evicts it.
func (d *GPUDevice) ReleaseImage(img *Image) {
	if d == nil || d.closed || img == nil || d.texCache == nil {
		return
	}
	key := img.UID()
	if _, ok := d.texCache[key]; !ok {
		return
	}
	if err := d.makeCurrent(); err != nil {
		return
	}
	d.dropTex(key)
}

func (d *GPUDevice) uploadDirty(src *Image, id C.GLuint) bool {
	r := src.Dirty
	if r.Empty() {
		r = XYWH(0, 0, float32(src.Width), float32(src.Height))
	}
	x0, y0, x1, y1 := clampPixelBounds(r, src.Width, src.Height)
	if x0 >= x1 || y0 >= y1 {
		return true
	}
	bpp := src.BytesPerPixel()
	// Full re-upload when the dirty box is most of the atlas.
	if (x1-x0)*(y1-y0)*bpp > len(src.Pix)*3/4 {
		return false
	}
	format := C.GLenum(C.GL_RGBA)
	if src.Format == FormatA8 {
		format = C.GL_ALPHA
	}
	C.glBindTexture(C.GL_TEXTURE_2D, id)
	stride := src.RowStride()
	w := x1 - x0
	// GLES2 has no UNPACK_ROW_LENGTH — one row at a time.
	for y := y0; y < y1; y++ {
		i := y*stride + x0*bpp
		C.glTexSubImage2D(C.GL_TEXTURE_2D, 0, C.GLint(x0), C.GLint(y), C.GLsizei(w), 1, format, C.GL_UNSIGNED_BYTE, unsafe.Pointer(&src.Pix[i]))
	}
	return true
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

// appendRectAA appends triangles covering r with per-vertex coverage in the
// uv.x channel. GLES2 has no analytic coverage, and a raw quad leaves a
// hard, aliased edge wherever a rect is not pixel aligned — a 0.5 px offset
// that the CPU rasterizer renders as a half-covered pixel.
//
// The rect is split into up to 3x3 bands snapped to whole pixels, each
// carrying its exact area coverage, so pixel-center sampling reproduces the
// analytic result (including rects narrower than one pixel). Draw these with
// u_cov = 1.
func appendRectAA(dst []float32, r Rect) []float32 {
	x0, y0, x1, y1 := r.Min.X, r.Min.Y, r.Max.X, r.Max.Y
	if !(x1 > x0) || !(y1 > y0) {
		return dst
	}
	xs, xc, nx := coverageBands(x0, x1)
	ys, yc, ny := coverageBands(y0, y1)
	for j := 0; j < ny; j++ {
		for i := 0; i < nx; i++ {
			cov := xc[i] * yc[j]
			if cov <= 0 {
				continue
			}
			qx0, qx1 := xs[i], xs[i+1]
			qy0, qy1 := ys[j], ys[j+1]
			if !(qx1 > qx0) || !(qy1 > qy0) {
				continue
			}
			dst = append(dst,
				qx0, qy0, cov, 0,
				qx1, qy0, cov, 0,
				qx1, qy1, cov, 0,
				qx0, qy0, cov, 0,
				qx1, qy1, cov, 0,
				qx0, qy1, cov, 0,
			)
		}
	}
	return dst
}

// coverageBands splits [lo, hi) into at most three pixel-aligned bands and
// returns their edges, each band's fractional coverage, and the band count.
func coverageBands(lo, hi float32) (edges [4]float32, cov [3]float32, n int) {
	p0 := float32(mathFloorF(lo))
	p1 := float32(mathCeil(hi))
	if p1-p0 <= 1 {
		// Entirely inside one pixel column/row.
		edges[0], edges[1] = p0, p0+1
		cov[0] = hi - lo
		return edges, cov, 1
	}
	first := p0 + 1 - lo
	last := hi - (p1 - 1)
	if p1-p0 == 2 {
		edges[0], edges[1], edges[2] = p0, p0+1, p1
		cov[0], cov[1] = first, last
		return edges, cov, 2
	}
	edges[0], edges[1], edges[2], edges[3] = p0, p0+1, p1-1, p1
	cov[0], cov[1], cov[2] = first, 1, last
	return edges, cov, 3
}

func mathCeil(v float32) int {
	i := int(v)
	if float32(i) < v {
		i++
	}
	return i
}

func mathFloorF(v float32) int {
	i := int(v)
	if float32(i) > v {
		i--
	}
	return i
}

// isPixelAligned reports whether a rect needs no AA fringe.
func isPixelAligned(r Rect) bool {
	return r.Min.X == float32(int(r.Min.X)) && r.Min.Y == float32(int(r.Min.Y)) &&
		r.Max.X == float32(int(r.Max.X)) && r.Max.Y == float32(int(r.Max.Y))
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

// markDrawn records that the target changed: the CPU snapshot is stale and
// multisample results need resolving before anything reads or presents them.
func (d *GPUDevice) markDrawn() {
	d.readDirty = true
	if d.msaa {
		d.msDirty = true
	}
}

func (d *GPUDevice) drawTris(verts []float32) {
	if len(verts) < 12 {
		return
	}
	C.glBindBuffer(C.GL_ARRAY_BUFFER, d.vbo)
	C.glBufferData(C.GL_ARRAY_BUFFER, C.GLsizeiptr(len(verts)*4), unsafe.Pointer(&verts[0]), C.GL_STREAM_DRAW)
	C.glEnableVertexAttribArray(0)
	C.glEnableVertexAttribArray(1)
	// Attribute offsets go through C: unsafe.Pointer(uintptr(8)) is not a
	// valid Go pointer (go vet flags it, -race checkptr aborts on it).
	C.pe_attrib(0, 16, 0)
	C.pe_attrib(1, 16, 8)
	C.glDrawArrays(C.GL_TRIANGLES, 0, C.GLsizei(len(verts)/4))
	C.glDisableVertexAttribArray(0)
	C.glDisableVertexAttribArray(1)
	d.markDrawn()
}

func (d *GPUDevice) fillOpaqueRects(rects []Rect, color Color, clip Clip) {
	if d == nil || d.closed || len(rects) == 0 {
		return
	}
	if err := d.makeCurrent(); err != nil {
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
	d.setBlend(BlendSrcOver)
	C.glUseProgram(d.prog)
	C.glUniform2f(d.locVP, C.GLfloat(d.w), C.GLfloat(d.h))
	C.glUniform1i(d.locMode, 0)
	C.glUniform4f(d.locColor, C.GLfloat(cr)/255, C.GLfloat(cg)/255, C.GLfloat(cb)/255, C.GLfloat(ca)/255)
	C.glUniform1i(d.locUseM, 0)
	C.glUniform1i(d.locCov, 0)
	C.glColorMask(C.GL_TRUE, C.GL_TRUE, C.GL_TRUE, C.GL_TRUE)
	C.glStencilFunc(C.GL_ALWAYS, 0, 0xFF)
	C.glStencilOp(C.GL_KEEP, C.GL_KEEP, C.GL_KEEP)
	C.glDisable(C.GL_STENCIL_TEST)
	// Coverage rides in uv.x, so the fringe pixels of a fractional rect are
	// blended — keep blending on even for an opaque color (an interior
	// pixel has coverage 1 and still overwrites).
	C.glUniform1i(d.locCov, 1)
	d.verts = d.verts[:0]
	for _, r := range rects {
		d.verts = appendRectAA(d.verts, r)
	}
	d.drawTris(d.verts)
	C.glUniform1i(d.locCov, 0)
	C.glEnable(C.GL_BLEND)
	C.glEnable(C.GL_STENCIL_TEST)
	C.glDisable(C.GL_SCISSOR_TEST)
	d.markDrawn()
}

var _ Device = (*GPUDevice)(nil)
