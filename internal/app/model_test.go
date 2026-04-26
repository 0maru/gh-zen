package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestInit_ReturnsNilCmd(t *testing.T) {
	if cmd := (model{}).Init(); cmd != nil {
		t.Fatalf("expected Init to return nil cmd, got %T", cmd)
	}
}

func TestUpdate_QuitOnQuitKeys(t *testing.T) {
	cases := []struct {
		name string
		msg  tea.KeyMsg
	}{
		{"q", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}},
		{"ctrl+c", tea.KeyMsg{Type: tea.KeyCtrlC}},
		{"esc", tea.KeyMsg{Type: tea.KeyEsc}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, cmd := (model{}).Update(tc.msg)
			if cmd == nil {
				t.Fatalf("expected quit command, got nil")
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Fatalf("expected tea.QuitMsg from cmd, got %T", cmd())
			}
		})
	}
}

func TestUpdate_NonQuitKey_NoCommand(t *testing.T) {
	got, cmd := (model{}).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if cmd != nil {
		t.Fatalf("expected nil cmd for non-quit key, got %T", cmd)
	}
	if got != (model{}) {
		t.Fatalf("expected model unchanged, got %+v", got)
	}
}

func TestUpdate_WindowSizeMsg_StoresDimensions(t *testing.T) {
	got, cmd := (model{}).Update(tea.WindowSizeMsg{Width: 40, Height: 24})
	if cmd != nil {
		t.Fatalf("expected nil cmd for window size, got %T", cmd)
	}
	mm, ok := got.(model)
	if !ok {
		t.Fatalf("expected model, got %T", got)
	}
	if mm.width != 40 || mm.height != 24 {
		t.Fatalf("expected width=40 height=24, got width=%d height=%d", mm.width, mm.height)
	}
}

func TestUpdate_UnicodeRunes_NoQuit(t *testing.T) {
	cases := []struct {
		name  string
		runes []rune
	}{
		{"japanese", []rune("あ")},
		{"emoji", []rune("👨‍👩‍👧")},
		{"mixed", []rune("aあ")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, cmd := (model{}).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: tc.runes})
			if cmd != nil {
				t.Fatalf("expected nil cmd for non-quit unicode key, got %T (%v)", cmd, cmd())
			}
		})
	}
}
