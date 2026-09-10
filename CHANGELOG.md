# Changelog

All notable changes to this project are documented in this file.

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
