// Package render blits a vt cell grid into a styled string for lipgloss/Bubble
// Tea to print. It coalesces runs of identically-styled cells into a single
// styled segment, passes truecolor through untouched, overlays the cursor as
// a reverse-video cell, and highlights search matches and visual selections.
package render

import (
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/matthewfritsch/neoclaude/internal/vt"
)

// renderer is pinned to TrueColor so that 24-bit child colors are emitted
// verbatim into the View string regardless of stdout auto-detection.
var renderer = func() *lipgloss.Renderer {
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(termenv.TrueColor)
	return r
}()

// Match is a highlighted span within one grid row (byte offsets into the
// rendered rune sequence, i.e. column indices).
type Match struct {
	Row      int
	ColStart int // inclusive
	ColEnd   int // exclusive
}

// Selection is a linewise visual selection region.
type Selection struct {
	Active   bool
	StartRow int
	EndRow   int // inclusive; StartRow <= EndRow guaranteed by caller
}

// Options controls how a grid is rendered.
type Options struct {
	CursorX, CursorY int
	CursorVisible    bool
	// SearchMatches highlights regex hits. Row/Col values are grid coordinates.
	SearchMatches []Match
	// Selection overlays a visual-mode highlight region.
	Selection Selection
}

// Blit renders a grid into a multi-line string (lines joined with "\n", no
// trailing newline).
func Blit(g vt.Grid, opts Options) string {
	// Build per-row match-column sets for O(1) lookup during blit.
	matchCols := buildMatchCols(opts.SearchMatches, g.Rows)

	var sb strings.Builder
	for y := 0; y < g.Rows; y++ {
		if y > 0 {
			sb.WriteByte('\n')
		}
		blitRow(&sb, g, y, opts, matchCols[y])
	}
	return sb.String()
}

// buildMatchCols returns a slice[rows] of sets of highlighted column indices.
func buildMatchCols(matches []Match, rows int) []map[int]bool {
	out := make([]map[int]bool, rows)
	for _, m := range matches {
		if m.Row < 0 || m.Row >= rows {
			continue
		}
		if out[m.Row] == nil {
			out[m.Row] = make(map[int]bool)
		}
		for c := m.ColStart; c < m.ColEnd; c++ {
			out[m.Row][c] = true
		}
	}
	return out
}

func blitRow(sb *strings.Builder, g vt.Grid, y int, opts Options, matchSet map[int]bool) {
	row := g.Cells[y]
	selRow := opts.Selection.Active && y >= opts.Selection.StartRow && y <= opts.Selection.EndRow

	x := 0
	for x < g.Cols {
		cell := row[x]
		cursorHere := opts.CursorVisible && y == opts.CursorY && x == opts.CursorX
		hitHere := matchSet != nil && matchSet[x]
		selHere := selRow

		// Cursor and highlighted cells are each rendered individually to avoid
		// breaking coalescing logic for the surrounding run.
		if cursorHere || hitHere || selHere {
			sb.WriteString(styleForCell(cell, cursorHere, hitHere, selHere).Render(string(cell.Rune)))
			x++
			continue
		}

		// Coalesce a run of identically-styled plain cells.
		var run strings.Builder
		run.WriteRune(cell.Rune)
		x++
		for x < g.Cols {
			next := row[x]
			nextCursor := opts.CursorVisible && y == opts.CursorY && x == opts.CursorX
			nextHit := matchSet != nil && matchSet[x]
			nextSel := selRow
			if nextCursor || nextHit || nextSel || !sameStyle(cell, next) {
				break
			}
			run.WriteRune(next.Rune)
			x++
		}
		sb.WriteString(styleForCell(cell, false, false, false).Render(run.String()))
	}
}

func sameStyle(a, b vt.Cell) bool {
	return a.FG == b.FG && a.BG == b.BG && a.Attrs == b.Attrs
}

func styleForCell(c vt.Cell, cursor, searchHit, selected bool) lipgloss.Style {
	s := renderer.NewStyle()

	if fg, ok := colorToLipgloss(c.FG); ok {
		s = s.Foreground(fg)
	}
	if bg, ok := colorToLipgloss(c.BG); ok {
		s = s.Background(bg)
	}
	if c.Attrs&vt.AttrBold != 0 {
		s = s.Bold(true)
	}
	if c.Attrs&vt.AttrUnderline != 0 {
		s = s.Underline(true)
	}
	if c.Attrs&vt.AttrItalic != 0 {
		s = s.Italic(true)
	}
	if c.Attrs&vt.AttrBlink != 0 {
		s = s.Blink(true)
	}

	// Priority: cursor > search hit > visual selection > child reverse attr.
	// XOR child reverse with cursor so a cursor on a reversed cell looks normal.
	childReverse := c.Attrs&vt.AttrReverse != 0
	switch {
	case cursor:
		reverse := !childReverse
		if reverse {
			s = s.Reverse(true)
		}
	case searchHit:
		// Yellow background highlight for search matches.
		s = s.Background(lipgloss.Color("3")).Foreground(lipgloss.Color("0"))
	case selected:
		// Blue background for visual selection.
		s = s.Background(lipgloss.Color("4")).Foreground(lipgloss.Color("15"))
	default:
		if childReverse {
			s = s.Reverse(true)
		}
	}
	return s
}

func colorToLipgloss(c vt.Color) (lipgloss.TerminalColor, bool) {
	switch c.Kind {
	case vt.ColorPalette:
		return lipgloss.ANSIColor(c.Palette), true
	case vt.ColorRGB:
		return lipgloss.Color(hex(c.R, c.G, c.B)), true
	default:
		return nil, false
	}
}

const hexDigits = "0123456789abcdef"

func hex(r, g, b uint8) string {
	out := []byte{'#', 0, 0, 0, 0, 0, 0}
	out[1] = hexDigits[r>>4]
	out[2] = hexDigits[r&0xf]
	out[3] = hexDigits[g>>4]
	out[4] = hexDigits[g&0xf]
	out[5] = hexDigits[b>>4]
	out[6] = hexDigits[b&0xf]
	return string(out)
}
