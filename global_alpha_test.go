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
