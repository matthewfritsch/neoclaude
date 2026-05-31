package buffer

import "testing"

func TestRingEmpty(t *testing.T) {
	r := NewRing(5)
	if r.Len() != 0 {
		t.Errorf("empty ring Len want 0 got %d", r.Len())
	}
	if lines := r.Lines(); lines != nil {
		t.Errorf("empty ring Lines want nil got %v", lines)
	}
}

func TestRingAppendNoWrap(t *testing.T) {
	r := NewRing(5)
	r.Append("a")
	r.Append("b")
	r.Append("c")
	lines := r.Lines()
	if len(lines) != 3 {
		t.Fatalf("want 3 got %d", len(lines))
	}
	if lines[0] != "a" || lines[1] != "b" || lines[2] != "c" {
		t.Errorf("unexpected lines: %v", lines)
	}
}

func TestRingWrap(t *testing.T) {
	r := NewRing(3)
	r.Append("a")
	r.Append("b")
	r.Append("c") // full
	r.Append("d") // evicts "a"
	lines := r.Lines()
	if len(lines) != 3 {
		t.Fatalf("want 3 got %d", len(lines))
	}
	if lines[0] != "b" || lines[1] != "c" || lines[2] != "d" {
		t.Errorf("after wrap want [b c d] got %v", lines)
	}
}

func TestRingCapOne(t *testing.T) {
	r := NewRing(1)
	r.Append("x")
	r.Append("y")
	lines := r.Lines()
	if len(lines) != 1 || lines[0] != "y" {
		t.Errorf("cap-1 ring: want [y] got %v", lines)
	}
}

func TestRingDoubleWrap(t *testing.T) {
	r := NewRing(3)
	for _, s := range []string{"a", "b", "c", "d", "e", "f"} {
		r.Append(s)
	}
	lines := r.Lines()
	if len(lines) != 3 {
		t.Fatalf("want 3 got %d", len(lines))
	}
	if lines[0] != "d" || lines[1] != "e" || lines[2] != "f" {
		t.Errorf("double-wrap: want [d e f] got %v", lines)
	}
}

func TestDefaultRing(t *testing.T) {
	r := DefaultRing()
	if r.cap != defaultScrollbackCap {
		t.Errorf("default cap want %d got %d", defaultScrollbackCap, r.cap)
	}
}
