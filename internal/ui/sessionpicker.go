package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"

	"github.com/matthewfritsch/neoclaude/internal/persist"
)

var (
	spBorder   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	spSelected = lipgloss.NewStyle().Reverse(true)
	spLive     = lipgloss.NewStyle().Foreground(lipgloss.Color("2")) // green — live buffer
	spClosed   = lipgloss.NewStyle().Foreground(lipgloss.Color("8")) // grey  — closed session
	spTitle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	spMatch    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3"))
)

// SessionEntry is one item in the named-session picker.
type SessionEntry struct {
	// Exactly one of the following is set:
	LiveBufID  int    // >= 0 means this is a live buffer; -1 means closed
	UUID       string // non-empty for closed sessions
	Name       string
	Cwd        string
	Display    string // fuzzy-matched string
	MatchSpans []int
}

// IsLive reports whether this entry is a currently open buffer.
func (e SessionEntry) IsLive() bool { return e.LiveBufID >= 0 }

// SessionPickerOpenMsg is sent to open the picker with a pre-built entry list.
type SessionPickerOpenMsg struct {
	Entries []SessionEntry
}

// SessionPickerChoiceMsg is sent when the user confirms an entry.
type SessionPickerChoiceMsg struct {
	Entry SessionEntry
}

// SessionPicker is the <leader>sn overlay: live buffers + closed persisted sessions.
type SessionPicker struct {
	active  bool
	query   string
	cursor  int
	all     []SessionEntry // full unfiltered list
	entries []SessionEntry // current filtered+ranked list
}

// Open populates and shows the picker.
func (p *SessionPicker) Open(entries []SessionEntry) {
	p.active = true
	p.query = ""
	p.cursor = 0
	p.all = entries
	p.filter()
}

// Close hides the picker.
func (p *SessionPicker) Close() { p.active = false }

// Active reports visibility.
func (p *SessionPicker) Active() bool { return p.active }

// Selected returns the currently highlighted entry, or nil.
func (p *SessionPicker) Selected() *SessionEntry {
	if len(p.entries) == 0 || p.cursor >= len(p.entries) {
		return nil
	}
	e := p.entries[p.cursor]
	return &e
}

func (p *SessionPicker) filter() {
	if p.query == "" {
		cp := make([]SessionEntry, len(p.all))
		copy(cp, p.all)
		for i := range cp {
			cp[i].MatchSpans = nil
		}
		p.entries = cp
	} else {
		displays := make([]string, len(p.all))
		for i, e := range p.all {
			displays[i] = e.Display
		}
		matches := fuzzy.Find(p.query, displays)
		p.entries = make([]SessionEntry, len(matches))
		for i, m := range matches {
			e := p.all[m.Index]
			e.MatchSpans = m.MatchedIndexes
			p.entries[i] = e
		}
	}
	if p.cursor >= len(p.entries) {
		if len(p.entries) > 0 {
			p.cursor = len(p.entries) - 1
		} else {
			p.cursor = 0
		}
	}
}

// HandleKey processes keys inside the picker. Returns (entry, true) when the
// user confirms a selection (Enter), (nil, false) otherwise.
func (p *SessionPicker) HandleKey(k tea.KeyMsg) (*SessionEntry, bool) {
	switch k.Type {
	case tea.KeyBackspace:
		if len(p.query) > 0 {
			p.query = p.query[:len(p.query)-1]
			p.filter()
		}
	case tea.KeyUp:
		if p.cursor > 0 {
			p.cursor--
		}
	case tea.KeyDown:
		if p.cursor < len(p.entries)-1 {
			p.cursor++
		}
	case tea.KeyEnter:
		if sel := p.Selected(); sel != nil {
			return sel, true
		}
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
			p.filter()
		}
	}
	return nil, false
}

// View renders the picker overlay.
func (p *SessionPicker) View(width, height int) string {
	if !p.active {
		return ""
	}
	boxW := min(width-4, 65)
	if boxW < 24 {
		boxW = 24
	}
	maxItems := min(height/2, 12)

	var sb strings.Builder
	sb.WriteString(spTitle.Render("  Sessions (live + closed)") + "\n")
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
		line := renderSessionEntry(e, absIdx == p.cursor, boxW-4)
		sb.WriteString("  " + line + "\n")
	}
	if len(p.entries) == 0 {
		sb.WriteString("  (no sessions)\n")
	}

	box := spBorder.Width(boxW).Render(sb.String())
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

func renderSessionEntry(e SessionEntry, selected bool, maxW int) string {
	// Build label: highlight fuzzy match chars.
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
			sb.WriteString(spMatch.Render(string(r)))
		} else {
			sb.WriteRune(r)
		}
	}
	label := sb.String()

	// Colour by live/closed status.
	if e.IsLive() {
		label = spLive.Render("● ") + label
	} else {
		label = spClosed.Render("○ ") + label + spClosed.Render(" (closed)")
	}

	// Simple truncation.
	visLen := lipgloss.Width(label)
	if visLen > maxW {
		raw := []rune(e.Display)
		if len(raw) > maxW-4 {
			raw = raw[:maxW-4]
		}
		label = string(raw) + "…"
	}

	if selected {
		return spSelected.Render(label)
	}
	return label
}

// BuildSessionEntries assembles the entry list for the session picker from live
// buffers (via liveEntries) and closed persisted records.
func BuildSessionEntries(liveEntries []SessionEntry, store *persist.Store, openUUIDs map[string]bool) []SessionEntry {
	out := make([]SessionEntry, 0, len(liveEntries)+8)
	// Live buffers first.
	out = append(out, liveEntries...)
	// Closed persisted sessions.
	for _, r := range store.Closed(openUUIDs) {
		out = append(out, SessionEntry{
			LiveBufID: -1,
			UUID:      r.UUID,
			Name:      r.Name,
			Cwd:       r.Cwd,
			Display:   r.Name + " " + r.Cwd,
		})
	}
	return out
}
