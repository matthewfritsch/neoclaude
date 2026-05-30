// Command neoclaude is a Neovim-flavored TUI that manages `claude` CLI sessions
// as PTY-wrapped buffers rendered through a vt10x emulator and blitted into a
// lipgloss view.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/matthewfritsch/neoclaude/internal/app"
)

const version = "0.0.0-dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("neoclaude %s\n", version)
		return
	}

	m := app.New()
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	m.Prog = p

	// Spawn the initial buffer before p.Run() so the user sees a session
	// immediately. cmdNew returns a tea.Cmd; we call it directly here to get
	// the synchronous result, then hand the model to the program.
	initCmd := m.CmdNew("")
	if initMsg := initCmd(); initMsg != nil {
		// Feed the init message through Update so the registry and resize logic
		// fire before the event loop starts.
		_, _ = m.Update(initMsg)
	}

	_, runErr := p.Run()

	// Teardown: kill all remaining sessions.
	for _, b := range m.Registry().All() {
		_ = b.Session.Kill()
	}

	if runErr != nil {
		fmt.Fprintf(os.Stderr, "neoclaude: %v\n", runErr)
		os.Exit(1)
	}
}
