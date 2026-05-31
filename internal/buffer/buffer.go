// Package buffer defines a single neoclaude buffer: a named PTY session paired
// with its vt10x emulator. A Buffer owns one session.Session and one vt.VT and
// nothing else; the registry owns the collection.
package buffer

import (
	"github.com/matthewfritsch/neoclaude/internal/session"
	"github.com/matthewfritsch/neoclaude/internal/vt"
)

// ID is a monotonically increasing buffer identifier assigned by the registry.
type ID int

// Buffer is the unit of multi-buffer management: session + vt + metadata.
type Buffer struct {
	ID         ID
	Name       string // short label shown in the statusline
	Cwd        string // working directory the session was spawned in
	Session    *session.Session
	VT         *vt.VT
	Scrollback *Ring // plain-text lines that have scrolled off the vt top
}

// New creates a Buffer. The caller is responsible for starting the session's
// ReadLoop and for calling Kill on teardown.
func New(id ID, name, cwd string, sess *session.Session, terminal *vt.VT) *Buffer {
	return &Buffer{
		ID:         id,
		Name:       name,
		Cwd:        cwd,
		Session:    sess,
		VT:         terminal,
		Scrollback: DefaultRing(),
	}
}
