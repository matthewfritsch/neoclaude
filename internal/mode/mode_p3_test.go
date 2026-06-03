package mode

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func rune3(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func TestLeaderSNOpensNamedSessions(t *testing.T) {
	f := NewWithLeader(' ')
	f.SetMode(Normal)
	f.HandleKey(rune3(' '))
	f.HandleKey(rune3('s'))
	action, m := f.HandleKey(rune3('n'))
	if action != ActionOpenNamedSessions {
		t.Errorf("leader+s+n: want ActionOpenNamedSessions, got %v", action)
	}
	if m != Normal {
		t.Errorf("leader+s+n: want Normal mode after open, got %v", m)
	}
}

func TestLeaderSGStillWorksP3(t *testing.T) {
	f := NewWithLeader(' ')
	f.SetMode(Normal)
	f.HandleKey(rune3(' '))
	f.HandleKey(rune3('s'))
	action, _ := f.HandleKey(rune3('g'))
	if action != ActionOpenGrep {
		t.Errorf("leader+s+g: want ActionOpenGrep, got %v", action)
	}
}

func TestLeaderSPartialStillPendingP3(t *testing.T) {
	f := NewWithLeader(' ')
	f.SetMode(Normal)
	f.HandleKey(rune3(' '))
	action, m := f.HandleKey(rune3('s'))
	if action != ActionNone || m != Normal {
		t.Errorf("partial 's': want ActionNone/Normal, got %v/%v", action, m)
	}
}

func TestLeaderSXUnrecognised(t *testing.T) {
	f := NewWithLeader(' ')
	f.SetMode(Normal)
	f.HandleKey(rune3(' '))
	f.HandleKey(rune3('s'))
	action, m := f.HandleKey(rune3('z'))
	if action != ActionNone || m != Normal {
		t.Errorf("unrecognised sz: want ActionNone/Normal, got %v/%v", action, m)
	}
	action2, m2 := f.HandleKey(rune3('i'))
	if action2 != ActionNone || m2 != Insert {
		t.Errorf("post-cancel i: want ActionNone/Insert, got %v/%v", action2, m2)
	}
}
