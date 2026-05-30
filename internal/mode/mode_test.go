package mode

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func key(t tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: t}
}

func rune_(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

var t0 = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

func TestStartsInInsert(t *testing.T) {
	f := New()
	if f.Mode() != Insert {
		t.Fatalf("want Insert got %v", f.Mode())
	}
}

func TestInsertForwardsKeys(t *testing.T) {
	f := New()
	action, m := f.HandleKey(rune_('a'), t0)
	if action != ActionForward {
		t.Errorf("want ActionForward got %v", action)
	}
	if m != Insert {
		t.Errorf("want Insert got %v", m)
	}
}

func TestSingleEscForwardedInInsert(t *testing.T) {
	f := New()
	action, m := f.HandleKey(key(tea.KeyEsc), t0)
	if action != ActionForward {
		t.Errorf("single Esc should be forwarded, got action=%v", action)
	}
	if m != Insert {
		t.Errorf("mode should still be Insert after single Esc, got %v", m)
	}
}

func TestDoubleEscWithinDelayEntersNormal(t *testing.T) {
	f := New()
	f.HandleKey(key(tea.KeyEsc), t0)
	action, m := f.HandleKey(key(tea.KeyEsc), t0.Add(100*time.Millisecond))
	if action != ActionNone {
		t.Errorf("second Esc should be swallowed (ActionNone), got %v", action)
	}
	if m != Normal {
		t.Errorf("want Normal after double-Esc, got %v", m)
	}
}

func TestDoubleEscOutsideDelayStaysInsert(t *testing.T) {
	f := New()
	f.HandleKey(key(tea.KeyEsc), t0)
	action, m := f.HandleKey(key(tea.KeyEsc), t0.Add(400*time.Millisecond))
	if action != ActionForward {
		t.Errorf("expired second Esc should be forwarded, got %v", action)
	}
	if m != Insert {
		t.Errorf("want Insert for expired double-Esc, got %v", m)
	}
}

func TestNonEscClearsPending(t *testing.T) {
	f := New()
	f.HandleKey(key(tea.KeyEsc), t0)
	f.HandleKey(rune_('x'), t0.Add(50*time.Millisecond))
	action, m := f.HandleKey(key(tea.KeyEsc), t0.Add(100*time.Millisecond))
	if action != ActionForward || m != Insert {
		t.Errorf("want forward+Insert after cleared pending, got action=%v mode=%v", action, m)
	}
}

func TestNormalIEntersInsert(t *testing.T) {
	f := New()
	f.SetMode(Normal)
	action, m := f.HandleKey(rune_('i'), t0)
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
	_, m := f.HandleKey(rune_('a'), t0)
	if m != Insert {
		t.Errorf("want Insert after a, got %v", m)
	}
}

func TestNormalColonEntersCommand(t *testing.T) {
	f := New()
	f.SetMode(Normal)
	action, m := f.HandleKey(rune_(':'), t0)
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
	action, _ := f.HandleKey(key(tea.KeyCtrlC), t0)
	if action != ActionQuit {
		t.Errorf("want ActionQuit in Normal+CtrlC, got %v", action)
	}
}

func TestCommandEscCancels(t *testing.T) {
	f := New()
	f.SetMode(Command)
	action, m := f.HandleKey(key(tea.KeyEsc), t0)
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
	action, m := f.HandleKey(key(tea.KeyEnter), t0)
	if action != ActionExecCommand {
		t.Errorf("want ActionExecCommand, got %v", action)
	}
	if m != Normal {
		t.Errorf("want Normal after Enter in Command, got %v", m)
	}
}

func TestDoubleEscBoundaryExact(t *testing.T) {
	f := New()
	f.HandleKey(key(tea.KeyEsc), t0)
	action, m := f.HandleKey(key(tea.KeyEsc), t0.Add(EscDelay))
	if action != ActionNone || m != Normal {
		t.Errorf("exact boundary: want Normal+swallow, got action=%v mode=%v", action, m)
	}
}
