package search

import (
	"testing"

	"github.com/matthewfritsch/neoclaude/internal/buffer"
	"github.com/matthewfritsch/neoclaude/internal/registry"
	"github.com/matthewfritsch/neoclaude/internal/session"
	"github.com/matthewfritsch/neoclaude/internal/vt"
)

func makeReg(names []string) *registry.Registry {
	reg := registry.New()
	for i, name := range names {
		id := buffer.ID(i)
		b := buffer.New(id, name, "/tmp/"+name, &session.Session{}, vt.New(80, 24))
		b.ID = id
		reg.Add(b)
	}
	return reg
}

func TestFuzzyFilterEmpty(t *testing.T) {
	reg := makeReg(nil)
	if entries := FuzzyFilter(reg, ""); entries != nil {
		t.Errorf("empty registry should return nil, got %v", entries)
	}
}

func TestFuzzyFilterNoQuery(t *testing.T) {
	reg := makeReg([]string{"alpha", "beta", "gamma"})
	entries := FuzzyFilter(reg, "")
	if len(entries) != 3 {
		t.Fatalf("no-query: want 3 entries got %d", len(entries))
	}
}

func TestFuzzyFilterMatches(t *testing.T) {
	reg := makeReg([]string{"claude-1", "claude-2", "other"})
	entries := FuzzyFilter(reg, "claude")
	if len(entries) < 2 {
		t.Fatalf("fuzzy 'claude' should match at least 2, got %d", len(entries))
	}
	// "other" should not appear
	for _, e := range entries {
		if e.Buf.Name == "other" {
			t.Error("'other' should not match 'claude'")
		}
	}
}

func TestFuzzyFilterRanking(t *testing.T) {
	// "alp" should rank "alpha" above "allephant" (if present); simple sanity
	reg := makeReg([]string{"allephant", "alpha"})
	entries := FuzzyFilter(reg, "alp")
	if len(entries) == 0 {
		t.Fatal("expected matches for 'alp'")
	}
	// The top result should be "alpha" (tighter match).
	if entries[0].Buf.Name != "alpha" {
		t.Errorf("expected 'alpha' ranked first, got %q", entries[0].Buf.Name)
	}
}

func TestFuzzyFilterMatchSpans(t *testing.T) {
	reg := makeReg([]string{"claude-1"})
	entries := FuzzyFilter(reg, "cl")
	if len(entries) != 1 {
		t.Fatalf("want 1 entry got %d", len(entries))
	}
	if len(entries[0].MatchSpans) == 0 {
		t.Error("match spans should be non-empty for a fuzzy hit")
	}
}
