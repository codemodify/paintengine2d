package paintengine2d

import "testing"

func TestParsePaintPref(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", PaintCPU},
		{"cpu", PaintCPU},
		{"CPU", PaintCPU},
		{"gpu", PaintGPU},
		{"auto", PaintAuto},
		{"weird", PaintCPU},
	}
	for _, c := range cases {
		if got := ParsePaintPref(c.in); got != c.want {
			t.Fatalf("ParsePaintPref(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestOpenSurfaceCPU(t *testing.T) {
	s, err := OpenSurfacePref(32, 24, PaintCPU)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.Kind() != BackendCPU {
		t.Fatalf("kind=%s", s.Kind())
	}
	w, h := s.Size()
	if w != 32 || h != 24 {
		t.Fatalf("size %dx%d", w, h)
	}
	ctx := NewContextSurface(s)
	ctx.Clear(RGB(0.2, 0.4, 0.8))
	img := s.Image()
	r, g, b, a := img.PremulAt(4, 4)
	if a < 250 || r < 40 {
		t.Fatalf("cpu clear r=%d g=%d b=%d a=%d", r, g, b, a)
	}
}

func TestGPUDeviceConstructorWithoutGL(t *testing.T) {
	if GPUAvailable() {
		t.Skip("EGL present — constructor succeeds")
	}
	if _, err := NewGPUDevice(16, 16); err == nil {
		t.Fatal("expected ErrGPUUnavailable")
	}
}

func TestGPUFillStrokeBlit(t *testing.T) {
	if !GPUAvailable() {
		t.Skip("no EGL/GLES")
	}
	dev, err := NewGPUDevice(80, 48)
	if err != nil {
		t.Fatal(err)
	}
	defer dev.Close()
	ctx := NewContextDevice(dev)
	ctx.Clear(RGB(0.10, 0.11, 0.14))
	ctx.DrawRoundRect(XYWH(8, 8, 40, 24), 6, 6, Fill(RGB(0.23, 0.51, 0.93)))
	ctx.DrawRoundRect(XYWH(8, 8, 40, 24).Inset(-3), 8, 8, Paint{
		Color:  RGB(0.45, 0.75, 1.0),
		Style:  StyleStroke,
		Stroke: Stroke{Width: 2, Cap: CapRound, Join: JoinRound, MiterLimit: 4},
	})
	ctx.DrawRect(XYWH(52, 6, 22, 36), Linear(LinearGradient{
		Start: Pt(52, 6), End: Pt(74, 42),
		Stops: []GradientStop{{0, RGB(0.9, 0.3, 0.2)}, {1, RGB(0.2, 0.4, 0.9)}},
	}))
	src := NewImage(8, 8)
	src.Clear(White)
	ctx.DrawImageRectPaint(src, XYWH(0, 0, 8, 8), XYWH(12, 34, 8, 8), Paint{Color: RGB(0.2, 0.9, 0.3), Filter: FilterNearest})

	img := ctx.Image()
	if img == nil {
		t.Fatal("gpu snapshot nil")
	}
	r, g, b, a := img.PremulAt(28, 20)
	if a < 200 || b < 80 || r > 120 {
		t.Fatalf("button fill rgba=%d,%d,%d,%d", r, g, b, a)
	}
	r, g, b, a = img.PremulAt(2, 2)
	if a < 20 {
		t.Fatalf("cleared background a=%d", a)
	}
	if r > 80 || g > 80 {
		t.Fatalf("background should stay dark, rgba=%d,%d,%d,%d", r, g, b, a)
	}
	_, _, _, a = img.PremulAt(63, 24)
	if a < 200 {
		t.Fatalf("gradient rect a=%d", a)
	}
}

func TestGPUClipPath(t *testing.T) {
	if !GPUAvailable() {
		t.Skip("no EGL/GLES")
	}
	dev, err := NewGPUDevice(64, 64)
	if err != nil {
		t.Fatal(err)
	}
	defer dev.Close()
	ctx := NewContextDevice(dev)
	ctx.Clear(Transparent)
	ctx.ClipPath(CirclePath(Pt(32, 32), 20))
	ctx.FillRect(XYWH(0, 0, 64, 64))
	img := ctx.Image()
	if alphaAt(img, 32, 32) < 200 {
		t.Fatalf("inside clip a=%d", alphaAt(img, 32, 32))
	}
	if alphaAt(img, 2, 2) > 20 {
		t.Fatalf("outside clip a=%d", alphaAt(img, 2, 2))
	}
}

func TestOpenSurfaceAutoFallsBack(t *testing.T) {
	s, err := OpenSurfacePref(16, 16, PaintAuto)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if GPUAvailable() && s.Kind() != BackendGPU {
		t.Fatalf("auto should pick gpu when available, got %s", s.Kind())
	}
	if !GPUAvailable() && s.Kind() != BackendCPU {
		t.Fatalf("auto should pick cpu without EGL, got %s", s.Kind())
	}
}
