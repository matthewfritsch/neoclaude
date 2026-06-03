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
	"bufio"
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

// ImportClaudeSessions scans ~/.claude/projects/ for JSONL session files and
// adds any that aren't already in the store. Returns the number imported.
func (s *Store) ImportClaudeSessions() (int, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return 0, err
	}
	projectsDir := filepath.Join(home, ".claude", "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return 0, err
	}

	known := make(map[string]bool, len(s.Records))
	for _, r := range s.Records {
		known[r.UUID] = true
	}

	count := 0
	for _, dir := range entries {
		if !dir.IsDir() {
			continue
		}
		cwd := decodeCwd(dir.Name())
		jsonls, _ := filepath.Glob(filepath.Join(projectsDir, dir.Name(), "*.jsonl"))
		for _, path := range jsonls {
			uuid := strings.TrimSuffix(filepath.Base(path), ".jsonl")
			if known[uuid] {
				continue
			}
			name := extractTitle(path)
			if name == "" {
				name = uuid[:8]
			}
			s.Upsert(Record{UUID: uuid, Name: name, Cwd: cwd})
			known[uuid] = true
			count++
		}
	}
	if count > 0 {
		_ = s.Save()
	}
	return count, nil
}

// decodeCwd reverses the CWD encoding used by Claude's project directories.
func decodeCwd(encoded string) string {
	parts := strings.Split(encoded, "-")
	if len(parts) < 2 {
		return "/" + encoded
	}
	var path string
	for i := 1; i < len(parts); i++ {
		candidate := path + "/" + parts[i]
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			path = candidate
		} else if path == "" {
			path = candidate
		} else {
			path = path + "-" + parts[i]
		}
	}
	if path == "" {
		return strings.ReplaceAll(encoded, "-", "/")
	}
	return path
}

// ExtractSessionText reads a JSONL session file and returns all user and
// assistant text content as lines suitable for grep.
func ExtractSessionText(uuid, cwd string) []string {
	path := sessionJSONLPath(uuid, cwd)
	if path == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 256*1024), 1024*1024)
	for scanner.Scan() {
		var rec struct {
			Type    string `json:"type"`
			Message struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal(scanner.Bytes(), &rec) != nil {
			continue
		}
		if rec.Type != "user" && rec.Type != "assistant" {
			continue
		}
		// User messages: content is a plain string.
		// Assistant messages: content is an array of blocks.
		var text string
		if rec.Message.Role == "user" {
			_ = json.Unmarshal(rec.Message.Content, &text)
		} else {
			var blocks []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}
			if json.Unmarshal(rec.Message.Content, &blocks) == nil {
				for _, b := range blocks {
					if b.Type == "text" {
						text += b.Text + "\n"
					}
				}
			}
		}
		for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
			if line != "" {
				lines = append(lines, line)
			}
		}
	}
	return lines
}

// sessionJSONLPath returns the path to a Claude session JSONL file, or ""
// if not found. Checks ~/.claude/projects/<encoded-cwd>/<uuid>.jsonl.
func sessionJSONLPath(uuid, cwd string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	encoded := strings.ReplaceAll(cwd, "/", "-")
	path := filepath.Join(home, ".claude", "projects", encoded, uuid+".jsonl")
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return ""
}

// extractTitle reads the first few lines of a JSONL file looking for a
// custom-title record.
func extractTitle(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for i := 0; i < 20 && scanner.Scan(); i++ {
		var rec struct {
			Type        string `json:"type"`
			CustomTitle string `json:"customTitle"`
		}
		if json.Unmarshal(scanner.Bytes(), &rec) == nil && rec.Type == "custom-title" && rec.CustomTitle != "" {
			return rec.CustomTitle
		}
	}
	return ""
}

