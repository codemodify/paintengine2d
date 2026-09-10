package paintengine2d

// Device is the low-level paint backend — the analogue of JUCE's
// LowLevelGraphicsContext. [Context] holds the public canvas state
// (transform, clip, convenience paints) and forwards resolved draw ops here.
//
// A GPU implementation can satisfy this interface without changing
// application code that talks to Context. v0 ships only [CPUDevice].
//
// All geometry is in user space; xform maps user → device pixels.
// Clip is already in device space.
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
	// paint.Color.A modulates the blit (zero-value paint = unmodulated).
	// paint.Filter selects nearest or bilinear. Shader is ignored.
	Blit(src *Image, srcRect, dstRect Rect, xform Matrix, paint Paint, clip Clip)
}

// Compile-time check: the CPU backend implements Device.
var _ Device = (*CPUDevice)(nil)
