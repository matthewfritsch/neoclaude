package render

import (
	"strings"
	"testing"

	"github.com/matthewfritsch/neoclaude/internal/vt"
)

// grid builds a single-row grid from a string with default styling.
func gridFromString(s string) vt.Grid {
	runes := []rune(s)
	cells := make([]vt.Cell, len(runes))
	for i, r := range runes {
		cells[i] = vt.Cell{Rune: r}
	}
	return vt.Grid{Cells: [][]vt.Cell{cells}, Cols: len(runes), Rows: 1}
}

// TestBlitPlainText confirms text round-trips through the blit (stripped of ANSI).
func TestBlitPlainText(t *testing.T) {
	out := Blit(gridFromString("hello"), Options{})
	if !strings.Contains(out, "hello") {
		t.Errorf("blit output %q missing 'hello'", out)
	}
}

// TestBlitMultiRow confirms rows are newline-joined with no trailing newline.
func TestBlitMultiRow(t *testing.T) {
	g := vt.Grid{
		Cells: [][]vt.Cell{
			{{Rune: 'a'}, {Rune: 'b'}},
			{{Rune: 'c'}, {Rune: 'd'}},
		},
		Cols: 2,
		Rows: 2,
	}
	out := Blit(g, Options{})
	if n := strings.Count(out, "\n"); n != 1 {
		t.Errorf("newline count: got %d want 1", n)
	}
	if strings.HasSuffix(out, "\n") {
		t.Error("output should not end with newline")
	}
}

// TestBlitTruecolor confirms a 24-bit color emits a truecolor SGR escape
// (38;2;...) rather than being quantized to a palette index.
func TestBlitTruecolor(t *testing.T) {
	g := vt.Grid{
		Cells: [][]vt.Cell{{
			{Rune: 'X', FG: vt.Color{Kind: vt.ColorRGB, R: 255, G: 128, B: 64}},
		}},
		Cols: 1,
		Rows: 1,
	}
	out := Blit(g, Options{})
	if !strings.Contains(out, "38;2;255;128;64") {
		t.Errorf("expected truecolor SGR in %q", out)
	}
}

// TestBlitCursorOverlay confirms the cursor cell gets reverse video while other
// cells do not.
func TestBlitCursorOverlay(t *testing.T) {
	g := gridFromString("ab")
	out := Blit(g, Options{CursorX: 0, CursorY: 0, CursorVisible: true})
	// Reverse video is SGR 7.
	if !strings.Contains(out, "\x1b[7m") {
		t.Errorf("expected reverse-video escape for cursor in %q", out)
	}
}

// TestColorToLipgloss validates color decoding decisions.
func TestColorToLipgloss(t *testing.T) {
	if _, ok := colorToLipgloss(vt.Color{Kind: vt.ColorDefault}); ok {
		t.Error("default color should report ok=false")
	}
	if _, ok := colorToLipgloss(vt.Color{Kind: vt.ColorPalette, Palette: 3}); !ok {
		t.Error("palette color should report ok=true")
	}
	if _, ok := colorToLipgloss(vt.Color{Kind: vt.ColorRGB, R: 1, G: 2, B: 3}); !ok {
		t.Error("rgb color should report ok=true")
	}
}

// TestHex checks the hex encoder.
func TestHex(t *testing.T) {
	if got := hex(255, 128, 64); got != "#ff8040" {
		t.Errorf("hex: got %q want #ff8040", got)
	}
	if got := hex(0, 0, 0); got != "#000000" {
		t.Errorf("hex: got %q want #000000", got)
	}
}
