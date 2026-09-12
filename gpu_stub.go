//go:build !linux || !cgo

package paintengine2d

// GPUDevice is the EGL/GLES paint backend. This stub is used when CGO is
// off or the OS is not Linux; every constructor returns [ErrGPUUnavailable].
type GPUDevice struct {
	w, h int
}

func gpuAvailable() bool { return false }

func gpuInfo() string { return "" }

func newGPUDevice(w, h int) (*GPUDevice, error) {
	_, _ = w, h
	return nil, ErrGPUUnavailable
}

func newGPUDeviceEGL(n EGLNative) (*GPUDevice, error) {
	_ = n
	return nil, ErrGPUUnavailable
}

func newGPUSurface(w, h int) (Surface, error) {
	_, _ = w, h
	return nil, ErrGPUUnavailable
}

func (d *GPUDevice) Size() (w, h int) { return d.w, d.h }
func (d *GPUDevice) Clear(c Color)    {}
func (d *GPUDevice) Fill(path *Path, xform Matrix, paint Paint, clip Clip) {
	_, _, _, _ = path, xform, paint, clip
}
func (d *GPUDevice) Stroke(path *Path, xform Matrix, paint Paint, clip Clip) {
	_, _, _, _ = path, xform, paint, clip
}
func (d *GPUDevice) Blit(src *Image, srcRect, dstRect Rect, xform Matrix, paint Paint, clip Clip) {
	_, _, _, _, _, _ = src, srcRect, dstRect, xform, paint, clip
}
func (d *GPUDevice) Image() *Image             { return nil }
func (d *GPUDevice) Snapshot() *Image          { return nil }
func (d *GPUDevice) Present() error            { return ErrGPUUnavailable }
func (d *GPUDevice) PresentRects([]Rect) error { return ErrGPUUnavailable }
func (d *GPUDevice) ClearRect(r Rect, c Color) { _, _ = r, c }
func (d *GPUDevice) Resize(w, h int) error     { _, _ = w, h; return ErrGPUUnavailable }
func (d *GPUDevice) Close() error              { return nil }
func (d *GPUDevice) MakeCurrent() error        { return ErrGPUUnavailable }
func (d *GPUDevice) Kind() BackendKind         { return BackendGPU }

func (d *GPUDevice) fillOpaqueRects(rects []Rect, color Color, clip Clip) {
	_, _, _ = rects, color, clip
}

var _ Device = (*GPUDevice)(nil)
