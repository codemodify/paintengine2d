package paintengine2d

import "testing"

func BenchmarkFillRect(b *testing.B) {
	img := NewImage(512, 512)
	ctx := NewContext(img)
	r := XYWH(16, 16, 480, 480)
	p := Fill(RGB(0.2, 0.4, 0.8))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.DrawRect(r, p)
	}
}

func BenchmarkFillComplexPath(b *testing.B) {
	img := NewImage(512, 512)
	ctx := NewContext(img)
	path := blobPath()
	p := Fill(RGB(0.9, 0.4, 0.2))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.DrawPath(path, p)
	}
}

func BenchmarkStroke(b *testing.B) {
	img := NewImage(512, 512)
	ctx := NewContext(img)
	path := blobPath()
	p := Paint{
		Color:  RGB(1, 1, 1),
		Style:  StyleStroke,
		Stroke: Stroke{Width: 6, Cap: CapRound, Join: JoinRound, MiterLimit: 4},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.DrawPath(path, p)
	}
}

func BenchmarkManySmallPaths(b *testing.B) {
	img := NewImage(512, 512)
	ctx := NewContext(img)
	p := Fill(RGB(0.3, 0.7, 0.4))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				ctx.DrawCircle(Pt(float32(x*32+16), float32(y*32+16)), 10, p)
			}
		}
	}
}

func BenchmarkGradientFill(b *testing.B) {
	img := NewImage(512, 512)
	ctx := NewContext(img)
	p := Linear(LinearGradient{
		Start: Pt(0, 0), End: Pt(512, 512),
		Stops: []GradientStop{{0, RGB(0.2, 0.4, 0.9)}, {1, RGB(0.9, 0.3, 0.2)}},
	})
	path := blobPath()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.DrawPath(path, p)
	}
}

func BenchmarkImageBlit(b *testing.B) {
	src := NewImage(128, 128)
	src.Clear(RGB(0.4, 0.5, 0.8))
	dst := NewImage(512, 512)
	ctx := NewContext(dst)
	sr := XYWH(0, 0, 128, 128)
	dr := XYWH(64, 64, 384, 384)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.DrawImageRect(src, sr, dr)
	}
}

func BenchmarkFillCircleUI(b *testing.B) {
	img := NewImage(64, 64)
	ctx := NewContext(img)
	p := Fill(RGB(0.2, 0.5, 0.9))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.DrawCircle(Pt(32, 32), 14, p)
	}
}

func BenchmarkStrokeRoundRectUI(b *testing.B) {
	img := NewImage(128, 48)
	ctx := NewContext(img)
	paint := Paint{Color: White, Style: StyleStroke, Stroke: Stroke{Width: 2, Cap: CapRound, Join: JoinRound, MiterLimit: 4}}
	r := XYWH(8, 8, 112, 32)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.DrawRoundRect(r, 6, 6, paint)
	}
}

func BenchmarkImageBlitNearestUI(b *testing.B) {
	src := NewImage(32, 32)
	src.Clear(RGB(0.4, 0.5, 0.8))
	dst := NewImage(64, 64)
	ctx := NewContext(dst)
	sr := XYWH(0, 0, 32, 32)
	dr := XYWH(8, 8, 32, 32)
	p := Paint{Color: White, Filter: FilterNearest}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.DrawImageRectPaint(src, sr, dr, p)
	}
}

func BenchmarkDrawLabel(b *testing.B) {
	img := NewImage(80, 20)
	ctx := NewContext(img)
	atlas := NewBitmapAtlas(White)
	paint := Paint{Color: White, Filter: FilterNearest}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.DrawLabel("READY", atlas, Pt(4, 4), paint)
	}
}

func BenchmarkClipPathRoundRect(b *testing.B) {
	img := NewImage(256, 256)
	p := RoundRectPath(XYWH(40, 40, 176, 176), 16, 16)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := NewContext(img)
		ctx.ClipPath(p)
		ctx.FillRect(XYWH(0, 0, 256, 256))
	}
}

func paintChrome(ctx *Context) {
	ctx.Clear(RGB(0.10, 0.11, 0.14))
	ctx.DrawRect(XYWH(0, 0, 640, 36), Linear(LinearGradient{
		Start: Pt(0, 0), End: Pt(640, 0),
		Stops: []GradientStop{{0, RGB(0.16, 0.18, 0.22)}, {1, RGB(0.12, 0.22, 0.28)}},
	}))
	atlas := NewBitmapAtlas(White)
	ctx.DrawLabel("APP", atlas, Pt(12, 12), Paint{Color: White, Filter: FilterNearest})
	for i := 0; i < 6; i++ {
		x := float32(16 + i*100)
		ctx.DrawRoundRect(XYWH(x, 56, 88, 32), 6, 6, Fill(RGB(0.23, 0.51, 0.93)))
		ctx.DrawRoundRect(XYWH(x, 56, 88, 32), 6, 6, Paint{
			Color: RGB(0.12, 0.28, 0.55), Style: StyleStroke,
			Stroke: Stroke{Width: 1, Cap: CapRound, Join: JoinRound, MiterLimit: 4},
		})
	}
	ctx.DrawRoundRect(XYWH(16, 104, 600, 220), 10, 10, Fill(RGB(0.16, 0.17, 0.20)))
	ctx.DrawRoundRect(XYWH(600, 112, 10, 120), 4, 4, Fill(RGB(0.45, 0.48, 0.55)))
	ctx.DrawCircle(Pt(80, 380), 28, Fill(RGB(0.2, 0.7, 0.45)))
	ctx.DrawRoundRect(XYWH(140, 352, 200, 48), 8, 8, Paint{
		Color: RGB(0.45, 0.75, 1.0), Style: StyleStroke,
		Stroke: Stroke{Width: 2, Cap: CapRound, Join: JoinRound, MiterLimit: 4},
	})
}

func BenchmarkChromeCPU(b *testing.B) {
	img := NewImage(640, 420)
	ctx := NewContext(img)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		paintChrome(ctx)
	}
}

func BenchmarkChromeGPU(b *testing.B) {
	if !GPUAvailable() {
		b.Skip("no EGL/GLES")
	}
	dev, err := NewGPUDevice(640, 420)
	if err != nil {
		b.Fatal(err)
	}
	defer dev.Close()
	ctx := NewContextDevice(dev)
	paintChrome(ctx)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		paintChrome(ctx)
	}
}

func BenchmarkFillRectSmallOnLarge(b *testing.B) {
	img := NewImage(1920, 1080)
	ctx := NewContext(img)
	p := Fill(RGB(0.23, 0.51, 0.93))
	r := XYWH(24, 80, 280, 24)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.DrawRect(r, p)
	}
}

func BenchmarkFillRectAASmallOnLarge(b *testing.B) {
	img := NewImage(1920, 1080)
	ctx := NewContext(img)
	p := Fill(RGBA(0.23, 0.51, 0.93, 0.35))
	r := XYWH(24.5, 80.25, 280, 24)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.DrawRect(r, p)
	}
}

func BenchmarkFillCircleSmallOnLarge(b *testing.B) {
	img := NewImage(1920, 1080)
	ctx := NewContext(img)
	p := Fill(RGB(0.2, 0.7, 0.4))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.DrawCircle(Pt(40, 40), 12, p)
	}
}

func BenchmarkDrawLabelWarm(b *testing.B) {
	img := NewImage(1920, 1080)
	ctx := NewContext(img)
	atlas := NewBitmapAtlas(White)
	paint := Paint{Color: White, Filter: FilterNearest}
	ctx.DrawLabel("Open File", atlas, Pt(32, 86), paint)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.DrawLabel("Open File", atlas, Pt(32, 86), paint)
	}
}

func BenchmarkMenuHoverCPU(b *testing.B) {
	img := NewImage(1920, 1080)
	ctx := NewContext(img)
	atlas := NewBitmapAtlas(White)
	bg := RGB(0.16, 0.17, 0.20)
	hi := RGB(0.23, 0.51, 0.93)
	label := Paint{Color: White, Filter: FilterNearest}
	row0 := XYWH(16, 80, 280, 24)
	row1 := XYWH(16, 104, 280, 24)
	ctx.Clear(bg)
	var dirty Damage
	ctx.SetDamage(&dirty)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		prev, next := row0, row1
		if i&1 == 0 {
			prev, next = row1, row0
		}
		dirty.Reset()
		dirty.Add(prev)
		dirty.Add(next)
		ctx.Save()
		ctx.ClipToDamage()
		ctx.ClearRect(prev, bg)
		ctx.DrawRect(next, Fill(hi))
		ctx.DrawLabel("Open File", atlas, Pt(24, prev.Min.Y+6), label)
		ctx.DrawLabel("Open File", atlas, Pt(24, next.Min.Y+6), label)
		ctx.Restore()
		_ = ctx.PresentDamage()
	}
}

func BenchmarkScrollViewport(b *testing.B) {
	img := NewImage(1920, 1080)
	ctx := NewContext(img)
	ctx.Clear(RGB(0.12, 0.13, 0.16))
	view := XYWH(0, 0, 1920, 1080)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.Scroll(0, 24, view)
	}
}

func BenchmarkClearLarge(b *testing.B) {
	img := NewImage(1920, 1080)
	ctx := NewContext(img)
	c := RGB(0.10, 0.11, 0.14)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.Clear(c)
	}
}

func BenchmarkFillRectLargeOpaque(b *testing.B) {
	img := NewImage(1920, 1080)
	ctx := NewContext(img)
	p := Fill(RGB(0.16, 0.17, 0.20))
	r := XYWH(0, 0, 1920, 1080)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.DrawRect(r, p)
	}
}

func BenchmarkSaveClipChurn(b *testing.B) {
	img := NewImage(1920, 1080)
	ctx := NewContext(img)
	p := Fill(RGB(0.23, 0.51, 0.93))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.Save()
		ctx.ClipRect(XYWH(16, float32(80+(i%40)*24), 400, 24))
		ctx.DrawRect(XYWH(16, float32(80+(i%40)*24), 400, 24), p)
		ctx.Restore()
	}
}

func BenchmarkDrawLabelList(b *testing.B) {
	img := NewImage(400, 800)
	ctx := NewContext(img)
	atlas := NewBitmapAtlas(White)
	paint := Paint{Color: White, Filter: FilterNearest}
	labels := []string{"Alpha", "Bravo", "Charlie", "Delta", "Echo", "Foxtrot", "Golf", "Hotel"}
	for _, s := range labels {
		ctx.DrawLabel(s, atlas, Pt(8, 8), paint)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for row, s := range labels {
			ctx.DrawLabel(s, atlas, Pt(8, float32(8+row*24)), paint)
		}
	}
}

func BenchmarkCopyImageLayer(b *testing.B) {
	src := NewImage(1920, 200)
	src.Clear(RGB(0.2, 0.3, 0.4))
	dst := NewImage(1920, 1080)
	ctx := NewContext(dst)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.CopyImage(src, XYWH(0, 0, 1920, 200), Pt(0, 400))
	}
}

func blobPath() *Path {
	p := NewPath()
	p.MoveTo(80, 260)
	p.CubicTo(40, 40, 220, 30, 260, 180)
	p.CubicTo(300, 330, 460, 80, 430, 280)
	p.CubicTo(400, 460, 140, 500, 80, 260)
	p.Close()
	return p
}
