package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/matthewfritsch/neoclaude/internal/mode"
)

var (
	statusBase  = lipgloss.NewStyle().Reverse(true).Bold(true)
	modeNormal  = lipgloss.NewStyle().Reverse(true).Bold(true).Foreground(lipgloss.Color("2")) // green
	modeInsert  = lipgloss.NewStyle().Reverse(true).Bold(true).Foreground(lipgloss.Color("4")) // blue
	modeCommand = lipgloss.NewStyle().Reverse(true).Bold(true).Foreground(lipgloss.Color("3")) // yellow
)

// Statusline renders the one-line status bar at the bottom of the screen.
// Parameters:
//
//	m        — current mode
//	name     — active buffer name (empty when no buffers open)
//	cwd      — active buffer cwd
//	idx      — 1-based active buffer index (0 = no buffers)
//	total    — total number of open buffers
//	width    — terminal width to pad/truncate to
func Statusline(m mode.Mode, name, cwd string, idx, total, width int) string {
	modeStr := fmt.Sprintf(" %s ", m.String())
	var modeRendered string
	switch m {
	case mode.Normal:
		modeRendered = modeNormal.Render(modeStr)
	case mode.Insert:
		modeRendered = modeInsert.Render(modeStr)
	case mode.Command:
		modeRendered = modeCommand.Render(modeStr)
	default:
		modeRendered = statusBase.Render(modeStr)
	}

	var middle string
	if total == 0 {
		middle = " no buffers — :new to start"
	} else {
		middle = fmt.Sprintf(" %s  %s", name, cwd)
	}

	var right string
	if total > 0 {
		right = fmt.Sprintf(" [%d/%d] ", idx, total)
	} else {
		right = " "
	}

	// Calculate visible widths (approximation: count runes, not ANSI bytes).
	modeVis := len([]rune(modeStr))
	rightVis := len([]rune(right))
	midMax := width - modeVis - rightVis
	if midMax < 0 {
		midMax = 0
	}
	// Truncate middle if needed.
	midRunes := []rune(middle)
	if len(midRunes) > midMax {
		midRunes = midRunes[:midMax]
	}
	// Pad middle to fill the gap between mode and right sections.
	padded := string(midRunes) + strings.Repeat(" ", midMax-len(midRunes))

	return modeRendered + statusBase.Render(padded+right)
}
