package app

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// Debug logging is gated by the NEOCLAUDE_DEBUG env var (any non-empty value).
// When enabled, dlog appends timestamped lines to $NEOCLAUDE_DEBUG if it looks
// like a path, else to /tmp/neoclaude-debug.log. Used to diagnose vt/cursor
// desync that only reproduces interactively.
var (
	dbgOnce sync.Once
	dbgFile *os.File
	dbgOn   bool
)

func dbgInit() {
	dbgOnce.Do(func() {
		v := os.Getenv("NEOCLAUDE_DEBUG")
		if v == "" {
			return
		}
		path := "/tmp/neoclaude-debug.log"
		if len(v) > 1 && (v[0] == '/' || v[0] == '.') {
			path = v
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			return
		}
		dbgFile = f
		dbgOn = true
		fmt.Fprintf(f, "=== neoclaude debug log %s ===\n", time.Now().Format(time.RFC3339))
	})
}

// escapeBytes renders raw terminal bytes with escapes visible (ESC as \e, other
// control bytes as \xNN) so cursor-control sequences are readable in the log.
// Only the trailing 200 bytes are kept to bound log size.
func escapeBytes(p []byte) string {
	const max = 200
	if len(p) > max {
		p = p[len(p)-max:]
	}
	var sb []byte
	for _, c := range p {
		switch {
		case c == 0x1b:
			sb = append(sb, '\\', 'e')
		case c == '\n':
			sb = append(sb, '\\', 'n')
		case c == '\r':
			sb = append(sb, '\\', 'r')
		case c >= 0x20 && c < 0x7f:
			sb = append(sb, c)
		default:
			sb = append(sb, []byte(fmt.Sprintf("\\x%02x", c))...)
		}
	}
	return string(sb)
}

func dlog(format string, args ...any) {
	dbgInit()
	if !dbgOn {
		return
	}
	fmt.Fprintf(dbgFile, time.Now().Format("15:04:05.000")+" "+format+"\n", args...)
}
