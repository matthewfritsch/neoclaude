package app

import tea "github.com/charmbracelet/bubbletea"

// EncodeKey translates a Bubble Tea key event into the byte sequence a real
// terminal delivers to the child PTY. Covers printable runes, common control
// keys, and cursor/navigation escape sequences.
func EncodeKey(k tea.KeyMsg) []byte {
	switch k.Type {
	case tea.KeyRunes:
		if k.Alt {
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
	// Control keys: KeyType value equals the ASCII control code (e.g. KeyCtrlA == 0x01).
	if t := int(k.Type); t > 0 && t < 0x20 {
		return []byte{byte(t)}
	}
	return nil
}
