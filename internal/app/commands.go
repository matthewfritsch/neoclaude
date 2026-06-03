package app

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/matthewfritsch/neoclaude/internal/buffer"
	"github.com/matthewfritsch/neoclaude/internal/persist"
	"github.com/matthewfritsch/neoclaude/internal/session"
	"github.com/matthewfritsch/neoclaude/internal/theme"
	"github.com/matthewfritsch/neoclaude/internal/vt"
)

// dispatch parses and executes a : command string (without the leading colon).
func (m *Model) dispatch(line string) tea.Cmd {
	line = strings.TrimSpace(line)
	// Split into verb + rest (rest is the verb's argument, e.g. a path or name).
	verb, rest, _ := strings.Cut(line, " ")
	rest = strings.TrimSpace(rest)

	switch verb {
	case "new":
		return m.cmdNew(rest)
	case "bn":
		m.reg.Next()
		m.logActive("bn")
		return nil
	case "bp":
		m.reg.Prev()
		m.logActive("bp")
		return nil
	case "bd":
		m.cmdBd()
		return nil
	case "name":
		m.cmdRename(rest)
		return nil
	case "theme":
		m.cmdTheme(rest)
		return nil
	case "q":
		m.quitting = true
		return tea.Quit
	case "commands":
		m.cmdShowCommands()
		return nil
	case "keybinds":
		m.cmdShowKeybinds()
		return nil
	default:
		return nil
	}
}

// CmdNew is the exported entry point used by main to spawn the initial buffer.
func (m *Model) CmdNew(path string) tea.Cmd { return m.cmdNew(path) }

// cmdNew spawns a new claude buffer. args is the optional cwd from :new [path].
// Name is always derived from the cwd basename and de-duplicated; use :name to
// rename after spawn.
func (m *Model) cmdNew(args string) tea.Cmd {
	return func() tea.Msg {
		cwd := strings.TrimSpace(args)

		// Resolve cwd.
		if cwd == "" {
			var err error
			cwd, err = os.Getwd()
			if err != nil {
				cwd = "."
			}
		}
		if cwd == "~" || strings.HasPrefix(cwd, "~/") {
			if home, err := os.UserHomeDir(); err == nil {
				cwd = home + cwd[1:]
			}
		}

		name := m.uniqueName(randomName())

		cols, rows := m.cols, m.rows
		if cols < 1 {
			cols = 80
		}
		if rows < 1 {
			rows = 23
		}

		// Generate UUID for this session so it can be resumed later.
		sessID := uuid.New().String()

		sess, err := session.Start(session.Opts{
			UUID: sessID,
			Name: name,
			Cwd:  cwd,
			Cols: uint16(cols),
			Rows: uint16(rows),
		})
		if err != nil {
			return PtyExitMsg{BufID: -1, Err: fmt.Errorf("spawn: %w", err)}
		}

		id := m.reg.NextID()
		terminal := vt.New(cols, rows)
		buf := buffer.New(id, name, cwd, sessID, sess, terminal)
		m.reg.Add(buf)

		// Persist immediately so a crash after spawn still records the session.
		m.store.Upsert(persist.Record{
			UUID: sessID,
			Name: name,
			Cwd:  cwd,
		})
		_ = m.store.Save()

		prog := m.Prog
		go sess.ReadLoop(
			func(b []byte) { prog.Send(PtyDataMsg{BufID: id, Data: b}) },
			func(e error) { prog.Send(PtyExitMsg{BufID: id, Err: e}) },
		)

		return bufferAddedMsg{bufID: id}
	}
}

// cmdResume spawns claude --resume <uuid> in the stored cwd and adds a buffer.
func (m *Model) cmdResume(rec persist.Record) tea.Cmd {
	return func() tea.Msg {
		if !persist.ClaudeSessionExists(rec.UUID, rec.Cwd) {
			m.store.Delete(rec.UUID)
			_ = m.store.Save()
			return PtyExitMsg{BufID: -1, Err: fmt.Errorf("session %s not found in claude storage", rec.UUID)}
		}

		cols, rows := m.cols, m.rows
		if cols < 1 {
			cols = 80
		}
		if rows < 1 {
			rows = 23
		}

		sess, err := session.Resume(rec.UUID, rec.Cwd, uint16(cols), uint16(rows))
		if err != nil {
			return PtyExitMsg{BufID: -1, Err: fmt.Errorf("resume: %w", err)}
		}

		// Keep the same UUID so persistence stays consistent.
		id := m.reg.NextID()
		name := m.uniqueName(rec.Name)
		terminal := vt.New(cols, rows)
		buf := buffer.New(id, name, rec.Cwd, rec.UUID, sess, terminal)
		m.reg.Add(buf)

		// Update lastSeen.
		m.store.Upsert(persist.Record{
			UUID: rec.UUID,
			Name: name,
			Cwd:  rec.Cwd,
		})
		_ = m.store.Save()

		prog := m.Prog
		go sess.ReadLoop(
			func(b []byte) { prog.Send(PtyDataMsg{BufID: id, Data: b}) },
			func(e error) { prog.Send(PtyExitMsg{BufID: id, Err: e}) },
		)

		return bufferAddedMsg{bufID: id}
	}
}

// bufferAddedMsg is sent after cmdNew/cmdResume completes so Update can resize.
type bufferAddedMsg struct{ bufID buffer.ID }

// cmdRename updates the active buffer's neoclaude label, persists it, and also
// renames the live claude session by forwarding claude's `/rename <name>` slash
// command into the child PTY.
//
// The forward is best-effort: a leading Ctrl-U (0x15) clears claude's input
// line so a partially-typed prompt doesn't corrupt the command — note this
// discards any unsent text in claude's input box. The neoclaude label + store
// are updated regardless of whether the forward takes effect.
func (m *Model) cmdRename(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	b := m.reg.Active()
	if b == nil {
		return
	}
	b.Name = name
	if b.SessionID != "" {
		m.store.Upsert(persist.Record{
			UUID: b.SessionID,
			Name: name,
			Cwd:  b.Cwd,
		})
		_ = m.store.Save()
	}
	// Forward to the live claude session: clear the input line, then run
	// claude's own /rename slash command.
	if b.Session != nil {
		_ = b.Session.Write([]byte("\x15/rename " + name + "\r"))
	}
}

// cmdBd kills and removes the active buffer.
func (m *Model) cmdBd() {
	b := m.reg.Active()
	if b == nil {
		return
	}
	_ = m.reg.Remove(b.ID)
}

// cmdTheme switches the active palette. Empty name is a no-op.
func (m *Model) cmdTheme(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	p := theme.Get(name)
	if p == nil {
		return
	}
	m.palette = p
	m.cfg.Theme = name
}

func (m *Model) cmdShowCommands() {
	m.infoLines = []string{
		"Commands",
		"",
		"  :new [path]     Open new buffer",
		"  :bd             Close active buffer",
		"  :bn             Next buffer",
		"  :bp             Previous buffer",
		"  :name <name>    Rename buffer + claude session",
		"  :theme <name>   Switch color theme",
		"  :q              Quit",
		"  :commands       This help",
		"  :keybinds       Show keybindings",
	}
}

func (m *Model) cmdShowKeybinds() {
	leader := string(m.cfg.LeaderRune)
	if leader == " " {
		leader = "Space"
	}
	m.infoLines = []string{
		"Keybindings",
		"",
		"  Normal Mode",
		"    i, a              Enter Insert mode",
		"    Enter             Enter Insert mode",
		"    :                 Open command line",
		"    /                 Open search",
		"    v                 Enter Visual mode",
		fmt.Sprintf("    %s %s        Buffer picker", leader, leader),
		fmt.Sprintf("    %s s g            Grep all buffers", leader),
		fmt.Sprintf("    %s s n            Session picker", leader),
		"    Ctrl-U / PgUp     Scroll up (half page)",
		"    Ctrl-D / PgDn     Scroll down (half page)",
		"    Mouse wheel       Scroll (3 lines)",
		"    n                 Next search match",
		"    N                 Previous search match",
		"    Ctrl-C            Quit",
		"",
		"  Insert Mode",
		"    Esc Esc           Return to Normal mode",
		"",
		"  Search Mode (/)",
		"    Enter             Confirm (n/N navigate after)",
		"    Esc               Cancel search",
		"",
		"  Visual Mode (v)",
		"    y                 Yank selection to clipboard",
		"    Esc               Exit visual mode",
		"",
		"  Command Line (:)",
		"    Tab               Complete command/argument",
		"    Up/Down           History with prefix matching",
	}
}

// logActive records the active buffer's vt size + cursor after a switch.
func (m *Model) logActive(via string) {
	b := m.reg.Active()
	if b == nil {
		dlog("switch(%s) -> no active buffer", via)
		return
	}
	cx, cy, cv := b.VT.Cursor()
	vc, vr := b.VT.Size()
	dlog("switch(%s) -> buf=%d idx=%d vtsize=%dx%d cursor=(%d,%d,vis=%v)",
		via, int(b.ID), m.reg.ActiveIndex(), vc, vr, cx, cy, cv)
}

// uniqueName returns name if no existing buffer uses it, otherwise appends
// ~2, ~3, … until a unique name is found.
func (m *Model) uniqueName(name string) string {
	used := make(map[string]bool)
	for _, b := range m.reg.All() {
		used[b.Name] = true
	}
	if !used[name] {
		return name
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s~%d", name, i)
		if !used[candidate] {
			return candidate
		}
	}
}
