package app

import (
	"bytes"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
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
	if !bytes.Contains([]byte(got), []byte("WorkItems[1]")) || !bytes.Contains([]byte(got), []byte("Review[2]")) {
		t.Fatalf("expected compact view to keep list and review panes, got:\n%s", got)
	}
}

func TestView_KeymapFollowsFocusedPane(t *testing.T) {
	m := newModel()
	initial := m.View()
	if !bytes.Contains([]byte(initial), []byte("Work Items keys: j/k move")) {
		t.Fatalf("expected initial header to show work item keys, got:\n%s", initial)
	}
	if got := strings.Split(initial, "\n")[1]; !strings.HasPrefix(got, "Work Items keys:") {
		t.Fatalf("expected keymap directly under title, got line %q", got)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	next := updated.(model).View()
	if !bytes.Contains([]byte(next), []byte("Review keys: [1]/[2]/[3] pane  tab/S-tab pane  q quit")) {
		t.Fatalf("expected focused review header, got:\n%s", next)
	}
	if got := strings.Split(next, "\n")[1]; !strings.HasPrefix(got, "Review keys:") {
		t.Fatalf("expected review keymap directly under title, got line %q", got)
	}
	if bytes.Contains([]byte(next), []byte("j/k move")) {
		t.Fatalf("expected preview keymap to omit movement keys, got:\n%s", next)
	}
}

func TestView_KeymapStylesOnlyKeys(t *testing.T) {
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(termenv.TrueColor)

	m := newModel()
	m.styles.Key = m.styles.Key.Renderer(r)
	got := m.keymapLine(defaultWidth)
	want := "Work Items keys: j/k move  g/G jump  [1]/[2]/[3] pane  tab/S-tab pane  q quit"

	if stripped := ansi.Strip(got); stripped != want {
		t.Fatalf("expected keymap text to stay unchanged, got %q", stripped)
	}
	if !strings.Contains(got, m.styles.Key.Render("j/k")+" move") {
		t.Fatalf("expected j/k key to be styled without styling description, got %q", got)
	}
	if strings.Contains(got, m.styles.Key.Render("move")) || strings.Contains(got, m.styles.Key.Render("jump")) {
		t.Fatalf("expected key descriptions to stay unstyled, got %q", got)
	}
}

func TestView_FullLayoutSeparatesPanes(t *testing.T) {
	got := newModel().View()
	lines := strings.Split(got, "\n")
	if len(lines) < 4 {
		t.Fatalf("expected full layout output, got:\n%s", got)
	}
	top := ansi.Strip(lines[2])
	if strings.Count(top, frameTopLeftGlyph) != 3 || strings.Count(top, frameTopRightGlyph) != 3 {
		t.Fatalf("expected three independent pane boxes, got top line %q", top)
	}
	if !strings.Contains(top, frameTopRightGlyph+" "+frameTopLeftGlyph) {
		t.Fatalf("expected visible gap between pane boxes, got top line %q", top)
	}
	for _, title := range []string{"Repositories[1]", "WorkItems[2]", "Review[3]"} {
		if !strings.Contains(top, title) {
			t.Fatalf("expected top border to include pane title %q, got %q", title, top)
		}
	}
	body := ansi.Strip(lines[3])
	if strings.Count(body, paneBorderGlyph) != 6 {
		t.Fatalf("expected each pane row to have left and right borders, got %q", body)
	}
}

func TestView_FrameFillsWindowHeight(t *testing.T) {
	m := newModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	got := updated.(model).View()
	lines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")

	if len(lines) != 24 {
		t.Fatalf("expected frame to fill window height, got %d lines:\n%s", len(lines), got)
	}
	top := ansi.Strip(lines[2])
	if !strings.HasPrefix(top, frameTopLeftGlyph) || !strings.HasSuffix(top, frameTopRightGlyph) {
		t.Fatalf("expected top frame border on line 3, got %q", lines[2])
	}
	last := ansi.Strip(lines[len(lines)-1])
	if !strings.HasPrefix(last, frameBottomLeftGlyph) || !strings.HasSuffix(last, frameBottomRightGlyph) {
		t.Fatalf("expected bottom frame border on last line, got %q", lines[len(lines)-1])
	}
	for i, line := range lines[2:] {
		if width := lipgloss.Width(line); width != 100 {
			t.Fatalf("expected frame line %d to use full width 100, got %d for %q", i+2, width, line)
		}
	}
}

func TestView_CompactNormalizesHiddenRepositoryFocus(t *testing.T) {
	m := newModel()
	m.width = 50
	m.focusedPane = paneRepositories
	got := m.View()

	if !bytes.Contains([]byte(got), []byte("Work Items keys:")) {
		t.Fatalf("expected compact view to focus visible work items pane, got:\n%s", got)
	}
	if bytes.Contains([]byte(got), []byte("Repositories keys:")) {
		t.Fatalf("expected compact view to hide repository keymap, got:\n%s", got)
	}
}

func TestView_CompactKeymapShowsVisiblePaneNumbers(t *testing.T) {
	m := newModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 50, Height: 20})
	got := updated.(model).View()

	if !bytes.Contains([]byte(got), []byte("[1]/[2] pane")) {
		t.Fatalf("expected compact keymap to show visible pane numbers, got:\n%s", got)
	}
	if bytes.Contains([]byte(got), []byte("[1]/[2]/[3] pane")) {
		t.Fatalf("expected compact keymap to omit hidden repository number, got:\n%s", got)
	}
}

func TestView_SelectedItemChangesPreview(t *testing.T) {
	m := newModelWithPreviewLoader(fakeDelayedPreviewLoader(0))
	loaded, _ := m.Update(requirePreviewResultMsg(t, m.Init()))
	m = loaded.(model)
	initial := m.View()

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	loading := updated.(model).View()
	if !bytes.Contains([]byte(loading), []byte("Loading preview...")) {
		t.Fatalf("expected moved preview to show loading state, got:\n%s", loading)
	}

	loaded, _ = updated.(model).Update(requirePreviewResultMsg(t, cmd))
	next := loaded.(model).View()

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
