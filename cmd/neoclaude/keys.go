package main

import (
	tea "github.com/charmbracelet/bubbletea"
)

// encodeKey translates a Bubble Tea key event into the byte sequence a real
// terminal would deliver to the child PTY. This is the raw passthrough path; it
// covers printable runes, common control keys, and cursor/navigation keys with
// their xterm escape sequences. The full keymap lands with the P1 FSM.
func encodeKey(k tea.KeyMsg) []byte {
	switch k.Type {
	case tea.KeyRunes:
		if k.Alt {
			// Alt prefixes the rune(s) with ESC.
			return append([]byte{0x1b}, []byte(string(k.Runes))...)
		}
		return []byte(string(k.Runes))
	case tea.KeySpace:
		return []byte{' '}
	case tea.KeyEnter:
		return []byte{'\r'}
	case tea.KeyTab:
		return []byte{'\t'}
	case tea.KeyShiftTab:
		return []byte("\x1b[Z")
	case tea.KeyBackspace:
		return []byte{0x7f}
	case tea.KeyEsc:
		return []byte{0x1b}
	case tea.KeyDelete:
		return []byte("\x1b[3~")
	case tea.KeyInsert:
		return []byte("\x1b[2~")
	case tea.KeyHome:
		return []byte("\x1b[H")
	case tea.KeyEnd:
		return []byte("\x1b[F")
	case tea.KeyPgUp:
		return []byte("\x1b[5~")
	case tea.KeyPgDown:
		return []byte("\x1b[6~")
	case tea.KeyUp:
		return []byte("\x1b[A")
	case tea.KeyDown:
		return []byte("\x1b[B")
	case tea.KeyRight:
		return []byte("\x1b[C")
	case tea.KeyLeft:
		return []byte("\x1b[D")
	}

	// Control keys: KeyType for ctrl combinations equals the ASCII control code
	// (e.g. KeyCtrlA == 0x01). Forward that single byte.
	if t := int(k.Type); t > 0 && t < 0x20 {
		return []byte{byte(t)}
	}
	return nil
}
