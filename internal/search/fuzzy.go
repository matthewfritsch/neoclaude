package search

import (
	"github.com/sahilm/fuzzy"

	"github.com/matthewfritsch/neoclaude/internal/buffer"
	"github.com/matthewfritsch/neoclaude/internal/registry"
)

// BufEntry is one item shown in the buffer picker.
type BufEntry struct {
	Buf        *buffer.Buffer
	Display    string // what fuzzy-matches against (name + cwd)
	MatchSpans []int  // matched character indices within Display (from sahilm/fuzzy)
}

// FuzzyFilter filters the open buffers by query and returns ranked BufEntries.
// Empty query returns all buffers in registry order.
func FuzzyFilter(reg *registry.Registry, query string) []BufEntry {
	bufs := reg.All()
	if len(bufs) == 0 {
		return nil
	}

	displays := make([]string, len(bufs))
	for i, b := range bufs {
		displays[i] = b.Name + " " + b.Cwd
		if b.Agent.String() != "claude" {
			displays[i] = b.Agent.String() + " " + displays[i]
		}
	}

	if query == "" {
		entries := make([]BufEntry, len(bufs))
		for i, b := range bufs {
			entries[i] = BufEntry{Buf: b, Display: displays[i]}
		}
		return entries
	}

	matches := fuzzy.Find(query, displays)
	entries := make([]BufEntry, len(matches))
	for i, m := range matches {
		entries[i] = BufEntry{
			Buf:        bufs[m.Index],
			Display:    displays[m.Index],
			MatchSpans: m.MatchedIndexes,
		}
	}
	return entries
}
