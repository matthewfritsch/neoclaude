package app

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/matthewfritsch/neoclaude/internal/buffer"
	"github.com/matthewfritsch/neoclaude/internal/session"
	"github.com/matthewfritsch/neoclaude/internal/vt"
)

// dispatch parses and executes a : command string (without the leading colon).
// It returns a tea.Cmd for async work (currently only :new spawns a goroutine).
func (m *Model) dispatch(line string) tea.Cmd {
	line = strings.TrimSpace(line)
	cmd, arg, _ := strings.Cut(line, " ")
	arg = strings.TrimSpace(arg)

	switch cmd {
	case "new":
		return m.cmdNew(arg)
	case "bn":
		m.reg.Next()
		return nil
	case "bp":
		m.reg.Prev()
		return nil
	case "bd":
		m.cmdBd()
		return nil
	default:
		// Unknown command: silently ignore for now (P1 simplification).
		return nil
	}
}

// CmdNew is the exported entry point used by main to spawn the initial buffer.
// Internally it delegates to the same logic as the :new command handler.
func (m *Model) CmdNew(path string) tea.Cmd { return m.cmdNew(path) }

// cmdNew spawns a new claude buffer with cwd=path (defaults to $PWD).
// It returns a tea.Cmd that creates the session synchronously but starts the
// read goroutine only after the program reference is available.
func (m *Model) cmdNew(path string) tea.Cmd {
	return func() tea.Msg {
		cwd := path
		if cwd == "" {
			var err error
			cwd, err = os.Getwd()
			if err != nil {
				cwd = "."
			}
		}
		// Expand ~ manually (os.UserHomeDir is cheap).
		if cwd == "~" || strings.HasPrefix(cwd, "~/") {
			if home, err := os.UserHomeDir(); err == nil {
				cwd = home + cwd[1:]
			}
		}

		cols, rows := m.cols, m.rows
		if cols < 1 {
			cols = 80
		}
		if rows < 1 {
			rows = 23
		}

		sess, err := session.Start([]string{"claude"}, cwd, uint16(cols), uint16(rows))
		if err != nil {
			// Return a ptyExitMsg with the error so the UI can surface it.
			return PtyExitMsg{BufID: -1, Err: fmt.Errorf("spawn: %w", err)}
		}

		id := m.reg.NextID()
		name := fmt.Sprintf("claude-%d", int(id)+1)
		terminal := vt.New(cols, rows)
		buf := buffer.New(id, name, cwd, sess, terminal)
		m.reg.Add(buf)

		// Start the read goroutine now — Prog is set before any tea.Cmd runs.
		prog := m.Prog
		go sess.ReadLoop(
			func(b []byte) { prog.Send(PtyDataMsg{BufID: id, Data: b}) },
			func(e error) { prog.Send(PtyExitMsg{BufID: id, Err: e}) },
		)

		return bufferAddedMsg{bufID: id}
	}
}

// bufferAddedMsg is sent after cmdNew completes so Update can trigger a resize.
type bufferAddedMsg struct{ bufID buffer.ID }

// cmdBd kills and removes the active buffer.
func (m *Model) cmdBd() {
	b := m.reg.Active()
	if b == nil {
		return
	}
	_ = m.reg.Remove(b.ID)
}
