package ui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/matthewfritsch/neoclaude/internal/render"
	"github.com/matthewfritsch/neoclaude/internal/search"
	"github.com/matthewfritsch/neoclaude/internal/theme"
	"github.com/matthewfritsch/neoclaude/internal/vt"
)

// byteToRuneCol converts a byte offset within line into a rune (cell) column.
// search returns byte offsets (regexp), but the renderer highlights by grid
// column, where ExtractLines maps one cell to one rune. Without this, matches
// on lines containing multi-byte runes (claude's box-drawing UI) shift right.
func byteToRuneCol(line string, byteOff int) int {
	if byteOff > len(line) {
		byteOff = len(line)
	}
	return utf8.RuneCountInString(line[:byteOff])
}

// SearchBar manages the / in-buffer incremental search widget.
type SearchBar struct {
	active    bool
	confirmed bool // true after Enter confirms; matches stay, bar hides
	query     string
	lines     []string // current corpus (scrollback ∪ grid)
	hits      []search.Hit
	cursor    int // index into hits
}

// Open activates the search bar and sets the corpus.
func (s *SearchBar) Open(lines []string) {
	s.active = true
	s.confirmed = false
	s.query = ""
	s.hits = nil
	s.cursor = 0
	s.lines = lines
}

// Close deactivates the search bar.
func (s *SearchBar) Close() {
	s.active = false
	s.confirmed = false
}

// Confirm locks in the current query — matches stay highlighted but the bar
// hides. n/N navigate in Normal mode until Esc clears.
func (s *SearchBar) Confirm() { s.confirmed = true }

// Confirmed reports whether the search has been confirmed with Enter.
func (s *SearchBar) Confirmed() bool { return s.confirmed }

// Active reports visibility.
func (s *SearchBar) Active() bool { return s.active }

// Query returns the current search query.
func (s *SearchBar) Query() string { return s.query }

// CurrentHit returns the grid coordinates of the active hit, or (-1,-1) if none.
func (s *SearchBar) CurrentHit() (row, col int) {
	if len(s.hits) == 0 {
		return -1, -1
	}
	h := s.hits[s.cursor]
	return h.Line, byteToRuneCol(h.Text, h.Col)
}

// Matches returns render.Match slices for all hits (for blit highlighting).
func (s *SearchBar) Matches(gridRows int) []render.Match {
	if len(s.hits) == 0 {
		return nil
	}
	var out []render.Match
	for _, h := range s.hits {
		if h.Line >= gridRows {
			continue
		}
		out = append(out, render.Match{
			Row:      h.Line,
			ColStart: byteToRuneCol(h.Text, h.Col),
			ColEnd:   byteToRuneCol(h.Text, h.MatchEnd),
		})
	}
	return out
}

// HandleKey processes typing keys in Search mode: printable runes, space, and
// backspace all edit the query (so queries may contain n/N/space). Esc is
// handled by the caller to close the search.
func (s *SearchBar) HandleKey(k tea.KeyMsg) {
	switch k.Type {
	case tea.KeyBackspace:
		if len(s.query) > 0 {
			s.query = s.query[:len(s.query)-1]
			s.reindex()
		}
	case tea.KeySpace:
		s.query += " "
		s.reindex()
	case tea.KeyRunes:
		s.query += string(k.Runes)
		s.reindex()
	}
}

// Next advances to the next hit (wraps).
func (s *SearchBar) Next() {
	if len(s.hits) == 0 {
		return
	}
	s.cursor = (s.cursor + 1) % len(s.hits)
}

// Prev moves to the previous hit (wraps).
func (s *SearchBar) Prev() {
	if len(s.hits) == 0 {
		return
	}
	s.cursor = (s.cursor - 1 + len(s.hits)) % len(s.hits)
}

func (s *SearchBar) reindex() {
	s.hits = search.SearchBuffer(0, "", s.lines, s.query)
	s.cursor = 0
}

// UpdateCorpus refreshes the search corpus and re-runs the query.
// Called when the active buffer's grid changes.
func (s *SearchBar) UpdateCorpus(scrollback []string, grid vt.Grid) {
	gridLines := vt.ExtractLines(grid)
	s.lines = append(scrollback, gridLines...)
	if s.query != "" {
		s.reindex()
	}
}

// View renders the one-line search bar for the bottom row.
func (s *SearchBar) View(width int, pal *theme.Palette) string {
	if !s.active {
		return ""
	}
	var style lipgloss.Style
	if pal != nil {
		style = lipgloss.NewStyle().
			Background(lipgloss.Color(pal.Selection)).
			Foreground(lipgloss.Color(pal.Fg))
	} else {
		style = lipgloss.NewStyle().Reverse(true)
	}
	count := ""
	if len(s.hits) > 0 {
		count = fmt.Sprintf(" [%d/%d]", s.cursor+1, len(s.hits))
	} else if s.query != "" {
		count = " [0/0]"
	}
	label := "/" + s.query + count
	pad := width - len([]rune(label))
	if pad < 0 {
		pad = 0
	}
	return style.Render(label + strings.Repeat(" ", pad))
}
