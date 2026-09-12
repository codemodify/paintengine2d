# Overnight status — 2026-09-12

Incremental paint: tight dirty rects, label reuse, GPU rect quad +
swap-with-damage. CPU remains the fallback.

## Tip

- Branch: `dev`
- Version: **0.9.1**
- Prior `dev` tip: v0.9.0 (retained Scene / Recorder)

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

## Added this wave

- Tight CPU paint bounds (geometry ∩ clip) for AA rects and paths
- `ClipToDamage` / `ClearRect` / `PresentDamage` for uitoolkit hover
- GPU FillRect skips stencil; `eglSwapBuffersWithDamage` when available
- Warm `DrawLabel` 0-alloc; glyph skip outside clip

## Hygiene

- Author: `codemodify <codemodify@linux.com>`
