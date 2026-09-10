# Changelog

All notable changes to this project are documented in this file.

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

- GPU `Device`, OpenType text, extra blend modes, SIMD, retained/damage

## 0.1.0 — 2026-09-10

Initial public release of **paintengine2d**: a from-scratch, pure-Go 2D paint /
raster engine (no CGO, no Skia/Cairo/Blend2D/AGG/Gio/NanoVG).

### Added

- Geometry, linear gradients, stroke caps/joins, Context/Device, CPU scanline AA
- Unit tests, golden PNGs, examples, comparison table
