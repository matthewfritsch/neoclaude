package vt

import "strings"

// ExtractLines converts a Grid snapshot into plain UTF-8 text lines.
// Each line has trailing whitespace trimmed. Empty trailing lines at the
// bottom of the grid are preserved (the caller decides how many to keep).
func ExtractLines(g Grid) []string {
	lines := make([]string, g.Rows)
	for y := 0; y < g.Rows; y++ {
		var sb strings.Builder
		for x := 0; x < g.Cols; x++ {
			sb.WriteRune(g.Cells[y][x].Rune)
		}
		lines[y] = strings.TrimRight(sb.String(), " \t")
	}
	return lines
}
