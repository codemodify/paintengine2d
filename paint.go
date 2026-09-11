package paintengine2d

// Style selects how a [Paint] is applied to geometry.
type Style uint8

const (
	// StyleFill fills the interior (default).
	StyleFill Style = iota
	// StyleStroke strokes the outline.
	StyleStroke
	// StyleStrokeAndFill fills, then strokes.
	StyleStrokeAndFill
)

// FillRule is the interior rule used when filling a path.
type FillRule uint8

const (
	// FillNonZero uses the non-zero winding rule (default, SVG/PDF).
	FillNonZero FillRule = iota
	// FillEvenOdd uses the even-odd rule.
	FillEvenOdd
)

// Cap is the stroke end-cap style. All three are implemented.
type Cap uint8

const (
	// CapButt squares off flush with the endpoint.
	CapButt Cap = iota
	// CapRound adds a semicircle of radius width/2.
	CapRound
	// CapSquare extends the stroke by width/2 past the endpoint.
	CapSquare
)

// Join is the stroke corner style. All three are implemented.
type Join uint8

const (
	// JoinMiter extends offset edges to their intersection, limited by
	// [Stroke.MiterLimit] (falls back to bevel when exceeded).
	JoinMiter Join = iota
	// JoinRound connects offsets with a circular arc.
	JoinRound
	// JoinBevel connects offsets with a straight line.
	JoinBevel
)

// TileMode controls how a gradient behaves outside [0, 1].
type TileMode uint8

const (
	// TileClamp clamps the parameter to [0, 1].
	TileClamp TileMode = iota
	// TileRepeat wraps the parameter (modulo 1).
	TileRepeat
	// TileMirror wraps, reversing every other unit interval.
	TileMirror
)

// FilterMode selects image resampling for [Context.DrawImageRect] / [Device.Blit].
type FilterMode uint8

const (
	// FilterBilinear is the default: 2×2 weighted sample (smooth scale).
	FilterBilinear FilterMode = iota
	// FilterNearest picks the covering source pixel (blocky, exact for 1:1).
	FilterNearest
)

// BlendMode is the Porter-Duff operator. v0.2 implements [BlendSrcOver] only;
// any other value is treated as src-over. Extra modes are planned, not faked.
type BlendMode uint8

const (
	// BlendSrcOver is the standard “source over destination” operator.
	BlendSrcOver BlendMode = iota
)

// Stroke describes outline stroking. Width is in user-space units and
// is transformed with the current matrix (Skia/SVG convention).
//
// Width <= 0 skips the stroke (no silent hairline). Dash is an SVG-style
// on/off array in user units; a single value is paired with itself; an
// empty or all-zero dash is a solid stroke. DashOffset shifts the pattern.
type Stroke struct {
	Width      float32
	Cap        Cap
	Join       Join
	MiterLimit float32
	Dash       []float32
	DashOffset float32
}

// DefaultStroke is a 1px-wide butt/miter stroke (miter limit 4).
func DefaultStroke() Stroke {
	return Stroke{Width: 1, Cap: CapButt, Join: JoinMiter, MiterLimit: 4}
}

func (s Stroke) normalized() Stroke {
	if s.MiterLimit < 1 {
		s.MiterLimit = 4
	}
	return s
}

// GradientStop is a color stop along a gradient. Offset is typically in [0, 1].
type GradientStop struct {
	Offset float32
	Color  Color
}

// LinearGradient is a two-point linear gradient shader in the coordinate
// space of the path at draw time (user space). The current transform maps
// the gradient into device pixels along with the geometry.
type LinearGradient struct {
	Start, End Point
	Stops      []GradientStop
	Tile       TileMode
}

// RadialGradient is a circular gradient in user space. t = 0 at Center
// (or at Inner, if Inner > 0) and t = 1 on the circle of radius Radius.
// Tile modes match [LinearGradient]. A non-zero Focal shifts the t=0 point
// (simple two-point radial; FocalRadius is reserved and ignored in v0.2).
type RadialGradient struct {
	Center Point
	Radius float32
	Inner  float32
	Focal  Point
	Stops  []GradientStop
	Tile   TileMode
}

// Paint is the bundled drawing style passed to draw calls — the analogue of
// Skia's SkPaint: color or shader, fill vs stroke, and fill rule.
//
// If Shader is non-nil it overrides Color. AntiAlias is reserved; the CPU
// backend always anti-aliases path edges (there is no jaggy mode).
// Filter applies to image blits. On blit / DrawGlyphs, Color RGB multiplies
// premul samples and Color.A modulates coverage (zero-value paint = untinted).
// Blend is src-over unless documented later.
type Paint struct {
	Color     Color
	Shader    Shader
	Style     Style
	Stroke    Stroke
	FillRule  FillRule
	Filter    FilterMode
	Blend     BlendMode
	AntiAlias bool
}

// Shader produces a color for a device-space sample. Implementations must
// be safe for concurrent use if a Context is shared across goroutines
// (the stock shaders are).
type Shader interface {
	// Shade returns a straight-alpha color at a device-space pixel center.
	// xform maps user space → device; callers may invert it as needed.
	Shade(x, y float32, xform Matrix) Color
}

// Fill returns an opaque (or alpha) fill paint.
func Fill(c Color) Paint {
	return Paint{Color: c, Style: StyleFill, AntiAlias: true, Stroke: DefaultStroke()}
}

// StrokePaint returns a stroke-only paint.
func StrokePaint(c Color, width float32) Paint {
	s := DefaultStroke()
	s.Width = width
	return Paint{Color: c, Style: StyleStroke, Stroke: s, AntiAlias: true}
}

// Linear returns a fill paint with a linear gradient shader.
func Linear(g LinearGradient) Paint {
	return Paint{Shader: g, Style: StyleFill, AntiAlias: true, Stroke: DefaultStroke()}
}

// Radial returns a fill paint with a radial gradient shader.
func Radial(g RadialGradient) Paint {
	return Paint{Shader: g, Style: StyleFill, AntiAlias: true, Stroke: DefaultStroke()}
}

// Shade implements [Shader] for a solid color (rarely needed; set Paint.Color).
func (c Color) Shade(x, y float32, xform Matrix) Color {
	_, _, _ = x, y, xform
	return c
}

// Shade implements [Shader] for [LinearGradient].
func (g LinearGradient) Shade(x, y float32, xform Matrix) Color {
	inv, ok := xform.Invert()
	// Evaluate in user space so Start/End stay aligned with the path.
	p := Point{x, y}
	if ok {
		p = inv.Transform(p)
	}
	d := g.End.Sub(g.Start)
	lenSq := d.LenSq()
	var t float32
	if lenSq < 1e-12 {
		t = 0
	} else {
		t = p.Sub(g.Start).Dot(d) / lenSq
	}
	t = tileParam(t, g.Tile)
	return sampleStops(g.Stops, t)
}

// Shade implements [Shader] for [RadialGradient].
func (g RadialGradient) Shade(x, y float32, xform Matrix) Color {
	inv, ok := xform.Invert()
	p := Point{x, y}
	if ok {
		p = inv.Transform(p)
	}
	origin := g.Center
	if g.Focal != (Point{}) && (g.Focal.X != g.Center.X || g.Focal.Y != g.Center.Y) {
		origin = g.Focal
	}
	r1 := g.Radius
	if r1 < 0 {
		r1 = -r1
	}
	r0 := g.Inner
	if r0 < 0 {
		r0 = -r0
	}
	d := p.Sub(origin).Len()
	var t float32
	span := r1 - r0
	if span < 1e-8 {
		if d <= r1 {
			t = 0
		} else {
			t = 1
		}
	} else {
		t = (d - r0) / span
	}
	t = tileParam(t, g.Tile)
	return sampleStops(g.Stops, t)
}

func tileParam(t float32, mode TileMode) float32 {
	switch mode {
	case TileRepeat:
		t = t - float32(mathFloor(t))
		if t < 0 {
			t += 1
		}
		return t
	case TileMirror:
		// Fold into [0, 2) then mirror.
		t = t - float32(mathFloor(t/2))*2
		if t < 0 {
			t += 2
		}
		if t > 1 {
			t = 2 - t
		}
		return t
	default:
		return clamp32(t, 0, 1)
	}
}

func sampleStops(stops []GradientStop, t float32) Color {
	n := len(stops)
	if n == 0 {
		return Transparent
	}
	if n == 1 {
		return stops[0].Color
	}
	s := stops
	// Hot path: already-sorted stops (the usual case) — no allocation.
	sorted := true
	for i := 1; i < n; i++ {
		if s[i].Offset < s[i-1].Offset {
			sorted = false
			break
		}
	}
	if !sorted {
		var stack [16]GradientStop
		if n <= len(stack) {
			copy(stack[:n], stops)
			insertionSortStops(stack[:n])
			s = stack[:n]
		} else {
			s = append([]GradientStop(nil), stops...)
			insertionSortStops(s)
		}
	}
	if t <= s[0].Offset {
		return s[0].Color
	}
	if t >= s[n-1].Offset {
		return s[n-1].Color
	}
	for i := 1; i < n; i++ {
		if t <= s[i].Offset {
			s0, s1 := s[i-1], s[i]
			span := s1.Offset - s0.Offset
			if span < 1e-8 {
				return s1.Color
			}
			return s0.Color.Lerp(s1.Color, (t-s0.Offset)/span)
		}
	}
	return s[n-1].Color
}

func insertionSortStops(s []GradientStop) {
	for i := 1; i < len(s); i++ {
		j := i
		for j > 0 && s[j].Offset < s[j-1].Offset {
			s[j], s[j-1] = s[j-1], s[j]
			j--
		}
	}
}

func mathFloor(v float32) int {
	i := int(v)
	if float32(i) > v {
		return i - 1
	}
	return i
}
