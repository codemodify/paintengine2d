# Overnight status — 2026-09-11

GPU flatten cache + atlas epoch. CPU remains the fallback.

## Tip

- Branch: `dev`
- Version: **0.8.1**
- Prior `dev` tip: `b220e31` (v0.8.0)

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

- GPU flatten/tessellation cache (warm Fill/Stroke skip CPU flatten)
- `Image.Epoch` / `Image.Touch` so rebaked atlases re-upload

## Hygiene

- Author: `codemodify <codemodify@linux.com>`
