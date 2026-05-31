package vt

import "testing"

func TestExtractLinesBasic(t *testing.T) {
	v := New(10, 3)
	v.Write([]byte("hello"))
	g := v.Snapshot()
	lines := ExtractLines(g)
	if len(lines) != 3 {
		t.Fatalf("want 3 lines got %d", len(lines))
	}
	if lines[0] != "hello" {
		t.Errorf("line0: want 'hello' got %q", lines[0])
	}
	// Rows 1 and 2 should be empty (trimmed trailing spaces).
	if lines[1] != "" {
		t.Errorf("line1: want '' got %q", lines[1])
	}
}

func TestExtractLinesTrimsTrailingSpace(t *testing.T) {
	v := New(10, 1)
	v.Write([]byte("ab"))
	g := v.Snapshot()
	lines := ExtractLines(g)
	// "ab" followed by 8 spaces → trimmed to "ab"
	if lines[0] != "ab" {
		t.Errorf("want 'ab' got %q", lines[0])
	}
}

func TestExtractLinesMultiRow(t *testing.T) {
	v := New(5, 2)
	v.Write([]byte("foo\r\nbar"))
	g := v.Snapshot()
	lines := ExtractLines(g)
	if lines[0] != "foo" {
		t.Errorf("row0: want 'foo' got %q", lines[0])
	}
	if lines[1] != "bar" {
		t.Errorf("row1: want 'bar' got %q", lines[1])
	}
}
