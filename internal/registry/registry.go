// Package registry holds the ordered collection of open buffers and tracks
// which one is active. All methods are NOT goroutine-safe — they are called
// exclusively from the Bubble Tea Update goroutine.
package registry

import (
	"errors"
	"fmt"

	"github.com/matthewfritsch/neoclaude/internal/buffer"
)

// Registry is the ordered list of open buffers plus an active index.
type Registry struct {
	bufs   []*buffer.Buffer
	active int // index into bufs; -1 when empty
	nextID buffer.ID
	byID   map[buffer.ID]*buffer.Buffer
}

// New returns an empty registry.
func New() *Registry {
	return &Registry{active: -1, byID: make(map[buffer.ID]*buffer.Buffer)}
}

// NextID allocates and returns the next buffer ID without creating a buffer.
func (r *Registry) NextID() buffer.ID {
	id := r.nextID
	r.nextID++
	return id
}

// Add appends b to the registry and makes it active.
func (r *Registry) Add(b *buffer.Buffer) {
	r.bufs = append(r.bufs, b)
	r.byID[b.ID] = b
	r.active = len(r.bufs) - 1
}

// Active returns the currently active buffer, or nil if the registry is empty.
func (r *Registry) Active() *buffer.Buffer {
	if r.active < 0 || r.active >= len(r.bufs) {
		return nil
	}
	return r.bufs[r.active]
}

// SetActive makes the buffer at index i active. Clamps to valid range.
func (r *Registry) SetActive(i int) {
	if len(r.bufs) == 0 {
		r.active = -1
		return
	}
	if i < 0 {
		i = 0
	}
	if i >= len(r.bufs) {
		i = len(r.bufs) - 1
	}
	r.active = i
}

// ActiveIndex returns the current active index (-1 when empty).
func (r *Registry) ActiveIndex() int { return r.active }

// Len returns the number of open buffers.
func (r *Registry) Len() int { return len(r.bufs) }

// All returns a copy of the ordered buffer slice.
func (r *Registry) All() []*buffer.Buffer {
	out := make([]*buffer.Buffer, len(r.bufs))
	copy(out, r.bufs)
	return out
}

// ByID looks up a buffer by its ID; returns nil if not found.
func (r *Registry) ByID(id buffer.ID) *buffer.Buffer {
	return r.byID[id]
}

// Next advances the active index forward (wraps around).
func (r *Registry) Next() {
	if len(r.bufs) == 0 {
		return
	}
	r.active = (r.active + 1) % len(r.bufs)
}

// Prev moves the active index backward (wraps around).
func (r *Registry) Prev() {
	if len(r.bufs) == 0 {
		return
	}
	r.active = (r.active - 1 + len(r.bufs)) % len(r.bufs)
}

// Remove kills the session for buffer id, removes it from the registry, and
// moves the active index to the nearest remaining buffer. Returns an error if
// id is not found.
func (r *Registry) Remove(id buffer.ID) error {
	idx := -1
	for i, b := range r.bufs {
		if b.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("registry: buffer %d not found", id)
	}

	b := r.bufs[idx]
	// Kill returns an error we deliberately ignore on removal — the buffer is
	// gone from our perspective regardless. Close the VT too so its response
	// drain goroutine exits (otherwise it leaks, blocked on emu.Read).
	_ = b.Session.Kill()
	_ = b.VT.Close()
	delete(r.byID, b.ID)
	r.bufs = append(r.bufs[:idx], r.bufs[idx+1:]...)

	if len(r.bufs) == 0 {
		r.active = -1
		return nil
	}
	// Keep active pointing at a valid buffer: prefer the one that was after the
	// removed slot; fall back to the last if we deleted the tail.
	if r.active >= len(r.bufs) {
		r.active = len(r.bufs) - 1
	}
	return nil
}

// ErrEmpty is returned by operations that require at least one buffer.
var ErrEmpty = errors.New("registry: no open buffers")
