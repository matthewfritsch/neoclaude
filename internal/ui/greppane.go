package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/matthewfritsch/neoclaude/internal/buffer"
	"github.com/matthewfritsch/neoclaude/internal/search"
	"github.com/matthewfritsch/neoclaude/internal/theme"
)

// GrepResultMsg is sent by the async grep tea.Cmd.
type GrepResultMsg struct {
	Query string
	Hits  []search.Hit
}

// GrepPane is the <leader>sg results overlay.
type GrepPane struct {
	active bool
	query  string
	hits   []search.Hit
	cursor int
}

// Open activates the pane and resets state.
func (g *GrepPane) Open() {
	g.active = true
	g.query = ""
	g.hits = nil
	g.cursor = 0
}

// Close hides the pane.
func (g *GrepPane) Close() { g.active = false }

// Active reports visibility.
func (g *GrepPane) Active() bool { return g.active }

// QueryStr returns the current grep query string.
func (g *GrepPane) QueryStr() string { return g.query }

// SetResults stores results from a completed grep.
func (g *GrepPane) SetResults(query string, hits []search.Hit) {
	g.query = query
	g.hits = hits
	g.cursor = 0
}

// SelectedHit returns the highlighted hit, or nil.
func (g *GrepPane) SelectedHit() *search.Hit {
	if len(g.hits) == 0 || g.cursor >= len(g.hits) {
		return nil
	}
	h := g.hits[g.cursor]
	return &h
}

// HandleKey processes movement keys inside the pane.
func (g *GrepPane) HandleKey(k tea.KeyMsg) (selected buffer.ID, confirm bool) {
	switch k.Type {
	case tea.KeyUp:
		if g.cursor > 0 {
			g.cursor--
		}
	case tea.KeyDown:
		if g.cursor < len(g.hits)-1 {
			g.cursor++
		}
	case tea.KeyEnter:
		if h := g.SelectedHit(); h != nil {
			return h.BufID, true
		}
	case tea.KeyRunes:
		switch string(k.Runes) {
		case "k":
			if g.cursor > 0 {
				g.cursor--
			}
		case "j":
			if g.cursor < len(g.hits)-1 {
				g.cursor++
			}
		}
	}
	return -1, false
}

// View renders the grep pane overlay.
func (g *GrepPane) View(width, height int, pal *theme.Palette) string {
	if !g.active {
		return ""
	}

	borderStyle, titleStyle, matchStyle, selStyle := grepStyles(pal)

	boxW := min(width-4, 70)
	if boxW < 30 {
		boxW = 30
	}
	maxItems := min(height/2, 15)

	var sb strings.Builder
	sb.WriteString(titleStyle.Render(fmt.Sprintf("  Grep: %q", g.query)) + "\n")
	sb.WriteString(strings.Repeat("─", boxW-2) + "\n")

	start := 0
	if g.cursor >= maxItems {
		start = g.cursor - maxItems + 1
	}
	shown := g.hits[start:]
	if len(shown) > maxItems {
		shown = shown[:maxItems]
	}
	for i, h := range shown {
		absIdx := start + i
		line := renderGrepHit(h, absIdx == g.cursor, boxW-6, matchStyle, selStyle)
		sb.WriteString("  " + line + "\n")
	}
	if len(g.hits) == 0 {
		if g.query == "" {
			sb.WriteString("  (type a query)\n")
		} else {
			sb.WriteString("  (no results)\n")
		}
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

func grepStyles(pal *theme.Palette) (border, title, match, sel lipgloss.Style) {
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

func renderGrepHit(h search.Hit, selected bool, maxW int, matchStyle, selStyle lipgloss.Style) string {
	prefix := fmt.Sprintf("%s:%d ", h.BufName, h.Line+1)
	text := h.Text
	var body string
	if h.Col <= len(text) && h.MatchEnd <= len(text) && h.Col < h.MatchEnd {
		before := text[:h.Col]
		matched := matchStyle.Render(text[h.Col:h.MatchEnd])
		after := text[h.MatchEnd:]
		body = before + matched + after
	} else {
		body = text
	}
	line := prefix + body
	visRunes := []rune(prefix + text)
	if len(visRunes) > maxW {
		line = prefix + string([]rune(text)[:max(0, maxW-len([]rune(prefix))-1)]) + "…"
	}
	if selected {
		return selStyle.Render(line)
	}
	return line
}

// GrepCmd returns a tea.Cmd that runs the grep asynchronously.
func GrepCmd(query string, corpus []GrepCorpusEntry) tea.Cmd {
	return func() tea.Msg {
		var allHits []search.Hit
		for _, e := range corpus {
			hits := search.SearchBuffer(e.BufID, e.Name, e.Lines, query)
			allHits = append(allHits, hits...)
		}
		return GrepResultMsg{Query: query, Hits: allHits}
	}
}

// GrepCorpusEntry is one buffer's contribution to the grep corpus.
type GrepCorpusEntry struct {
	BufID buffer.ID
	Name  string
	Lines []string
}
