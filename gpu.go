package paintengine2d

import "errors"

// ErrGPUUnavailable is returned when a GPU device is requested but EGL/GLES
// is not available (CGO off, missing libraries, or EGL init failure).
var ErrGPUUnavailable = errors.New("paintengine2d: GPU backend unavailable")

// EGL platform tokens for [NewGPUDeviceEGL] (KHR values). Zero means
// eglGetDisplay(nativeDisplay) — correct for a classic X11 Display*.
const (
	EGLPlatformDefault     uint32 = 0
	EGLPlatformX11         uint32 = 0x31D5
	EGLPlatformWayland     uint32 = 0x31D8
	EGLPlatformGBM         uint32 = 0x31D7
	EGLPlatformSurfaceless uint32 = 0x31DD
)

// EGLNative identifies an existing native window for the GPU backend.
// Display and Window are EGLNativeDisplayType / EGLNativeWindowType
// (X11 Display* + Window, or wl_display* + wl_egl_window*).
//
// Window configs request EGL_ALPHA_SIZE 0 by default so an opaque UI cannot
// present as a fully transparent ARGB surface (the v0.4.1 Wayland pitfall).
// Set Alpha to ask for an 8-bit alpha visual instead — what a compositor
// needs for translucent chrome or a shaped window.
//
// Share is an existing EGLContext to share objects with (textures, buffers).
// A compositor with one device per output uploads a glyph atlas once when
// its devices share a context.
//
// The EGLDisplay is reference counted per process: two devices on the same
// native display share one initialized display, and only the last Close
// terminates it.
type EGLNative struct {
	Display  uintptr
	Window   uintptr
	Platform uint32
	Width    int
	Height   int
	// Alpha requests EGL_ALPHA_SIZE 8 (translucent / ARGB window).
	Alpha bool
	// Share is an EGLContext to share GL objects with (0 = none).
	Share uintptr
}

// EGLAdopt binds a [GPUDevice] to an EGL display and context the caller
// already created and owns. paintengine2d neither destroys the context nor
// terminates the display; it only creates its own FBO and program on it.
//
// Use this when the host (a compositor, or a toolkit that already runs EGL)
// owns the GL state: pass the display, the context, and the draw surface
// (0 for offscreen/surfaceless rendering).
//
// The adopting goroutine must be the one that keeps the context current;
// [GPUDevice] records the calling OS thread and refuses calls from another.
type EGLAdopt struct {
	Display uintptr // EGLDisplay
	Context uintptr // EGLContext
	Draw    uintptr // EGLSurface (0 = no window surface)
	Width   int
	Height  int
}

// GPUAvailable reports whether this process can create an offscreen
// [GPUDevice] (Linux + CGO + working EGL). Safe to call from tests.
func GPUAvailable() bool { return gpuAvailable() }

// GPU flatten/tessellation reuse and atlas epoch live on [GPUDevice]:
// identical Fill/Stroke geometry is not CPU-flattened again, and an
// [Image.Epoch] bump (or [Image.Touch]) invalidates the texture cache.
// Axis-aligned rect fills skip stencil-and-cover. [GPUDevice.PresentRects]
// hints the compositor via eglSwapBuffersWithDamage when the extension
// exists. When EGL_KHR_partial_update or EGL_BUFFER_PRESERVED is available,
// only dirty boxes are blitted to the window (not a full FBO copy).
// [DrawSceneDamage] calls [GPUDevice.SetPresentDamage] so [Present] can
// swap those boxes without a second rect list.

// NewGPUDevice creates an offscreen GLES framebuffer of w×h device pixels.
func NewGPUDevice(w, h int) (*GPUDevice, error) { return newGPUDevice(w, h) }

// NewGPUDeviceEGL binds a [GPUDevice] to an existing native window.
func NewGPUDeviceEGL(n EGLNative) (*GPUDevice, error) { return newGPUDeviceEGL(n) }

// NewGPUDeviceAdopt binds a [GPUDevice] to an EGL display/context owned by
// the caller. See [EGLAdopt].
func NewGPUDeviceAdopt(a EGLAdopt) (*GPUDevice, error) { return newGPUDeviceAdopt(a) }

// GPUInfo is a short renderer string (empty when GPU is unavailable).
func GPUInfo() string { return gpuInfo() }
