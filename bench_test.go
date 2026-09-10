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

func blobPath() *Path {
	p := NewPath()
	p.MoveTo(80, 260)
	p.CubicTo(40, 40, 220, 30, 260, 180)
	p.CubicTo(300, 330, 460, 80, 430, 280)
	p.CubicTo(400, 460, 140, 500, 80, 260)
	p.Close()
	return p
}
