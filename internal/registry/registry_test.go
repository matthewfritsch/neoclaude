package registry

import (
	"testing"

	"github.com/matthewfritsch/neoclaude/internal/buffer"
	"github.com/matthewfritsch/neoclaude/internal/session"
	"github.com/matthewfritsch/neoclaude/internal/vt"
)

// makeBuffer creates a buffer with a nil session for registry tests that don't
// exercise Kill (Remove calls Kill, so tests that call Remove need a real session
// or must handle the error). For pure ordering tests we stub session as nil and
// override Remove.
func makeBuf(id buffer.ID) *buffer.Buffer {
	return buffer.New(id, "test", "/tmp", "", &session.Session{}, vt.New(80, 24))
}

func TestEmptyRegistry(t *testing.T) {
	r := New()
	if r.Active() != nil {
		t.Error("empty registry should return nil Active")
	}
	if r.Len() != 0 {
		t.Errorf("empty Len want 0 got %d", r.Len())
	}
	if r.ActiveIndex() != -1 {
		t.Errorf("empty ActiveIndex want -1 got %d", r.ActiveIndex())
	}
}

func TestAddMakesActive(t *testing.T) {
	r := New()
	b0 := makeBuf(0)
	b1 := makeBuf(1)
	r.Add(b0)
	r.Add(b1)
	if r.Active() != b1 {
		t.Error("last added buffer should be active")
	}
	if r.Len() != 2 {
		t.Errorf("Len want 2 got %d", r.Len())
	}
}

func TestNextWraps(t *testing.T) {
	r := New()
	r.Add(makeBuf(0))
	r.Add(makeBuf(1))
	r.Add(makeBuf(2))
	// active starts at 2 (last added)
	r.Next() // → 0
	if r.ActiveIndex() != 0 {
		t.Errorf("Next wrap: want index 0 got %d", r.ActiveIndex())
	}
}

func TestPrevWraps(t *testing.T) {
	r := New()
	r.Add(makeBuf(0))
	r.Add(makeBuf(1))
	r.Add(makeBuf(2))
	// active starts at 2
	r.Prev() // → 1
	if r.ActiveIndex() != 1 {
		t.Errorf("Prev: want index 1 got %d", r.ActiveIndex())
	}
	r.SetActive(0)
	r.Prev() // → 2 (wrap)
	if r.ActiveIndex() != 2 {
		t.Errorf("Prev wrap: want index 2 got %d", r.ActiveIndex())
	}
}

func TestNextOnEmpty(t *testing.T) {
	r := New()
	r.Next() // should not panic
	if r.ActiveIndex() != -1 {
		t.Error("Next on empty should leave index at -1")
	}
}

func TestByID(t *testing.T) {
	r := New()
	b := makeBuf(42)
	r.Add(b)
	if r.ByID(42) != b {
		t.Error("ByID should return the added buffer")
	}
	if r.ByID(99) != nil {
		t.Error("ByID for unknown ID should return nil")
	}
}

func TestRemoveNotFound(t *testing.T) {
	r := New()
	if err := r.Remove(99); err == nil {
		t.Error("Remove of unknown ID should return error")
	}
}

func TestRemoveMiddlePreservesActive(t *testing.T) {
	r := New()
	ids := []buffer.ID{0, 1, 2}
	for _, id := range ids {
		r.Add(makeBuf(id))
	}
	r.SetActive(2) // active = buf[2] (id=2)
	// Remove the middle buffer (id=1, index=1). Active was index 2, becomes
	// index 1 after the slice shrinks, pointing at what was buf[2].
	_ = r.Remove(1) // Kill on nil session is safe (Session.Kill handles nil cmd)
	if r.Len() != 2 {
		t.Errorf("Len after remove want 2 got %d", r.Len())
	}
	// active should clamp to last valid index if it was beyond end
	if r.Active() == nil {
		t.Error("Active should not be nil after remove with remaining buffers")
	}
}

func TestRemoveLastLeavesEmpty(t *testing.T) {
	r := New()
	r.Add(makeBuf(0))
	_ = r.Remove(0)
	if r.Len() != 0 {
		t.Errorf("Len after last remove want 0 got %d", r.Len())
	}
	if r.ActiveIndex() != -1 {
		t.Errorf("ActiveIndex after empty want -1 got %d", r.ActiveIndex())
	}
	if r.Active() != nil {
		t.Error("Active after last remove should be nil")
	}
}

func TestAllReturnsCopy(t *testing.T) {
	r := New()
	r.Add(makeBuf(0))
	r.Add(makeBuf(1))
	all := r.All()
	if len(all) != 2 {
		t.Fatalf("All len want 2 got %d", len(all))
	}
	// Mutating the returned slice should not affect the registry.
	all[0] = nil
	if r.All()[0] == nil {
		t.Error("All should return a copy, not the backing slice")
	}
}

func TestNextIDMonotonic(t *testing.T) {
	r := New()
	a := r.NextID()
	b := r.NextID()
	c := r.NextID()
	if !(a < b && b < c) {
		t.Errorf("NextID should be strictly increasing: got %d %d %d", a, b, c)
	}
}
