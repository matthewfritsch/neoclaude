package mode

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func key(t tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: t}
}

func rune_(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func TestStartsInNormal(t *testing.T) {
	f := New()
	if f.Mode() != Normal {
		t.Fatalf("want Normal got %v", f.Mode())
	}
}

func TestInsertForwardsKeys(t *testing.T) {
	f := New()
	f.SetMode(Insert)
	action, m := f.HandleKey(rune_('a'))
	if action != ActionForward {
		t.Errorf("want ActionForward got %v", action)
	}
	if m != Insert {
		t.Errorf("want Insert got %v", m)
	}
}

func TestEscExitsInsertToNormal(t *testing.T) {
	f := New()
	action, m := f.HandleKey(key(tea.KeyEsc))
	if action != ActionNone {
		t.Errorf("Esc should be consumed (ActionNone), got %v", action)
	}
	if m != Normal {
		t.Errorf("want Normal after Esc, got %v", m)
	}
}

func TestNormalIEntersInsert(t *testing.T) {
	f := New()
	f.SetMode(Normal)
	action, m := f.HandleKey(rune_('i'))
	if action != ActionNone {
		t.Errorf("want ActionNone for i in Normal, got %v", action)
	}
	if m != Insert {
		t.Errorf("want Insert, got %v", m)
	}
}

func TestNormalAEntersInsert(t *testing.T) {
	f := New()
	f.SetMode(Normal)
	_, m := f.HandleKey(rune_('a'))
	if m != Insert {
		t.Errorf("want Insert after a, got %v", m)
	}
}

func TestNormalColonEntersCommand(t *testing.T) {
	f := New()
	f.SetMode(Normal)
	action, m := f.HandleKey(rune_(':'))
	if action != ActionEnterCommand {
		t.Errorf("want ActionEnterCommand, got %v", action)
	}
	if m != Command {
		t.Errorf("want Command, got %v", m)
	}
}

func TestNormalCtrlCQuits(t *testing.T) {
	f := New()
	f.SetMode(Normal)
	action, _ := f.HandleKey(key(tea.KeyCtrlC))
	if action != ActionQuit {
		t.Errorf("want ActionQuit in Normal+CtrlC, got %v", action)
	}
}

func TestCommandEscCancels(t *testing.T) {
	f := New()
	f.SetMode(Command)
	action, m := f.HandleKey(key(tea.KeyEsc))
	if action != ActionCancelCommand {
		t.Errorf("want ActionCancelCommand, got %v", action)
	}
	if m != Normal {
		t.Errorf("want Normal after Esc in Command, got %v", m)
	}
}

func TestCommandEnterExecs(t *testing.T) {
	f := New()
	f.SetMode(Command)
	action, m := f.HandleKey(key(tea.KeyEnter))
	if action != ActionExecCommand {
		t.Errorf("want ActionExecCommand, got %v", action)
	}
	if m != Normal {
		t.Errorf("want Normal after Enter in Command, got %v", m)
	}
}

func TestEscNeverForwardedFromInsert(t *testing.T) {
	f := New()
	for i := 0; i < 5; i++ {
		f.SetMode(Insert)
		action, m := f.HandleKey(key(tea.KeyEsc))
		if action == ActionForward {
			t.Errorf("iteration %d: Esc should never be forwarded", i)
		}
		if m != Normal {
			t.Errorf("iteration %d: want Normal, got %v", i, m)
		}
	}
}
