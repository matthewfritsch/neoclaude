package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/matthewfritsch/neoclaude/internal/mode"
)

// Init implements tea.Model. The initial buffer is spawned by main before
// p.Run(), so nothing async is needed here.
func (m *Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.cols = msg.Width
		m.rows = msg.Height - 1 // reserve one row for status/cmdline
		if m.rows < 1 {
			m.rows = 1
		}
		// Resize every open buffer's vt and pty.
		for _, b := range m.reg.All() {
			b.VT.Resize(m.cols, m.rows)
			_ = b.Session.Resize(uint16(m.cols), uint16(m.rows))
		}
		return m, nil

	case PtyDataMsg:
		if b := m.reg.ByID(msg.BufID); b != nil {
			b.VT.Write(msg.Data)
		}
		return m, nil

	case PtyExitMsg:
		if msg.BufID < 0 {
			// Spawn error from cmdNew — nothing to remove.
			return m, nil
		}
		if b := m.reg.ByID(msg.BufID); b != nil {
			_ = m.reg.Remove(b.ID)
		}
		if m.reg.Len() == 0 {
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil

	case bufferAddedMsg:
		// Resize the new buffer to current terminal size.
		if b := m.reg.ByID(msg.bufID); b != nil {
			b.VT.Resize(m.cols, m.rows)
			_ = b.Session.Resize(uint16(m.cols), uint16(m.rows))
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m *Model) handleKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Command mode: feed all non-Enter/Esc keys to the cmdline widget first.
	// Enter and Esc are handled by the FSM below.
	if m.fsm.Mode() == mode.Command {
		if k.Type != tea.KeyEnter && k.Type != tea.KeyEsc {
			m.cmdline.HandleKey(k)
			return m, nil
		}
	}

	action, _ := m.fsm.HandleKey(k, time.Now())

	switch action {
	case mode.ActionQuit:
		m.quitting = true
		return m, tea.Quit

	case mode.ActionForward:
		if b := m.reg.Active(); b != nil {
			if enc := EncodeKey(k); len(enc) > 0 {
				_ = b.Session.Write(enc)
			}
		}

	case mode.ActionEnterCommand:
		m.cmdline.Open()

	case mode.ActionExecCommand:
		line := m.cmdline.Value()
		m.cmdline.Close()
		if cmd := m.dispatch(line); cmd != nil {
			return m, cmd
		}

	case mode.ActionCancelCommand:
		m.cmdline.Close()
	}

	return m, nil
}
