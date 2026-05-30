// Package render blits a vt cell grid into a styled string for lipgloss/Bubble
// Tea to print. It coalesces runs of identically-styled cells into a single
// styled segment to keep ANSI output compact, passes truecolor through
// untouched, and overlays the cursor as a reverse-video cell.
package render

import (
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/matthewfritsch/neoclaude/internal/vt"
)

// renderer is pinned to TrueColor so that 24-bit child colors are emitted
// verbatim into the View string (which Bubble Tea writes to the real terminal),
// rather than being quantized based on stdout auto-detection. Palette colors
// still emit their compact ANSI form.
var renderer = func() *lipgloss.Renderer {
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(termenv.TrueColor)
	return r
}()

// Options controls how a grid is rendered.
type Options struct {
	// CursorX, CursorY locate the active cursor cell (grid coordinates).
	CursorX, CursorY int
	// CursorVisible toggles the reverse-video cursor overlay.
	CursorVisible bool
}

// Blit renders a grid into a multi-line string. Lines are joined with "\n";
// there is no trailing newline.
func Blit(g vt.Grid, opts Options) string {
	var sb strings.Builder
	for y := 0; y < g.Rows; y++ {
		if y > 0 {
			sb.WriteByte('\n')
		}
		blitRow(&sb, g, y, opts)
	}
	return sb.String()
}

func blitRow(sb *strings.Builder, g vt.Grid, y int, opts Options) {
	row := g.Cells[y]
	x := 0
	for x < g.Cols {
		cell := row[x]
		cursorHere := opts.CursorVisible && y == opts.CursorY && x == opts.CursorX

		// The cursor cell is rendered on its own so its reverse-video overlay
		// does not break run coalescing of the surrounding cells.
		if cursorHere {
			sb.WriteString(styleForCell(cell, true).Render(string(cell.Rune)))
			x++
			continue
		}

		// Coalesce a run of cells sharing identical styling (and not the cursor).
		start := x
		var run strings.Builder
		run.WriteRune(cell.Rune)
		x++
		for x < g.Cols {
			next := row[x]
			nextCursor := opts.CursorVisible && y == opts.CursorY && x == opts.CursorX
			if nextCursor || !sameStyle(cell, next) {
				break
			}
			run.WriteRune(next.Rune)
			x++
		}
		_ = start
		sb.WriteString(styleForCell(cell, false).Render(run.String()))
	}
}

func sameStyle(a, b vt.Cell) bool {
	return a.FG == b.FG && a.BG == b.BG && a.Attrs == b.Attrs
}

func styleForCell(c vt.Cell, cursor bool) lipgloss.Style {
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
	// Reverse from the child stream, XOR'd with the cursor overlay so that a
	// cursor on an already-reversed cell renders normally (still distinct).
	reverse := (c.Attrs&vt.AttrReverse != 0) != cursor
	if reverse {
		s = s.Reverse(true)
	}
	return s
}

// colorToLipgloss converts a decoded vt color to a lipgloss color. The bool is
// false for default colors (let the terminal decide). Truecolor passes through
// as a "#rrggbb" hex string; palette colors pass through as their ANSI index so
// termenv emits the compact SGR form. Truecolor is never quantized here.
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
