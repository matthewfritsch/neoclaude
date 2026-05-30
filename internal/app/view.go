package app

import (
	"github.com/matthewfritsch/neoclaude/internal/render"
	"github.com/matthewfritsch/neoclaude/internal/ui"
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

	b := m.reg.Active()

	// Bottom row: cmdline takes priority over statusline when active.
	var bottomRow string
	if m.cmdline.Active() {
		bottomRow = m.cmdline.View(width)
	} else {
		name, cwd := "", ""
		idx, total := 0, m.reg.Len()
		if b != nil {
			name = b.Name
			cwd = b.Cwd
			idx = m.reg.ActiveIndex() + 1
		}
		bottomRow = ui.Statusline(m.fsm.Mode(), name, cwd, idx, total, width)
	}

	// Body: blit the active buffer's grid, or show the empty-state message.
	var body string
	if b == nil {
		rows := m.rows
		if rows < 1 {
			rows = 1
		}
		body = emptyBody(width, rows)
	} else {
		x, y, visible := b.VT.Cursor()
		body = render.Blit(b.VT.Snapshot(), render.Options{
			CursorX:       x,
			CursorY:       y,
			CursorVisible: visible,
		})
	}

	return body + "\n" + bottomRow
}

// emptyBody renders blank lines for the body area when no buffers are open.
func emptyBody(width, rows int) string {
	if width < 1 {
		width = 1
	}
	line := ""
	out := line
	for i := 1; i < rows; i++ {
		out += "\n" + line
	}
	return out
}
