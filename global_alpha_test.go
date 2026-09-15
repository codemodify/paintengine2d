package paintengine2d

import "testing"

// redOn paints a red draw over white at global alpha a and returns the
// green channel at (10, 10): 255 untouched, 0 fully red, ~128 at half.
func redOn(t *testing.T, a float32, draw func(ctx *Context)) uint8 {
	t.Helper()
	img := NewImage(20, 20)
	ctx := NewContext(img)
	ctx.Clear(White)
	ctx.SetAlpha(a)
	draw(ctx)
	_, g, _, _ := img.PremulAt(10, 10)
	return g
}

func near(got uint8, want int) bool { return int(got) >= want-8 && int(got) <= want+8 }

func TestSetAlphaFadesEveryKindOfDraw(t *testing.T) {
	red := RGBA(1, 0, 0, 1)
	grad := LinearGradient{Start: Pt(0, 0), End: Pt(20, 0), Stops: []GradientStop{{0, red}, {1, red}}}
	icon := NewImage(20, 20)
	icon.Clear(red)
	tri := NewPath()
	tri.MoveTo(0, 0)
	tri.LineTo(40, 0)
	tri.LineTo(0, 40)
	tri.Close()
	two := NewPath()
	two.AddRect(XYWH(0, 0, 20, 12))
	two.AddRect(XYWH(0, 12, 20, 8))
	draws := map[string]func(ctx *Context){
		"solid rect":    func(ctx *Context) { ctx.DrawRect(XYWH(0, 0, 20, 20), Fill(red)) },
		"gradient rect": func(ctx *Context) { ctx.DrawRect(XYWH(0, 0, 20, 20), Paint{Shader: grad}) },
		"gradient path": func(ctx *Context) { ctx.DrawPath(tri, Paint{Shader: grad}) },
		"rect list":     func(ctx *Context) { ctx.DrawPath(two, Fill(red)) },
		"stroke":        func(ctx *Context) { ctx.DrawLine(Pt(10, 0), Pt(10, 20), StrokePaint(red, 4)) },
		"image":         func(ctx *Context) { ctx.DrawImageRect(icon, XYWH(0, 0, 20, 20), XYWH(0, 0, 20, 20)) },
		"tinted image": func(ctx *Context) {
			ctx.DrawImageRectPaint(icon, XYWH(0, 0, 20, 20), XYWH(0, 0, 20, 20), Paint{Color: White})
		},
	}
	for name, draw := range draws {
		if g := redOn(t, 1, draw); !near(g, 0) {
			t.Errorf("%s at alpha 1: green %d, want 0", name, g)
		}
		if g := redOn(t, 0.5, draw); !near(g, 128) {
			t.Errorf("%s at alpha 0.5: green %d, want about 128", name, g)
		}
		if g := redOn(t, 0, draw); g != 255 {
			t.Errorf("%s at alpha 0: green %d, want untouched 255", name, g)
		}
	}
}

// The global alpha multiplies the paint's own opacity, and Save / Restore
// keep it.
func TestSetAlphaComposesAndRestores(t *testing.T) {
	red := RGBA(1, 0, 0, 1)
	g := redOn(t, 0.5, func(ctx *Context) {
		ctx.DrawRect(XYWH(0, 0, 20, 20), Fill(red).WithOpacity(0.5))
	})
	if !near(g, 191) {
		t.Fatalf("0.5 × 0.5: green %d, want about 191", g)
	}
	img := NewImage(20, 20)
	ctx := NewContext(img)
	ctx.Clear(White)
	ctx.Save()
	ctx.SetAlpha(0.25)
	if a := ctx.Alpha(); a != 0.25 {
		t.Fatalf("Alpha %v, want 0.25", a)
	}
	ctx.Restore()
	if a := ctx.Alpha(); a != 1 {
		t.Fatalf("Restore left alpha %v, want 1", a)
	}
	ctx.DrawRect(XYWH(0, 0, 20, 20), Fill(red))
	if _, gg, _, _ := img.PremulAt(10, 10); gg != 0 {
		t.Fatalf("after Restore the draw should be opaque, green %d", gg)
	}
}

// A recorded scene keeps the fade: the recorder sees the faded paint.
func TestSetAlphaRecords(t *testing.T) {
	rec := NewRecorder(20, 20)
	ctx := NewContextDevice(rec)
	ctx.SetAlpha(0.5)
	ctx.DrawRect(XYWH(0, 0, 20, 20), Fill(RGBA(1, 0, 0, 1)))
	img := NewImage(20, 20)
	img.Clear(White)
	DrawScene(rec.Finish(), NewCPUDevice(img))
	if _, g, _, _ := img.PremulAt(10, 10); !near(g, 128) {
		t.Fatalf("replayed fade: green %d, want about 128", g)
	}
}

// DrawLayer fades a many-layered drawing as one: a dark frame under a white
// fill at half opacity is half white over the background, not the darker
// mix two separately faded shapes give.
func TestDrawLayerIsGroupOpacity(t *testing.T) {
	face := func(ctx *Context) {
		ctx.DrawRect(XYWH(0, 0, 20, 20), Fill(Black))
		ctx.DrawRect(XYWH(2, 2, 16, 16), Fill(White))
	}
	img := NewImage(20, 20)
	ctx := NewContext(img)
	ctx.Clear(RGBA(1, 0, 0, 1))
	ctx.DrawLayer(XYWH(0, 0, 20, 20), 0.5, face)
	r, g, _, _ := img.PremulAt(10, 10)
	if !near(r, 255) || !near(g, 128) {
		t.Fatalf("group opacity: rgb(%d,%d) at the centre, want white at half over red (255,128)", r, g)
	}
	// SetAlpha alone darkens the centre: the black shows through.
	ctx.Clear(RGBA(1, 0, 0, 1))
	ctx.SetAlpha(0.5)
	face(ctx)
	ctx.SetAlpha(1)
	if r2, _, _, _ := img.PremulAt(10, 10); r2 >= r {
		t.Fatalf("per-draw alpha should be darker than the layer (red %d vs %d)", r2, r)
	}
	// The frame edge: black at half over red.
	ctx.Clear(RGBA(1, 0, 0, 1))
	ctx.Translate(3, 0) // the layer follows the transform
	ctx.DrawLayer(XYWH(0, 0, 20, 20), 0.5, face)
	if r, g, _, _ := img.PremulAt(4, 10); !near(r, 128) || g > 8 {
		t.Fatalf("layer frame at x=4: rgb(%d,%d), want half black over red", r, g)
	}
	// Alpha 0 draws nothing; alpha 1 draws straight through.
	img2 := NewImage(20, 20)
	c2 := NewContext(img2)
	c2.Clear(White)
	c2.DrawLayer(XYWH(0, 0, 20, 20), 0, face)
	if r, _, _, _ := img2.PremulAt(0, 0); r != 255 {
		t.Fatal("alpha 0 painted")
	}
	c2.DrawLayer(XYWH(0, 0, 20, 20), 1, face)
	if r, _, _, _ := img2.PremulAt(0, 0); r != 0 {
		t.Fatal("alpha 1 should paint the face as is")
	}
}

// A cross-fade mixes the two drawings: a box that is there only in from
// fades out over the background, and two opaque faces never let the
// background through mid-way.
func TestDrawCrossFade(t *testing.T) {
	box := func(col Color) func(*Context) {
		return func(ctx *Context) { ctx.DrawRect(XYWH(0, 0, 20, 20), Fill(col)) }
	}
	nothing := func(*Context) {}
	img := NewImage(20, 20)
	ctx := NewContext(img)
	ctx.Clear(White)
	ctx.DrawCrossFade(XYWH(0, 0, 20, 20), 0.75, box(Black), nothing)
	if r, _, _, _ := img.PremulAt(10, 10); !near(r, 191) {
		t.Fatalf("a quarter of the black box left over white: R=%d, want ~191", r)
	}
	ctx.Clear(White)
	ctx.DrawCrossFade(XYWH(0, 0, 20, 20), 0.5, box(RGBA(1, 0, 0, 1)), box(RGBA(0, 0, 1, 1)))
	if r, g, b, _ := img.PremulAt(10, 10); !near(r, 128) || !near(b, 128) || g > 8 {
		t.Fatalf("red to blue half way: rgb(%d,%d,%d), want (128,0,128) with no white", r, g, b)
	}
	ctx.Clear(White)
	ctx.DrawCrossFade(XYWH(0, 0, 20, 20), 1, box(Black), box(RGBA(0, 0, 1, 1)))
	if r, _, b, _ := img.PremulAt(10, 10); r != 0 || b != 255 {
		t.Fatal("t = 1 is the target drawing")
	}
}
