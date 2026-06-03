// Package persist manages the sessions.json store that survives process exits,
// enabling named-session resume via claude's own -r/--resume flag.
//
// File location: $XDG_STATE_HOME/neoclaude/sessions.json
// (falls back to ~/.local/state/neoclaude/sessions.json when XDG_STATE_HOME is unset).
//
// Writes are atomic (temp file + rename) so a crash mid-write never leaves a
// truncated JSON file.
package persist

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Record describes one named session.
type Record struct {
	UUID     string    `json:"uuid"`
	Name     string    `json:"name"`
	Cwd      string    `json:"cwd"`
	LastSeen time.Time `json:"last_seen"`
}

// Store is the in-memory view of sessions.json.
type Store struct {
	path    string
	Records []Record `json:"sessions"`
}

// storePath returns the platform-appropriate path for sessions.json.
func storePath() string {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, "neoclaude", "sessions.json")
}

// Load reads the store from disk. If the file does not exist an empty store is
// returned with no error. Other I/O or parse errors are returned directly.
func Load() (*Store, error) {
	path := storePath()
	s := &Store{path: path}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, fmt.Errorf("persist: read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, s); err != nil {
		// Corrupt file: return an empty store so the app can continue.
		return &Store{path: path}, fmt.Errorf("persist: parse %s: %w", path, err)
	}
	return s, nil
}

// Save writes the store to disk atomically (temp file in the same dir + rename).
func (s *Store) Save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("persist: mkdir: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("persist: marshal: %w", err)
	}

	// Write to a temp file in the same directory so rename is atomic on POSIX.
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("persist: write tmp: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("persist: rename: %w", err)
	}
	return nil
}

// Upsert inserts or updates the record for r.UUID. LastSeen is always set to
// the current time.
func (s *Store) Upsert(r Record) {
	r.LastSeen = time.Now().UTC()
	for i, rec := range s.Records {
		if rec.UUID == r.UUID {
			s.Records[i] = r
			return
		}
	}
	s.Records = append(s.Records, r)
}

// Delete removes the record with the given UUID. No-op if not found.
func (s *Store) Delete(uuid string) {
	out := s.Records[:0]
	for _, r := range s.Records {
		if r.UUID != uuid {
			out = append(out, r)
		}
	}
	s.Records = out
}

// Closed returns records whose UUID is not in openUUIDs — i.e. sessions that
// exist in the store but have no currently open buffer.
func (s *Store) Closed(openUUIDs map[string]bool) []Record {
	var out []Record
	for _, r := range s.Records {
		if !openUUIDs[r.UUID] {
			out = append(out, r)
		}
	}
	return out
}

// ByUUID returns a pointer to the record with the given UUID, or nil.
func (s *Store) ByUUID(uuid string) *Record {
	for i := range s.Records {
		if s.Records[i].UUID == uuid {
			return &s.Records[i]
		}
	}
	return nil
}

// ClaudeSessionExists checks whether claude's own session JSONL file exists
// for the given UUID and working directory. Claude stores sessions at
// ~/.claude/projects/<encoded-cwd>/<uuid>.jsonl where encoded-cwd replaces
// every "/" with "-".
func ClaudeSessionExists(uuid, cwd string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	encoded := strings.ReplaceAll(cwd, "/", "-")
	path := filepath.Join(home, ".claude", "projects", encoded, uuid+".jsonl")
	_, err = os.Stat(path)
	return err == nil
}
