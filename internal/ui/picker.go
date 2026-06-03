package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/matthewfritsch/neoclaude/internal/registry"
	"github.com/matthewfritsch/neoclaude/internal/search"
	"github.com/matthewfritsch/neoclaude/internal/theme"
)

// Picker is the <leader><leader> fuzzy buffer picker overlay.
type Picker struct {
	active  bool
	query   string
	cursor  int // index into filtered entries
	entries []search.BufEntry
	reg     *registry.Registry
}

// NewPicker creates a Picker bound to the registry.
func NewPicker(reg *registry.Registry) *Picker { return &Picker{reg: reg} }

// Open resets and shows the picker.
func (p *Picker) Open() {
	p.active = true
	p.query = ""
	p.cursor = 0
	p.refresh()
}

// Close hides the picker.
func (p *Picker) Close() { p.active = false }

// Active reports visibility.
func (p *Picker) Active() bool { return p.active }

// SelectedID returns the buffer.ID of the currently highlighted entry, or -1.
func (p *Picker) SelectedID() int {
	if len(p.entries) == 0 || p.cursor >= len(p.entries) {
		return -1
	}
	return int(p.entries[p.cursor].Buf.ID)
}

func (p *Picker) refresh() {
	p.entries = search.FuzzyFilter(p.reg, p.query)
	if p.cursor >= len(p.entries) {
		p.cursor = max(0, len(p.entries)-1)
	}
}

// HandleKey processes picker-mode keys.
func (p *Picker) HandleKey(k tea.KeyMsg) bool {
	switch k.Type {
	case tea.KeyBackspace:
		if len(p.query) > 0 {
			p.query = p.query[:len(p.query)-1]
			p.refresh()
		}
		return true
	case tea.KeyUp:
		if p.cursor > 0 {
			p.cursor--
		}
		return true
	case tea.KeyDown:
		if p.cursor < len(p.entries)-1 {
			p.cursor++
		}
		return true
	case tea.KeyRunes:
		switch string(k.Runes) {
		case "k":
			if p.cursor > 0 {
				p.cursor--
			}
		case "j":
			if p.cursor < len(p.entries)-1 {
				p.cursor++
			}
		default:
			p.query += string(k.Runes)
			p.refresh()
		}
		return true
	}
	return false
}

// View renders the picker overlay centred in width x height.
func (p *Picker) View(width, height int, pal *theme.Palette) string {
	if !p.active {
		return ""
	}

	borderStyle, titleStyle, matchStyle, selStyle := pickerStyles(pal)

	boxW := min(width-4, 60)
	if boxW < 20 {
		boxW = 20
	}
	maxItems := min(height/2, 10)

	var sb strings.Builder
	sb.WriteString(titleStyle.Render("  Buffers") + "\n")
	sb.WriteString(fmt.Sprintf("  > %s\n", p.query))
	sb.WriteString(strings.Repeat("─", boxW-2) + "\n")

	start := 0
	if p.cursor >= maxItems {
		start = p.cursor - maxItems + 1
	}
	shown := p.entries[start:]
	if len(shown) > maxItems {
		shown = shown[:maxItems]
	}
	for i, e := range shown {
		absIdx := start + i
		line := renderEntry(e, absIdx == p.cursor, boxW-4, matchStyle, selStyle)
		sb.WriteString("  " + line + "\n")
	}
	if len(p.entries) == 0 {
		sb.WriteString("  (no matches)\n")
	}

	box := borderStyle.Width(boxW).Render(sb.String())
	pad := (width - lipgloss.Width(box)) / 2
	if pad < 0 {
		pad = 0
	}
	prefix := strings.Repeat(" ", pad)
	lines := strings.Split(box, "\n")
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = prefix + l
	}
	return strings.Join(out, "\n")
}

func pickerStyles(pal *theme.Palette) (border, title, match, sel lipgloss.Style) {
	if pal != nil {
		border = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(pal.Border)).
			Padding(0, 1)
		title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(pal.Accent))
		match = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(pal.Match))
		sel = lipgloss.NewStyle().
			Background(lipgloss.Color(pal.Selection)).
			Foreground(lipgloss.Color(pal.Fg))
	} else {
		border = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
		title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
		match = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3"))
		sel = lipgloss.NewStyle().Reverse(true)
	}
	return
}

func renderEntry(e search.BufEntry, selected bool, maxW int, matchStyle, selStyle lipgloss.Style) string {
	runes := []rune(e.Display)
	matchSet := make(map[int]bool, len(e.MatchSpans))
	for _, idx := range e.MatchSpans {
		if idx < len(runes) {
			matchSet[idx] = true
		}
	}
	var sb strings.Builder
	for i, r := range runes {
		if matchSet[i] {
			sb.WriteString(matchStyle.Render(string(r)))
		} else {
			sb.WriteRune(r)
		}
	}
	line := sb.String()
	visLen := lipgloss.Width(line)
	if visLen > maxW {
		disp := []rune(e.Display)
		if len(disp) > maxW-1 {
			disp = disp[:maxW-1]
		}
		line = string(disp) + "…"
	}
	if selected {
		return selStyle.Render(line)
	}
	return line
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
