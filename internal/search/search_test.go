package search

import (
	"testing"

	"github.com/matthewfritsch/neoclaude/internal/buffer"
)

func TestSearchBufferBasic(t *testing.T) {
	lines := []string{"hello world", "foo bar", "world again"}
	hits := SearchBuffer(0, "buf", lines, "world")
	if len(hits) != 2 {
		t.Fatalf("want 2 hits got %v", hits)
	}
	if hits[0].Line != 0 || hits[1].Line != 2 {
		t.Errorf("unexpected line indices: %d %d", hits[0].Line, hits[1].Line)
	}
}

func TestSearchBufferEmpty(t *testing.T) {
	if hits := SearchBuffer(0, "b", []string{"x"}, ""); hits != nil {
		t.Error("empty query should return nil")
	}
}

func TestSearchBufferBadRegex(t *testing.T) {
	if hits := SearchBuffer(0, "b", []string{"x"}, "[bad"); hits != nil {
		t.Error("bad regex should return nil")
	}
}

func TestSearchBufferSpan(t *testing.T) {
	lines := []string{"abcfoobar"}
	hits := SearchBuffer(0, "b", lines, "foo")
	if len(hits) != 1 {
		t.Fatalf("want 1 hit got %d", len(hits))
	}
	h := hits[0]
	if h.Col != 3 || h.MatchEnd != 6 {
		t.Errorf("span: want Col=3 MatchEnd=6 got Col=%d MatchEnd=%d", h.Col, h.MatchEnd)
	}
	if h.Text != "abcfoobar" {
		t.Errorf("text: want 'abcfoobar' got %q", h.Text)
	}
}

func TestSearchBufferBufID(t *testing.T) {
	hits := SearchBuffer(buffer.ID(7), "mybuf", []string{"match"}, "match")
	if len(hits) != 1 {
		t.Fatalf("want 1 hit")
	}
	if hits[0].BufID != 7 || hits[0].BufName != "mybuf" {
		t.Errorf("bufid/name not propagated: %+v", hits[0])
	}
}

func TestAllMatches(t *testing.T) {
	spans := AllMatches("abcabcabc", "abc")
	if len(spans) != 3 {
		t.Fatalf("want 3 spans got %d", len(spans))
	}
	if spans[0] != [2]int{0, 3} || spans[1] != [2]int{3, 6} {
		t.Errorf("unexpected spans: %v", spans)
	}
}

func TestAllMatchesEmpty(t *testing.T) {
	if AllMatches("abc", "") != nil {
		t.Error("empty query should return nil")
	}
}

func TestAllMatchesBadRegex(t *testing.T) {
	if AllMatches("abc", "[bad") != nil {
		t.Error("bad regex should return nil")
	}
}
