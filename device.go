package paintengine2d

// Device is the low-level paint backend — the analogue of JUCE's
// LowLevelGraphicsContext. [Context] holds the public canvas state
// (transform, clip, convenience paints) and forwards resolved draw ops here.
//
// This interface is the stable retarget seam for both a UI framework and a
// compositor/WM: CPU today, a GPU [Device] later, same Paint call sites.
//
// [CPUDevice] is always available. On Linux with CGO, [GPUDevice] implements
// the same interface via EGL / OpenGL ES 2 (stencil-and-cover + textured blit).
// [Recorder] implements Device by capturing a retained [Scene] for [DrawScene].
//
// All geometry is in user space; xform maps user → device pixels.
// Clip is already in device space.
//
// Optional methods (not on this interface, so existing backends stay
// source-compatible): ClearRect(Rect, Color) for dirty-rect erase;
// Present() / PresentRects([]Rect) / SetPresentDamage([]Rect) for GPU
// swap-with-damage (and EGL_KHR_partial_update when present);
// Scroll(dx, dy int, r Rect) for intra-surface memmove. [Context]
// type-asserts these. See [Context.ClearRect], [Context.PresentDamage],
// [Context.Scroll], [DrawSceneDamage].
type Device interface {
	// Size is the device pixmap / target size in pixels.
	Size() (w, h int)

	// Clear fills the entire target, ignoring clip (Skia drawColor / canvas
	// clear convention). Use a clipped Fill of a huge rect to honor clip.
	Clear(c Color)

	// Fill paints the interior of path after applying xform.
	Fill(path *Path, xform Matrix, paint Paint, clip Clip)

	// Stroke paints an expanded outline of path after applying xform.
	// Stroke parameters come from paint.Stroke. Width is in user space.
	Stroke(path *Path, xform Matrix, paint Paint, clip Clip)

	// Blit draws src[srcRect] into dstRect (user space), then xform.
	// paint.Color is an RGB multiplier on premul samples (white atlas theming);
	// Color.A modulates coverage. Zero-value paint is unmodulated.
	// paint.Filter selects nearest or bilinear. Shader is ignored.
	Blit(src *Image, srcRect, dstRect Rect, xform Matrix, paint Paint, clip Clip)
}

// Compile-time check: the CPU backend implements Device.
var _ Device = (*CPUDevice)(nil)

// GPUDevice also implements Device (real type on linux+cgo, stub otherwise).
