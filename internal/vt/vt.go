// Package vt wraps charmbracelet/x/vt (SafeEmulator) and exposes a
// snapshot-friendly cell grid plus cursor state for the renderer.
//
// Library migration (was hinshun/vt10x, now charmbracelet/x/vt):
//
// hinshun/vt10x (abandoned 2022) misparsed modern keyboard-protocol sequences
// that claude emits (Kitty keyboard push/pop \e[<u, \e[>1u, xterm
// modifyOtherKeys \e[>4;2m).  Those sequences caused vt10x to reset the cursor
// to row 0, so claude's input echoes at the top of the screen instead of the
// input line.  charmbracelet/x/vt handles these sequences correctly and also
// provides built-in scrollback — no separate ring buffer needed from the VT
// layer (we keep buffer.Ring for P2 backward compatibility; the emulator's own
// scrollback is also available via ScrollbackCellAt).
//
// Response forwarding: the emulator generates responses to DA queries, cursor-
// position reports, keyboard-protocol acknowledgements, etc.  These bytes are
// written into an internal io.Pipe (PipeWriter side).  Reading from that pipe
// blocks when empty.  We drain it in a background goroutine that accumulates
// bytes into an atomic buffer; DrainResponses() swaps out that buffer so the
// Bubble Tea Update loop can forward the bytes to the child PTY without
// blocking.
package vt

import (
	"image/color"
	"sync"
	"unicode/utf8"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	xvt "github.com/charmbracelet/x/vt"
)

// Public types — kept identical to the vt10x era so render/blit.go, extract.go
// and all search code compile unchanged.

// ColorKind distinguishes how a Color value should be interpreted by the renderer.
type ColorKind uint8

const (
	// ColorDefault means "use the terminal default" (fg/bg/cursor).
	ColorDefault ColorKind = iota
	// ColorPalette is an index into the 256-color palette (0..255).
	// Indices 0..15 are the ANSI/bright-ANSI colors.
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
// Rune holds the first rune of the grapheme cluster.  Wide characters and
// combining marks are a TODO — claude output is overwhelmingly single-rune.
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

// VT wraps a charmbracelet/x/vt SafeEmulator.
// All exported methods are safe for concurrent use: SafeEmulator carries its
// own RWMutex, and we add a separate mu only for the fields we own (cols, rows,
// cursorVisible, responseBuf).
type VT struct {
	mu            sync.Mutex
	emu           *xvt.SafeEmulator
	cols, rows    int
	cursorVisible bool
	responseBuf   []byte // drained from emu.Read() by the background goroutine
	responseMu    sync.Mutex
}

// New creates a VT with the given dimensions.  cols/rows are clamped to >= 1.
// A background goroutine is started immediately to drain emulator responses
// (DA replies, CPR, kbd-protocol acks) into an internal buffer so that
// DrainResponses() can forward them to the child PTY without blocking.
func New(cols, rows int) *VT {
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	emu := xvt.NewSafeEmulator(cols, rows)

	emu.SetScrollbackSize(10_000)

	v := &VT{
		emu:           emu,
		cols:          cols,
		rows:          rows,
		cursorVisible: true,
	}

	// Track cursor visibility via callbacks.  The emulator fires this when
	// DECTCEM (?25h/l) changes.
	emu.SetCallbacks(xvt.Callbacks{
		CursorVisibility: func(visible bool) {
			v.mu.Lock()
			v.cursorVisible = visible
			v.mu.Unlock()
		},
	})

	// Drain goroutine: reads responses the emulator wants to send back to the
	// child (e.g. \x1b[?1;2c for DA1, \x1b[0n for DSR, kbd-protocol acks).
	// Read() blocks on an io.Pipe so this must run in its own goroutine.
	// We accumulate into responseBuf; DrainResponses() atomically swaps it.
	go v.drainLoop()

	return v
}

func (v *VT) drainLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := v.emu.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			v.responseMu.Lock()
			v.responseBuf = append(v.responseBuf, chunk...)
			v.responseMu.Unlock()
		}
		if err != nil {
			// io.EOF means the emulator was closed; exit cleanly.
			return
		}
	}
}

// DrainResponses returns (and clears) any emulator-generated response bytes
// that should be forwarded to the child PTY.  Returns nil if none are pending.
// Called from the Bubble Tea Update loop after each VT.Write().
func (v *VT) DrainResponses() []byte {
	v.responseMu.Lock()
	defer v.responseMu.Unlock()
	if len(v.responseBuf) == 0 {
		return nil
	}
	out := v.responseBuf
	v.responseBuf = nil
	return out
}

// Write feeds raw child output bytes into the emulator.
// OSC sequences are stripped before writing because the upstream parser
// treats 0x9C as C1 ST even inside UTF-8 multibyte sequences, causing
// OSC titles containing non-ASCII (e.g. ✳ = E2 9C B3) to leak their
// payload to the screen.
func (v *VT) Write(p []byte) {
	_, _ = v.emu.Write(stripOSC(p))
}

// stripOSC removes OSC (Operating System Command) sequences from the byte
// stream. An OSC starts with ESC ] and is terminated by BEL (0x07) or
// ST (ESC \). We strip them entirely rather than fixing the C1/UTF-8
// collision because neoclaude manages its own chrome and doesn't need
// window titles or other OSC features from the child.
func stripOSC(p []byte) []byte {
	// Fast path: no ESC ] in the data.
	hasOSC := false
	for i := 0; i+1 < len(p); i++ {
		if p[i] == 0x1b && p[i+1] == 0x5d {
			hasOSC = true
			break
		}
	}
	if !hasOSC {
		return p
	}

	out := make([]byte, 0, len(p))
	i := 0
	for i < len(p) {
		if i+1 < len(p) && p[i] == 0x1b && p[i+1] == 0x5d {
			i += 2
			for i < len(p) {
				if p[i] == 0x07 {
					i++
					break
				}
				if i+1 < len(p) && p[i] == 0x1b && p[i+1] == 0x5c {
					i += 2
					break
				}
				i++
			}
			continue
		}
		out = append(out, p[i])
		i++
	}
	return out
}

// Close releases the emulator, which closes its response pipe and lets the
// drain goroutine exit (Read returns io.EOF). Safe to call once per VT.
func (v *VT) Close() error {
	return v.emu.Close()
}

// Resize changes the emulated screen dimensions.  cols/rows are clamped to >= 1.
func (v *VT) Resize(cols, rows int) {
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	v.mu.Lock()
	v.cols = cols
	v.rows = rows
	v.mu.Unlock()
	v.emu.Resize(cols, rows)
}

// Size returns the current dimensions.
func (v *VT) Size() (cols, rows int) {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.cols, v.rows
}

// ScrollbackLen returns the number of lines in the emulator's scrollback buffer.
func (v *VT) ScrollbackLen() int {
	return v.emu.ScrollbackLen()
}

// SetScrollbackSize sets the maximum number of scrollback lines.
func (v *VT) SetScrollbackSize(n int) {
	v.emu.SetScrollbackSize(n)
}

// Snapshot copies the current screen into a Grid for rendering.
func (v *VT) Snapshot() Grid {
	return v.SnapshotAt(0)
}

// SnapshotAt returns a grid scrolled back by offset lines from the bottom.
// offset=0 is the live terminal view. offset=N shows N lines earlier.
func (v *VT) SnapshotAt(offset int) Grid {
	v.mu.Lock()
	cols, rows := v.cols, v.rows
	v.mu.Unlock()

	sbLen := v.emu.ScrollbackLen()

	if offset <= 0 {
		g := Grid{Cells: make([][]Cell, rows), Cols: cols, Rows: rows}
		for y := 0; y < rows; y++ {
			row := make([]Cell, cols)
			for x := 0; x < cols; x++ {
				row[x] = cellAt(v.emu, x, y)
			}
			g.Cells[y] = row
		}
		return g
	}

	if offset > sbLen {
		offset = sbLen
	}

	g := Grid{Cells: make([][]Cell, rows), Cols: cols, Rows: rows}
	for y := 0; y < rows; y++ {
		row := make([]Cell, cols)
		// virtualLine: 0 = top of scrollback, sbLen = screen row 0
		virtualLine := sbLen - offset + y
		for x := 0; x < cols; x++ {
			if virtualLine < sbLen {
				row[x] = scrollbackCellAt(v.emu, x, virtualLine)
			} else {
				row[x] = cellAt(v.emu, x, virtualLine-sbLen)
			}
		}
		g.Cells[y] = row
	}
	return g
}

// scrollbackCellAt extracts one Cell from the scrollback buffer at (x,y).
func scrollbackCellAt(emu *xvt.SafeEmulator, x, y int) Cell {
	c := emu.ScrollbackCellAt(x, y)
	if c == nil {
		return Cell{Rune: ' '}
	}
	r := firstRune(c.Content)
	if r == 0 {
		r = ' '
	}
	return Cell{
		Rune:  r,
		FG:    colorFrom(c.Style.Fg),
		BG:    colorFrom(c.Style.Bg),
		Attrs: attrsFrom(c.Style),
	}
}

// Cursor returns the cursor position and whether it is visible.
func (v *VT) Cursor() (x, y int, visible bool) {
	pos := v.emu.CursorPosition()
	v.mu.Lock()
	vis := v.cursorVisible
	v.mu.Unlock()
	return pos.X, pos.Y, vis
}

// cellAt extracts one Cell from the emulator at (x,y).
func cellAt(emu *xvt.SafeEmulator, x, y int) Cell {
	c := emu.CellAt(x, y)
	if c == nil {
		return Cell{Rune: ' '}
	}
	r := firstRune(c.Content)
	if r == 0 {
		r = ' '
	}
	return Cell{
		Rune:  r,
		FG:    colorFrom(c.Style.Fg),
		BG:    colorFrom(c.Style.Bg),
		Attrs: attrsFrom(c.Style),
	}
}

// firstRune returns the first rune of s, or 0 for an empty string.
// TODO: handle multi-rune grapheme clusters (wide chars, combining marks).
func firstRune(s string) rune {
	if s == "" {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return 0
	}
	return r
}

// colorFrom converts a charmbracelet/x/ansi color.Color to our Color.
// We type-switch on the concrete ansi types to preserve palette indices
// (important for P4 theming — ANSI16 remap only applies to palette colors,
// not truecolor).  Any unknown concrete type falls back to RGB via RGBA().
func colorFrom(c color.Color) Color {
	if c == nil {
		return Color{Kind: ColorDefault}
	}
	switch v := c.(type) {
	case ansi.BasicColor:
		// ANSI 0..15
		return Color{Kind: ColorPalette, Palette: uint8(v)}
	case ansi.IndexedColor:
		// 256-palette 0..255
		return Color{Kind: ColorPalette, Palette: uint8(v)}
	case ansi.TrueColor:
		// Packed 24-bit: 0xRRGGBB (deprecated but still emitted by some paths)
		u := uint32(v)
		return Color{
			Kind: ColorRGB,
			R:    uint8((u >> 16) & 0xff),
			G:    uint8((u >> 8) & 0xff),
			B:    uint8(u & 0xff),
		}
	case ansi.RGBColor:
		return Color{Kind: ColorRGB, R: v.R, G: v.G, B: v.B}
	default:
		// Fallback: use RGBA() which returns 16-bit components; >>8 to 8-bit.
		r16, g16, b16, _ := c.RGBA()
		return Color{
			Kind: ColorRGB,
			R:    uint8(r16 >> 8),
			G:    uint8(g16 >> 8),
			B:    uint8(b16 >> 8),
		}
	}
}

// attrsFrom maps ultraviolet Style attrs to our Attr bitset.
func attrsFrom(s uv.Style) Attr {
	var a Attr
	if s.Attrs&uv.AttrReverse != 0 {
		a |= AttrReverse
	}
	if s.Underline != uv.UnderlineNone {
		a |= AttrUnderline
	}
	if s.Attrs&uv.AttrBold != 0 {
		a |= AttrBold
	}
	if s.Attrs&uv.AttrItalic != 0 {
		a |= AttrItalic
	}
	if s.Attrs&uv.AttrBlink != 0 {
		a |= AttrBlink
	}
	return a
}
