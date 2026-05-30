// Package ui provides the statusline and command-line widgets for neoclaude.
package ui

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Cmdline is a minimal inline text-input widget for the : command line.
// It does not depend on charmbracelet/bubbles — just a []rune buffer + cursor.
type Cmdline struct {
	buf    []rune
	cursor int // insertion point (0..len(buf))
	active bool
}

// Open resets the cmdline and marks it active (called on : in Normal mode).
func (c *Cmdline) Open() {
	c.buf = c.buf[:0]
	c.cursor = 0
	c.active = true
}

// Close marks the cmdline inactive.
func (c *Cmdline) Close() { c.active = false }

// Active reports whether the cmdline is visible.
func (c *Cmdline) Active() bool { return c.active }

// Value returns the current text without the leading colon.
func (c *Cmdline) Value() string { return string(c.buf) }

// HandleKey processes a key while the cmdline is active. Returns true if the
// caller should re-render. Enter and Esc are handled by the mode FSM before
// this is called, so we only see typing keys here.
func (c *Cmdline) HandleKey(k tea.KeyMsg) bool {
	switch k.Type {
	case tea.KeyBackspace:
		if c.cursor > 0 {
			c.buf = append(c.buf[:c.cursor-1], c.buf[c.cursor:]...)
			c.cursor--
		}
		return true

	case tea.KeyDelete:
		if c.cursor < len(c.buf) {
			c.buf = append(c.buf[:c.cursor], c.buf[c.cursor+1:]...)
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

	case tea.KeyRunes:
		ins := k.Runes
		c.buf = append(c.buf[:c.cursor], append(ins, c.buf[c.cursor:]...)...)
		c.cursor += len(ins)
		return true

	case tea.KeySpace:
		c.buf = append(c.buf[:c.cursor], append([]rune{' '}, c.buf[c.cursor:]...)...)
		c.cursor++
		return true
	}
	return false
}

// tabComplete fills in the longest unambiguous path prefix for the argument of
// a :new command. If there is exactly one match it appends a trailing slash for
// directories.
func (c *Cmdline) tabComplete() {
	line := string(c.buf)
	// Only complete the argument to :new.
	cmd, arg, _ := strings.Cut(line, " ")
	if strings.TrimSpace(cmd) != "new" {
		return
	}
	matches := CompletePath(arg)
	if len(matches) == 0 {
		return
	}
	if len(matches) == 1 {
		// Replace the arg portion with the single match.
		newLine := cmd + " " + matches[0]
		c.buf = []rune(newLine)
		c.cursor = len(c.buf)
		return
	}
	// Multiple matches: complete to the longest common prefix.
	prefix := longestCommonPrefix(matches)
	if len(prefix) > len(arg) {
		newLine := cmd + " " + prefix
		c.buf = []rune(newLine)
		c.cursor = len(c.buf)
	}
}

// CompletePath returns filesystem entries whose path starts with prefix.
// Directories get a trailing slash appended.
func CompletePath(prefix string) []string {
	// Expand ~ at the start.
	expanded := expandHome(prefix)

	dir := filepath.Dir(expanded)
	base := filepath.Base(expanded)

	// filepath.Dir of an empty string or bare name returns ".".
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
		// Restore ~ prefix if the original had it.
		if strings.HasPrefix(prefix, "~/") || prefix == "~" {
			home, _ := os.UserHomeDir()
			if strings.HasPrefix(full, home) {
				full = "~" + full[len(home):]
			}
		}
		if e.IsDir() {
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

var (
	cmdlineStyle = lipgloss.NewStyle().Reverse(true)
)

// View renders the cmdline into a fixed-width string. When inactive it returns
// an empty string (caller fills the status row).
func (c *Cmdline) View(width int) string {
	if !c.active {
		return ""
	}
	// Build the display: ":<before-cursor>|<after-cursor>" where | is the
	// cursor rendered as a reversed block.
	before := ":" + string(c.buf[:c.cursor])
	after := string(c.buf[c.cursor:])

	cursorChar := " "
	if c.cursor < len(c.buf) {
		cursorChar = string(c.buf[c.cursor])
		after = string(c.buf[c.cursor+1:])
	}

	cursorRendered := cmdlineStyle.Render(cursorChar)
	line := before + cursorRendered + after

	// Pad to width so the row is fully filled.
	visible := len([]rune(before)) + len([]rune(cursorChar)) + len([]rune(after))
	if visible < width {
		line += strings.Repeat(" ", width-visible)
	}
	return line
}
