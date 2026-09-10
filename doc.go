// Package paintengine2d is a from-scratch, pure-Go 2D paint / raster engine.
//
// It is a software canvas: you build paths, configure paints (solid color or
// linear gradient, fill or stroke), and a CPU scanline anti-aliased backend
// composites into a premultiplied RGBA pixmap. There is no CGO and no
// dependency on Skia, Cairo, Blend2D, AGG, Gio, or NanoVG.
//
// # Quick start
//
//	img := paintengine2d.NewImage(640, 360)
//	ctx := paintengine2d.NewContext(img)
//	ctx.Clear(paintengine2d.RGB(0.12, 0.13, 0.16))
//	ctx.DrawCircle(paintengine2d.Pt(180, 180), 90, paintengine2d.Fill(paintengine2d.RGB(0.2, 0.5, 0.95)))
//	_ = img.WritePNGFile("out.png")
//
// # Coordinate space
//
// User space is a right-handed Cartesian plane with +X right and +Y down,
// matching typical 2D canvas / Skia conventions. The current [Matrix] maps
// user space to device pixels. Pixel (0, 0) is the top-left of the pixmap;
// a rectangle covering the first pixel is [0, 1] × [0, 1].
//
// # Pixel format
//
// [Image] stores premultiplied 8-bit sRGB RGBA. [NewImage] is tightly
// packed (stride = width * 4). [WrapImage] accepts a caller buffer whose
// row stride may be padded (swapchain / X11 / Wayland shm). PNG
// encode/decode converts to and from straight (non-premultiplied) alpha.
//
// # Backends
//
// [Context] is the public canvas (save/restore, transform, clip, draw).
// Rasterization is delegated to a [Device]. v0 ships [CPUDevice]. A future
// GPU device can implement the same interface without changing call sites.
//
// # Consumers (other repositories)
//
// This module is a shared library: import github.com/codemodify/paintengine2d.
// It has no windowing and no app main loop.
//
//	Engine  →  UI framework  →  apps          (widgets, layout, windows-as-app-UI)
//	Engine  →  WM / DE       →  surfaces      (borders, titlebars, panels; X11 + Wayland)
//
// Both paint into caller-owned buffers via [NewImage] or [WrapImage].
// [Context] / [Device] / [Damage] / [GlyphRun] are the stable surface.
// X11, Wayland, and Win32 peers live in those other repos.
//
// # UI subset
//
// The production target is a UI-shaped CPU canvas: paths, AA fill/stroke,
// linear gradients, clip, images, and dirty-rect recording. Text is a hook:
// [FontAtlas] + [GlyphRun] + [Context.DrawGlyphs], with [NullShaper] for
// bitmap labels. IME, OpenType, and a11y belong in a framework above this
// package. GPU, PDF, and HDR are out of scope.
//
// # Skia / JUCE mapping
//
//	SkCanvas / juce::Graphics  →  [Context]
//	SkPaint / juce::FillType   →  [Paint]
//	SkPath                     →  [Path]
//	SkCanvas::save/restore     →  [Context.Save] / [Context.Restore]
//	clipRect / clipPath        →  [Context.ClipRect] / [Context.ClipPath]
//	drawImageRect              →  [Context.DrawImageRect]
//	SkTextBlob                 →  [GlyphRun] + [Shaper] (hook only)
//
// # Performance contracts
//
// Opaque integer-aligned [Context.FillRect] is allocation-free.
// Repeated strokes reuse flatten/outline scratch on [CPUDevice].
// [Context.ClipPath] rasterizes only the path's device-space bounds.
// Non-finite coordinates and matrices are ignored (no panic).
package paintengine2d
