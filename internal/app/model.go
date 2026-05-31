package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/matthewfritsch/neoclaude/internal/config"
	"github.com/matthewfritsch/neoclaude/internal/mode"
	"github.com/matthewfritsch/neoclaude/internal/registry"
	"github.com/matthewfritsch/neoclaude/internal/ui"
)

// Model is the root Bubble Tea model. It owns the buffer registry, mode FSM,
// command-line widget, buffer picker, search bar, grep pane, and config.
type Model struct {
	// Prog is set by main after tea.NewProgram returns so ReadLoop goroutines
	// can call program.Send.
	Prog *tea.Program

	cfg     *config.Config
	reg     *registry.Registry
	fsm     *mode.FSM
	cmdline *ui.Cmdline
	picker  *ui.Picker
	search  *ui.SearchBar
	grep    *ui.GrepPane

	cols int
	rows int // terminal rows minus status line

	// visual selection anchors (row indices, inclusive)
	visualStart int
	visualEnd   int

	needInitial bool
	initialPath string

	quitting bool
}

// New returns a Model with an empty registry. Config is loaded from disk;
// defaults are used silently on any error.
func New() *Model {
	cfg, _ := config.Load()
	reg := registry.New()
	return &Model{
		cfg:         cfg,
		reg:         reg,
		fsm:         mode.NewWithLeader(cfg.LeaderRune),
		cmdline:     &ui.Cmdline{},
		picker:      ui.NewPicker(reg),
		search:      &ui.SearchBar{},
		grep:        &ui.GrepPane{},
		needInitial: true,
	}
}

// Registry exposes the registry for main-level wiring.
func (m *Model) Registry() *registry.Registry { return m.reg }

// FSM exposes the mode FSM.
func (m *Model) FSM() *mode.FSM { return m.fsm }

// Config exposes the loaded config.
func (m *Model) Config() *config.Config { return m.cfg }
