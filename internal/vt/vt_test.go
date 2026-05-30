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

// TestTruecolorPassthrough is the core R1 validation: a 24-bit SGR color must
// survive into the grid as an exact RGB triple (not quantized).
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
