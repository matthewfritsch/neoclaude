package app

import (
	"strings"

	"github.com/matthewfritsch/neoclaude/internal/buffer"
	"github.com/matthewfritsch/neoclaude/internal/mode"
	"github.com/matthewfritsch/neoclaude/internal/render"
	"github.com/matthewfritsch/neoclaude/internal/ui"
	"github.com/matthewfritsch/neoclaude/internal/vt"
)

// View implements tea.Model.
func (m *Model) View() string {
	if m.quitting {
		return ""
	}

	width := m.cols
	if width < 1 {
		width = 80
	}
	rows := m.rows
	if rows < 1 {
		rows = 1
	}

	b := m.reg.Active()
	body := blitBuf(m, b, width, rows)

	// Overlay: picker and grep pane float above the buffer grid.
	if m.picker.Active() {
		overlay := m.picker.View(width, rows)
		composed := overlayCenter(body, overlay, rows)
		status := ui.Statusline(m.fsm.Mode(), activeName(b), activeCwd(b), activeIdx(m), m.reg.Len(), width)
		return composed + "\n" + status
	}
	if m.grep.Active() {
		overlay := m.grep.View(width, rows)
		composed := overlayCenter(body, overlay, rows)
		q := m.grep.QueryStr()
		label := q
		if label == "" {
			label = "(grep — type to search, Esc to close)"
		}
		status := ui.Statusline(m.fsm.Mode(), label, "", 0, 0, width)
		return composed + "\n" + status
	}

	// Bottom row: search bar > cmdline > statusline.
	var bottomRow string
	switch {
	case m.search.Active():
		bottomRow = m.search.View(width)
	case m.cmdline.Active():
		bottomRow = m.cmdline.View(width)
	default:
		bottomRow = ui.Statusline(m.fsm.Mode(), activeName(b), activeCwd(b), activeIdx(m), m.reg.Len(), width)
	}

	return body + "\n" + bottomRow
}

func blitBuf(m *Model, b *buffer.Buffer, width, rows int) string {
	if b == nil {
		return emptyBody(width, rows)
	}

	x, y, visible := b.VT.Cursor()
	opts := render.Options{
		CursorX:       x,
		CursorY:       y,
		CursorVisible: visible,
	}

	snap := b.VT.Snapshot()

	if m.search.Active() {
		opts.SearchMatches = m.search.Matches(snap.Rows)
	}
	if m.fsm.Mode() == mode.Visual {
		opts.Selection = render.Selection{
			Active:   true,
			StartRow: m.visualStart,
			EndRow:   m.visualEnd,
		}
	}

	return render.Blit(snap, opts)
}

func emptyBody(width, rows int) string {
	lines := make([]string, rows)
	return strings.Join(lines, "\n")
}

// overlayCenter composites an overlay string on top of a body by replacing the
// middle rows. Both body and overlay are newline-separated strings.
func overlayCenter(body, overlay string, totalRows int) string {
	bodyLines := strings.Split(body, "\n")
	for len(bodyLines) < totalRows {
		bodyLines = append(bodyLines, "")
	}
	overlayLines := strings.Split(overlay, "\n")
	startRow := (totalRows - len(overlayLines)) / 2
	if startRow < 0 {
		startRow = 0
	}
	for i, ol := range overlayLines {
		row := startRow + i
		if row >= len(bodyLines) {
			break
		}
		bodyLines[row] = ol
	}
	return strings.Join(bodyLines, "\n")
}

func activeName(b *buffer.Buffer) string {
	if b == nil {
		return ""
	}
	return b.Name
}

func activeCwd(b *buffer.Buffer) string {
	if b == nil {
		return ""
	}
	return b.Cwd
}

func activeIdx(m *Model) int {
	if m.reg.Active() == nil {
		return 0
	}
	return m.reg.ActiveIndex() + 1
}

// Ensure vt package is used (ExtractLines is called from update.go).
var _ = vt.ExtractLines
