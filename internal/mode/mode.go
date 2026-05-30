// Package mode implements the modal editing FSM for neoclaude.
//
// Modes: Normal, Insert, Command. (Visual/Search/Picker are P2.)
//
// Double-Esc contract (Insert→Normal):
//   - First Esc in Insert: arm escPending, record escAt, FORWARD the Esc byte
//     to the child (so a single Esc still reaches claude).
//   - Second Esc within escDelay (300ms): transition to Normal, swallow the
//     second Esc (child does NOT receive it).
//   - Any non-Esc key while pending: clear pending, forward the key normally.
//   - Pending naturally expires; the next Update call checks the timestamp.
package mode

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Mode is the current editing mode.
type Mode int

const (
	Normal  Mode = iota
	Insert       // all keys forwarded to pty except double-Esc
	Command      // collecting a : command line
)

func (m Mode) String() string {
	switch m {
	case Normal:
		return "NORMAL"
	case Insert:
		return "INSERT"
	case Command:
		return "COMMAND"
	default:
		return "NORMAL"
	}
}

// Action is what the caller should do after HandleKey returns.
type Action int

const (
	// ActionNone: no special behaviour; mode may have changed.
	ActionNone Action = iota
	// ActionForward: encode the key and write it to the active session.
	ActionForward
	// ActionQuit: user requested a graceful quit (Ctrl-C in Normal).
	ActionQuit
	// ActionEnterCommand: transition to Command mode (caller opens cmdline).
	ActionEnterCommand
	// ActionExecCommand: the cmdline has been confirmed (Enter in Command).
	ActionExecCommand
	// ActionCancelCommand: Esc in Command mode cancels the cmdline.
	ActionCancelCommand
)

// EscDelay is the window for the double-Esc Insert→Normal chord.
const EscDelay = 300 * time.Millisecond

// FSM holds the mode state machine.
type FSM struct {
	mode       Mode
	escPending bool
	escAt      time.Time
}

// New returns an FSM starting in Insert mode so the first buffer is immediately
// usable without pressing `i`.
func New() *FSM {
	return &FSM{mode: Insert}
}

// Mode returns the current mode.
func (f *FSM) Mode() Mode { return f.mode }

// HandleKey processes one key event and returns the action the caller should
// take plus the (possibly updated) mode. now is the current time; pass
// time.Now() from the caller so tests can inject a fixed clock.
func (f *FSM) HandleKey(k tea.KeyMsg, now time.Time) (Action, Mode) {
	switch f.mode {
	case Insert:
		return f.handleInsert(k, now)
	case Normal:
		return f.handleNormal(k, now)
	case Command:
		return f.handleCommand(k, now)
	}
	return ActionNone, f.mode
}

// SetMode forces the FSM into a specific mode. Used by the command dispatcher
// after executing a command so the FSM and the app stay in sync.
func (f *FSM) SetMode(m Mode) {
	f.mode = m
	f.escPending = false
}

// --- Insert mode ---

func (f *FSM) handleInsert(k tea.KeyMsg, now time.Time) (Action, Mode) {
	if k.Type == tea.KeyEsc {
		if f.escPending && now.Sub(f.escAt) <= EscDelay {
			// Second Esc within window → Normal. Swallow: do NOT forward.
			f.escPending = false
			f.mode = Normal
			return ActionNone, Normal
		}
		// First Esc (or expired pending): arm chord, forward to child.
		f.escPending = true
		f.escAt = now
		return ActionForward, Insert
	}

	// Any non-Esc clears a stale pending and forwards normally.
	f.escPending = false

	if k.Type == tea.KeyCtrlC {
		// Ctrl-C in Insert passes through to the child (interrupt signal).
		return ActionForward, Insert
	}

	return ActionForward, Insert
}

// --- Normal mode ---

func (f *FSM) handleNormal(k tea.KeyMsg, _ time.Time) (Action, Mode) {
	f.escPending = false

	if k.Type == tea.KeyCtrlC {
		return ActionQuit, Normal
	}

	if k.Type == tea.KeyRunes {
		switch string(k.Runes) {
		case "i", "a":
			f.mode = Insert
			return ActionNone, Insert
		case ":":
			f.mode = Command
			return ActionEnterCommand, Command
		}
	}

	// Enter in Normal also enters Insert.
	if k.Type == tea.KeyEnter {
		f.mode = Insert
		return ActionNone, Insert
	}

	return ActionNone, Normal
}

// --- Command mode ---

func (f *FSM) handleCommand(k tea.KeyMsg, _ time.Time) (Action, Mode) {
	if k.Type == tea.KeyEsc {
		f.mode = Normal
		return ActionCancelCommand, Normal
	}
	if k.Type == tea.KeyEnter {
		f.mode = Normal
		return ActionExecCommand, Normal
	}
	// All other keys are consumed by the cmdline widget; let the caller handle.
	return ActionNone, Command
}
