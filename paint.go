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

// Stroke describes outline stroking. Width is in user-space units and
// is transformed with the current matrix (Skia/SVG convention).
type Stroke struct {
	Width      float32
	Cap        Cap
	Join       Join
	MiterLimit float32
}

// DefaultStroke is a 1px-wide butt/miter stroke (miter limit 4).
func DefaultStroke() Stroke {
	return Stroke{Width: 1, Cap: CapButt, Join: JoinMiter, MiterLimit: 4}
}

func (s Stroke) normalized() Stroke {
	if s.Width <= 0 {
		s.Width = 1
	}
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

// Paint is the bundled drawing style passed to draw calls — the analogue of
// Skia's SkPaint: color or shader, fill vs stroke, and fill rule.
//
// If Shader is non-nil it overrides Color. AntiAlias is reserved; the CPU
// backend always anti-aliases path edges in v0 (there is no jaggy mode).
type Paint struct {
	Color     Color
	Shader    Shader
	Style     Style
	Stroke    Stroke
	FillRule  FillRule
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
	if len(stops) == 0 {
		return Transparent
	}
	if len(stops) == 1 {
		return stops[0].Color
	}
	// Assume stops are sorted by Offset. We sort lazily at first use
	// would require mutation; Context copies paints. Sort a tiny stack buf.
	s0, s1 := stops[0], stops[len(stops)-1]
	// Find bracketing stops.
	if t <= stops[0].Offset {
		return stops[0].Color
	}
	if t >= stops[len(stops)-1].Offset {
		return stops[len(stops)-1].Color
	}
	for i := 1; i < len(stops); i++ {
		if t <= stops[i].Offset {
			s0, s1 = stops[i-1], stops[i]
			span := s1.Offset - s0.Offset
			if span < 1e-8 {
				return s1.Color
			}
			u := (t - s0.Offset) / span
			return s0.Color.Lerp(s1.Color, u)
		}
	}
	return s1.Color
}

func mathFloor(v float32) int {
	i := int(v)
	if float32(i) > v {
		return i - 1
	}
	return i
}
