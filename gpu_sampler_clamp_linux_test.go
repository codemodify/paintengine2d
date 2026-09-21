//go:build linux && cgo

package paintengine2d

import "testing"

// The GPU's textures are GL_CLAMP_TO_EDGE, so it has always clamped; this
// pins that it does, and that it now paints the same picture as the CPU
// (sampler_clamp_test.go) instead of a CPU that faded where it did not.
func TestBilinearBlitClampsToEdgeGPU(t *testing.T) {
	d := gpuDev(t, 28, 40)
	defer d.Close()
	ctx := NewContextDevice(d)
	clampScene(ctx)
	gpu := d.Snapshot()
	checkClamped(t, gpu, "gpu")

	cpu := NewImage(28, 40)
	clampScene(NewContext(cpu))
	for y := 0; y < 40; y++ {
		for x := 0; x < 28; x++ {
			r0, g0, b0, a0 := cpu.PremulAt(x, y)
			r1, g1, b1, a1 := gpu.PremulAt(x, y)
			if absDiff(r0, r1) > 4 || absDiff(g0, g1) > 4 || absDiff(b0, b1) > 4 || absDiff(a0, a1) > 4 {
				t.Fatalf("(%d, %d): cpu %d,%d,%d,%d gpu %d,%d,%d,%d", x, y, r0, g0, b0, a0, r1, g1, b1, a1)
			}
		}
	}
}
