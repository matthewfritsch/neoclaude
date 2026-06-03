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
	m.StartServer()
	defer m.StopServer()

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	m.Prog = p

	_, runErr := p.Run()

	for _, b := range m.Registry().All() {
		_ = b.Session.Shutdown()
	}

	if runErr != nil {
		fmt.Fprintf(os.Stderr, "neoclaude: %v\n", runErr)
		os.Exit(1)
	}
}
