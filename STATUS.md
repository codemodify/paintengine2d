# Overnight status — 2026-09-12

Dirty DrawScene, GPU partial present, pane-group layer blit. CPU remains
the fallback.

## Tip

- Branch: `dev`
- Version: **0.10.0**
- Prior `dev` tip: v0.9.2 (scroll / clip COW / TouchRect)

## Enable GPU

```
UITK_PAINT=auto   # default: try EGL, else CPU
UITK_PAINT=gpu    # require GPU
UITK_PAINT=cpu    # force CPU (CGO_ENABLED=0 tests)
```

## uitoolkit bump

`go get github.com/codemodify/paintengine2d@v0.10.0`

Use `DrawSceneDamage` + `Present` / `PresentRects` (not raw `eglSwapBuffers`).
`BakeGroup` both splitter panes; drag = `Xform` + blit.

## Hygiene

- Author: `codemodify <codemodify@linux.com>`
