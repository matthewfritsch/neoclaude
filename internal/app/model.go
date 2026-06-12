package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/matthewfritsch/neoclaude/internal/buffer"
	"github.com/matthewfritsch/neoclaude/internal/config"
	"github.com/matthewfritsch/neoclaude/internal/mode"
	"github.com/matthewfritsch/neoclaude/internal/persist"
	"github.com/matthewfritsch/neoclaude/internal/registry"
	"github.com/matthewfritsch/neoclaude/internal/server"
	"github.com/matthewfritsch/neoclaude/internal/theme"
	"github.com/matthewfritsch/neoclaude/internal/ui"
)

// Model is the root Bubble Tea model. It owns the buffer registry, mode FSM,
// command-line widget, buffer picker, search bar, grep pane, session picker,
// persist store, and config.
type Model struct {
	// Prog is set by main after tea.NewProgram returns so ReadLoop goroutines
	// can call program.Send.
	Prog *tea.Program

	cfg           *config.Config
	reg           *registry.Registry
	fsm           *mode.FSM
	cmdline       *ui.Cmdline
	picker        *ui.Picker
	search        *ui.SearchBar
	grep          *ui.GrepPane
	sessionPicker *ui.SessionPicker
	store         *persist.Store
	palette       *theme.Palette
	srv           *server.Server
	startedAt     time.Time

	cols int
	rows int // terminal rows minus status line

	// visual selection anchors (row indices, inclusive)
	visualStart int // anchor row (where v was pressed)
	visualEnd   int // cursor row (moved by j/k)

	// Mouse drag-select state
	mouseSelActive bool   // selection visible (during and after drag)
	mouseDragging  bool   // left button held
	mouseSelStart  [2]int // [row, col]
	mouseSelEnd    [2]int // [row, col]

	// infoLines, when non-nil, renders a centered text overlay (Esc to close).
	infoLines  []string
	infoScroll int

	// PTY data batching: accumulate writes and flush on a 16ms tick so the
	// VT has a complete frame before View() runs. Reduces partial-render
	// artifacts from Claude's multi-chunk screen redraws.
	ptyPending     map[buffer.ID][]byte
	ptyTickRunning bool

	needInitial bool
	initialPath string

	quitting bool
}

// New returns a Model with an empty registry. Config and persist store are
// loaded from disk; defaults are used silently on any error.
func New() *Model {
	cfg, _ := config.Load()
	reg := registry.New()
	store, _ := persist.Load()
	pal := theme.Get(cfg.Theme)
	if pal == nil {
		pal = theme.Default()
	}
	cmdline := &ui.Cmdline{
		Completions: map[string][]string{
			"theme": theme.List(),
		},
	}
	return &Model{
		cfg:           cfg,
		reg:           reg,
		fsm:           mode.NewWithLeader(cfg.LeaderRune),
		cmdline:       cmdline,
		picker:        ui.NewPicker(reg),
		search:        &ui.SearchBar{},
		grep:          &ui.GrepPane{},
		sessionPicker: &ui.SessionPicker{},
		store:         store,
		palette:       pal,
		startedAt:     time.Now(),
		needInitial:   true,
	}
}

// Registry exposes the registry for main-level wiring.
func (m *Model) Registry() *registry.Registry { return m.reg }

// FSM exposes the mode FSM.
func (m *Model) FSM() *mode.FSM { return m.fsm }

// Config exposes the loaded config.
func (m *Model) Config() *config.Config { return m.cfg }

// StartServer launches the background API server. Call StopServer on teardown.
func (m *Model) StartServer() {
	s, err := server.New(m)
	if err != nil {
		dlog("server start failed: %v", err)
		return
	}
	m.srv = s
	s.Start()
	dlog("server listening on %s", server.SocketPath())
}

// StopServer shuts down the background API server.
func (m *Model) StopServer() {
	if m.srv != nil {
		m.srv.Stop()
	}
}

// Sessions implements server.Provider.
func (m *Model) Sessions() []server.SessionInfo {
	bufs := m.reg.All()
	out := make([]server.SessionInfo, len(bufs))
	for i, b := range bufs {
		out[i] = server.SessionInfo{
			ID:        int(b.ID),
			Agent:     b.Agent.String(),
			Name:      b.Name,
			Cwd:       b.Cwd,
			SessionID: b.SessionID,
			Status:    b.Status(),
			Pid:       b.Session.Pid(),
		}
	}
	return out
}

// Status implements server.Provider.
func (m *Model) Status() server.StatusInfo {
	return server.StatusInfo{
		ActiveBuffer: m.reg.ActiveIndex(),
		TotalBuffers: m.reg.Len(),
		Uptime:       time.Since(m.startedAt).Truncate(time.Second).String(),
	}
}
