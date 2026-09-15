package paintengine2d

import (
	"runtime"
	"testing"
)

func gpuDev(t *testing.T, w, h int) *GPUDevice {
	t.Helper()
	if !GPUAvailable() {
		t.Skip("no EGL/GLES")
	}
	d, err := NewGPUDevice(w, h)
	if err != nil {
		t.Skipf("gpu device: %v", err)
	}
	return d
}

// Regression: Close() called eglTerminate on a display shared by every
// device in the process, so closing one window's device silently killed the
// others (every later draw became a no-op and Present returned
// EGL_NOT_INITIALIZED).
func TestGPUCloseKeepsOtherDevicesAlive(t *testing.T) {
	a := gpuDev(t, 32, 32)
	defer a.Close()
	b, err := NewGPUDevice(32, 32)
	if err != nil {
		t.Skipf("second device: %v", err)
	}
	a.Clear(Red)
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	a.Clear(Blue)
	a.Fill(RectPath(XYWH(0, 0, 32, 32)), Identity(), Fill(Green), Clip{})
	img := a.Snapshot()
	if img == nil {
		t.Fatal("snapshot after closing the other device")
	}
	r, g, _, al := img.PremulAt(4, 4)
	if g < 200 || r > 60 || al < 200 {
		t.Fatalf("device A stopped painting after B closed: %d/%d/%d", r, g, al)
	}
	if err := a.Err(); err != nil {
		t.Fatalf("device A recorded an error: %v", err)
	}
	if err := a.Present(); err != nil {
		t.Fatalf("Present after another device closed: %v", err)
	}
}

// Regression: the availability probe created and destroyed its own device,
// terminating the shared display underneath a device the caller owned.
// DrawSceneDamage used to call GPUAvailable() mid-replay.
func TestGPUAvailableProbeDoesNotDisturbLiveDevice(t *testing.T) {
	d := gpuDev(t, 64, 48)
	defer d.Close()
	d.Clear(White)
	if !GPUAvailable() {
		t.Fatal("probe should report available")
	}
	rec := NewRecorder(64, 48)
	ctx := NewContextDevice(rec)
	ctx.DrawCircle(Pt(32, 24), 10, Fill(Red))
	ctx.DrawRect(XYWH(0, 0, 8, 8), Fill(Blue))
	DrawScene(rec.Finish(), d)

	img := d.Snapshot()
	if img == nil {
		t.Fatal("snapshot")
	}
	if r, _, _, a := img.PremulAt(32, 24); r < 200 || a < 200 {
		t.Fatalf("scene replay after a probe painted nothing: r=%d a=%d", r, a)
	}
	if _, _, b, a := img.PremulAt(2, 2); b < 200 || a < 200 {
		t.Fatalf("rect batch after a probe: b=%d a=%d", b, a)
	}
	if err := d.Err(); err != nil {
		t.Fatalf("device error: %v", err)
	}
}

// The CPU and GPU backends must agree within a tolerance, edges included.
func TestGPUMatchesCPUWithinTolerance(t *testing.T) {
	d := gpuDev(t, 64, 64)
	defer d.Close()
	cimg := NewImage(64, 64)
	cdev := NewCPUDevice(cimg)

	paint := func(dev Device) {
		dev.Clear(White)
		dev.Fill(CirclePath(Pt(32, 32), 20), Identity(), Fill(Black), Clip{})
		dev.Fill(RectPath(XYWH(2.5, 2.5, 10, 10)), Identity(), Fill(RGBA(0, 0, 1, 0.5)), Clip{})
		dev.Fill(RectPath(XYWH(48, 4, 12, 12)), Identity(), Fill(Red), Clip{})
	}
	paint(cdev)
	paint(d)
	gimg := d.Snapshot()
	if gimg == nil {
		t.Fatal("snapshot")
	}

	// A fractional-edge opaque rect is analytically anti-aliased on both
	// backends: the half-covered column must not be a hard edge.
	cr, _, _, _ := cimg.PremulAt(2, 5)
	gr, _, _, _ := gimg.PremulAt(2, 5)
	if diff8(cr, gr) > 40 {
		t.Fatalf("fractional rect edge: CPU R=%d GPU R=%d", cr, gr)
	}
	if gr == 255 || gr == 0 {
		t.Fatalf("GPU rect edge is not anti-aliased (R=%d)", gr)
	}

	var worst, bad, total, partial, cpuPartial int
	levels := map[uint8]bool{}
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			c1, c2, c3, _ := cimg.PremulAt(x, y)
			g1, g2, g3, _ := gimg.PremulAt(x, y)
			dm := diff8(c1, g1)
			if v := diff8(c2, g2); v > dm {
				dm = v
			}
			if v := diff8(c3, g3); v > dm {
				dm = v
			}
			total += dm
			if dm > worst {
				worst = dm
			}
			if dm > 24 {
				bad++
			}
			// Partially covered pixels prove the edges are anti-aliased.
			if g1 != 0 && g1 != 255 {
				levels[g1] = true
				partial++
			}
			if c1 != 0 && c1 != 255 {
				cpuPartial++
			}
		}
	}
	mean := float64(total) / float64(64*64)
	t.Logf("cpu/gpu parity: mean %.2f, worst %d, %d pixels >24, partial cpu=%d gpu=%d (%d levels), msaa=%v samples=%d",
		mean, worst, bad, cpuPartial, partial, len(levels), d.Antialiased(), d.Samples())
	if mean > 4 {
		t.Fatalf("mean abs difference %.2f is too high", mean)
	}
	badLimit, worstLimit := 200, 96
	if !d.Antialiased() {
		// Without MSAA only the rect fringe is analytic; a curve edge is hard.
		badLimit, worstLimit = 400, 255
	}
	if bad > badLimit || worst > worstLimit {
		t.Fatalf("%d/%d pixels differ by more than 24 (worst %d)", bad, 64*64, worst)
	}
	// Rect edges are analytically anti-aliased on every driver; curve edges
	// need the multisample target.
	if partial == 0 || len(levels) < 2 {
		t.Fatalf("no analytic rect fringe: %d partial pixels in %d levels", partial, len(levels))
	}
	if d.Antialiased() && (partial < cpuPartial/3 || len(levels) < 4) {
		t.Fatalf("MSAA target did not anti-alias curves: %d partial pixels in %d levels (CPU %d)",
			partial, len(levels), cpuPartial)
	}
}

func diff8(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}

// An EGL context belongs to one OS thread; using the device from another
// goroutine must report an error instead of silently drawing nothing.
func TestGPUThreadAffinityIsReported(t *testing.T) {
	d := gpuDev(t, 32, 32)
	defer d.Close()
	d.Clear(White)

	errc := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		errc <- d.MakeCurrent()
	}()
	err := <-errc
	if err == nil {
		t.Skip("the other goroutine happened to land on the owning thread")
	}
	if got := d.Err(); got == nil {
		t.Fatal("cross-thread use should be recorded on the device")
	}
	d.ClearErr()
	if d.Err() != nil {
		t.Fatal("ClearErr")
	}
	// The owning thread still works.
	d.Fill(RectPath(XYWH(0, 0, 32, 32)), Identity(), Fill(Green), Clip{})
	if _, g, _, _ := d.Snapshot().PremulAt(4, 4); g < 200 {
		t.Fatal("owning thread should still paint")
	}
}

// BeginFrame/EndFrame bracket a batch of draws and surface GL errors.
func TestGPUBeginEndFrame(t *testing.T) {
	d := gpuDev(t, 32, 32)
	defer d.Close()
	if err := d.BeginFrame(); err != nil {
		t.Fatal(err)
	}
	d.Clear(White)
	d.Fill(RectPath(XYWH(4, 4, 16, 16)), Identity(), Fill(Red), Clip{})
	if err := d.EndFrame(); err != nil {
		t.Fatalf("EndFrame: %v", err)
	}
	if r, _, _, _ := d.Snapshot().PremulAt(8, 8); r < 200 {
		t.Fatal("frame did not paint")
	}
}

// A GPUDevice can be built on an EGL display/context the caller owns; the
// device must not destroy them.
func TestGPUAdoptExternalContext(t *testing.T) {
	host := gpuDev(t, 32, 32)
	defer host.Close()
	dpy, ctx, surf := host.EGLHandles()
	if dpy == 0 || ctx == 0 {
		t.Skip("no handles to adopt")
	}
	d, err := NewGPUDeviceAdopt(EGLAdopt{Display: dpy, Context: ctx, Draw: surf, Width: 32, Height: 32})
	if err != nil {
		t.Fatalf("adopt: %v", err)
	}
	d.Clear(White)
	d.Fill(RectPath(XYWH(0, 0, 32, 32)), Identity(), Fill(Red), Clip{})
	if r, _, _, _ := d.Snapshot().PremulAt(4, 4); r < 200 {
		t.Fatal("adopted device did not paint")
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	// The host context survives its adopter.
	if err := host.MakeCurrent(); err != nil {
		t.Fatalf("host context destroyed by the adopted device: %v", err)
	}
	host.Clear(Blue)
	if _, _, b, _ := host.Snapshot().PremulAt(4, 4); b < 200 {
		t.Fatal("host device broken after the adopted one closed")
	}
}

// Paint.Opacity reaches the GPU backend too.
func TestGPUHonoursOpacity(t *testing.T) {
	d := gpuDev(t, 32, 32)
	defer d.Close()
	d.Clear(White)
	p := Fill(Black)
	p.Opacity = 0.5
	d.Fill(RectPath(XYWH(0, 0, 32, 32)), Identity(), p, Clip{})
	r, _, _, _ := d.Snapshot().PremulAt(16, 16)
	if r < 100 || r > 160 {
		t.Fatalf("half-opacity GPU fill R=%d", r)
	}

	d.Clear(White)
	icon := NewImage(8, 8)
	icon.Clear(Black)
	d.Blit(icon, XYWH(0, 0, 8, 8), XYWH(0, 0, 8, 8), Identity(), Paint{Color: RGBA(1, 1, 1, 0)}, Clip{})
	if r, _, _, _ := d.Snapshot().PremulAt(4, 4); r < 200 {
		t.Fatalf("alpha-0 tint must paint nothing on the GPU, got R=%d", r)
	}

	// Gradients take the layer alpha too, as on the CPU: a fading
	// gradient (Context.SetAlpha) must not paint opaque.
	grad := LinearGradient{Start: Pt(0, 0), End: Pt(32, 0), Stops: []GradientStop{{0, Black}, {1, Black}}}
	for name, path := range map[string]*Path{
		"rect": RectPath(XYWH(0, 0, 32, 32)),
		"triangle": func() *Path {
			p := NewPath()
			p.MoveTo(0, 0)
			p.LineTo(64, 0)
			p.LineTo(0, 64)
			p.Close()
			return p
		}(),
	} {
		d.Clear(White)
		d.Fill(path, Identity(), Paint{Shader: grad, Opacity: 0.5}, Clip{})
		if r, _, _, _ := d.Snapshot().PremulAt(8, 8); r < 100 || r > 160 {
			t.Fatalf("half-opacity GPU gradient %s R=%d", name, r)
		}
	}
}

// Fractional and sub-pixel rectangles must carry analytic coverage on the
// GPU, matching the CPU rasterizer closely.
func TestGPURectCoverageMatchesCPU(t *testing.T) {
	d := gpuDev(t, 32, 32)
	defer d.Close()
	cimg := NewImage(32, 32)
	cdev := NewCPUDevice(cimg)

	cases := []struct {
		name string
		r    Rect
		x, y int
	}{
		{"half pixel edge", XYWH(4.5, 4, 10, 10), 4, 8},
		{"quarter pixel edge", XYWH(4.75, 4, 10, 10), 4, 8},
		{"sub-pixel wide", XYWH(8.2, 4, 0.4, 10), 8, 8},
		{"straddling a boundary", XYWH(10.8, 4, 0.4, 10), 10, 8},
	}
	for _, c := range cases {
		cdev.Clear(White)
		d.Clear(White)
		cdev.Fill(RectPath(c.r), Identity(), Fill(Black), Clip{})
		d.Fill(RectPath(c.r), Identity(), Fill(Black), Clip{})
		cr, _, _, _ := cimg.PremulAt(c.x, c.y)
		gr, _, _, _ := d.Snapshot().PremulAt(c.x, c.y)
		if diff8(cr, gr) > 40 {
			t.Errorf("%s: pixel (%d,%d) CPU R=%d GPU R=%d", c.name, c.x, c.y, cr, gr)
		}
	}
}

// The GPU backdrop blur matches the CPU one: grey stripes inside the rect,
// the stripes untouched outside, and a rounded clip keeps its corners.
func TestGPUBackdropBlurMatchesCPU(t *testing.T) {
	const w, h = 96, 64
	d := gpuDev(t, w, h)
	defer d.Close()
	gctx := NewContextDevice(d)
	cimg := NewImage(w, h)
	cctx := NewContext(cimg)
	for _, ctx := range []*Context{gctx, cctx} {
		stripes(ctx, w, h)
		ctx.Save()
		ctx.ClipRoundRect(XYWH(24, 12, 48, 40), 10, 10)
		ctx.BackdropBlur(XYWH(24, 12, 48, 40), 4)
		ctx.Restore()
	}
	gimg := d.Snapshot()
	for _, p := range [][2]int{{40, 30}, {48, 32}, {60, 40}} {
		gr, _, _, _ := gimg.PremulAt(p[0], p[1])
		cr, _, _, _ := cimg.PremulAt(p[0], p[1])
		if gr < 90 || gr > 170 {
			t.Errorf("GPU blur at %v: %d, want grey", p, gr)
		}
		if diff := int(gr) - int(cr); diff > 14 || diff < -14 {
			t.Errorf("GPU %d vs CPU %d at %v", gr, cr, p)
		}
	}
	if r, _, _, _ := gimg.PremulAt(8, 30); r != 0 {
		t.Errorf("GPU blur leaked outside: %d", r)
	}
	if r, _, _, _ := gimg.PremulAt(24, 12); r != 0 {
		t.Errorf("GPU blur ignored the rounded clip at the corner: %d", r)
	}
}

// Replaying a recorded blur on the GPU (the app's path) blurs as well.
func TestGPUBackdropBlurScene(t *testing.T) {
	const w, h = 96, 64
	d := gpuDev(t, w, h)
	defer d.Close()
	rec := NewRecorder(w, h)
	ctx := NewContextDevice(rec)
	stripes(ctx, w, h)
	ctx.BackdropBlur(XYWH(24, 12, 48, 40), 4)
	DrawScene(rec.Finish(), d)
	img := d.Snapshot()
	if r, _, _, _ := img.PremulAt(48, 32); r < 90 || r > 170 {
		t.Fatalf("scene blur on the GPU: %d, want grey", r)
	}
	if r, _, _, _ := img.PremulAt(8, 32); r != 0 {
		t.Fatalf("scene blur leaked: %d", r)
	}
}
