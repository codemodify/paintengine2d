//go:build linux && cgo

package paintengine2d

import (
	"testing"
	"unsafe"
)

func TestGPUFlattenCacheAndAtlasEpoch(t *testing.T) {
	if !GPUAvailable() {
		t.Skip("no EGL/GLES")
	}
	dev, err := NewGPUDevice(64, 48)
	if err != nil {
		t.Fatal(err)
	}
	defer dev.Close()
	p := CirclePath(Pt(24, 24), 14)
	paint := Fill(RGB(0.2, 0.6, 0.9))
	clip := Clip{}
	xf := Identity()
	dev.Fill(p, xf, paint, clip)
	miss := dev.tess.misses
	hits := dev.tess.hits
	fl := dev.tess.flattens
	dev.Fill(p, xf, paint, clip)
	if dev.tess.flattens != fl || dev.tess.hits != hits+1 {
		t.Fatalf("second fill should hit tess cache flatten=%d→%d hits=%d→%d", fl, dev.tess.flattens, hits, dev.tess.hits)
	}
	if miss == 0 && hits == 0 {
		t.Fatal("expected a cache miss on the first fill")
	}

	atlas := NewImage(8, 8)
	atlas.Clear(White)
	e0 := atlas.Epoch
	dev.Blit(atlas, XYWH(0, 0, 8, 8), XYWH(4, 4, 8, 8), xf, Paint{Color: White, Filter: FilterNearest}, clip)
	key := uintptr(unsafe.Pointer(atlas))
	tex1, ok := dev.texCache[key]
	if !ok {
		t.Fatal("expected atlas in tex cache")
	}
	atlas.Clear(RGB(0.2, 0.9, 0.3))
	if atlas.Epoch == e0 {
		t.Fatal("atlas clear should bump epoch")
	}
	dev.Blit(atlas, XYWH(0, 0, 8, 8), XYWH(4, 4, 8, 8), xf, Paint{Color: White, Filter: FilterNearest}, clip)
	tex2 := dev.texCache[key]
	if tex2.epoch != atlas.Epoch {
		t.Fatalf("tex cache epoch %d want %d", tex2.epoch, atlas.Epoch)
	}
	_ = tex1
}

func TestGPUDrawSceneBatchesRects(t *testing.T) {
	if !GPUAvailable() {
		t.Skip("no EGL/GLES")
	}
	rec := NewRecorder(80, 40)
	ctx := NewContextDevice(rec)
	ctx.Clear(RGB(0.1, 0.1, 0.12))
	ctx.DrawRect(XYWH(4, 4, 16, 12), Fill(RGB(0.8, 0.2, 0.2)))
	ctx.DrawRect(XYWH(28, 4, 16, 12), Fill(RGB(0.8, 0.2, 0.2)))
	ctx.DrawRect(XYWH(52, 4, 16, 12), Fill(RGB(0.8, 0.2, 0.2)))
	scene := rec.Finish()

	dev, err := NewGPUDevice(80, 40)
	if err != nil {
		t.Fatal(err)
	}
	defer dev.Close()
	DrawScene(scene, dev)
	img := dev.Snapshot()
	if img == nil {
		t.Fatal("snapshot")
	}
	_, _, _, a := img.PremulAt(10, 8)
	if a < 200 {
		t.Fatalf("batched rect a=%d", a)
	}
	_, _, _, a = img.PremulAt(34, 8)
	if a < 200 {
		t.Fatalf("second batched rect a=%d", a)
	}
}

func TestGPUFillRectSkipsTessCache(t *testing.T) {
	if !GPUAvailable() {
		t.Skip("no EGL/GLES")
	}
	dev, err := NewGPUDevice(64, 48)
	if err != nil {
		t.Fatal(err)
	}
	defer dev.Close()
	p := RectPath(XYWH(4, 6, 20, 12))
	clip := Clip{HasScissor: true, Scissor: XYWH(0, 0, 64, 48)}
	dev.Fill(p, Identity(), Fill(RGB(0.2, 0.5, 0.9)), clip)
	if dev.tess.flattens != 0 || dev.tess.misses != 0 {
		t.Fatalf("axis-aligned FillRect should skip tess flatten=%d miss=%d", dev.tess.flattens, dev.tess.misses)
	}
	img := dev.Snapshot()
	_, _, _, a := img.PremulAt(10, 10)
	if a < 200 {
		t.Fatalf("gpu rect a=%d", a)
	}
}

func TestGPUClearRectAndPresentRects(t *testing.T) {
	if !GPUAvailable() {
		t.Skip("no EGL/GLES")
	}
	dev, err := NewGPUDevice(48, 32)
	if err != nil {
		t.Fatal(err)
	}
	defer dev.Close()
	ctx := NewContextDevice(dev)
	ctx.Clear(RGB(0.10, 0.11, 0.14))
	ctx.ClearRect(XYWH(4, 4, 12, 8), RGB(0.9, 0.2, 0.2))
	img := ctx.Image()
	r, _, _, a := img.PremulAt(8, 6)
	if a < 200 || r < 200 {
		t.Fatalf("cleared rect rgba r=%d a=%d", r, a)
	}
	r, _, _, a = img.PremulAt(30, 20)
	if r > 80 || a < 20 {
		t.Fatalf("outside ClearRect should stay background r=%d a=%d", r, a)
	}
	if err := ctx.PresentRects([]Rect{XYWH(4, 4, 12, 8)}); err != nil {
		t.Fatal(err)
	}
}
