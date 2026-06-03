package app

import "github.com/matthewfritsch/neoclaude/internal/buffer"

// PtyDataMsg carries a chunk of raw PTY output from a buffer's read goroutine.
type PtyDataMsg struct {
	BufID buffer.ID
	Data  []byte
}

// PtyExitMsg is sent when a buffer's child process exits.
type PtyExitMsg struct {
	BufID buffer.ID
	Err   error
}

// ptyFlushMsg triggers flushing batched PTY data to VT emulators.
type ptyFlushMsg struct{}
