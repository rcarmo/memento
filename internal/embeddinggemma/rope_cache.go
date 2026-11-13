package embeddinggemma

import "math"

const (
	ropeTilePositions   = 64
	ropeLocalDimension  = 256
	ropeGlobalDimension = 512
	ropeLocalHalf       = ropeLocalDimension / 2
	ropeGlobalHalf      = ropeGlobalDimension / 2
)

// ropeTile is a bounded, request-owned RoPE angle cache. It intentionally has
// no package-level state: a Workspace can retain one while it is exclusively
// locked for an embedding request. Its largest table is 64 * 256 sin/cos
// float32 pairs (128 KiB), rather than a context-length table.
//
// prepare must be called for the token tile before apply. The model's local
// and global heads have fixed widths, so their frequencies are initialized
// once in this cache using the same float64 Pow expression as rope.
type ropeTile struct {
	base, count int
	global      bool
	prepared    bool

	localFrequency  [ropeLocalHalf]float64
	globalFrequency [ropeGlobalHalf]float64
	localReady      bool
	globalReady     bool

	sin [ropeTilePositions][ropeGlobalHalf]float32
	cos [ropeTilePositions][ropeGlobalHalf]float32
}

// prepare computes the angles for [base, base+count). count is bounded by the
// projection tile size. Repreparing the same tile is a no-op, allowing callers
// that process Q and K separately to share it.
func (r *ropeTile) prepare(base, count int, global bool) {
	if count < 1 || count > ropeTilePositions || base < 0 {
		panic("embeddinggemma: invalid RoPE tile")
	}
	if r.prepared && r.base == base && r.count == count && r.global == global {
		return
	}

	if global {
		frequencies := r.globalFrequencies()
		for token := 0; token < count; token++ {
			pos := float64(base + token)
			for i := 0; i < ropeGlobalHalf; i++ {
				s, c := math.Sincos(pos * frequencies[i])
				r.sin[token][i] = float32(s)
				r.cos[token][i] = float32(c)
			}
		}
	} else {
		frequencies := r.localFrequencies()
		for token := 0; token < count; token++ {
			pos := float64(base + token)
			for i := 0; i < ropeLocalHalf; i++ {
				s, c := math.Sincos(pos * frequencies[i])
				r.sin[token][i] = float32(s)
				r.cos[token][i] = float32(c)
			}
		}
	}
	r.base, r.count, r.global, r.prepared = base, count, global, true
}

// apply rotates x using angles from the current prepared tile. Callers must
// supply the head width selected by prepare and a position in that tile.
func (r *ropeTile) apply(x []float32, pos int) {
	row := pos - r.base
	if r.global {
		for i := 0; i < ropeGlobalHalf; i++ {
			a, b := x[i], x[i+ropeGlobalHalf]
			s, c := r.sin[row][i], r.cos[row][i]
			x[i] = a*c - b*s
			x[i+ropeGlobalHalf] = b*c + a*s
		}
		return
	}
	for i := 0; i < ropeLocalHalf; i++ {
		a, b := x[i], x[i+ropeLocalHalf]
		s, c := r.sin[row][i], r.cos[row][i]
		x[i] = a*c - b*s
		x[i+ropeLocalHalf] = b*c + a*s
	}
}

func (r *ropeTile) localFrequencies() *[ropeLocalHalf]float64 {
	if !r.localReady {
		for i := range r.localFrequency {
			r.localFrequency[i] = math.Pow(10000.0, -float64(2*i)/float64(ropeLocalDimension))
		}
		r.localReady = true
	}
	return &r.localFrequency
}

func (r *ropeTile) globalFrequencies() *[ropeGlobalHalf]float64 {
	if !r.globalReady {
		for i := range r.globalFrequency {
			r.globalFrequency[i] = math.Pow(1e6, -float64(2*i)/float64(ropeGlobalDimension))
		}
		r.globalReady = true
	}
	return &r.globalFrequency
}
