# Overnight status — 2026-09-11

Retained Scene / Recorder / DrawScene. CPU remains the fallback.

## Tip

- Branch: `dev`
- Version: **0.9.0**
- Prior `dev` tip: v0.8.1 (flatten cache + atlas epoch)

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

- `Scene` / `Recorder` / `DrawScene` retained graph
- GPU opaque axis-aligned rect batches
- v0.8.1 flatten/tessellation cache + `Image.Epoch` still on `dev`

## Hygiene

- Author: `codemodify <codemodify@linux.com>`
