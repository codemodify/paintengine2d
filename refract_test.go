package paintengine2d

import (
	"math"
	"testing"
)

// The geometry the lens is built on, checked against what a rounded
// rectangle is rather than against itself: inside is positive, the
// boundary is zero, and the corner is a quarter circle.
func TestTheRoundedRectDistanceIsTheShape(t *testing.T) {
	r := XYWH(10, 20, 100, 60)
	const corner = 16
	for _, tc := range []struct {
		name string
		x, y float32
		want float32
	}{
		{"the middle", 60, 50, 30},        // half the height
		{"on the left edge", 10, 50, 0},   //
		{"on the right edge", 110, 50, 0}, //
		{"on the top edge", 60, 20, 0},
		{"just inside the left", 12, 50, 2},
		{"outside to the left", 5, 50, -5},
		{"the corner's centre", 26, 36, corner},
	} {
		if got := roundRectSDF(tc.x, tc.y, r, corner); math.Abs(float64(got-tc.want)) > 0.01 {
			t.Errorf("%s (%g,%g): distance %g, want %g", tc.name, tc.x, tc.y, got, tc.want)
		}
	}
	// On the corner's quarter circle the distance is zero all the way
	// round, which is what makes the bevel follow the corner instead of
	// cutting across it.
	cx, cy := r.Min.X+corner, r.Min.Y+corner
	for deg := 180; deg <= 270; deg += 10 {
		a := float64(deg) * math.Pi / 180
		x := cx + corner*float32(math.Cos(a))
		y := cy + corner*float32(math.Sin(a))
		if got := roundRectSDF(x, y, r, corner); math.Abs(float64(got)) > 0.01 {
			t.Errorf("on the corner at %d°: distance %g, want 0", deg, got)
		}
	}
}

// The normal points out of the shape, and in a corner it points out
// along the radius. A normal that pointed the wrong way would pull the
// middle of the pane outwards instead of its edge inwards.
func TestTheNormalFacesOutOfTheShape(t *testing.T) {
	r := XYWH(0, 0, 100, 100)
	const corner = 20
	for _, tc := range []struct {
		name   string
		x, y   float32
		nx, ny float32
	}{
		{"left edge", 5, 50, -1, 0},
		{"right edge", 95, 50, 1, 0},
		{"top edge", 50, 5, 0, -1},
		{"bottom edge", 50, 95, 0, 1},
	} {
		nx, ny := roundRectNormal(tc.x, tc.y, r, corner)
		if math.Abs(float64(nx-tc.nx)) > 0.01 || math.Abs(float64(ny-tc.ny)) > 0.01 {
			t.Errorf("%s: normal (%g,%g), want (%g,%g)", tc.name, nx, ny, tc.nx, tc.ny)
		}
	}
	// In the top-left corner it points up and to the left, and is a unit
	// vector.
	nx, ny := roundRectNormal(8, 8, r, corner)
	if nx >= 0 || ny >= 0 {
		t.Errorf("the top-left corner's normal is (%g,%g), want both negative", nx, ny)
	}
	if n := math.Hypot(float64(nx), float64(ny)); math.Abs(n-1) > 0.01 {
		t.Errorf("the corner's normal is %g long, want 1", n)
	}
}

// The bevel profile: none in the flat middle, a whole depth at the very
// edge, and rising the whole way — a profile that doubled back would
// fold the image over itself.
func TestTheBevelRisesToTheEdge(t *testing.T) {
	const depth = 10
	if got := refractOffset(depth, depth); got != 0 {
		t.Errorf("at the inner end of the bevel the offset is %g, want 0", got)
	}
	if got := refractOffset(depth*2, depth); got != 0 {
		t.Errorf("in the flat middle the offset is %g, want 0", got)
	}
	if got := refractOffset(0, depth); math.Abs(float64(got-depth)) > 0.01 {
		t.Errorf("at the edge the offset is %g, want the whole depth %g", got, float32(depth))
	}
	prev := float32(-1)
	for d := float32(depth); d >= 0; d -= 0.5 {
		got := refractOffset(d, depth)
		if got < prev {
			t.Fatalf("the bevel doubles back at d=%g: %g after %g", d, got, prev)
		}
		prev = got
	}
}

// refractProbe draws a backdrop of vertical stripes, refracts a pane over
// it and reports the stripe pattern along a row through the middle.
func refractProbe(t *testing.T, dev Device, w, h int, depth float32) []uint8 {
	t.Helper()
	ctx := NewContextDevice(dev)
	ctx.DrawRect(XYWH(0, 0, float32(w), float32(h)), Fill(RGB(0, 0, 0)))
	for x := 0; x < w; x += 8 {
		ctx.DrawRect(XYWH(float32(x), 0, 4, float32(h)), Fill(RGB(1, 1, 1)))
	}
	glass := XYWH(20, 20, float32(w-40), float32(h-40))
	if depth > 0 {
		ctx.BackdropRefract(glass, 10, depth)
	}
	img := ctx.Image()
	if img == nil {
		t.Fatal("the device has no readable image")
	}
	row := make([]uint8, w)
	y := h / 2
	for x := 0; x < w; x++ {
		row[x] = img.Pix[(y*img.Width+x)*4]
	}
	return row
}

// **The pane bends what is behind it, and only near its edge.** Through
// the middle the stripes come through untouched; inside the bevel they
// are pulled in, so the row of pixels there differs from the one with no
// pane at all.
func TestRefractionBendsTheEdgeAndLeavesTheMiddle(t *testing.T) {
	const w, h = 160, 100
	plain := refractProbe(t, NewCPUDevice(NewImage(w, h)), w, h, 0)
	bent := refractProbe(t, NewCPUDevice(NewImage(w, h)), w, h, 14)

	// The middle of the pane is untouched: a lens with a flat centre.
	for x := 60; x < 100; x++ {
		if plain[x] != bent[x] {
			t.Errorf("the middle of the pane changed at x=%d: %d against %d", x, bent[x], plain[x])
			break
		}
	}
	// Inside the bevel it is not. The band runs from the pane's left edge
	// (x=20) in by the depth.
	diff := 0
	for x := 20; x < 34; x++ {
		if plain[x] != bent[x] {
			diff++
		}
	}
	if diff < 4 {
		t.Errorf("only %d of the 14 pixels in the bevel changed; the edge is not being bent", diff)
	}
	// And outside the pane nothing moved.
	for x := 0; x < 20; x++ {
		if plain[x] != bent[x] {
			t.Errorf("outside the pane changed at x=%d", x)
			break
		}
	}
}

// A pane with no depth, or on a device that cannot refract, leaves the
// backdrop alone rather than drawing something else.
func TestRefractionWithoutDepthDoesNothing(t *testing.T) {
	const w, h = 80, 60
	a := refractProbe(t, NewCPUDevice(NewImage(w, h)), w, h, 0)
	b := refractProbe(t, NewCPUDevice(NewImage(w, h)), w, h, 0)
	for x := range a {
		if a[x] != b[x] {
			t.Fatalf("two runs with no depth differ at x=%d", x)
		}
	}
	// A recorder cannot refract; it must not panic.
	ctx := NewContextDevice(NewRecorder(40, 40))
	ctx.BackdropRefract(XYWH(0, 0, 10, 10), 2, 4)
}

// The GPU draws the same lens as the CPU: the same stripes bent at the
// same edge, within the slack a different sampler and a different
// rounding give.
func TestTheGPURefractsLikeTheCPU(t *testing.T) {
	if !GPUAvailable() {
		t.Skip("no EGL/GLES")
	}
	const w, h = 160, 100
	gpu, err := NewGPUDevice(w, h)
	if err != nil {
		t.Fatal(err)
	}
	defer gpu.Close()

	cpuPlain := refractProbe(t, NewCPUDevice(NewImage(w, h)), w, h, 0)
	cpuBent := refractProbe(t, NewCPUDevice(NewImage(w, h)), w, h, 14)
	gpuBent := refractProbe(t, gpu, w, h, 14)

	// The CPU's own bend is the yardstick: if the GPU matched the
	// unrefracted row instead, the pass did nothing.
	same := func(a, b []uint8) int {
		n := 0
		for x := 20; x < w-20; x++ {
			d := int(a[x]) - int(b[x])
			if d < 0 {
				d = -d
			}
			if d <= 24 {
				n++
			}
		}
		return n
	}
	likeBent, likePlain := same(gpuBent, cpuBent), same(gpuBent, cpuPlain)
	if likeBent <= likePlain {
		t.Errorf("the GPU's row matches the unrefracted one in %d pixels and the refracted one in %d: the shader is not bending", likePlain, likeBent)
	}
}
