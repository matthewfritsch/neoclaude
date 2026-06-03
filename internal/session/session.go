// Package session manages a single PTY-wrapped child process (a `claude` CLI
// session). It spawns the child on a pseudo-terminal, streams its output to the
// Bubble Tea program via a read goroutine, forwards keystrokes, handles
// resizes, and tears the child down cleanly.
package session

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"syscall"

	"github.com/creack/pty"
)

// Session is a running child on a PTY.
type Session struct {
	cmd  *exec.Cmd
	ptmx *os.File
}

// Opts configures how a new session is started.
type Opts struct {
	// UUID is passed as --session-id to claude so the session can be resumed
	// later with Resume(). Required for named sessions; leave empty for the
	// initial anonymous buffer spawned at startup.
	UUID string
	// Name is kept for neoclaude's own display; not passed to claude.
	Name string
	// Cwd is the working directory for the child process.
	Cwd string
	// Cols/Rows are the initial PTY dimensions.
	Cols, Rows uint16
}

// Start launches a new claude session on a PTY using opts.
// The argv is always ["claude", ...flags...]; callers provide flags via Opts.
func Start(opts Opts) (*Session, error) {
	argv := []string{"claude"}
	if opts.UUID != "" {
		argv = append(argv, "--session-id", opts.UUID)
	}
	return startProcess(argv, opts.Cwd, opts.Cols, opts.Rows)
}

// Resume spawns a claude session that resumes a prior conversation identified
// by uuid using claude's own -r/--resume flag. Context is fully restored by
// claude itself.
func Resume(uuid, cwd string, cols, rows uint16) (*Session, error) {
	if uuid == "" {
		return nil, errors.New("session: resume requires a non-empty uuid")
	}
	argv := []string{"claude", "--resume", uuid}
	return startProcess(argv, cwd, cols, rows)
}

// startProcess is the shared PTY-launch helper.
func startProcess(argv []string, cwd string, cols, rows uint16) (*Session, error) {
	if len(argv) == 0 {
		return nil, errors.New("session: empty argv")
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	// Put the child in its own process group so we can signal the whole group
	// on teardown and avoid orphaned grandchildren.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if cols < 1 {
		cols = 80
	}
	if rows < 1 {
		rows = 24
	}
	ws := &pty.Winsize{Rows: rows, Cols: cols}
	ptmx, err := pty.StartWithSize(cmd, ws)
	if err != nil {
		return nil, err
	}
	return &Session{cmd: cmd, ptmx: ptmx}, nil
}

// Pid returns the child process id, or 0 if not started.
func (s *Session) Pid() int {
	if s.cmd == nil || s.cmd.Process == nil {
		return 0
	}
	return s.cmd.Process.Pid
}

// ReadLoop reads child output until EOF or error, invoking onData with a fresh
// copy of each chunk (the internal buffer is reused, so the copy is required
// before handing bytes to another goroutine). On exit it invokes onExit with
// the terminating error (nil on clean EOF). Run this in its own goroutine,
// started after the tea.Program exists so the send callbacks are valid.
func (s *Session) ReadLoop(onData func([]byte), onExit func(error)) {
	buf := make([]byte, 32*1024)
	for {
		n, err := s.ptmx.Read(buf)
		if n > 0 {
			cp := make([]byte, n)
			copy(cp, buf[:n])
			onData(cp)
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = nil
			}
			onExit(err)
			return
		}
	}
}

// Write forwards bytes (encoded keystrokes) to the child.
func (s *Session) Write(p []byte) error {
	_, err := s.ptmx.Write(p)
	return err
}

// Resize updates the PTY window size.
func (s *Session) Resize(cols, rows uint16) error {
	return pty.Setsize(s.ptmx, &pty.Winsize{Rows: rows, Cols: cols})
}

// Kill terminates the child's process group and closes the PTY. It is safe to
// call more than once.
func (s *Session) Kill() error {
	var firstErr error
	if s.cmd != nil && s.cmd.Process != nil {
		pid := s.cmd.Process.Pid
		// Negative pid signals the whole process group (Setsid above makes the
		// child a group leader, so pgid == pid).
		if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
			// Fall back to killing just the child if the group signal fails.
			if err2 := s.cmd.Process.Kill(); err2 != nil {
				firstErr = err2
			}
		}
		_, _ = s.cmd.Process.Wait()
	}
	if s.ptmx != nil {
		if err := s.ptmx.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
