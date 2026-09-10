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

func blobPath() *Path {
	p := NewPath()
	p.MoveTo(80, 260)
	p.CubicTo(40, 40, 220, 30, 260, 180)
	p.CubicTo(300, 330, 460, 80, 430, 280)
	p.CubicTo(400, 460, 140, 500, 80, 260)
	p.Close()
	return p
}
