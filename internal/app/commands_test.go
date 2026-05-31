package app

import (
	"testing"

	"github.com/matthewfritsch/neoclaude/internal/buffer"
	"github.com/matthewfritsch/neoclaude/internal/persist"
	"github.com/matthewfritsch/neoclaude/internal/registry"
	"github.com/matthewfritsch/neoclaude/internal/vt"
)

// modelForNames builds a minimal Model whose registry contains buffers with the
// given names. Used to test uniqueName and cmdRename without spawning sessions.
func modelForNames(names []string) *Model {
	reg := registry.New()
	for i, n := range names {
		id := buffer.ID(i)
		reg.Add(buffer.New(id, n, "/tmp", "", nil, vt.New(5, 5)))
	}
	store, _ := persist.Load()
	return &Model{reg: reg, store: store}
}

// TestUniqueNameNoConflict: a fresh name is returned unchanged.
func TestUniqueNameNoConflict(t *testing.T) {
	m := modelForNames([]string{"alpha", "beta"})
	if got := m.uniqueName("gamma"); got != "gamma" {
		t.Errorf("want 'gamma', got %q", got)
	}
}

// TestUniqueNameFirstConflict: existing "work" → "work~2".
func TestUniqueNameFirstConflict(t *testing.T) {
	m := modelForNames([]string{"work"})
	if got := m.uniqueName("work"); got != "work~2" {
		t.Errorf("want 'work~2', got %q", got)
	}
}

// TestUniqueNameMultipleConflicts: existing "foo", "foo~2", "foo~3" → "foo~4".
func TestUniqueNameMultipleConflicts(t *testing.T) {
	m := modelForNames([]string{"foo", "foo~2", "foo~3"})
	if got := m.uniqueName("foo"); got != "foo~4" {
		t.Errorf("want 'foo~4', got %q", got)
	}
}

// TestUniqueNameEmpty: empty registry — name passes through.
func TestUniqueNameEmpty(t *testing.T) {
	m := modelForNames(nil)
	if got := m.uniqueName("mydir"); got != "mydir" {
		t.Errorf("want 'mydir', got %q", got)
	}
}

// TestCmdRenameUpdatesLabel verifies :name updates the active buffer's Name field.
func TestCmdRenameUpdatesLabel(t *testing.T) {
	m := modelForNames([]string{"old"})
	m.cmdRename("new-label")
	b := m.reg.Active()
	if b == nil {
		t.Fatal("no active buffer")
	}
	if b.Name != "new-label" {
		t.Errorf("want 'new-label', got %q", b.Name)
	}
}

// TestCmdRenameEmpty: :name with empty string is a no-op.
func TestCmdRenameEmpty(t *testing.T) {
	m := modelForNames([]string{"keep"})
	m.cmdRename("")
	if got := m.reg.Active().Name; got != "keep" {
		t.Errorf("empty rename should be noop, got %q", got)
	}
}

// TestCmdRenameNoBuffer: no active buffer — should not panic.
func TestCmdRenameNoBuffer(t *testing.T) {
	m := modelForNames(nil)
	m.cmdRename("anything") // no active buffer, should be silent no-op
}

// TestCmdRenameWithSessionIDPersists verifies that a buffer with a SessionID
// gets its store record updated on rename.
func TestCmdRenameWithSessionIDPersists(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	reg := registry.New()
	id := buffer.ID(0)
	b := buffer.New(id, "orig", "/tmp", "test-uuid-1234", nil, vt.New(5, 5))
	reg.Add(b)
	store, _ := persist.Load()
	store.Upsert(persist.Record{UUID: "test-uuid-1234", Name: "orig", Cwd: "/tmp"})
	m := &Model{reg: reg, store: store}

	m.cmdRename("renamed")

	r := store.ByUUID("test-uuid-1234")
	if r == nil {
		t.Fatal("record not found after rename")
	}
	if r.Name != "renamed" {
		t.Errorf("store record name: want 'renamed', got %q", r.Name)
	}
}

// TestDispatchNameCommand verifies :name dispatches to cmdRename.
func TestDispatchNameCommand(t *testing.T) {
	m := modelForNames([]string{"buf"})
	m.dispatch("name shiny")
	if got := m.reg.Active().Name; got != "shiny" {
		t.Errorf(":name shiny should rename buffer, got %q", got)
	}
}
