// Package vt wraps the hinshun/vt10x virtual terminal emulator and exposes a
// snapshot-friendly cell grid plus cursor state for the renderer.
//
// Library choice (R1 gate): we use github.com/hinshun/vt10x. It correctly
// handles alt-screen (?1049h/l), scroll regions, 256-color and 24-bit truecolor
// SGR sequences, and exposes an accessible cell grid via Cell(x,y). See
// internal/vt/vt_test.go for the validation that gates this choice.
package vt

import (
	"sync"

	vt10x "github.com/hinshun/vt10x"
)

// ColorKind distinguishes how a Color value should be interpreted by the
// renderer. The underlying vt10x library packs ANSI indices, 256-palette
// indices, truecolor RGB, and "default" sentinels all into a single uint32, so
// we decode that ambiguity once here.
type ColorKind uint8

const (
	// ColorDefault means "use the terminal default" (fg/bg/cursor).
	ColorDefault ColorKind = iota
	// ColorPalette is an index into the 256-color palette (0..255); indices
	// 0..15 are the ANSI/bright-ANSI colors.
	ColorPalette
	// ColorRGB is a 24-bit truecolor value.
	ColorRGB
)

// Color is a decoded color: a kind plus either a palette index or RGB triple.
type Color struct {
	Kind    ColorKind
	Palette uint8 // valid when Kind == ColorPalette
	R, G, B uint8 // valid when Kind == ColorRGB
}

// Attr is a bitset of text attributes for a cell.
type Attr uint8

const (
	AttrReverse Attr = 1 << iota
	AttrUnderline
	AttrBold
	AttrItalic
	AttrBlink
)

// Cell is a single screen cell: its rune plus styling.
type Cell struct {
	Rune   rune
	FG, BG Color
	Attrs  Attr
}

// Grid is an immutable snapshot of the terminal screen.
type Grid struct {
	Cells [][]Cell // [row][col]
	Cols  int
	Rows  int
}

// VT wraps a vt10x terminal. All exported methods are safe for concurrent use
// with the emulator's internal locking; callers should still serialize Write
// vs Snapshot at the application level (the Bubble Tea Update loop does this
// naturally since it is single-goroutine).
type VT struct {
	mu   sync.Mutex
	term vt10x.Terminal
	cols int
	rows int
}

// New creates a VT with the given dimensions. cols/rows are clamped to >= 1.
func New(cols, rows int) *VT {
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	return &VT{
		term: vt10x.New(vt10x.WithSize(cols, rows)),
		cols: cols,
		rows: rows,
	}
}

// Write feeds raw child output bytes into the emulator.
func (v *VT) Write(p []byte) {
	v.mu.Lock()
	defer v.mu.Unlock()
	_, _ = v.term.Write(p)
}

// Resize changes the emulated screen dimensions. cols/rows are clamped to >= 1.
func (v *VT) Resize(cols, rows int) {
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	v.term.Resize(cols, rows)
	v.cols = cols
	v.rows = rows
}

// Size returns the current dimensions.
func (v *VT) Size() (cols, rows int) {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.cols, v.rows
}

// Snapshot copies the current screen into a Grid for rendering.
func (v *VT) Snapshot() Grid {
	v.mu.Lock()
	defer v.mu.Unlock()

	cols, rows := v.cols, v.rows
	g := Grid{
		Cells: make([][]Cell, rows),
		Cols:  cols,
		Rows:  rows,
	}
	for y := 0; y < rows; y++ {
		row := make([]Cell, cols)
		for x := 0; x < cols; x++ {
			glyph := v.term.Cell(x, y)
			r := glyph.Char
			if r == 0 {
				r = ' '
			}
			row[x] = Cell{
				Rune:  r,
				FG:    decodeColor(glyph.FG),
				BG:    decodeColor(glyph.BG),
				Attrs: decodeAttr(glyph.Mode),
			}
		}
		g.Cells[y] = row
	}
	return g
}

// Cursor returns the cursor position and whether it is visible.
func (v *VT) Cursor() (x, y int, visible bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	c := v.term.Cursor()
	return c.X, c.Y, v.term.CursorVisible()
}

// vt10x attribute bit layout (from state.go). Kept local because the library
// does not export them.
const (
	vtAttrReverse = 1 << iota
	vtAttrUnderline
	vtAttrBold
	vtAttrGfx
	vtAttrItalic
	vtAttrBlink
	vtAttrWrap
)

func decodeAttr(mode int16) Attr {
	var a Attr
	m := int(mode)
	if m&vtAttrReverse != 0 {
		a |= AttrReverse
	}
	if m&vtAttrUnderline != 0 {
		a |= AttrUnderline
	}
	if m&vtAttrBold != 0 {
		a |= AttrBold
	}
	if m&vtAttrItalic != 0 {
		a |= AttrItalic
	}
	if m&vtAttrBlink != 0 {
		a |= AttrBlink
	}
	return a
}

// decodeColor untangles the vt10x Color packing:
//
//	value >= 1<<24      -> default sentinel (DefaultFG/BG/Cursor)
//	value in [0, 255]   -> palette index (ANSI 0..15 or 256-color 16..255)
//	value in (255, 1<<24) -> 24-bit truecolor RGB (r<<16 | g<<8 | b)
//
// The (255, 1<<24) range is how setAttr stores `38;2;r;g;b`. There is a known,
// benign ambiguity: a truecolor value of pure blue 0..255 (r=0,g=0,b<=255)
// collides with a 256-palette index. claude's truecolor output overwhelmingly
// has a non-zero r or g component, so this does not affect P0 fidelity.
func decodeColor(c vt10x.Color) Color {
	v := uint32(c)
	if v >= 1<<24 {
		return Color{Kind: ColorDefault}
	}
	if v <= 255 {
		return Color{Kind: ColorPalette, Palette: uint8(v)}
	}
	return Color{
		Kind: ColorRGB,
		R:    uint8((v >> 16) & 0xff),
		G:    uint8((v >> 8) & 0xff),
		B:    uint8(v & 0xff),
	}
}
