package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/matthewfritsch/neoclaude/internal/buffer"
	"github.com/matthewfritsch/neoclaude/internal/mode"
	"github.com/matthewfritsch/neoclaude/internal/persist"
	"github.com/matthewfritsch/neoclaude/internal/search"
	"github.com/matthewfritsch/neoclaude/internal/ui"
	"github.com/matthewfritsch/neoclaude/internal/vt"
)

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	m.infoLines = m.motdLines()
	m.infoScroll = 0
	return nil
}

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
		// Flush any pending PTY data at the old size before resizing.
		m.flushPtyPending()
		for _, b := range m.reg.All() {
			b.VT.Resize(m.cols, m.rows)
			_ = b.Session.Resize(uint16(m.cols), uint16(m.rows))
		}
		if m.needInitial {
			m.needInitial = false
		}
		return m, nil

	case tea.MouseMsg:
		if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
			if b := m.reg.Active(); b != nil && m.fsm.Mode() == mode.Normal {
				if msg.Button == tea.MouseButtonWheelUp {
					b.ScrollOffset += 3
					max := b.VT.ScrollbackLen()
					if b.ScrollOffset > max {
						b.ScrollOffset = max
					}
				} else {
					b.ScrollOffset -= 3
					if b.ScrollOffset < 0 {
						b.ScrollOffset = 0
					}
				}
			}
		}
		return m, nil

	case PtyDataMsg:
		if b := m.reg.ByID(msg.BufID); b != nil {
			dumpRaw(msg.Data)
			b.LastDataAt = time.Now()
			b.ScrollOffset = 0
			if m.ptyPending == nil {
				m.ptyPending = make(map[buffer.ID][]byte)
			}
			m.ptyPending[msg.BufID] = append(m.ptyPending[msg.BufID], msg.Data...)
			if !m.ptyTickRunning {
				m.ptyTickRunning = true
				return m, tea.Tick(16*time.Millisecond, func(time.Time) tea.Msg {
					return ptyFlushMsg{}
				})
			}
		}
		return m, nil

	case ptyFlushMsg:
		m.ptyTickRunning = false
		m.flushPtyPending()
		return m, nil

	case PtyExitMsg:
		if msg.BufID < 0 {
			return m, nil
		}
		if b := m.reg.ByID(msg.BufID); b != nil {
			_ = m.reg.Remove(b.ID)
		}
		return m, nil

	case bufferAddedMsg:
		if b := m.reg.ByID(msg.bufID); b != nil {
			b.VT.Resize(m.cols, m.rows)
			_ = b.Session.Resize(uint16(m.cols), uint16(m.rows))
			dlog("bufferAdded buf=%d resized to %dx%d", int(b.ID), m.cols, m.rows)
		}
		m.fsm.SetMode(mode.Insert)
		return m, nil

	case ui.GrepResultMsg:
		m.grep.SetResults(msg.Query, msg.Hits)
		return m, nil

	case sessionPickerOpenMsg:
		m.sessionPicker.Open(msg.entries)
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m *Model) handleKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	cur := m.fsm.Mode()

	// --- Info overlay (blocks everything, scrollable) ---
	if len(m.infoLines) > 0 {
		switch k.Type {
		case tea.KeyEsc:
			m.infoLines = nil
			m.infoScroll = 0
		case tea.KeyUp:
			if m.infoScroll > 0 {
				m.infoScroll--
			}
		case tea.KeyDown:
			m.infoScroll++
		case tea.KeyRunes:
			switch string(k.Runes) {
			case "j":
				m.infoScroll++
			case "k":
				if m.infoScroll > 0 {
					m.infoScroll--
				}
			}
		}
		return m, nil
	}

	// --- Session picker overlay (<leader>sn) ---
	if m.sessionPicker.Active() {
		if k.Type == tea.KeyEsc {
			m.sessionPicker.Close()
			m.fsm.SetMode(mode.Normal)
			return m, nil
		}
		sel, action := m.sessionPicker.HandleKey(k)
		switch action {
		case ui.PickerSelect:
			m.sessionPicker.Close()
			m.fsm.SetMode(mode.Normal)
			if sel.IsLive() {
				for i, b := range m.reg.All() {
					if int(b.ID) == sel.LiveBufID {
						m.reg.SetActive(i)
						break
					}
				}
				return m, nil
			}
			return m, m.cmdResume(closedRecord(sel))
		case ui.PickerDelete:
			m.store.Delete(sel.UUID)
			_ = m.store.Save()
		}
		return m, nil
	}

	// --- Buffer picker overlay (<leader><leader>) ---
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

	// --- Search mode (typing phase) ---
	if cur == mode.Search && k.Type != tea.KeyEsc {
		if k.Type == tea.KeyEnter {
			m.search.Confirm()
			m.fsm.SetMode(mode.Normal)
			return m, nil
		}
		m.search.HandleKey(k)
		return m, nil
	}

	// --- Normal mode with confirmed search: n/N navigate, Esc clears ---
	if cur == mode.Normal && m.search.Active() && m.search.Confirmed() {
		if k.Type == tea.KeyEsc {
			m.search.Close()
			return m, nil
		}
		if k.Type == tea.KeyRunes {
			switch string(k.Runes) {
			case "n":
				m.search.Next()
				return m, nil
			case "N":
				m.search.Prev()
				return m, nil
			}
		}
	}

	// --- Normal mode scroll: Ctrl-U/D, PgUp/PgDn ---
	if cur == mode.Normal {
		if scrolled := m.handleScroll(k); scrolled {
			return m, nil
		}
	}

	action, _ := m.fsm.HandleKey(k)

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
		m.cmdline.PushHistory(line)
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

	case mode.ActionOpenNamedSessions:
		return m, m.openSessionPicker()

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

// openSessionPicker builds the entry list and shows the session picker.
func (m *Model) openSessionPicker() tea.Cmd {
	return func() tea.Msg {
		openUUIDs := make(map[string]bool)
		liveEntries := make([]ui.SessionEntry, 0, m.reg.Len())
		for _, b := range m.reg.All() {
			if b.SessionID != "" {
				openUUIDs[b.SessionID] = true
			}
			liveEntries = append(liveEntries, ui.SessionEntry{
				LiveBufID: int(b.ID),
				UUID:      b.SessionID,
				Name:      b.Name,
				Cwd:       b.Cwd,
				Display:   b.Name + " " + b.Cwd,
			})
		}
		entries := ui.BuildSessionEntries(liveEntries, m.store, openUUIDs)
		return sessionPickerOpenMsg{entries: entries}
	}
}

// sessionPickerOpenMsg triggers the session picker UI to open.
type sessionPickerOpenMsg struct{ entries []ui.SessionEntry }

// closedRecord reconstructs a persist.Record from a closed SessionEntry.
func closedRecord(e *ui.SessionEntry) persist.Record {
	return persist.Record{UUID: e.UUID, Name: e.Name, Cwd: e.Cwd}
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
	openUUIDs := make(map[string]bool)
	corpus := make([]ui.GrepCorpusEntry, 0, m.reg.Len())
	for _, b := range m.reg.All() {
		if b.SessionID != "" {
			openUUIDs[b.SessionID] = true
		}
		lines := append(b.Scrollback.Lines(), vt.ExtractLines(b.VT.Snapshot())...)
		corpus = append(corpus, ui.GrepCorpusEntry{
			BufID: b.ID,
			Name:  b.Name,
			Lines: lines,
		})
	}
	closed := m.store.Closed(openUUIDs)
	return func() tea.Msg {
		for _, r := range closed {
			lines := persist.ExtractSessionText(r.UUID, r.Cwd)
			if len(lines) > 0 {
				corpus = append(corpus, ui.GrepCorpusEntry{
					BufID: -1,
					Name:  r.Name + " (closed)",
					Lines: lines,
				})
			}
		}
		var allHits []search.Hit
		for _, e := range corpus {
			hits := search.SearchBuffer(e.BufID, e.Name, e.Lines, query)
			allHits = append(allHits, hits...)
		}
		return ui.GrepResultMsg{Query: query, Hits: allHits}
	}
}

func (m *Model) flushPtyPending() {
	for id, data := range m.ptyPending {
		if b := m.reg.ByID(id); b != nil {
			b.VT.Write(data)
			if resp := b.VT.DrainResponses(); len(resp) > 0 {
				_ = b.Session.Write(resp)
			}
			if ab := m.reg.Active(); ab != nil && ab.ID == id && m.search.Active() {
				m.search.UpdateCorpus(b.Scrollback.Lines(), b.VT.Snapshot())
			}
		}
	}
	m.ptyPending = nil
}

func (m *Model) handleScroll(k tea.KeyMsg) bool {
	b := m.reg.Active()
	if b == nil {
		return false
	}
	half := m.rows / 2
	if half < 1 {
		half = 1
	}
	max := b.VT.ScrollbackLen()

	delta := 0
	switch k.Type {
	case tea.KeyCtrlU, tea.KeyPgUp:
		delta = half
	case tea.KeyCtrlD, tea.KeyPgDown:
		delta = -half
	default:
		return false
	}

	b.ScrollOffset += delta
	if b.ScrollOffset > max {
		b.ScrollOffset = max
	}
	if b.ScrollOffset < 0 {
		b.ScrollOffset = 0
	}
	return true
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
