package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/matthewfritsch/neoclaude/internal/mode"
	"github.com/matthewfritsch/neoclaude/internal/registry"
	"github.com/matthewfritsch/neoclaude/internal/ui"
)

// Model is the root Bubble Tea model. It owns the buffer registry, the mode
// FSM, and the command-line widget. All mutable state lives here; Update
// returns a new copy via pointer swap (the struct is small, all heavy state is
// behind pointers).
type Model struct {
	// Prog is set by main after tea.NewProgram returns so ReadLoop goroutines
	// can call program.Send.
	Prog *tea.Program

	reg     *registry.Registry
	fsm     *mode.FSM
	cmdline *ui.Cmdline

	cols int
	rows int // terminal rows minus status line

	quitting bool
}

// New returns a Model with an empty registry in Insert mode.
func New() *Model {
	return &Model{
		reg:     registry.New(),
		fsm:     mode.New(),
		cmdline: &ui.Cmdline{},
	}
}

// Registry exposes the registry for main-level wiring.
func (m *Model) Registry() *registry.Registry { return m.reg }

// FSM exposes the mode FSM (read-only from outside app).
func (m *Model) FSM() *mode.FSM { return m.fsm }
