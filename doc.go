// Package paintengine2d is a from-scratch 2D paint engine with a pure-Go
// public API and a CPU or GPU backend.
//
// The default canvas is a software scanline-AA rasterizer. On Linux with
// CGO, [GPUDevice] implements the same [Device] seam via EGL / OpenGL ES 2
// (stencil-and-cover fill/stroke, linear/radial ramps, textured blit).
// There is no dependency on Skia, Cairo, Blend2D, AGG, Gio, or NanoVG.
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
// Rasterization is delegated to a [Device]. [CPUDevice] is always available.
// [GPUDevice] (Linux + CGO + EGL) implements the same interface.
// [OpenSurface] / [EnvPaint] pick cpu, gpu, or auto.
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
// v0.8.0 is the GPU milestone: paths, AA fill/stroke (CPU), GPU
// stencil-and-cover (Linux EGL), linear (and radial) gradients, clip
// rect/path, images with RGB blit tint, WrapImage stride, dirty-rect
// recording, and text hooks ([FontAtlas] + [GlyphRun] + [Shaper] +
// [Context.DrawGlyphs], with [NullShaper] for bitmap labels; Paint.Color
// themes a white atlas). IME, OpenType, and a11y belong in a framework
// above this package. PDF and HDR remain out of scope.
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
// Integer 1:1 nearest blit is allocation-free.
// Warm path fill and stroke reuse flatten / stroke-pool scratch (0 allocs).
// [Context.ClipPath] rasterizes only the path's device-space bounds.
// Non-finite coordinates and matrices are ignored (no panic).
package paintengine2d
