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
// Window configs request EGL_ALPHA_SIZE 0 so an opaque UI cannot present
// as a fully transparent ARGB surface (the v0.4.1 Wayland pitfall).
type EGLNative struct {
	Display  uintptr
	Window   uintptr
	Platform uint32
	Width    int
	Height   int
}

// GPUAvailable reports whether this process can create an offscreen
// [GPUDevice] (Linux + CGO + working EGL). Safe to call from tests.
func GPUAvailable() bool { return gpuAvailable() }

// GPU flatten/tessellation reuse and atlas epoch live on [GPUDevice]:
// identical Fill/Stroke geometry is not CPU-flattened again, and an
// [Image.Epoch] bump (or [Image.Touch]) invalidates the texture cache.

// NewGPUDevice creates an offscreen GLES framebuffer of w×h device pixels.
func NewGPUDevice(w, h int) (*GPUDevice, error) { return newGPUDevice(w, h) }

// NewGPUDeviceEGL binds a [GPUDevice] to an existing native window.
func NewGPUDeviceEGL(n EGLNative) (*GPUDevice, error) { return newGPUDeviceEGL(n) }

// GPUInfo is a short renderer string (empty when GPU is unavailable).
func GPUInfo() string { return gpuInfo() }
