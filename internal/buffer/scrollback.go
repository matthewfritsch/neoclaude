// Package buffer scrollback: a fixed-capacity ring buffer of plain-text lines
// that were scrolled off the top of the vt grid.
//
// Usage: when the vt emits a scroll-up (line 0 evicted), append that line here.
// For P2 we use a simpler approach: ExtractLines(grid) gives the current
// visible lines; grep unions those with the ring contents.
//
// TODO(config): expose cap as ScrollbackLines in config.Config.
package buffer

const defaultScrollbackCap = 10_000

// Ring is a fixed-capacity ring buffer of plain-text lines.
type Ring struct {
	lines []string
	cap   int
	head  int // index of the oldest entry; also write position when full
	count int // number of valid entries (0..cap)
}

// NewRing creates a ring with the given capacity. Cap is clamped to >= 1.
func NewRing(cap int) *Ring {
	if cap < 1 {
		cap = 1
	}
	return &Ring{lines: make([]string, cap), cap: cap}
}

// DefaultRing creates a ring with the default scrollback capacity.
func DefaultRing() *Ring { return NewRing(defaultScrollbackCap) }

// Append adds a line to the ring, evicting the oldest if at capacity.
func (r *Ring) Append(line string) {
	r.lines[r.head] = line
	r.head = (r.head + 1) % r.cap
	if r.count < r.cap {
		r.count++
	}
}

// Lines returns all buffered lines in order from oldest to newest.
func (r *Ring) Lines() []string {
	if r.count == 0 {
		return nil
	}
	out := make([]string, r.count)
	if r.count < r.cap {
		// Not yet wrapped: entries are at [0..count).
		copy(out, r.lines[:r.count])
		return out
	}
	// Wrapped: oldest is at head.
	n := copy(out, r.lines[r.head:])
	copy(out[n:], r.lines[:r.head])
	return out
}

// Len returns the number of lines currently buffered.
func (r *Ring) Len() int { return r.count }
