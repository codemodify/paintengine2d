# Changelog

All notable changes to this project are documented in this file.

## 0.10.0 — 2026-09-12

Dirty `DrawScene`, GPU partial present, and pane-group layer replay —
the three engine blockers uitoolkit v0.14.0 documented after its dirty-clip
hover cut.

### Added

- `DrawSceneDamage` / `DrawSceneRects` — replay a [Scene] clipped to dirty
  device boxes. Recorded `Clear` becomes `ClearRect`. Ops whose bounds miss
  every dirty rect are skipped. `DrawScene` is the full-surface form.
- `BakeGroup` / `GroupLocalBounds` / `GroupNode.Layer` / `InvalidateLayer`
  — bake a pane once; [DrawScene] blits the layer through `Xform` and does
  not re-walk children (splitter drag). Opaque integer translations
  `CopyFrom` (memcpy) on CPU.
- `GPUDevice.SetPresentDamage` — [DrawSceneDamage] stores the dirty list so
  `Present()` swap-with-damage without a second copy. Empty dirty skips swap.
- `GPUDevice.PartialUpdate` / `SwapPreserves` — EGL capability queries.
- `Clip.IntersectDevice` — tighten a device scissor (used by dirty replay).
- Benches: `BenchmarkDrawSceneFull`, `BenchmarkDrawSceneDamageHover`,
  `BenchmarkSplitterReraster`, `BenchmarkSplitterLayerBlit`

### Changed

- GPU window configs try `EGL_SWAP_BEHAVIOR_PRESERVED_BIT` first.
- `PresentRects` scissor-blits dirty boxes when `EGL_BUFFER_PRESERVED` **or**
  `EGL_KHR_partial_update` (eglSetDamageRegionKHR) is available and buffer
  age is > 0. Swap still carries damage even when the blit must be full.

### uitoolkit

Module **v0.10.0**. Bump with:

```
go get github.com/codemodify/paintengine2d@v0.10.0
```

```go
DrawSceneDamage(scene, dev, &dirty) // not Clear + DrawScene
_ = ctx.Present()                   // uses damage from DrawSceneDamage
// or: _ = ctx.PresentRects(dirty.Rects)  // not raw eglSwapBuffers

BakeGroup(leftPane)
BakeGroup(rightPane)
leftPane.Xform = Translation(0, 0)
rightPane.Xform = Translation(splitX, 0)
DrawSceneDamage(scene, dev, &dirty)
```

Do not full-`Clear` on hover. Do not re-record pane children on splitter
drag — change `Xform` and blit the baked layers.

## 0.9.2 — 2026-09-12

Broader UI paint sweep: scrolling, large fills, clip-stack churn, resize
storms, glyph cache under rapid scroll, GPU atlas sub-upload.

### Added

- `Image.Scroll` / `Context.Scroll` / `CPUDevice.Scroll` / `GPUDevice.Scroll`
  — memmove (CPU) or FBO copy (GPU) for list/document wheel ticks
- `Image.CopyFrom` / `Context.CopyImage` — SRC stamp of a cached layer
- `Image.TouchRect` / `BumpRect` + `Image.Dirty` — GPU `glTexSubImage2D`
  of one glyph cell instead of re-uploading the atlas
- `Context.SyncSize` — reset root clip after `Surface.Resize`
- `CPUDevice.SetImage` — retarget without dropping flatten/stroke scratch
- `GPUDevice.SnapshotRect` — cropped read-back
- Benches: `BenchmarkScrollViewport`, `BenchmarkClearLarge`,
  `BenchmarkFillRectLargeOpaque`, `BenchmarkSaveClipChurn`,
  `BenchmarkDrawLabelList`, `BenchmarkCopyImageLayer`

### Changed

- `Image.Clear` / opaque `FillRect` / `ClearRect` fill one row and memcpy
- `DrawLabel` keeps a 48-entry shaped-run LRU (list scroll hits)
- `Save` shares clip masks (COW) and skips paint clone when solid / no dash
- `CPUSurface.Resize` keeps the same `CPUDevice` (live Context stays valid)
- GPU window surfaces request `EGL_BUFFER_PRESERVED`; `PresentRects` then
  blits only dirty boxes (full blit if the surface did not preserve)

### uitoolkit

Module **v0.9.2**. Prefer:

```go
ctx.Scroll(0, dy, view)           // not redraw the whole list
ctx.ClearRect(exposed, bg)
// paint only the new strip
ctx.SyncSize()                    // after Surface.Resize
atlas.Image.TouchRect(cell)       // after packing one glyph
_ = ctx.PresentDamage()
```

Do not `Clear` the full surface on scroll or hover. Do not `Bump()` the
whole atlas when one cell changed.

## 0.9.1 — 2026-09-12

Incremental paint for desktop UI hover (menu row highlight vs full-surface
redraw). Aligns with Qt/KDE dirty-rect / swap-with-damage expectations.

### Added

- `Context.ClipDeviceRect` / `Context.ClipToDamage` — apply a dirty union
  as a device-space scissor so a hover cannot walk the full pixmap
- `Context.ClearRect` / `Image.ClearRect` / `CPUDevice.ClearRect` /
  `GPUDevice.ClearRect` — partial erase (honors clip; `Clear` still
  resets the whole surface)
- `Context.Present` / `PresentRects` / `PresentDamage` — GPU swap; when
  EGL_KHR/EXT_swap_buffers_with_damage is present the compositor gets
  the dirty boxes
- Microbenches: `BenchmarkFillRectSmallOnLarge`,
  `BenchmarkFillRectAASmallOnLarge`, `BenchmarkFillCircleSmallOnLarge`,
  `BenchmarkDrawLabelWarm`, `BenchmarkMenuHoverCPU`

### Changed

- CPU AA / path fill intersects the clip with the geometry AABB (+1 px
  AA pad). A translucent menu row on a 1920×1080 surface no longer
  scans every clip row
- `CoverageRow` / `RectCoverage` clear and clamp only the clip X span
- GPU `Fill` of an axis-aligned solid/gradient rect skips stencil-and-cover
  (colored quad; tess cache is not touched)
- `DrawPath` / `DrawGlyphs` skip geometry that `QuickReject`s the clip
- `DrawLabel` reuses a shaped `GlyphRun` on the Context (warm 0-alloc)
- Clip-path mask and GPU mask upload reuse scratch buffers
- 1:1 nearest blit skips `maskAt` when there is no path mask

### uitoolkit

Module **v0.9.1**. After recording invalidations:

```go
ctx.SetDamage(&dirty)
ctx.Save()
ctx.ClipToDamage()
ctx.ClearRect(row, bg)      // not ctx.Clear
ctx.DrawRect(row, highlight)
ctx.DrawLabel(title, atlas, origin, paint)
ctx.Restore()
_ = ctx.PresentDamage()
```

Do not call `Clear` on hover. `Device` is unchanged; the new methods are
optional type assertions.

## 0.9.0 — 2026-09-11

Retained scene graph. Widgets record once; the compositor replays and
the GPU batches opaque axis-aligned rects. CPU [Device] is unchanged.
Pure Go API; CGO GPU remains Linux-only.

### Added

- `Recorder` — a [Device] that captures draws into a [Scene]
- `Scene` / `GroupNode` / `Recorder.BeginGroup` / `EndGroup` / `Attach`
- `DrawScene` replays onto any [Device]
- GPU batches consecutive opaque axis-aligned rect fills
- `Image.Bump` (atlas epoch; alias of `Touch`) for in-place glyph packs
- Tests: recorder matches immediate CPU paint; attach + group xform;
  scratch-path clone; GPU rect replay

## 0.8.1 — 2026-09-11

GPU hot-path reuse. Fill/Stroke no longer CPU-flatten identical geometry
every draw. Glyph/icon atlases re-upload when pixels change.

### Added

- GPU flatten + tessellation cache keyed on path content, transform,
  fill rule, and stroke style (triangle fans reused on cache hit)
- `Image.Epoch` / `Image.Bump` / `Image.Touch` — atlas epoch for in-place pixmap updates
- `FontAtlas.Epoch` reads the sheet's image epoch
- Tests: cache hits across path instances; epoch bump on Clear/SetColor/Touch

### Changed

- `GPUDevice.Fill` / `Stroke` look up cached contours and fans
- `GPUDevice` texture cache includes `Image.Epoch` so a rebaked atlas
  is not stuck on stale GPU texels

## 0.8.0 — 2026-09-11

GPU milestone. Pure-Go API; optional CGO for Linux EGL / OpenGL ES 2.
CPU remains the fallback and the `CGO_ENABLED=0` path.

### Added

- `Surface` / `CPUSurface` / `OpenSurface` / `NewContextSurface`
- `UITK_PAINT=cpu|gpu|auto` (`EnvPaint`, `ParsePaintPref`)
- `GPUDevice` — Linux EGL/GLES2 stencil-and-cover fill and stroke,
  linear/radial 1D ramps, textured blit + tint, clip-mask upload,
  offscreen pbuffer/FBO and `NewGPUDeviceEGL` for toolkit windows
- Opaque EGL window configs (`EGL_ALPHA_SIZE` 0) so GPU present cannot
  revive a fully transparent ARGB window
- Tests: GPU fill/stroke/blit/clip (skipped without EGL)
- Benches: `BenchmarkChromeCPU` / `BenchmarkChromeGPU`

### Changed

- `Context.Image` read-backs a GPU snapshot
- README / godoc: GPU is shipping, not a hook

## 0.7.2 — 2026-09-11

Blit RGB tint so a white icon/glyph atlas can be themed from `Paint.Color`.
Still pixels-only: no widgets, no windowing.

### Added

- `Paint.Color` RGB multiplies premul src-over blit samples (`Device.Blit`,
  `DrawImageRectPaint`, `DrawGlyphs` / `DrawLabel`). Alpha still modulates
  coverage. Zero-value paint stays unmodulated.
- Goldens: `image_tint`, `glyph_tint` (37 scenes)
- Tests: white-atlas tint, premul multiply, 1:1 nearest + scaled bilinear,
  tinted glyphs, tinted 1:1 0-alloc

## 0.7.1 — 2026-09-11

Production hardening on the v0.7 UI-foundation surface. Still pixels-only:
no widgets, no windowing.

### Added

- `Context.ClipEmpty`, `Context.ClipRoundRect`, `Damage.Count`
- Goldens: `ui_focus_ring`, `ui_scroll_thumb`, `ui_overlap_damage`,
  `aa_thin_diag`, `aa_tiny_glyphs`, `clip_xform_edge`, `wrap_shm_pad`
  (35 scenes)
- Fuzz: `FuzzClipTransform`
- Tests: focus ring, scroll thumb, overlapping damage, thin-diagonal AA,
  tiny glyphs, clip∩transform, padded shm stress, 1:1 blit 0-alloc

### Changed

- Stroke expansion reuses a `StrokePool` on `CPUDevice` (warm stroke 0 alloc)
- Flatten no longer allocates a leftover contour slice per draw
- Clip-path scratch pixmap grows instead of reallocating on size mismatch
- Rasterizer borrows the edge slice (no per-draw copy)

### Overnight status

`CGO_ENABLED=0 go test ./...` green. Short fuzz campaigns (8s each, including
`FuzzClipTransform`) green. FillRect, 1:1 nearest blit, warm path fill, and
warm stroke are 0 alloc. Stale PRs #7–#10 closed (work already on `dev`).

## 0.7.0 — 2026-09-10

**UI-foundation ready.** The CPU paint core is complete enough to start a
separate desktop UI framework (and a WM/DE) on top. This repo stays
pixels-only: no widgets, no windowing.

### Added

- `Context.DrawArc`, `Context.StrokeRect`, `Context.ClipPathRule`
- `GlyphRun.Bounds` for label hit-testing and dirty inflation
- Bitmap atlas lowercase aliases and `/ _ % ( ) =` punctuation
- Integer 1:1 nearest blit fast path (labels / icons)
- Goldens: `arc_stroke`, `evenodd_clip`, `ui_button`, `dash_offset`,
  `rotated_blit`, `radial_tiles` (28 scenes)
- Fuzz: `FuzzWrapImage`, `FuzzDamage`, `FuzzTextHooks`

### Fixed

- `Damage` merges overlapping boxes before `MaxRects` collapse
- `SetStroke(width <= 0)` is a no-op (no silent 1px hairline)
- Draw damage is clipped to scissor ∩ canvas

### Docs

- API stability contract for a UI kit
- UI-foundation checklist in the README

## 0.6.0 — 2026-09-10

Shared engine for two future consumers: a UI framework and a WM/DE.
Still no windowing, widgets, or compositor in this repo.

### Added

- `WrapImage` + `Image.Stride` / `RowStride` so a toolkit or WM can paint
  into an existing packed or padded premul buffer (swapchain / shm / surface)
- README layering: Engine → UI framework → apps; Engine → WM/DE → surfaces

## 0.5.0 — 2026-09-10

Foundation APIs for a forthcoming desktop UI framework. Still no widgets.

### Added

- `Context.Size`, `SaveCount`, `DeviceClipBounds`, `LocalClipBounds`, `QuickReject`
- `Damage.Overlaps` so a retained layer can skip clean widgets
- README / godoc: this is the paint engine for that framework, not a toolkit

## 0.4.0 — 2026-09-10

UI-engine quality bar: correctness, AA, alloc tightness, denser regression.

### Changed

- Stroke flattening uses a device-pixel tolerance (`0.2 / ApproxScale`)
- Scanline intersections at a shared X merge winding (vertex-safe)
- Round caps/joins tessellate from radius (chord ~0.35 px)
- `ClipPath` rasterizes only the path's device bounds
- Flatten and stroke-outline slices are reused across draws
- Non-finite path points and matrices are ignored (no panic)

### Added

- Goldens: `roundrect_ui`, `stroke_scaled`, `diagonal_aa`, `glyph_clip`
- Robustness suite (NaN, bowtie, scaled stroke, nested path clip)
- `FuzzCoverageRow`; UI-sized benches (circle, round-rect stroke, label, clip)
- Godoc Skia/JUCE mapping and performance contracts

## 0.3.0 — 2026-09-10

UI-subset production bar: dirty-rect helper and text hooks.

### Added

- `Damage` coalesce / clip / max-rects collapse; `Context.SetDamage` records
  device-space dirty boxes on draw
- Text hooks: `FontAtlas`, `AtlasCell`, `GlyphRun`, `Shaper`, `NullShaper`,
  `Context.DrawGlyphs`, `Context.DrawLabel`
- `NewBitmapAtlas` — built-in 5×7 ASCII sheet for labels (not a shaper)
- Golden `ui_label`; README repositioned as a UI paint layer, not Skia

### Framework work that stays above this library

IME, OpenType/HarfBuzz, bidi, line breaking, accessibility, widget layout.

## 0.2.0 — 2026-09-10

Production CPU bar: radial gradients, dashed strokes, image filter modes,
fuzz targets, and a larger regression suite.

### Added

- `RadialGradient` (center / inner / radius, optional focal, tile modes)
- SVG-style `Stroke.Dash` + `DashOffset` (odd arrays doubled; width ≤ 0 is a no-op)
- `FilterNearest` and `FilterBilinear` on `Paint` for image blits
- `BlendMode` field (only `BlendSrcOver` is implemented; others are not faked)
- Native Go fuzz: `FuzzPathBuild`, `FuzzMatrix`, `FuzzRasterDraw`, `FuzzClipAndImage`
- Goldens: `radial`, `dashes`, `image_filter`
- Benches: `BenchmarkGradientFill`, `BenchmarkImageBlit`
- Harvest tests (Skia/Cairo/JUCE/AGG-style scenarios, our own code)

### Changed

- Stroke width ≤ 0 no longer becomes a 1px hairline
- Gradient stop sampling avoids per-pixel allocs when stops are sorted
- README feature matrix, comparison table, and “known limitations vs Skia”

### Planned (unchanged)

- GPU `Device`, HarfBuzz/OpenType shaping, extra blend modes, SIMD

## 0.1.0 — 2026-09-10

Initial public release of **paintengine2d**: a from-scratch, pure-Go 2D paint /
raster engine (no CGO, no Skia/Cairo/Blend2D/AGG/Gio/NanoVG).

### Added

- Geometry, linear gradients, stroke caps/joins, Context/Device, CPU scanline AA
- Unit tests, golden PNGs, examples, comparison table
