# Changelog

All notable changes to this project are documented in this file.

## 0.1.0 — 2026-09-10

Initial public release of **paintengine2d**: a from-scratch, pure-Go 2D paint /
raster engine (no CGO, no Skia/Cairo/Blend2D/AGG/Gio/NanoVG).

### Added

- Geometry: `Point`, `Rect`, affine `Matrix`, `Path` (move/line/quad/cubic/close)
  plus helpers for rectangles, rounded rects, ellipses, circles, and arcs
- Paint: straight-alpha sRGB `Color`, solid fill, linear gradients (clamp /
  repeat / mirror), stroke width with butt/round/square caps and
  miter/round/bevel joins
- `Context` canvas: Save/Restore, transform, `ClipRect` / `ClipPath`,
  `FillPath` / `StrokePath` / `DrawPath`, `FillRect`, `DrawImage` /
  `DrawImageRect`, `Clear`, plus circle/oval/round-rect/line helpers
- `Device` / `CPUDevice` split so a future GPU backend can plug in without
  breaking the public canvas API
- CPU scanline anti-aliased rasterizer (8× vertical samples + analytical
  horizontal coverage) into an owned premultiplied RGBA pixmap
- PNG encode/decode via the Go standard library
- Unit tests, golden PNG tests (`UPDATE_GOLDENS=1`), and benchmarks
- Canvas contract tests (save/restore, clip stack, fill rules, stroke
  caps/joins, gradient tile modes, compositing, empty/degenerate geometry)
- Examples: `examples/hello`, `examples/gallery`, `examples/paths`
- README comparison table vs Gio, Fyne, Skia/Cairo bindings, NanoVG, Blend2D,
  AGG, Dear ImGui, and HTML Canvas

### Planned (not in 0.1.0)

- GPU `Device`
- OpenType shaping / text
- Retained canvas and Evas-style dirty rectangles (`Damage` is a stub)
- SIMD scanline kernels
- Radial gradients, dash patterns, blend modes beyond src-over
