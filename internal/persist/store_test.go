package persist

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// setXDGState overrides XDG_STATE_HOME for a test and returns the override dir.
func setXDGState(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	return dir
}

func TestMissingFileReturnsEmpty(t *testing.T) {
	setXDGState(t)
	s, err := Load()
	if err != nil {
		t.Fatalf("missing file should not error, got: %v", err)
	}
	if len(s.Records) != 0 {
		t.Errorf("want 0 records, got %d", len(s.Records))
	}
}

func TestRoundTrip(t *testing.T) {
	setXDGState(t)
	s, _ := Load()
	s.Upsert(Record{UUID: "aaa", Name: "work", Cwd: "/home/user/work"})
	s.Upsert(Record{UUID: "bbb", Name: "tmp", Cwd: "/tmp"})
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	s2, err := Load()
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	if len(s2.Records) != 2 {
		t.Fatalf("want 2 records, got %d", len(s2.Records))
	}
	if s2.Records[0].UUID != "aaa" || s2.Records[0].Name != "work" {
		t.Errorf("first record mismatch: %+v", s2.Records[0])
	}
	if s2.Records[1].UUID != "bbb" || s2.Records[1].Cwd != "/tmp" {
		t.Errorf("second record mismatch: %+v", s2.Records[1])
	}
}

func TestUpsertUpdatesExisting(t *testing.T) {
	setXDGState(t)
	s, _ := Load()
	s.Upsert(Record{UUID: "aaa", Name: "old", Cwd: "/old"})
	s.Upsert(Record{UUID: "aaa", Name: "new", Cwd: "/new"})
	if len(s.Records) != 1 {
		t.Errorf("upsert should not duplicate, got %d records", len(s.Records))
	}
	if s.Records[0].Name != "new" || s.Records[0].Cwd != "/new" {
		t.Errorf("upsert did not update: %+v", s.Records[0])
	}
}

func TestUpsertSetsLastSeen(t *testing.T) {
	setXDGState(t)
	before := time.Now().Add(-time.Second)
	s, _ := Load()
	s.Upsert(Record{UUID: "xxx"})
	if s.Records[0].LastSeen.Before(before) {
		t.Error("LastSeen should be set to approximately now")
	}
}

func TestDelete(t *testing.T) {
	setXDGState(t)
	s, _ := Load()
	s.Upsert(Record{UUID: "aaa"})
	s.Upsert(Record{UUID: "bbb"})
	s.Delete("aaa")
	if len(s.Records) != 1 {
		t.Fatalf("want 1 record after delete, got %d", len(s.Records))
	}
	if s.Records[0].UUID != "bbb" {
		t.Errorf("wrong record remains: %+v", s.Records[0])
	}
}

func TestDeleteNoop(t *testing.T) {
	setXDGState(t)
	s, _ := Load()
	s.Upsert(Record{UUID: "aaa"})
	s.Delete("nonexistent") // should not panic or remove anything
	if len(s.Records) != 1 {
		t.Errorf("delete of missing uuid should be noop")
	}
}

func TestClosed(t *testing.T) {
	setXDGState(t)
	s, _ := Load()
	s.Upsert(Record{UUID: "open1", Name: "a"})
	s.Upsert(Record{UUID: "open2", Name: "b"})
	s.Upsert(Record{UUID: "closed1", Name: "c"})
	s.Upsert(Record{UUID: "closed2", Name: "d"})

	open := map[string]bool{"open1": true, "open2": true}
	closed := s.Closed(open)
	if len(closed) != 2 {
		t.Fatalf("want 2 closed, got %d", len(closed))
	}
	uuids := map[string]bool{closed[0].UUID: true, closed[1].UUID: true}
	if !uuids["closed1"] || !uuids["closed2"] {
		t.Errorf("wrong closed records: %v", closed)
	}
}

func TestClosedAllOpen(t *testing.T) {
	setXDGState(t)
	s, _ := Load()
	s.Upsert(Record{UUID: "a"})
	closed := s.Closed(map[string]bool{"a": true})
	if len(closed) != 0 {
		t.Errorf("want 0 closed when all are open")
	}
}

func TestXDGPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	p := storePath()
	want := filepath.Join(dir, "neoclaude", "sessions.json")
	if p != want {
		t.Errorf("path: got %q want %q", p, want)
	}
}

func TestAtomicWrite(t *testing.T) {
	setXDGState(t)
	s, _ := Load()
	s.Upsert(Record{UUID: "atomic"})
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	// Temp file must not be left behind.
	tmp := s.path + ".tmp"
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Error("temp file should not exist after successful Save")
	}
	// The real file must exist.
	if _, err := os.Stat(s.path); err != nil {
		t.Errorf("sessions.json should exist after Save: %v", err)
	}
}

func TestByUUID(t *testing.T) {
	setXDGState(t)
	s, _ := Load()
	s.Upsert(Record{UUID: "find-me", Name: "hello"})
	r := s.ByUUID("find-me")
	if r == nil {
		t.Fatal("ByUUID returned nil")
	}
	if r.Name != "hello" {
		t.Errorf("name: got %q want hello", r.Name)
	}
	if s.ByUUID("missing") != nil {
		t.Error("ByUUID for missing uuid should return nil")
	}
}

func TestSaveCreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "deep", "nested"))
	s, _ := Load()
	s.Upsert(Record{UUID: "dir-test"})
	if err := s.Save(); err != nil {
		t.Fatalf("Save should create dirs: %v", err)
	}
}
