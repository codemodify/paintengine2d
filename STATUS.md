# Status — 2026-09-13

Correctness release. Review fixes across the rasterizer, the dirty-rect
replay and the GPU backend; parent-space group clips; explicit paint
opacity. CPU remains the fallback.

## Tip

- Branch: `dev`
- Version: **0.11.0**
- Prior `dev` tip: v0.10.0 (dirty DrawScene / GPU partial present / BakeGroup)

## Enable GPU

```
UITK_PAINT=auto     # default: try EGL, else CPU
UITK_PAINT=gpu      # require GPU
UITK_PAINT=cpu      # force CPU (CGO_ENABLED=0 tests)
UITK_PAINT_MSAA=0   # GPU without the multisample target (software GL)
```

## uitoolkit bump

`go get github.com/codemodify/paintengine2d@v0.11.0`

`DrawSceneDamage` is now safe to use: it replays once per dirty box, never
skips a group that holds a `Clear`, and pads blits. Re-enable it in place of
the full-surface `DrawScene` + full `Present`.

Scroll views: put the viewport on the cached group
(`g.SetClipRect(viewport)`) and scroll with `g.Xform`. The group clip is in
the parent's space, so the content slides under a viewport that stays put —
do not bake the viewport into the ops' own clips.

Fades: set `Paint.Opacity` instead of rewriting colors (and note that a tint
with `Color.A == 0` is now invisible, as it always should have been).

Per-frame pixmaps blitted on the GPU: call `GPUDevice.ReleaseImage` when
done, or let the LRU budget evict them.

Check `Context.Err()` once per frame — the GPU backend records lost
contexts, failed swaps and wrong-thread use there instead of failing mute.

## Hygiene

- Author: `codemodify <codemodify@linux.com>`
