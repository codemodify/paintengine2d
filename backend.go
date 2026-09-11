package paintengine2d

import (
	"os"
	"strings"
)

// UITK_PAINT selects the paint backend for [OpenSurface] and for
// toolkit windows that honor the same variable.
//
//	cpu   — always [CPUDevice] (default when CGO is off or EGL is missing)
//	gpu   — require the Linux EGL/GLES [GPUDevice]; OpenSurface errors if init fails
//	auto  — GPU when EGL works, otherwise CPU
const (
	EnvPaint  = "UITK_PAINT"
	PaintCPU  = "cpu"
	PaintGPU  = "gpu"
	PaintAuto = "auto"
)

// BackendKind identifies the active paint implementation.
type BackendKind uint8

const (
	// BackendCPU is the portable scanline AA rasterizer.
	BackendCPU BackendKind = iota
	// BackendGPU is the Linux EGL / OpenGL ES device.
	BackendGPU
)

func (k BackendKind) String() string {
	switch k {
	case BackendGPU:
		return "gpu"
	default:
		return "cpu"
	}
}

// ParsePaintPref normalizes a UITK_PAINT value to cpu, gpu, or auto.
func ParsePaintPref(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case PaintGPU:
		return PaintGPU
	case PaintAuto:
		return PaintAuto
	default:
		return PaintCPU
	}
}

// EnvPaintPref returns the normalized UITK_PAINT choice.
// An unset or empty value is auto (try GPU, fall back to CPU).
func EnvPaintPref() string {
	s := strings.TrimSpace(os.Getenv(EnvPaint))
	if s == "" {
		return PaintAuto
	}
	return ParsePaintPref(s)
}

// Surface is a CPU pixmap or GPU framebuffer that yields a [Device].
//
// [Context] talks only to Device; Surface owns the target lifetime,
// resize, and (for GPU) the EGL swapchain / FBO.
type Surface interface {
	Size() (w, h int)
	Resize(w, h int) error
	Device() Device
	// Image is a CPU-readable premul RGBA snapshot. GPU surfaces read back.
	Image() *Image
	Kind() BackendKind
	Close() error
}

// CPUSurface is a [Surface] backed by a premul [Image] and [CPUDevice].
type CPUSurface struct {
	img *Image
	dev *CPUDevice
}

// NewCPUSurface wraps img. The image must outlive the surface.
func NewCPUSurface(img *Image) *CPUSurface {
	if img == nil {
		panic("paintengine2d: NewCPUSurface(nil)")
	}
	return &CPUSurface{img: img, dev: NewCPUDevice(img)}
}

// NewCPUSurfaceSize allocates a transparent packed pixmap.
func NewCPUSurfaceSize(w, h int) *CPUSurface {
	return NewCPUSurface(NewImage(w, h))
}

func (s *CPUSurface) Size() (w, h int)  { return s.img.Width, s.img.Height }
func (s *CPUSurface) Device() Device    { return s.dev }
func (s *CPUSurface) Image() *Image     { return s.img }
func (s *CPUSurface) Kind() BackendKind { return BackendCPU }
func (s *CPUSurface) Close() error      { return nil }

func (s *CPUSurface) Resize(w, h int) error {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	if s.img.Width == w && s.img.Height == h {
		return nil
	}
	s.img = NewImage(w, h)
	s.dev = NewCPUDevice(s.img)
	return nil
}

// OpenSurface allocates a w×h target using [EnvPaintPref].
// auto / gpu use EGL when available; cpu (and failed auto) use [CPUSurface].
func OpenSurface(w, h int) (Surface, error) {
	return OpenSurfacePref(w, h, EnvPaintPref())
}

// OpenSurfacePref is [OpenSurface] with an explicit cpu|gpu|auto preference.
func OpenSurfacePref(w, h int, pref string) (Surface, error) {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	switch ParsePaintPref(pref) {
	case PaintGPU:
		return newGPUSurface(w, h)
	case PaintAuto:
		if s, err := newGPUSurface(w, h); err == nil {
			return s, nil
		}
		return NewCPUSurfaceSize(w, h), nil
	default:
		return NewCPUSurfaceSize(w, h), nil
	}
}

// NewContextSurface draws through s.Device.
func NewContextSurface(s Surface) *Context {
	if s == nil {
		panic("paintengine2d: NewContextSurface(nil)")
	}
	return NewContextDevice(s.Device())
}
