// Package mode implements the modal editing FSM for neoclaude.
//
// Modes: Normal, Insert, Command, Search, Visual, Picker.
//
// Double-Esc contract (Insert→Normal):
//   - First Esc in Insert: arm escPending, record escAt, FORWARD the Esc byte
//     to the child (so a single Esc still reaches claude).
//   - Second Esc within EscDelay (300ms): transition to Normal, swallow.
//   - Any non-Esc key while pending: clear pending, forward normally.
//
// Leader contract (Normal only):
//   - Pressing the leader key arms leaderPending.
//   - Next key(s) form a chord: leader+leader → picker, leader+s+g → grep.
//   - Any unrecognised follow-up clears pending with no action.
//   - Leader keys fire ONLY in Normal; in Insert all keys (incl space) go to pty.
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
	Search       // in-buffer / search bar
	Visual       // visual (linewise) selection
	Picker       // fuzzy buffer picker overlay
)

func (m Mode) String() string {
	switch m {
	case Normal:
		return "NORMAL"
	case Insert:
		return "INSERT"
	case Command:
		return "COMMAND"
	case Search:
		return "SEARCH"
	case Visual:
		return "VISUAL"
	case Picker:
		return "PICKER"
	default:
		return "NORMAL"
	}
}

// Action is what the caller should do after HandleKey returns.
type Action int

const (
	ActionNone          Action = iota
	ActionForward              // encode key → write to active session pty
	ActionQuit                 // Ctrl-C in Normal → graceful quit
	ActionEnterCommand         // : in Normal → open cmdline
	ActionExecCommand          // Enter in Command → execute cmdline
	ActionCancelCommand        // Esc in Command → cancel cmdline
	ActionOpenPicker           // <leader><leader> → open buffer picker
	ActionOpenGrep             // <leader>sg → open grep pane
	ActionOpenSearch           // / in Normal → open in-buffer search bar
	ActionEnterVisual          // v in Normal → enter Visual mode
	ActionExitOverlay          // Esc from Search/Visual/Picker → back to Normal
	ActionSearchNext           // n → next search match
	ActionSearchPrev           // N → prev search match
	ActionVisualYank           // y in Visual → yank selection to clipboard
)

// EscDelay is the window for the double-Esc Insert→Normal chord.
const EscDelay = 300 * time.Millisecond

// FSM holds the mode state machine.
type FSM struct {
	mode       Mode
	leader     rune // configured leader rune (default ' ')
	escPending bool
	escAt      time.Time
	// leader chord state (Normal only)
	leaderPending bool
	leaderSeq     string // keys pressed after leader so far ("s", "sg", …)
}

// New returns an FSM starting in Insert mode with the default leader (space).
func New() *FSM { return NewWithLeader(' ') }

// NewWithLeader returns an FSM with a specific leader rune.
func NewWithLeader(leader rune) *FSM {
	return &FSM{mode: Insert, leader: leader}
}

// Mode returns the current mode.
func (f *FSM) Mode() Mode { return f.mode }

// SetLeader updates the leader rune (called after config loads).
func (f *FSM) SetLeader(r rune) { f.leader = r }

// SetMode forces the FSM into a specific mode.
func (f *FSM) SetMode(m Mode) {
	f.mode = m
	f.escPending = false
	f.leaderPending = false
	f.leaderSeq = ""
}

// effectiveRune returns the rune a key represents for Normal-mode matching,
// treating tea.KeySpace as ' ' (Bubble Tea delivers space as its own key type,
// not as KeyRunes, so leader=space would never match without this). ok is false
// for non-character keys (arrows, Enter, Esc, …).
func effectiveRune(k tea.KeyMsg) (rune, bool) {
	switch k.Type {
	case tea.KeyRunes:
		if len(k.Runes) == 1 {
			return k.Runes[0], true
		}
	case tea.KeySpace:
		return ' ', true
	}
	return 0, false
}

// HandleKey processes one key event and returns the action the caller should
// take plus the (possibly updated) mode. now should be time.Now() from caller.
func (f *FSM) HandleKey(k tea.KeyMsg, now time.Time) (Action, Mode) {
	switch f.mode {
	case Insert:
		return f.handleInsert(k, now)
	case Normal:
		return f.handleNormal(k)
	case Command:
		return f.handleCommand(k)
	case Search:
		return f.handleSearch(k)
	case Visual:
		return f.handleVisual(k)
	case Picker:
		return f.handlePicker(k)
	}
	return ActionNone, f.mode
}

// --- Insert ---

func (f *FSM) handleInsert(k tea.KeyMsg, now time.Time) (Action, Mode) {
	if k.Type == tea.KeyEsc {
		if f.escPending && now.Sub(f.escAt) <= EscDelay {
			f.escPending = false
			f.mode = Normal
			return ActionNone, Normal
		}
		f.escPending = true
		f.escAt = now
		return ActionForward, Insert
	}
	f.escPending = false
	return ActionForward, Insert
}

// --- Normal ---

func (f *FSM) handleNormal(k tea.KeyMsg) (Action, Mode) {
	// Leader chord in progress.
	if f.leaderPending {
		return f.handleLeaderSeq(k)
	}

	if k.Type == tea.KeyCtrlC {
		return ActionQuit, Normal
	}

	if r, ok := effectiveRune(k); ok {
		// Arm leader chord.
		if r == f.leader {
			f.leaderPending = true
			f.leaderSeq = ""
			return ActionNone, Normal
		}

		switch r {
		case 'i', 'a':
			f.mode = Insert
			return ActionNone, Insert
		case ':':
			f.mode = Command
			return ActionEnterCommand, Command
		case '/':
			f.mode = Search
			return ActionOpenSearch, Search
		case 'v':
			f.mode = Visual
			return ActionEnterVisual, Visual
		}
	}

	if k.Type == tea.KeyEnter {
		f.mode = Insert
		return ActionNone, Insert
	}

	return ActionNone, Normal
}

// handleLeaderSeq processes keys that arrive after the leader is armed.
func (f *FSM) handleLeaderSeq(k tea.KeyMsg) (Action, Mode) {
	r, ok := effectiveRune(k)
	if !ok {
		// Non-character key after leader → cancel chord.
		f.leaderPending = false
		f.leaderSeq = ""
		return ActionNone, Normal
	}

	// <leader><leader> → buffer picker.
	if r == f.leader && f.leaderSeq == "" {
		f.leaderPending = false
		f.leaderSeq = ""
		f.mode = Picker
		return ActionOpenPicker, Picker
	}

	f.leaderSeq += string(r)

	// <leader>sg → grep.
	if f.leaderSeq == "sg" {
		f.leaderPending = false
		f.leaderSeq = ""
		return ActionOpenGrep, Normal
	}

	// Partial: "s" could still become "sg" — keep pending.
	if f.leaderSeq == "s" {
		return ActionNone, Normal
	}

	// Unrecognised sequence → cancel silently.
	f.leaderPending = false
	f.leaderSeq = ""
	return ActionNone, Normal
}

// --- Command ---

func (f *FSM) handleCommand(k tea.KeyMsg) (Action, Mode) {
	if k.Type == tea.KeyEsc {
		f.mode = Normal
		return ActionCancelCommand, Normal
	}
	if k.Type == tea.KeyEnter {
		f.mode = Normal
		return ActionExecCommand, Normal
	}
	return ActionNone, Command
}

// --- Search ---

func (f *FSM) handleSearch(k tea.KeyMsg) (Action, Mode) {
	if k.Type == tea.KeyEsc {
		f.mode = Normal
		return ActionExitOverlay, Normal
	}
	if k.Type == tea.KeyRunes {
		switch string(k.Runes) {
		case "n":
			return ActionSearchNext, Search
		case "N":
			return ActionSearchPrev, Search
		}
	}
	// All other keys feed the search input widget (caller handles).
	return ActionNone, Search
}

// --- Visual ---

func (f *FSM) handleVisual(k tea.KeyMsg) (Action, Mode) {
	if k.Type == tea.KeyEsc {
		f.mode = Normal
		return ActionExitOverlay, Normal
	}
	if k.Type == tea.KeyRunes && string(k.Runes) == "y" {
		f.mode = Normal
		return ActionVisualYank, Normal
	}
	// Arrow keys / j/k for selection movement handled by caller.
	return ActionNone, Visual
}

// --- Picker ---

func (f *FSM) handlePicker(k tea.KeyMsg) (Action, Mode) {
	if k.Type == tea.KeyEsc {
		f.mode = Normal
		return ActionExitOverlay, Normal
	}
	if k.Type == tea.KeyEnter {
		f.mode = Normal
		return ActionExecCommand, Normal // reuse: caller checks mode was Picker
	}
	return ActionNone, Picker
}
