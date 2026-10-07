package paintengine2d

import "testing"

// Sampling is state on the texture object in GLES2, not part of the draw call,
// so a cached texture went on sampling the way it was first created however
// the caller asked for it afterwards. uploadImage passed the requested filter
// to the texture it *created* and returned a cache hit without touching it, so
// an unchanged image blitted once bilinear and then again nearest came back
// blended the second time — and the cache entry did not even record which
// filter it was set to.
//
// Both filters are legitimately wanted for one image: an ImagePattern always
// asks for nearest so a dither stays crisp, while a Blit asks for whatever its
// paint says. A caller should not have to keep two copies of an image.

// checkerImage is pure black and white, so any third colour in a drawing of it
// came from blending.
func checkerImage(n, cell int) *Image {
	img := NewImage(n, n)
	ctx := NewContext(img)
	ctx.Clear(Black)
	for y := 0; y < n; y += cell {
		for x := 0; x < n; x += cell {
			if (x/cell+y/cell)%2 == 0 {
				ctx.DrawRect(XYWH(float32(x), float32(y), float32(cell), float32(cell)), Fill(White))
			}
		}
	}
	img.Touch()
	return img
}

// blendedInterior counts pixels that are neither black nor white, ignoring a
// margin: the drawing's own outer edge blends against the ground whatever the
// filter is.
func blendedInterior(img *Image, margin int) int {
	n := 0
	for y := margin; y < img.Height-margin; y++ {
		for x := margin; x < img.Width-margin; x++ {
			r, g, b, a := img.PremulAt(x, y)
			if a < 250 {
				continue
			}
			black := r < 8 && g < 8 && b < 8
			white := r > 247 && g > 247 && b > 247
			if !black && !white {
				n++
			}
		}
	}
	return n
}

func TestASecondBlitGetsTheFilterItAsksFor(t *testing.T) {
	const side = 64
	src := checkerImage(8, 2)
	dst := XYWH(0, 0, side, side) // magnified 8x, so the filters differ visibly

	// Nearest on its own first, as the measure of what nearest looks like.
	alone := gpuDev(t, side, side)
	defer alone.Close()
	alone.Clear(Transparent)
	alone.Blit(src, XYWH(0, 0, 8, 8), dst, Identity(), Paint{Filter: FilterNearest}, Clip{})
	want := blendedInterior(alone.Snapshot(), 4)
	if want != 0 {
		t.Skipf("nearest alone already blends %d interior pixels on this driver", want)
	}

	// Now the same image, bilinear first and nearest second. The second draw
	// is a cache hit on an unchanged image, which is the whole case.
	d := gpuDev(t, side, side)
	defer d.Close()
	d.Clear(Transparent)
	d.Blit(src, XYWH(0, 0, 8, 8), dst, Identity(), Paint{Filter: FilterBilinear}, Clip{})
	d.Clear(Transparent)
	d.Blit(src, XYWH(0, 0, 8, 8), dst, Identity(), Paint{Filter: FilterNearest}, Clip{})
	if got := blendedInterior(d.Snapshot(), 4); got != 0 {
		t.Errorf("after a bilinear blit, a nearest blit of the same image blended %d interior pixels", got)
	}
}

// And the other way round: nearest first, then bilinear, must actually blend.
// A fix that simply forced nearest everywhere would pass the test above.
func TestASecondBlitCanAskForBilinearToo(t *testing.T) {
	const side = 64
	src := checkerImage(8, 2)
	dst := XYWH(0, 0, side, side)
	d := gpuDev(t, side, side)
	defer d.Close()
	d.Clear(Transparent)
	d.Blit(src, XYWH(0, 0, 8, 8), dst, Identity(), Paint{Filter: FilterNearest}, Clip{})
	d.Clear(Transparent)
	d.Blit(src, XYWH(0, 0, 8, 8), dst, Identity(), Paint{Filter: FilterBilinear}, Clip{})
	if got := blendedInterior(d.Snapshot(), 4); got == 0 {
		t.Error("after a nearest blit, a bilinear blit of the same image blended nothing")
	}
}

// The cache records what the texture is set to, so the second draw of a pair
// that wants the same filter costs nothing.
func TestTheCacheRemembersTheFilterItSet(t *testing.T) {
	const side = 32
	src := checkerImage(8, 2)
	d := gpuDev(t, side, side)
	defer d.Close()
	dst := XYWH(0, 0, side, side)
	d.Blit(src, XYWH(0, 0, 8, 8), dst, Identity(), Paint{Filter: FilterNearest}, Clip{})
	e, ok := d.texCache[src.UID()]
	if !ok {
		t.Fatal("the image is not in the texture cache")
	}
	if e.filter != FilterNearest {
		t.Errorf("the entry records %v, want the filter the blit asked for", e.filter)
	}
	d.Blit(src, XYWH(0, 0, 8, 8), dst, Identity(), Paint{Filter: FilterBilinear}, Clip{})
	if e.filter != FilterBilinear {
		t.Errorf("after a bilinear blit the entry records %v", e.filter)
	}
}
