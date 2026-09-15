package paintengine2d

// PathCache keeps recorded path snapshots across frames. A retained UI
// records much the same shapes every frame; a recorder interning into a
// shared cache clones each once instead of once per frame. Hand the cache
// to every frame's recorder ([Recorder.UsePathCache]) and call
// [PathCache.EndFrame] once per frame: snapshots unused for a few seconds
// are dropped, so the cache follows the UI instead of growing with it.
type PathCache struct {
	m     map[uint64][]cachedPath
	frame uint32
	n     int
}

type cachedPath struct {
	p    *Path
	used uint32
}

const (
	// pathCacheKeep is how many frames an unused snapshot survives.
	pathCacheKeep = 240
	// pathCacheSweep is how often (in frames) old snapshots are swept.
	pathCacheSweep = 60
	// pathCacheMax bounds the entries; past it every frame sweeps.
	pathCacheMax = 1 << 13
)

// NewPathCache returns an empty cache.
func NewPathCache() *PathCache { return &PathCache{m: make(map[uint64][]cachedPath)} }

// Len is the number of cached snapshots.
func (c *PathCache) Len() int {
	if c == nil {
		return 0
	}
	return c.n
}

func (c *PathCache) intern(path *Path) *Path {
	h := hashPath(path)
	list := c.m[h]
	for i := range list {
		if pathEqual(list[i].p, path) {
			list[i].used = c.frame
			return list[i].p
		}
	}
	p := path.Clone()
	c.m[h] = append(list, cachedPath{p: p, used: c.frame})
	c.n++
	return p
}

// EndFrame advances the cache's clock and sweeps stale snapshots now and
// then. Scenes still holding a swept path keep it; it is only forgotten.
func (c *PathCache) EndFrame() {
	if c == nil {
		return
	}
	c.frame++
	if c.frame%pathCacheSweep != 0 && c.n <= pathCacheMax {
		return
	}
	keep := uint32(pathCacheKeep)
	if c.n > pathCacheMax {
		keep = pathCacheSweep
	}
	for h, list := range c.m {
		out := list[:0]
		for _, e := range list {
			if c.frame-e.used <= keep {
				out = append(out, e)
			}
		}
		for i := len(out); i < len(list); i++ {
			list[i] = cachedPath{}
		}
		c.n -= len(list) - len(out)
		if len(out) == 0 {
			delete(c.m, h)
		} else {
			c.m[h] = out
		}
	}
}
