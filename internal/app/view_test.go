package app

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

var update = flag.Bool("update", false, "update golden files")

// requireEqualGolden compares got to testdata/<TestName>.golden.
// Run `go test ./... -update` to regenerate the file after intentional UI changes.
//
// Stand-in for github.com/charmbracelet/x/exp/golden until network access
// is available to vendor it; the contract is the same.
func requireEqualGolden(t *testing.T, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", t.Name()+".golden")
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run with -update to create): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("output mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestView_Initial(t *testing.T) {
	requireEqualGolden(t, []byte(newModel().View()))
}

func TestView_Compact_HidesRepositoryPane(t *testing.T) {
	m := newModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 50, Height: 20})
	got := updated.(model).View()
	if bytes.Contains([]byte(got), []byte("Repositories")) {
		t.Fatalf("expected compact view to hide repository pane, got:\n%s", got)
	}
	if !bytes.Contains([]byte(got), []byte("Work Items")) || !bytes.Contains([]byte(got), []byte("Preview")) {
		t.Fatalf("expected compact view to keep list and preview, got:\n%s", got)
	}
}

func TestView_SelectedItemChangesPreview(t *testing.T) {
	m := newModel()
	initial := m.View()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	next := updated.(model).View()

	if initial == next {
		t.Fatalf("expected preview to change after moving selection")
	}
	if !bytes.Contains([]byte(next), []byte("feat/config-loader")) {
		t.Fatalf("expected moved preview to include selected branch, got:\n%s", next)
	}
	if !bytes.Contains([]byte(next), []byte("PR #24 open")) {
		t.Fatalf("expected moved preview to include linked PR, got:\n%s", next)
	}
	if !bytes.Contains([]byte(next), []byte("Review: review requested")) {
		t.Fatalf("expected moved preview to include review state, got:\n%s", next)
	}
}
