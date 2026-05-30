// Command neoclaude is a Neovim-flavored TUI that manages `claude` CLI sessions
// as PTY-wrapped buffers rendered through a vt10x emulator.
//
// This is the P0 SPIKE: one hardcoded `claude` child in $PWD, full-screen,
// fully interactive. Quit with double-Esc or Ctrl-C. The full mode FSM and
// multi-buffer registry arrive in P1.
package main

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/matthewfritsch/neoclaude/internal/render"
	"github.com/matthewfritsch/neoclaude/internal/session"
	"github.com/matthewfritsch/neoclaude/internal/vt"
)

const version = "0.0.0-dev"

// escDelay is the window within which a second Esc is treated as a quit chord
// (P0 simplification; P1 uses this for the INSERT->NORMAL FSM transition).
const escDelay = 300 * time.Millisecond

// --- Bubble Tea messages (subset of the locked set used by P0) ---

type ptyDataMsg struct{ data []byte }
type ptyExitMsg struct{ err error }

type model struct {
	prog *tea.Program
	sess *session.Session
	vt   *vt.VT

	cwd  string
	cols int
	rows int

	escPending bool
	escAt      time.Time

	quitting bool
	exitErr  error
}

func newModel(cwd string) *model {
	// Reserve one row for the status line.
	cols, rows := 80, 23
	return &model{
		vt:   vt.New(cols, rows),
		cwd:  cwd,
		cols: cols,
		rows: rows,
	}
}

func (m *model) Init() tea.Cmd {
	return nil
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Order matters: resize the emulator grid first, then the PTY, so the
		// child's next repaint targets the new geometry.
		m.cols = msg.Width
		m.rows = msg.Height - 1 // status line
		if m.rows < 1 {
			m.rows = 1
		}
		m.vt.Resize(m.cols, m.rows)
		if m.sess != nil {
			_ = m.sess.Resize(uint16(m.cols), uint16(m.rows))
		}
		return m, nil

	case ptyDataMsg:
		m.vt.Write(msg.data)
		return m, nil

	case ptyExitMsg:
		m.exitErr = msg.err
		m.quitting = true
		return m, tea.Quit

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *model) handleKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	now := time.Now()

	// Ctrl+C is a dedicated quit in the spike (claude would otherwise receive
	// it; full passthrough/interrupt semantics are a P1 concern).
	if k.Type == tea.KeyCtrlC {
		m.quitting = true
		return m, tea.Quit
	}

	if k.Type == tea.KeyEsc {
		if m.escPending && now.Sub(m.escAt) <= escDelay {
			// Second Esc within the window: quit. Do NOT forward to the child.
			m.quitting = true
			return m, tea.Quit
		}
		// First Esc: arm the chord and forward it to the child so a single Esc
		// still reaches claude.
		m.escPending = true
		m.escAt = now
		if m.sess != nil {
			_ = m.sess.Write(encodeKey(k))
		}
		return m, nil
	}

	// Any non-Esc key clears a pending chord and is forwarded raw.
	m.escPending = false
	if m.sess != nil {
		if b := encodeKey(k); len(b) > 0 {
			_ = m.sess.Write(b)
		}
	}
	return m, nil
}

func (m *model) View() string {
	if m.quitting {
		return ""
	}
	x, y, visible := m.vt.Cursor()
	body := render.Blit(m.vt.Snapshot(), render.Options{
		CursorX:       x,
		CursorY:       y,
		CursorVisible: visible,
	})
	status := statusStyle.Render(fmt.Sprintf("[P0 spike] %s — double-Esc to quit", m.cwd))
	return body + "\n" + status
}

var statusStyle = lipgloss.NewStyle().
	Reverse(true).
	Bold(true)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("neoclaude %s\n", version)
		return
	}

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	m := newModel(cwd)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	m.prog = p

	sess, err := session.Start([]string{"claude"}, cwd, uint16(m.cols), uint16(m.rows))
	if err != nil {
		fmt.Fprintf(os.Stderr, "neoclaude: failed to start claude: %v\n", err)
		os.Exit(1)
	}
	m.sess = sess

	// Start the read goroutine only after the program exists so Send is valid.
	go sess.ReadLoop(
		func(b []byte) { p.Send(ptyDataMsg{data: b}) },
		func(e error) { p.Send(ptyExitMsg{err: e}) },
	)

	_, runErr := p.Run()

	// Teardown: always kill the child so no orphan claude survives.
	_ = sess.Kill()

	if runErr != nil {
		fmt.Fprintf(os.Stderr, "neoclaude: %v\n", runErr)
		os.Exit(1)
	}
	if m.exitErr != nil {
		fmt.Fprintf(os.Stderr, "neoclaude: claude exited: %v\n", m.exitErr)
	}
}
