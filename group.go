package paintengine2d

import "math"

// maxLayerDim is the largest BakeGroup pixmap edge. Larger groups stay
// live-walked so a degenerate bounds cannot allocate a huge atlas.
const maxLayerDim = 8192

// GroupLocalBounds is the axis-aligned box of g's children in group-local
// space (Xform is not applied). A baked [GroupNode.Layer] returns the
// layer rectangle at [GroupNode.LayerOrigin].
func GroupLocalBounds(g *GroupNode) Rect {
	if g == nil {
		return Rect{}
	}
	if g.Layer != nil {
		return XYWH(g.LayerOrigin.X, g.LayerOrigin.Y, float32(g.Layer.Width), float32(g.Layer.Height))
	}
	var u Rect
	has := false
	var walk func(Node, Matrix)
	walk = func(n Node, xf Matrix) {
		switch t := n.(type) {
		case *GroupNode:
			if t == nil {
				return
			}
			nx := xf.Mul(t.Xform)
			if t.Layer != nil {
				r := nx.TransformRect(XYWH(t.LayerOrigin.X, t.LayerOrigin.Y, float32(t.Layer.Width), float32(t.Layer.Height)))
				if !has {
					u, has = r, true
				} else {
					u = u.Union(r)
				}
				return
			}
			for _, ch := range t.Children {
				walk(ch, nx)
			}
		case *drawOp:
			r := opContentBounds(t, xf)
			if r.Empty() {
				return
			}
			if !has {
				u, has = r, true
			} else {
				u = u.Union(r)
			}
		}
	}
	for _, ch := range g.Children {
		walk(ch, Identity())
	}
	return u
}

// BakeGroup rasters g.Children once into [GroupNode.Layer] (local space).
// Later [DrawScene] / [DrawSceneDamage] blits the layer through
// [GroupNode.Xform] and does not re-walk children — the splitter-drag
// contract. Returns the layer (nil if the group is empty or too large).
//
// Call again after [GroupNode.InvalidateLayer] when children change.
// Changing only Xform does not require a re-bake.
func BakeGroup(g *GroupNode) *Image {
	if g == nil {
		return nil
	}
	g.InvalidateLayer()
	b := GroupLocalBounds(g)
	if b.Empty() || !b.Finite() {
		return nil
	}
	x0 := int(math.Floor(float64(b.Min.X)))
	y0 := int(math.Floor(float64(b.Min.Y)))
	x1 := int(math.Ceil(float64(b.Max.X)))
	y1 := int(math.Ceil(float64(b.Max.Y)))
	if float32(x0) != b.Min.X || float32(y0) != b.Min.Y || float32(x1) != b.Max.X || float32(y1) != b.Max.Y {
		x0--
		y0--
		x1++
		y1++
	}
	w, h := x1-x0, y1-y0
	if w < 1 || h < 1 || w > maxLayerDim || h > maxLayerDim {
		return nil
	}
	img := NewImage(w, h)
	dev := NewCPUDevice(img)
	walk := sceneWalker{dev: dev}
	xf := Translation(float32(-x0), float32(-y0))
	for _, ch := range g.Children {
		walk.walk(ch, xf)
	}
	walk.flush()
	g.Layer = img
	g.LayerOrigin = Pt(float32(x0), float32(y0))
	g.LayerOpaque = imageFullyOpaque(img)
	return img
}

func imageFullyOpaque(img *Image) bool {
	if img == nil || img.Width < 1 || img.Height < 1 {
		return false
	}
	for y := 0; y < img.Height; y++ {
		row := img.Pix[y*img.Stride : y*img.Stride+img.Width*4]
		for x := 0; x < img.Width; x++ {
			if row[x*4+3] != 255 {
				return false
			}
		}
	}
	return true
}
