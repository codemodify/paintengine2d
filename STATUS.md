# Overnight status — 2026-09-11

GPU milestone. CPU remains the fallback.

## Tip

- Branch: `dev`
- Version: **0.8.0**
- Prior `dev` tip: `02b2939` (v0.7.2)

## Enable GPU

```
UITK_PAINT=auto   # default: try EGL, else CPU
UITK_PAINT=gpu    # require GPU
UITK_PAINT=cpu    # force CPU (CGO_ENABLED=0 tests)
```

## Verified

- `CGO_ENABLED=0 go test ./...` — pass
- `CGO_ENABLED=1 go test ./...` — pass (Mesa llvmpipe)
- Chrome bench (640×420): CPU 6.24 ms/frame, GPU 1.61 ms/frame (~4×)

## Added this wave

- Device/Surface/Context seam works on CPU or GPU
- Linux EGL/GLES2 `GPUDevice` (stencil-and-cover, gradients, blit, clips)
- `NewGPUDeviceEGL` for uitoolkit Wayland/X11 swapchains

## Hygiene

- Author: `codemodify <codemodify@linux.com>`
