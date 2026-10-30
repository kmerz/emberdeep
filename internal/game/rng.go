package game

// RNG is a small deterministic splitmix64 generator.
type RNG struct{ s uint64 }

func NewRNG(seed uint64) *RNG {
	r := &RNG{s: seed*0x9E3779B97F4A7C15 + 0xD1B54A32D192ED03}
	r.Next()
	return r
}

func (r *RNG) Next() uint64 {
	r.s += 0x9E3779B97F4A7C15
	z := r.s
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

// Intn returns a value in [0,n).
func (r *RNG) Intn(n int) int {
	if n <= 1 {
		return 0
	}
	return int(r.Next() % uint64(n))
}

func (r *RNG) Float() float64 { return float64(r.Next()>>11) / float64(1<<53) }

func (r *RNG) Chance(p float64) bool { return r.Float() < p }
