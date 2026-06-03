package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/matthewfritsch/neoclaude/internal/mode"
	"github.com/matthewfritsch/neoclaude/internal/theme"
)

// Statusline renders the one-line status bar at the bottom of the screen.
// pending is the in-progress chord (e.g. "<Space>s"), shown bottom-right.
func Statusline(m mode.Mode, name, cwd, pending string, idx, total, width int, pal *theme.Palette) string {
	modeStr := fmt.Sprintf(" %s ", m.String())

	var modeRendered string
	var barStyle lipgloss.Style

	if pal != nil {
		barStyle = lipgloss.NewStyle().
			Background(lipgloss.Color(pal.Selection)).
			Foreground(lipgloss.Color(pal.Fg))

		badge := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(pal.Bg))
		switch m {
		case mode.Normal:
			modeRendered = badge.Background(lipgloss.Color(pal.ANSI16[2])).Render(modeStr)
		case mode.Insert:
			modeRendered = badge.Background(lipgloss.Color(pal.ANSI16[4])).Render(modeStr)
		case mode.Command:
			modeRendered = badge.Background(lipgloss.Color(pal.ANSI16[3])).Render(modeStr)
		default:
			modeRendered = badge.Background(lipgloss.Color(pal.Accent)).Render(modeStr)
		}
	} else {
		barStyle = lipgloss.NewStyle().Reverse(true).Bold(true)
		base := lipgloss.NewStyle().Reverse(true).Bold(true)
		switch m {
		case mode.Normal:
			modeRendered = base.Foreground(lipgloss.Color("2")).Render(modeStr)
		case mode.Insert:
			modeRendered = base.Foreground(lipgloss.Color("4")).Render(modeStr)
		case mode.Command:
			modeRendered = base.Foreground(lipgloss.Color("3")).Render(modeStr)
		default:
			modeRendered = base.Render(modeStr)
		}
	}

	var middle string
	if total == 0 {
		middle = " no buffers — :new to start"
	} else {
		middle = fmt.Sprintf(" %s  %s", name, cwd)
	}

	var right string
	if pending != "" {
		right = fmt.Sprintf(" %s ", pending)
	}
	if total > 0 {
		right += fmt.Sprintf("[%d/%d] ", idx, total)
	} else {
		right += " "
	}

	modeVis := len([]rune(modeStr))
	rightVis := len([]rune(right))
	midMax := width - modeVis - rightVis
	if midMax < 0 {
		midMax = 0
	}
	midRunes := []rune(middle)
	if len(midRunes) > midMax {
		midRunes = midRunes[:midMax]
	}
	padded := string(midRunes) + strings.Repeat(" ", midMax-len(midRunes))

	return modeRendered + barStyle.Render(padded+right)
}
