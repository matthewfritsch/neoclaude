// Package mode implements the modal editing FSM for neoclaude.
//
// Modes: Normal, Insert, Command, Search, Visual, Picker.
//
// Esc contract: Esc is NEVER forwarded to the child PTY.
//   - Insert: single Esc → Normal immediately.
//   - Command/Search/Visual/Picker: Esc → Normal (cancel/close).
//   - Normal: Esc clears overlays (search, etc.) or is a no-op.
//   Claude uses Ctrl-C for cancel; escape sequences (arrows, etc.) arrive as
//   distinct key types and are unaffected.
//
// Leader contract (Normal only):
//   - Pressing the leader key arms leaderPending.
//   - Next key(s) form a chord: leader+leader → picker, leader+s+g → grep,
//     leader+s+n → named-session picker.
//   - Any unrecognised follow-up clears pending with no action.
//   - Leader keys fire ONLY in Normal; in Insert all keys (incl space) go to pty.
package mode

import (
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
	ActionNone              Action = iota
	ActionForward                  // encode key → write to active session pty
	ActionQuit                     // Ctrl-C in Normal → graceful quit
	ActionEnterCommand             // : in Normal → open cmdline
	ActionExecCommand              // Enter in Command → execute cmdline
	ActionCancelCommand            // Esc in Command → cancel cmdline
	ActionOpenPicker               // <leader><leader> → open buffer picker
	ActionOpenGrep                 // <leader>sg → open grep pane
	ActionOpenNamedSessions        // <leader>sn → open named-session picker
	ActionOpenSearch               // / in Normal → open in-buffer search bar
	ActionEnterVisual              // v in Normal → enter Visual mode
	ActionExitOverlay              // Esc from Search/Visual/Picker → back to Normal
	ActionSearchNext               // n → next search match
	ActionSearchPrev               // N → prev search match
	ActionVisualYank               // y in Visual → yank selection to clipboard
)

// FSM holds the mode state machine.
type FSM struct {
	mode   Mode
	leader rune // configured leader rune (default ' ')
	// leader chord state (Normal only)
	leaderPending bool
	leaderSeq     string // keys pressed after leader so far ("s", "sg", "sn", …)
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

// PendingKeys returns the current in-progress chord for display (e.g. "<Space>s").
// Returns "" when no chord is active.
func (f *FSM) PendingKeys() string {
	if !f.leaderPending {
		return ""
	}
	leader := string(f.leader)
	if f.leader == ' ' {
		leader = "<Space>"
	}
	return leader + f.leaderSeq
}

// SetMode forces the FSM into a specific mode.
func (f *FSM) SetMode(m Mode) {
	f.mode = m
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
// take plus the (possibly updated) mode.
func (f *FSM) HandleKey(k tea.KeyMsg) (Action, Mode) {
	switch f.mode {
	case Insert:
		return f.handleInsert(k)
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

func (f *FSM) handleInsert(k tea.KeyMsg) (Action, Mode) {
	if k.Type == tea.KeyEsc {
		f.mode = Normal
		return ActionNone, Normal
	}
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

	switch f.leaderSeq {
	case "sg":
		f.leaderPending = false
		f.leaderSeq = ""
		return ActionOpenGrep, Normal
	case "sn":
		f.leaderPending = false
		f.leaderSeq = ""
		return ActionOpenNamedSessions, Normal
	case "s":
		// Partial — keep pending; next key completes "sg" or "sn".
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
