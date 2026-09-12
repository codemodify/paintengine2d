# Overnight status — 2026-09-12

UI paint sweep: scroll memmove, large fills, clip COW, resize in-place,
label LRU, GPU atlas sub-upload. CPU remains the fallback.

## Tip

- Branch: `dev`
- Version: **0.9.2**
- Prior `dev` tip: v0.9.1 (incremental hover)

## Enable GPU

```
UITK_PAINT=auto   # default: try EGL, else CPU
UITK_PAINT=gpu    # require GPU
UITK_PAINT=cpu    # force CPU (CGO_ENABLED=0 tests)
```

## Verified

- `CGO_ENABLED=0 go test ./...` — pass
- `CGO_ENABLED=1 go test ./...` — pass (Mesa llvmpipe)
- AA FillRect 280×24 on 1920×1080: **1.3 ms → 32 µs** (~40×)
- Circle r=12 on 1920×1080: **1.3 ms → 13 µs** (~100×)
- Menu hover (two rows + labels): **15 µs, 0 alloc**
- Warm DrawLabel: **0 alloc**
- Scroll 1920×1080: **0.27 ms, 0 alloc**
- Save/ClipRect/Fill churn: **1.4 µs, 0 alloc**
- Cached 8-label list: **11 µs, 0 alloc**

## Added this wave

- `Scroll` / `CopyImage` for list wheel ticks
- Row-memcpy Clear / opaque FillRect
- 48-entry DrawLabel LRU; Save COW clip masks
- `CPUSurface.Resize` keeps Device; `Context.SyncSize`
- GPU `TouchRect` sub-upload; preserved-buffer PresentRects

## Hygiene

- Author: `codemodify <codemodify@linux.com>`
