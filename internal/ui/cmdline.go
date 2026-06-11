package ui

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/matthewfritsch/neoclaude/internal/theme"
)

var knownCommands = []string{
	"bd", "bn", "bp", "claude", "codex", "commands", "import", "keybinds", "motd", "name", "new", "new_claude", "new_codex", "q", "theme",
}

// Cmdline is a minimal inline text-input widget for the : command line.
type Cmdline struct {
	buf    []rune
	cursor int
	active bool

	// Completions maps a command verb to its possible argument values.
	Completions map[string][]string

	history    []string
	histIdx    int // -1 = not browsing history
	histPrefix string
}

// Open resets the cmdline and marks it active.
func (c *Cmdline) Open() {
	c.buf = c.buf[:0]
	c.cursor = 0
	c.active = true
	c.histIdx = -1
}

// Close marks the cmdline inactive.
func (c *Cmdline) Close() { c.active = false }

// Active reports whether the cmdline is visible.
func (c *Cmdline) Active() bool { return c.active }

// Value returns the current text without the leading colon.
func (c *Cmdline) Value() string { return string(c.buf) }

// PushHistory records an executed command for up-arrow recall.
func (c *Cmdline) PushHistory(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	if len(c.history) > 0 && c.history[len(c.history)-1] == line {
		return
	}
	c.history = append(c.history, line)
}

// HandleKey processes a key while the cmdline is active.
func (c *Cmdline) HandleKey(k tea.KeyMsg) bool {
	switch k.Type {
	case tea.KeyBackspace:
		if c.cursor > 0 {
			c.buf = append(c.buf[:c.cursor-1], c.buf[c.cursor:]...)
			c.cursor--
			c.histIdx = -1
		}
		return true

	case tea.KeyDelete:
		if c.cursor < len(c.buf) {
			c.buf = append(c.buf[:c.cursor], c.buf[c.cursor+1:]...)
			c.histIdx = -1
		}
		return true

	case tea.KeyLeft:
		if c.cursor > 0 {
			c.cursor--
		}
		return true

	case tea.KeyRight:
		if c.cursor < len(c.buf) {
			c.cursor++
		}
		return true

	case tea.KeyHome, tea.KeyCtrlA:
		c.cursor = 0
		return true

	case tea.KeyEnd, tea.KeyCtrlE:
		c.cursor = len(c.buf)
		return true

	case tea.KeyTab:
		c.tabComplete()
		return true

	case tea.KeyUp:
		c.historyUp()
		return true

	case tea.KeyDown:
		c.historyDown()
		return true

	case tea.KeyRunes:
		ins := k.Runes
		c.buf = append(c.buf[:c.cursor], append(ins, c.buf[c.cursor:]...)...)
		c.cursor += len(ins)
		c.histIdx = -1
		return true

	case tea.KeySpace:
		c.buf = append(c.buf[:c.cursor], append([]rune{' '}, c.buf[c.cursor:]...)...)
		c.cursor++
		c.histIdx = -1
		return true
	}
	return false
}

func (c *Cmdline) historyUp() {
	if len(c.history) == 0 {
		return
	}
	if c.histIdx == -1 {
		c.histPrefix = string(c.buf)
		c.histIdx = len(c.history)
	}
	for i := c.histIdx - 1; i >= 0; i-- {
		if strings.HasPrefix(c.history[i], c.histPrefix) {
			c.histIdx = i
			c.buf = []rune(c.history[i])
			c.cursor = len(c.buf)
			return
		}
	}
}

func (c *Cmdline) historyDown() {
	if c.histIdx == -1 {
		return
	}
	for i := c.histIdx + 1; i < len(c.history); i++ {
		if strings.HasPrefix(c.history[i], c.histPrefix) {
			c.histIdx = i
			c.buf = []rune(c.history[i])
			c.cursor = len(c.buf)
			return
		}
	}
	c.histIdx = -1
	c.buf = []rune(c.histPrefix)
	c.cursor = len(c.buf)
}

func (c *Cmdline) tabComplete() {
	line := string(c.buf)
	cmd, arg, hasSpace := strings.Cut(line, " ")
	cmd = strings.TrimSpace(cmd)

	if !hasSpace {
		var matches []string
		for _, name := range knownCommands {
			if strings.HasPrefix(name, cmd) {
				matches = append(matches, name)
			}
		}
		if len(matches) == 1 {
			c.buf = []rune(matches[0])
			c.cursor = len(c.buf)
		} else if len(matches) > 1 {
			prefix := longestCommonPrefix(matches)
			if len(prefix) > len(cmd) {
				c.buf = []rune(prefix)
				c.cursor = len(c.buf)
			}
		}
		return
	}

	switch cmd {
	case "new", "new_claude", "new_codex", "claude", "codex":
		c.completeFromList(cmd, arg, CompletePath(arg))
	case "theme":
		if c.Completions != nil {
			var matches []string
			for _, name := range c.Completions["theme"] {
				if strings.HasPrefix(name, arg) {
					matches = append(matches, name)
				}
			}
			c.completeFromList(cmd, arg, matches)
		}
	}
}

func (c *Cmdline) completeFromList(cmd, arg string, matches []string) {
	if len(matches) == 0 {
		return
	}
	if len(matches) == 1 {
		newLine := cmd + " " + matches[0]
		c.buf = []rune(newLine)
		c.cursor = len(c.buf)
		return
	}
	prefix := longestCommonPrefix(matches)
	if len(prefix) > len(arg) {
		newLine := cmd + " " + prefix
		c.buf = []rune(newLine)
		c.cursor = len(c.buf)
	}
}

// CompletePath returns filesystem entries whose path starts with prefix.
func CompletePath(prefix string) []string {
	expanded := expandHome(prefix)

	dir := filepath.Dir(expanded)
	base := filepath.Base(expanded)

	if expanded == "" || expanded == "." || strings.HasSuffix(expanded, "/") {
		dir = expanded
		if dir == "" {
			dir = "."
		}
		base = ""
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var matches []string
	for _, e := range entries {
		name := e.Name()
		if base != "" && !strings.HasPrefix(name, base) {
			continue
		}
		full := filepath.Join(dir, name)
		isDir := e.IsDir()
		if !isDir {
			if info, err := os.Stat(full); err == nil && info.IsDir() {
				isDir = true
			}
		}
		if strings.HasPrefix(prefix, "~/") || prefix == "~" {
			home, _ := os.UserHomeDir()
			if strings.HasPrefix(full, home) {
				full = "~" + full[len(home):]
			}
		}
		if isDir {
			full += "/"
		}
		matches = append(matches, full)
	}
	return matches
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return p
		}
		return home + p[1:]
	}
	return p
}

func longestCommonPrefix(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	prefix := ss[0]
	for _, s := range ss[1:] {
		for !strings.HasPrefix(s, prefix) {
			if len(prefix) == 0 {
				return ""
			}
			prefix = prefix[:len(prefix)-1]
		}
	}
	return prefix
}

// View renders the cmdline into a fixed-width string.
func (c *Cmdline) View(width int, pal *theme.Palette) string {
	if !c.active {
		return ""
	}
	var cursorStyle lipgloss.Style
	if pal != nil {
		cursorStyle = lipgloss.NewStyle().
			Background(lipgloss.Color(pal.Fg)).
			Foreground(lipgloss.Color(pal.Bg))
	} else {
		cursorStyle = lipgloss.NewStyle().Reverse(true)
	}

	before := ":" + string(c.buf[:c.cursor])
	after := string(c.buf[c.cursor:])

	cursorChar := " "
	if c.cursor < len(c.buf) {
		cursorChar = string(c.buf[c.cursor])
		after = string(c.buf[c.cursor+1:])
	}

	cursorRendered := cursorStyle.Render(cursorChar)
	line := before + cursorRendered + after

	visible := len([]rune(before)) + len([]rune(cursorChar)) + len([]rune(after))
	if visible < width {
		line += strings.Repeat(" ", width-visible)
	}
	return line
}
