// Package buffer defines a single neoclaude buffer: a named PTY session paired
// with its vt10x emulator. A Buffer owns one session.Session and one vt.VT and
// nothing else; the registry owns the collection.
package buffer

import (
	"time"

	"github.com/matthewfritsch/neoclaude/internal/agent"
	"github.com/matthewfritsch/neoclaude/internal/session"
	"github.com/matthewfritsch/neoclaude/internal/vt"
)

// ID is a monotonically increasing buffer identifier assigned by the registry.
type ID int

// Buffer is the unit of multi-buffer management: session + vt + metadata.
type Buffer struct {
	ID           ID
	Agent        agent.Type
	Name         string // short label shown in the statusline (neoclaude's own label)
	Cwd          string // working directory the session was spawned in
	SessionID    string // UUID passed to claude --session-id (empty for anonymous buffers)
	Session      *session.Session
	VT           *vt.VT
	Scrollback   *Ring // plain-text lines that have scrolled off the vt top
	ScrollOffset int   // lines scrolled back from bottom; 0 = live view
	LastDataAt   time.Time
}

// Status returns "idle" or "busy" based on PTY activity.
func (b *Buffer) Status() string {
	if b.LastDataAt.IsZero() || time.Since(b.LastDataAt) > 3*time.Second {
		return "idle"
	}
	return "busy"
}

// New creates a Buffer. The caller is responsible for starting the session's
// ReadLoop and for calling Kill on teardown.
func New(id ID, name, cwd, sessionID string, sess *session.Session, terminal *vt.VT) *Buffer {
	return NewWithAgent(id, agent.Claude, name, cwd, sessionID, sess, terminal)
}

// NewWithAgent creates a Buffer for the selected agent.
func NewWithAgent(id ID, kind agent.Type, name, cwd, sessionID string, sess *session.Session, terminal *vt.VT) *Buffer {
	return &Buffer{
		ID:         id,
		Agent:      agent.Normalize(string(kind)),
		Name:       name,
		Cwd:        cwd,
		SessionID:  sessionID,
		Session:    sess,
		VT:         terminal,
		Scrollback: DefaultRing(),
	}
}
