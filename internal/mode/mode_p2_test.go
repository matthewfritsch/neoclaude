package mode

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func rune2(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

// --- Mode.String() ---

func TestModeString(t *testing.T) {
	cases := []struct {
		m    Mode
		want string
	}{
		{Normal, "NORMAL"},
		{Insert, "INSERT"},
		{Command, "COMMAND"},
		{Search, "SEARCH"},
		{Visual, "VISUAL"},
		{Picker, "PICKER"},
		{Mode(99), "NORMAL"}, // unknown → default
	}
	for _, tc := range cases {
		if got := tc.m.String(); got != tc.want {
			t.Errorf("Mode(%d).String() = %q want %q", tc.m, got, tc.want)
		}
	}
}

// --- Leader chord ---

func TestLeaderSpaceDefaultOpensPicker(t *testing.T) {
	f := NewWithLeader(' ')
	f.SetMode(Normal)
	action, m := f.HandleKey(rune2(' '))
	if action != ActionNone || m != Normal {
		t.Errorf("first leader: want ActionNone/Normal, got %v/%v", action, m)
	}
	action, m = f.HandleKey(rune2(' '))
	if action != ActionOpenPicker {
		t.Errorf("leader+leader: want ActionOpenPicker, got %v", action)
	}
	if m != Picker {
		t.Errorf("leader+leader: want Picker mode, got %v", m)
	}
}

func TestLeaderSGOpensGrep(t *testing.T) {
	f := NewWithLeader(' ')
	f.SetMode(Normal)
	f.HandleKey(rune2(' '))
	f.HandleKey(rune2('s'))
	action, m := f.HandleKey(rune2('g'))
	if action != ActionOpenGrep {
		t.Errorf("leader+s+g: want ActionOpenGrep, got %v", action)
	}
	if m != Normal {
		t.Errorf("leader+s+g: want Normal mode after grep open, got %v", m)
	}
}

func TestLeaderUnrecognisedCancels(t *testing.T) {
	f := NewWithLeader(' ')
	f.SetMode(Normal)
	f.HandleKey(rune2(' '))
	action, m := f.HandleKey(rune2('x'))
	if action != ActionNone || m != Normal {
		t.Errorf("unrecognised chord: want ActionNone/Normal, got %v/%v", action, m)
	}
	action2, m2 := f.HandleKey(rune2('i'))
	if action2 != ActionNone || m2 != Insert {
		t.Errorf("post-cancel i: want ActionNone/Insert, got %v/%v", action2, m2)
	}
}

func TestLeaderCommaConfigured(t *testing.T) {
	f := NewWithLeader(',')
	f.SetMode(Normal)
	f.HandleKey(rune2(','))
	action, m := f.HandleKey(rune2(','))
	if action != ActionOpenPicker || m != Picker {
		t.Errorf("comma leader: want ActionOpenPicker/Picker, got %v/%v", action, m)
	}
}

func TestLeaderPartialSNotFiredPrematurely(t *testing.T) {
	f := NewWithLeader(' ')
	f.SetMode(Normal)
	f.HandleKey(rune2(' '))
	action, m := f.HandleKey(rune2('s'))
	if action != ActionNone || m != Normal {
		t.Errorf("partial 's': want ActionNone/Normal, got %v/%v", action, m)
	}
}

// --- Normal / in Search ---

func TestNormalSlashOpensSearch(t *testing.T) {
	f := New()
	f.SetMode(Normal)
	action, m := f.HandleKey(rune2('/'))
	if action != ActionOpenSearch {
		t.Errorf("want ActionOpenSearch, got %v", action)
	}
	if m != Search {
		t.Errorf("want Search mode, got %v", m)
	}
}

func TestSearchEscExitsToNormal(t *testing.T) {
	f := New()
	f.SetMode(Search)
	action, m := f.HandleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if action != ActionExitOverlay || m != Normal {
		t.Errorf("search Esc: want ActionExitOverlay/Normal, got %v/%v", action, m)
	}
}

func TestSearchNNext(t *testing.T) {
	f := New()
	f.SetMode(Search)
	action, _ := f.HandleKey(rune2('n'))
	if action != ActionSearchNext {
		t.Errorf("n in Search: want ActionSearchNext, got %v", action)
	}
	action, _ = f.HandleKey(rune2('N'))
	if action != ActionSearchPrev {
		t.Errorf("N in Search: want ActionSearchPrev, got %v", action)
	}
}

// --- Visual ---

func TestNormalVEntersVisual(t *testing.T) {
	f := New()
	f.SetMode(Normal)
	action, m := f.HandleKey(rune2('v'))
	if action != ActionEnterVisual || m != Visual {
		t.Errorf("v in Normal: want ActionEnterVisual/Visual, got %v/%v", action, m)
	}
}

func TestVisualEscExits(t *testing.T) {
	f := New()
	f.SetMode(Visual)
	action, m := f.HandleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if action != ActionExitOverlay || m != Normal {
		t.Errorf("Esc in Visual: want ActionExitOverlay/Normal, got %v/%v", action, m)
	}
}

func TestVisualYYanks(t *testing.T) {
	f := New()
	f.SetMode(Visual)
	action, m := f.HandleKey(rune2('y'))
	if action != ActionVisualYank || m != Normal {
		t.Errorf("y in Visual: want ActionVisualYank/Normal, got %v/%v", action, m)
	}
}

// --- Picker ---

func TestPickerEscExits(t *testing.T) {
	f := New()
	f.SetMode(Picker)
	action, m := f.HandleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if action != ActionExitOverlay || m != Normal {
		t.Errorf("Esc in Picker: want ActionExitOverlay/Normal, got %v/%v", action, m)
	}
}

func TestPickerEnterConfirms(t *testing.T) {
	f := New()
	f.SetMode(Picker)
	action, m := f.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if action != ActionExecCommand || m != Normal {
		t.Errorf("Enter in Picker: want ActionExecCommand/Normal, got %v/%v", action, m)
	}
}

// --- SetLeader ---

func TestSetLeader(t *testing.T) {
	f := New()
	f.SetMode(Normal)
	f.SetLeader(',')
	f.HandleKey(rune2(','))
	action, m := f.HandleKey(rune2(','))
	if action != ActionOpenPicker || m != Picker {
		t.Errorf("SetLeader comma: want ActionOpenPicker/Picker, got %v/%v", action, m)
	}
}
