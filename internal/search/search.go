// Package search provides in-memory text search over buffer lines and
// cross-buffer grep, plus fuzzy matching for the buffer picker.
package search

import (
	"regexp"

	"github.com/matthewfritsch/neoclaude/internal/buffer"
)

// Hit is one search result.
type Hit struct {
	BufID    buffer.ID
	BufName  string
	Line     int    // 0-based line index within the corpus
	Col      int    // 0-based byte offset of match start within Text
	MatchEnd int    // byte offset of match end (Col..MatchEnd is the match span)
	Text     string // full line text
}

// SearchBuffer runs a regex query over lines and returns all hits.
// lines is the union of scrollback + visible grid lines for one buffer.
// Returns nil on empty query or compile error.
func SearchBuffer(bufID buffer.ID, bufName string, lines []string, query string) []Hit {
	if query == "" {
		return nil
	}
	re, err := regexp.Compile(query)
	if err != nil {
		return nil
	}
	var hits []Hit
	for i, line := range lines {
		loc := re.FindStringIndex(line)
		if loc == nil {
			continue
		}
		hits = append(hits, Hit{
			BufID:    bufID,
			BufName:  bufName,
			Line:     i,
			Col:      loc[0],
			MatchEnd: loc[1],
			Text:     line,
		})
	}
	return hits
}

// AllMatches returns every match on the line (not just the first), useful for
// in-buffer / highlighting.
func AllMatches(line, query string) [][2]int {
	if query == "" {
		return nil
	}
	re, err := regexp.Compile(query)
	if err != nil {
		return nil
	}
	locs := re.FindAllStringIndex(line, -1)
	spans := make([][2]int, len(locs))
	for i, l := range locs {
		spans[i] = [2]int{l[0], l[1]}
	}
	return spans
}
