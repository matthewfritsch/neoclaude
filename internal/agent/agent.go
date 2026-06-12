package agent

import "strings"

// Type identifies the CLI backing a managed buffer.
type Type string

const (
	Claude Type = "claude"
	Codex  Type = "codex"
)

// Normalize returns a supported agent, defaulting to Claude for empty/unknown
// values so older persisted records remain Claude sessions.
func Normalize(value string) Type {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(Codex):
		return Codex
	default:
		return Claude
	}
}

func (a Type) String() string {
	return string(Normalize(string(a)))
}

func (a Type) Command() string {
	return string(Normalize(string(a)))
}

func (a Type) SupportsResume() bool {
	return Normalize(string(a)) == Claude
}
