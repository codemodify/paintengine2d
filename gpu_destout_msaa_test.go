//go:build linux && cgo

package paintengine2d

import (
	"os"
	"testing"
)

// An erase through a one-byte image under multisampling: every pixel the
// eraser says to take away entirely must end up entirely transparent, and
// every pixel it leaves alone untouched.
//
// It does not hold on every GPU. On Mesa's iris (Intel Arrow Lake, 4x MSAA)
// whole 2x2 blocks next to a change in the eraser are left unwritten — 318
// pixels round a disc drawn as one quad — whatever the filter, the texture
// format or how the texel is chosen, and never without multisampling;
// per-sample shading hides most of it but not all (8 pixels on the quads'
// diagonals). So it is a probe to run by hand (PE_PROBE_MSAA_DESTOUT=1),
// not a promise, and uitoolkit wipes the pixels it needs entirely
// transparent with a fill of whole pixels instead of trusting this.
var testTile = 0

func TestGPUDestOutImageIsSampleExact(t *testing.T) {
	if os.Getenv("PE_PROBE_MSAA_DESTOUT") == "" {
		t.Skip("a probe of the GPU's multisampled dest-out; set PE_PROBE_MSAA_DESTOUT=1")
	}
	const n = 200
	dev, err := NewGPUDevice(n, n)
	if err != nil {
		t.Skipf("no GPU: %v", err)
	}
	defer dev.Close()
	t.Logf("samples %d", dev.Samples())
	ctx := NewContextDevice(dev)
	ctx.Clear(Transparent)
	// Saturated content with antialiased path edges everywhere.
	for i := 0; i < 40; i++ {
		p := NewPath()
		p.AddCircle(Pt(float32(10+i*5), float32(100)), float32(30+i%7))
		ctx.DrawPath(p, Paint{Color: RGBA(1, 0, float32(i%2), 1), AntiAlias: true})
	}
	ctx.DrawRect(XYWH(0, 0, n, n), Paint{Color: RGBA(0, 0.8, 0, 1)})
	// The eraser: a disc's inverse coverage.
	cov := NewImageA8(n, n)
	cc := NewContext(cov)
	hole := NewPath()
	hole.AddRect(XYWH(0, 0, n, n))
	hole.AddCircle(Pt(n/2, n/2), 70.3)
	cc.DrawPath(hole, Paint{Color: White, AntiAlias: true, FillRule: FillEvenOdd})
	tile := 25
	if testTile > 0 {
		tile = testTile
	}
	for y := 0; y < n; y += tile {
		for x := 0; x < n; x += tile {
			r := XYWH(float32(x), float32(y), float32(tile), float32(tile))
			ctx.DrawImageRectPaint(cov, r, r, Paint{Color: White, Blend: BlendDestOut, Filter: FilterNearest})
		}
	}
	img := dev.Snapshot()
	bad := 0
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			e := cov.Pix[y*cov.RowStride()+x]
			_, _, _, a := img.At(x, y).RGBA()
			a >>= 8
			if (e == 255 && a != 0) || (e == 0 && a != 255) {
				if bad < 12 {
					t.Logf("px %d,%d eraser %d alpha %d", x, y, e, a)
				}
				bad++
			}
		}
	}
	if bad > 0 {
		t.Fatalf("%d pixels not erased exactly", bad)
	}
}
