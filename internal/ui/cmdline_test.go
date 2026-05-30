package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func keyMsg(t tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: t}
}

func runeMsg(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func spaceMsg() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeySpace}
}

func TestCmdlineOpenClose(t *testing.T) {
	var c Cmdline
	if c.Active() {
		t.Error("should be inactive before Open")
	}
	c.Open()
	if !c.Active() {
		t.Error("should be active after Open")
	}
	c.Close()
	if c.Active() {
		t.Error("should be inactive after Close")
	}
}

func TestCmdlineTyping(t *testing.T) {
	var c Cmdline
	c.Open()
	c.HandleKey(runeMsg('n'))
	c.HandleKey(runeMsg('e'))
	c.HandleKey(runeMsg('w'))
	if c.Value() != "new" {
		t.Errorf("want 'new' got %q", c.Value())
	}
}

func TestCmdlineBackspace(t *testing.T) {
	var c Cmdline
	c.Open()
	c.HandleKey(runeMsg('a'))
	c.HandleKey(runeMsg('b'))
	c.HandleKey(keyMsg(tea.KeyBackspace))
	if c.Value() != "a" {
		t.Errorf("want 'a' after backspace got %q", c.Value())
	}
}

func TestCmdlineOpenResetsBuffer(t *testing.T) {
	var c Cmdline
	c.Open()
	c.HandleKey(runeMsg('x'))
	c.Open() // re-open clears
	if c.Value() != "" {
		t.Errorf("Open should reset buffer, got %q", c.Value())
	}
}

func TestCmdlineSpace(t *testing.T) {
	var c Cmdline
	c.Open()
	c.HandleKey(runeMsg('n'))
	c.HandleKey(spaceMsg())
	c.HandleKey(runeMsg('x'))
	if c.Value() != "n x" {
		t.Errorf("want 'n x' got %q", c.Value())
	}
}

func TestCmdlineCursorLeftRight(t *testing.T) {
	var c Cmdline
	c.Open()
	c.HandleKey(runeMsg('a'))
	c.HandleKey(runeMsg('b'))
	c.HandleKey(keyMsg(tea.KeyLeft)) // cursor before 'b'
	c.HandleKey(runeMsg('X'))        // insert X between a and b
	if c.Value() != "aXb" {
		t.Errorf("want 'aXb' got %q", c.Value())
	}
}

func TestCmdlineHomeEnd(t *testing.T) {
	var c Cmdline
	c.Open()
	for _, r := range "abc" {
		c.HandleKey(runeMsg(r))
	}
	c.HandleKey(keyMsg(tea.KeyHome))
	c.HandleKey(runeMsg('X'))
	if c.Value() != "Xabc" {
		t.Errorf("want 'Xabc' got %q", c.Value())
	}
	c.HandleKey(keyMsg(tea.KeyEnd))
	c.HandleKey(runeMsg('Z'))
	if c.Value() != "XabcZ" {
		t.Errorf("want 'XabcZ' got %q", c.Value())
	}
}

// TestCompletePathDirs checks that CompletePath returns entries for a known dir.
func TestCompletePathDirs(t *testing.T) {
	// /tmp always exists on Linux.
	matches := CompletePath("/tm")
	found := false
	for _, m := range matches {
		if m == "/tmp/" {
			found = true
		}
	}
	if !found {
		t.Errorf("CompletePath('/tm') should include '/tmp/', got %v", matches)
	}
}

func TestCompletePathEmpty(t *testing.T) {
	// An empty prefix lists the current directory — just verify no panic and
	// that entries come back (. always has contents).
	matches := CompletePath("")
	if len(matches) == 0 {
		t.Error("CompletePath('') should return entries for current dir")
	}
}

func TestCompletePathNoMatch(t *testing.T) {
	matches := CompletePath("/nonexistent_path_xyz_abc/")
	if len(matches) != 0 {
		t.Errorf("expected no matches for nonexistent path, got %v", matches)
	}
}

func TestLongestCommonPrefix(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{[]string{"abc", "abd", "abe"}, "ab"},
		{[]string{"foo"}, "foo"},
		{[]string{"a", "b"}, ""},
		{[]string{}, ""},
	}
	for _, c := range cases {
		got := longestCommonPrefix(c.in)
		if got != c.want {
			t.Errorf("longestCommonPrefix(%v) = %q want %q", c.in, got, c.want)
		}
	}
}

func TestTabCompleteNewArg(t *testing.T) {
	var c Cmdline
	c.Open()
	// Type "new /tm" then Tab — should expand to "new /tmp/"
	for _, r := range "new /tm" {
		c.HandleKey(runeMsg(r))
	}
	c.HandleKey(keyMsg(tea.KeyTab))
	if !strings.HasPrefix(c.Value(), "new /tmp") {
		t.Errorf("tab complete: want prefix 'new /tmp', got %q", c.Value())
	}
}
