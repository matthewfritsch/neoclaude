package vt

import "testing"

// TestSnapshotBasicText feeds plain text and verifies the grid captures it.
func TestSnapshotBasicText(t *testing.T) {
	v := New(10, 3)
	v.Write([]byte("hi"))
	g := v.Snapshot()
	if g.Cols != 10 || g.Rows != 3 {
		t.Fatalf("dims: got %dx%d want 10x3", g.Cols, g.Rows)
	}
	if got := g.Cells[0][0].Rune; got != 'h' {
		t.Errorf("cell(0,0): got %q want 'h'", got)
	}
	if got := g.Cells[0][1].Rune; got != 'i' {
		t.Errorf("cell(1,0): got %q want 'i'", got)
	}
	// Empty cells should be spaces, never NUL.
	if got := g.Cells[0][2].Rune; got != ' ' {
		t.Errorf("cell(2,0): got %q want space", got)
	}
}

// TestTruecolorPassthrough validates that a 24-bit SGR color survives into
// the grid as an exact RGB triple (not quantized).
func TestTruecolorPassthrough(t *testing.T) {
	v := New(5, 1)
	v.Write([]byte("\x1b[38;2;255;128;64mX\x1b[0m"))
	c := v.Snapshot().Cells[0][0]
	if c.Rune != 'X' {
		t.Fatalf("rune: got %q want 'X'", c.Rune)
	}
	if c.FG.Kind != ColorRGB {
		t.Fatalf("fg kind: got %d want ColorRGB", c.FG.Kind)
	}
	if c.FG.R != 255 || c.FG.G != 128 || c.FG.B != 64 {
		t.Errorf("fg rgb: got (%d,%d,%d) want (255,128,64)", c.FG.R, c.FG.G, c.FG.B)
	}
}

// Test256Color validates palette-index decoding.
func Test256Color(t *testing.T) {
	v := New(5, 1)
	v.Write([]byte("\x1b[38;5;196mY"))
	c := v.Snapshot().Cells[0][0]
	if c.FG.Kind != ColorPalette {
		t.Fatalf("fg kind: got %d want ColorPalette", c.FG.Kind)
	}
	if c.FG.Palette != 196 {
		t.Errorf("fg palette: got %d want 196", c.FG.Palette)
	}
}

// TestAnsiAttrs validates bold + reverse decoding.
func TestAnsiAttrs(t *testing.T) {
	v := New(5, 1)
	v.Write([]byte("\x1b[1;7mZ"))
	c := v.Snapshot().Cells[0][0]
	if c.Attrs&AttrBold == 0 {
		t.Error("bold attr not set")
	}
	if c.Attrs&AttrReverse == 0 {
		t.Error("reverse attr not set")
	}
}

// TestAltScreen exercises the alt-screen swap that claude relies on.
func TestAltScreen(t *testing.T) {
	v := New(5, 2)
	v.Write([]byte("MAIN"))
	v.Write([]byte("\x1b[?1049h"))      // enter alt screen
	v.Write([]byte("\x1b[H\x1b[2JALT")) // home, clear, write
	if got := v.Snapshot().Cells[0][0].Rune; got != 'A' {
		t.Errorf("alt screen cell(0,0): got %q want 'A'", got)
	}
	v.Write([]byte("\x1b[?1049l")) // exit alt screen -> main restored
	if got := v.Snapshot().Cells[0][0].Rune; got != 'M' {
		t.Errorf("restored main cell(0,0): got %q want 'M'", got)
	}
}

// TestCursor validates cursor position reporting after advancing.
func TestCursor(t *testing.T) {
	v := New(10, 2)
	v.Write([]byte("abc"))
	x, y, visible := v.Cursor()
	if x != 3 || y != 0 {
		t.Errorf("cursor: got (%d,%d) want (3,0)", x, y)
	}
	if !visible {
		t.Error("cursor should be visible by default")
	}
}

// TestResize confirms dimensions update.
func TestResize(t *testing.T) {
	v := New(10, 5)
	v.Resize(20, 8)
	if c, r := v.Size(); c != 20 || r != 8 {
		t.Errorf("size after resize: got %dx%d want 20x8", c, r)
	}
	if g := v.Snapshot(); g.Cols != 20 || g.Rows != 8 {
		t.Errorf("grid after resize: got %dx%d want 20x8", g.Cols, g.Rows)
	}
}

// TestKbdProtocolNoCursorJump is the regression test for the bug that motivated
// the migration from hinshun/vt10x to charmbracelet/x/vt.
//
// claude emits Kitty keyboard protocol push/pop and xterm modifyOtherKeys
// sequences on startup.  With vt10x these caused the cursor to jump to row 0.
// The chunk captured in the wild was: \e[<u \e[>1u \e[>4;2m
//
// This test verifies that feeding those sequences does NOT move the cursor.
func TestKbdProtocolNoCursorJump(t *testing.T) {
	v := New(80, 24)
	// Place cursor at a known position (row 5, col 10) by writing lines.
	v.Write([]byte("\x1b[6;11H")) // CUP row=6 col=11 (1-based)
	x0, y0, _ := v.Cursor()
	if x0 != 10 || y0 != 5 {
		t.Fatalf("pre-condition: cursor at (%d,%d) want (10,5)", x0, y0)
	}

	// Feed the exact problematic sequences from the captured chunk.
	v.Write([]byte("\x1b[<u"))    // Kitty keyboard: push flags (CSI < u)
	v.Write([]byte("\x1b[>1u"))   // Kitty keyboard: pop flags (CSI > 1 u)
	v.Write([]byte("\x1b[>4;2m")) // xterm modifyOtherKeys level 2

	x1, y1, _ := v.Cursor()
	if x1 != x0 || y1 != y0 {
		t.Errorf("kbd-protocol sequences moved cursor from (%d,%d) to (%d,%d) — regression!",
			x0, y0, x1, y1)
	}
}

// TestCursorVisibilityToggle verifies that DECTCEM hide/show is tracked.
func TestCursorVisibilityToggle(t *testing.T) {
	v := New(10, 5)
	_, _, vis := v.Cursor()
	if !vis {
		t.Error("cursor should start visible")
	}
	v.Write([]byte("\x1b[?25l")) // hide cursor
	_, _, vis = v.Cursor()
	if vis {
		t.Error("cursor should be hidden after ?25l")
	}
	v.Write([]byte("\x1b[?25h")) // show cursor
	_, _, vis = v.Cursor()
	if !vis {
		t.Error("cursor should be visible after ?25h")
	}
}
