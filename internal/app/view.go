package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

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
	if b == nil && m.reg.Len() == 0 {
		body := m.motd(width, rows)
		status := ui.Statusline(m.fsm.Mode(), "neoclaude", "", m.fsm.PendingKeys(), 0, 0, width, m.palette)
		return body + "\n" + status
	}
	body := blitBuf(m, b, width, rows)

	// Info overlay (highest priority — :commands, :keybinds).
	if len(m.infoLines) > 0 {
		bc := ""
		if m.palette != nil {
			bc = m.palette.Border
		}
		overlay := renderInfoOverlay(m.infoLines, m.infoScroll, width, rows, bc)
		composed := overlayCenter(body, overlay, rows)
		status := ui.Statusline(m.fsm.Mode(), "Esc to close", "", "", 0, 0, width, m.palette)
		return composed + "\n" + status
	}

	if m.sessionPicker.Active() {
		overlay := m.sessionPicker.View(width, rows, m.palette)
		composed := overlayCenter(body, overlay, rows)
		status := ui.Statusline(m.fsm.Mode(), "Sessions — Enter to switch/resume, Esc to close", "", "", 0, 0, width, m.palette)
		return composed + "\n" + status
	}

	if m.picker.Active() {
		overlay := m.picker.View(width, rows, m.palette)
		composed := overlayCenter(body, overlay, rows)
		status := ui.Statusline(m.fsm.Mode(), activeName(b), activeCwd(b), "", activeIdx(m), m.reg.Len(), width, m.palette)
		return composed + "\n" + status
	}
	if m.grep.Active() {
		overlay := m.grep.View(width, rows, m.palette)
		composed := overlayCenter(body, overlay, rows)
		q := m.grep.QueryStr()
		label := q
		if label == "" {
			label = "(grep — type to search, Esc to close)"
		}
		status := ui.Statusline(m.fsm.Mode(), label, "", "", 0, 0, width, m.palette)
		return composed + "\n" + status
	}

	var bottomRow string
	switch {
	case m.search.Active() && !m.search.Confirmed():
		bottomRow = m.search.View(width, m.palette)
	case m.cmdline.Active():
		bottomRow = m.cmdline.View(width, m.palette)
	default:
		bottomRow = ui.Statusline(m.fsm.Mode(), activeName(b), activeCwd(b), m.fsm.PendingKeys(), activeIdx(m), m.reg.Len(), width, m.palette)
	}

	return body + "\n" + bottomRow
}

func blitBuf(m *Model, b *buffer.Buffer, width, rows int) string {
	if b == nil {
		return emptyBody(width, rows)
	}

	x, y, visible := b.VT.Cursor()
	if b.ScrollOffset > 0 {
		visible = false
	}
	opts := render.Options{
		CursorX:       x,
		CursorY:       y,
		CursorVisible: visible,
	}

	if m.palette != nil {
		opts.ANSI16 = m.palette.ANSI16Ptr()
		opts.MatchBg = m.palette.Match
		opts.MatchFg = m.palette.Bg
		opts.SelectionBg = m.palette.Selection
		opts.SelectionFg = m.palette.Fg
	}

	snap := b.VT.SnapshotAt(b.ScrollOffset)

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

func (m *Model) motd(width, rows int) string {
	center := func(s string) string {
		pad := (width - len([]rune(s))) / 2
		if pad < 0 {
			pad = 0
		}
		return strings.Repeat(" ", pad) + s
	}

	var lines []string
	lines = append(lines, "")
	lines = append(lines, center("neoclaude"))
	lines = append(lines, center("Neovim-flavored Claude Code multiplexer"))
	lines = append(lines, "")
	lines = append(lines, center(":new [path]       open a new session"))
	lines = append(lines, center(":import           import sessions from ~/.claude/"))

	leader := string(m.cfg.LeaderRune)
	if leader == " " {
		leader = "Space"
	}
	lines = append(lines, center(leader+" s n             resume a previous session"))
	lines = append(lines, center(":q                quit"))
	lines = append(lines, "")

	closed := m.store.Closed(nil)
	if len(closed) > 0 {
		lines = append(lines, center("--- recent sessions ---"))
		show := closed
		if len(show) > 5 {
			show = show[len(show)-5:]
		}
		for i := len(show) - 1; i >= 0; i-- {
			r := show[i]
			lines = append(lines, center(r.Name+"  "+r.Cwd))
		}
	}

	// Pad to fill the screen.
	for len(lines) < rows {
		lines = append(lines, "")
	}
	if len(lines) > rows {
		lines = lines[:rows]
	}
	return strings.Join(lines, "\n")
}

func emptyBody(width, rows int) string {
	lines := make([]string, rows)
	return strings.Join(lines, "\n")
}

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

func renderInfoOverlay(lines []string, scroll, width, height int, borderColor string) string {
	borderStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	if borderColor != "" {
		borderStyle = borderStyle.BorderForeground(lipgloss.Color(borderColor))
	}

	maxW := 0
	for _, l := range lines {
		if w := len([]rune(l)); w > maxW {
			maxW = w
		}
	}
	boxW := maxW + 4
	if boxW > width-4 {
		boxW = width - 4
	}
	if boxW < 20 {
		boxW = 20
	}

	// Apply scroll: show a window of lines that fits in the overlay.
	maxVisible := height - 6 // account for border + padding
	if maxVisible < 3 {
		maxVisible = 3
	}
	if scroll > len(lines)-maxVisible {
		scroll = len(lines) - maxVisible
	}
	if scroll < 0 {
		scroll = 0
	}
	visible := lines[scroll:]
	if len(visible) > maxVisible {
		visible = visible[:maxVisible]
	}

	content := strings.Join(visible, "\n")
	if scroll > 0 {
		content = "  ▲ scroll up\n" + content
	}
	if scroll+len(visible) < len(lines) {
		content = content + "\n  ▼ scroll down"
	}

	box := borderStyle.Width(boxW).Render(content)

	pad := (width - lipgloss.Width(box)) / 2
	if pad < 0 {
		pad = 0
	}
	prefix := strings.Repeat(" ", pad)
	boxLines := strings.Split(box, "\n")
	out := make([]string, len(boxLines))
	for i, l := range boxLines {
		out[i] = prefix + l
	}
	return strings.Join(out, "\n")
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

var _ = vt.ExtractLines
