package raster

import "testing"

func FuzzCoverageRow(f *testing.F) {
	f.Add(uint8(0), uint8(2), uint8(8), uint8(2), uint8(10), uint8(8))
	f.Fuzz(func(t *testing.T, x0, y0, x1, y1, x2, y2 uint8) {
		c := [][]Vec2{{{float32(x0), float32(y0)}, {float32(x1), float32(y1)}, {float32(x2), float32(y2)}, {float32(x0), float32(y0)}}}
		edges := BuildEdges(c, nil)
		var r Rasterizer
		r.ResetEdges(edges)
		cover := r.CoverageRow(int(y1%16), 16, FillNonZero, 0, 16)
		for _, v := range cover {
			if v > 255 {
				t.Fatalf("coverage %d", v)
			}
		}
		_ = ExpandStroke(c, []bool{true}, StrokeOpts{Width: float32(1 + x0%8), Cap: int(y0 % 3), Join: int(x1 % 3), MiterLimit: 4})
	})
}
