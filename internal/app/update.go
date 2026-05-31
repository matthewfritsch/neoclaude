package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/matthewfritsch/neoclaude/internal/mode"
	"github.com/matthewfritsch/neoclaude/internal/ui"
	"github.com/matthewfritsch/neoclaude/internal/vt"
)

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.cols = msg.Width
		m.rows = msg.Height - 1
		if m.rows < 1 {
			m.rows = 1
		}
		dlog("WindowSize cols=%d rows=%d bufs=%d", m.cols, m.rows, m.reg.Len())
		for _, b := range m.reg.All() {
			b.VT.Resize(m.cols, m.rows)
			_ = b.Session.Resize(uint16(m.cols), uint16(m.rows))
		}
		if m.needInitial {
			m.needInitial = false
			return m, m.cmdNew(m.initialPath)
		}
		return m, nil

	case PtyDataMsg:
		if b := m.reg.ByID(msg.BufID); b != nil {
			b.VT.Write(msg.Data)
			// Forward any emulator-generated responses (DA replies, CPR,
			// keyboard-protocol acks) back to the child PTY.  These are
			// produced by the emulator in reaction to the bytes we just fed it
			// and must reach the child or it may hang waiting for answers.
			if resp := b.VT.DrainResponses(); len(resp) > 0 {
				_ = b.Session.Write(resp)
			}
			// Keep search corpus fresh for the active buffer.
			if ab := m.reg.Active(); ab != nil && ab.ID == msg.BufID && m.search.Active() {
				m.search.UpdateCorpus(b.Scrollback.Lines(), b.VT.Snapshot())
			}
		}
		return m, nil

	case PtyExitMsg:
		if msg.BufID < 0 {
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
		if b := m.reg.ByID(msg.bufID); b != nil {
			b.VT.Resize(m.cols, m.rows)
			_ = b.Session.Resize(uint16(m.cols), uint16(m.rows))
			dlog("bufferAdded buf=%d resized to %dx%d", int(b.ID), m.cols, m.rows)
		}
		return m, nil

	case ui.GrepResultMsg:
		m.grep.SetResults(msg.Query, msg.Hits)
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m *Model) handleKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	cur := m.fsm.Mode()

	// --- Picker overlay ---
	if cur == mode.Picker {
		if k.Type == tea.KeyEsc {
			m.fsm.SetMode(mode.Normal)
			m.picker.Close()
			return m, nil
		}
		if k.Type == tea.KeyEnter {
			if id := m.picker.SelectedID(); id >= 0 {
				for i, b := range m.reg.All() {
					if int(b.ID) == id {
						m.reg.SetActive(i)
						cx, cy, cv := b.VT.Cursor()
						vc, vr := b.VT.Size()
						dlog("switch(picker) -> buf=%d idx=%d vtsize=%dx%d cursor=(%d,%d,vis=%v)",
							int(b.ID), i, vc, vr, cx, cy, cv)
						break
					}
				}
			}
			m.fsm.SetMode(mode.Normal)
			m.picker.Close()
			return m, nil
		}
		m.picker.HandleKey(k)
		return m, nil
	}

	// --- Grep pane overlay ---
	if m.grep.Active() {
		if k.Type == tea.KeyEsc {
			m.grep.Close()
			m.fsm.SetMode(mode.Normal)
			return m, nil
		}
		// Typing updates the live grep query.
		if k.Type == tea.KeyRunes || k.Type == tea.KeyBackspace || k.Type == tea.KeySpace {
			q := m.grep.QueryStr()
			switch k.Type {
			case tea.KeyBackspace:
				if len(q) > 0 {
					q = q[:len(q)-1]
				}
			case tea.KeySpace:
				q += " "
			default:
				q += string(k.Runes)
			}
			m.grep.SetResults(q, nil)
			return m, m.runGrep(q)
		}
		if selID, ok := m.grep.HandleKey(k); ok {
			for i, b := range m.reg.All() {
				if b.ID == selID {
					m.reg.SetActive(i)
					break
				}
			}
			m.grep.Close()
			m.fsm.SetMode(mode.Normal)
		}
		return m, nil
	}

	// --- Command mode ---
	if cur == mode.Command && k.Type != tea.KeyEnter && k.Type != tea.KeyEsc {
		m.cmdline.HandleKey(k)
		return m, nil
	}

	// --- Search mode: feed all typing (incl n/N/space) to the bar; only Esc
	// falls through to the FSM to close the search. (Vim-style Enter-confirm
	// then n/N navigation is a planned follow-up.)
	if cur == mode.Search && k.Type != tea.KeyEsc {
		m.search.HandleKey(k)
		return m, nil
	}

	action, _ := m.fsm.HandleKey(k, time.Now())

	switch action {
	case mode.ActionQuit:
		m.quitting = true
		return m, tea.Quit

	case mode.ActionForward:
		if b := m.reg.Active(); b != nil {
			if enc := EncodeKey(k); len(enc) > 0 {
				cx, cy, cv := b.VT.Cursor()
				vc, vr := b.VT.Size()
				dlog("forward buf=%d active=%d vtsize=%dx%d cursor=(%d,%d,vis=%v) enc=%q",
					int(b.ID), m.reg.ActiveIndex(), vc, vr, cx, cy, cv, string(enc))
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

	case mode.ActionOpenPicker:
		m.picker.Open()

	case mode.ActionOpenGrep:
		m.grep.Open()
		return m, m.runGrep("")

	case mode.ActionOpenSearch:
		m.openSearch()

	case mode.ActionEnterVisual:
		if b := m.reg.Active(); b != nil {
			_, y, _ := b.VT.Cursor()
			m.visualStart = y
			m.visualEnd = y
		}

	case mode.ActionSearchNext:
		m.search.Next()

	case mode.ActionSearchPrev:
		m.search.Prev()

	case mode.ActionVisualYank:
		m.yankVisual()

	case mode.ActionExitOverlay:
		m.search.Close()
		m.picker.Close()
	}

	// Track visual selection end row while in Visual mode.
	if m.fsm.Mode() == mode.Visual {
		if b := m.reg.Active(); b != nil {
			_, y, _ := b.VT.Cursor()
			if y < m.visualStart {
				m.visualEnd = m.visualStart
				m.visualStart = y
			} else {
				m.visualEnd = y
			}
		}
	}

	return m, nil
}

func (m *Model) openSearch() {
	b := m.reg.Active()
	if b == nil {
		return
	}
	corpus := append(b.Scrollback.Lines(), vt.ExtractLines(b.VT.Snapshot())...)
	m.search.Open(corpus)
}

func (m *Model) runGrep(query string) tea.Cmd {
	corpus := make([]ui.GrepCorpusEntry, 0, m.reg.Len())
	for _, b := range m.reg.All() {
		lines := append(b.Scrollback.Lines(), vt.ExtractLines(b.VT.Snapshot())...)
		corpus = append(corpus, ui.GrepCorpusEntry{
			BufID: b.ID,
			Name:  b.Name,
			Lines: lines,
		})
	}
	return ui.GrepCmd(query, corpus)
}

func (m *Model) yankVisual() {
	b := m.reg.Active()
	if b == nil {
		return
	}
	lines := vt.ExtractLines(b.VT.Snapshot())
	start, end := m.visualStart, m.visualEnd
	if start > end {
		start, end = end, start
	}
	if start < 0 {
		start = 0
	}
	if end >= len(lines) {
		end = len(lines) - 1
	}
	if start > end {
		return
	}
	text := ""
	for i := start; i <= end; i++ {
		if i > start {
			text += "\n"
		}
		text += lines[i]
	}
	_ = writeClipboard(text)
}
