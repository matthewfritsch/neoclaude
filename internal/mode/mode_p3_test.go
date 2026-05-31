package mode

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

var tp3 = time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)

func rune3(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

// TestLeaderSNOpensNamedSessions verifies <leader>sn emits ActionOpenNamedSessions.
func TestLeaderSNOpensNamedSessions(t *testing.T) {
	f := NewWithLeader(' ')
	f.SetMode(Normal)
	f.HandleKey(rune3(' '), tp3) // arm leader
	f.HandleKey(rune3('s'), tp3) // partial "s"
	action, m := f.HandleKey(rune3('n'), tp3)
	if action != ActionOpenNamedSessions {
		t.Errorf("leader+s+n: want ActionOpenNamedSessions, got %v", action)
	}
	if m != Normal {
		t.Errorf("leader+s+n: want Normal mode after open, got %v", m)
	}
}

// TestLeaderSGStillWorks verifies the existing <leader>sg is unchanged.
func TestLeaderSGStillWorksP3(t *testing.T) {
	f := NewWithLeader(' ')
	f.SetMode(Normal)
	f.HandleKey(rune3(' '), tp3)
	f.HandleKey(rune3('s'), tp3)
	action, _ := f.HandleKey(rune3('g'), tp3)
	if action != ActionOpenGrep {
		t.Errorf("leader+s+g: want ActionOpenGrep, got %v", action)
	}
}

// TestLeaderSPartialStillPending verifies "s" alone stays pending.
func TestLeaderSPartialStillPendingP3(t *testing.T) {
	f := NewWithLeader(' ')
	f.SetMode(Normal)
	f.HandleKey(rune3(' '), tp3)
	action, m := f.HandleKey(rune3('s'), tp3)
	if action != ActionNone || m != Normal {
		t.Errorf("partial 's': want ActionNone/Normal, got %v/%v", action, m)
	}
}

// TestLeaderSXUnrecognised verifies an unknown two-char sequence after "s" cancels.
func TestLeaderSXUnrecognised(t *testing.T) {
	f := NewWithLeader(' ')
	f.SetMode(Normal)
	f.HandleKey(rune3(' '), tp3)
	f.HandleKey(rune3('s'), tp3)
	action, m := f.HandleKey(rune3('z'), tp3) // "sz" is not a valid chord
	if action != ActionNone || m != Normal {
		t.Errorf("unrecognised sz: want ActionNone/Normal, got %v/%v", action, m)
	}
	// After cancel, leader is cleared — pressing 'i' should enter Insert normally.
	action2, m2 := f.HandleKey(rune3('i'), tp3)
	if action2 != ActionNone || m2 != Insert {
		t.Errorf("post-cancel i: want ActionNone/Insert, got %v/%v", action2, m2)
	}
}
