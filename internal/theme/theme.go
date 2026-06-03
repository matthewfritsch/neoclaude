// Package theme provides named colour palettes that drive neoclaude's chrome
// styling and ANSI-16 remap of child (claude) output.
//
// Architecture:
//
//	Palette      — chrome hex strings + ANSI16[16] terminal-color hex strings.
//	builtin.go   — hand-coded well-known palettes (onedark, tokyonight-night,
//	               catppuccin-mocha, gruvbox-dark).
//	fetch.go     — download a Neovim theme tarball from GitHub and extract
//	               terminal_color_* / chrome colour values via regex.
//	cache.go     — persist fetched palettes as JSON under
//	               $XDG_DATA_HOME/neoclaude/themes/<name>.json.
//
// ANSI16 remap ceiling:
//
//	claude's truecolor (38;2;r;g;b) output is NEVER remapped — those colours
//	are already theme-accurate. Only palette indices 0..15 (ANSI/bright-ANSI)
//	are remapped. Indices 16..255 (extended 256-colour) are left alone.
package theme

import "github.com/charmbracelet/lipgloss"

// Palette is a set of colours that drives both the neoclaude chrome and the
// ANSI16 remap of child terminal output.
//
// All colour fields are "#rrggbb" hex strings or empty string ("") to mean
// "use terminal default / inherit".
type Palette struct {
	Name string

	// Chrome colours — used by statusline, pickers, search bar, etc.
	Bg        string // background of status/overlay areas
	Fg        string // primary text
	Accent    string // active/selected highlight (e.g. mode badge)
	Muted     string // secondary text (cwd, closed-session label)
	Selection string // visual-selection background
	Border    string // overlay borders
	Match     string // search-match / fuzzy-match highlight

	// ANSI16 maps child terminal colour indices 0..15 to theme-specific hex.
	// Leave an entry as "" to pass the raw ANSI index through unchanged.
	ANSI16 [16]string
}

// Lip returns the palette colour as a lipgloss.Color, or the empty string
// (lipgloss treats "" as "use terminal default").
func (p *Palette) Lip(hex string) lipgloss.Color {
	return lipgloss.Color(hex)
}

// ANSI16Color returns the remapped hex for terminal colour index i (0..15).
// Returns "" if the palette has no entry for that index (pass-through).
func (p *Palette) ANSI16Color(i int) string {
	if i < 0 || i > 15 {
		return ""
	}
	return p.ANSI16[i]
}

// ANSI16Ptr returns a pointer to a copy of the ANSI16 array, suitable for
// storing in render.Options. Returns nil if every entry is "".
func (p *Palette) ANSI16Ptr() *[16]string {
	all := p.ANSI16
	for _, v := range all {
		if v != "" {
			out := all
			return &out
		}
	}
	return nil
}
