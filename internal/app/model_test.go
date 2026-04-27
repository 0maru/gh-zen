package app

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/0maru/gh-zen/internal/workbench"
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
	start := newModel()
	got, cmd := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if cmd != nil {
		t.Fatalf("expected nil cmd for non-quit key, got %T", cmd)
	}
	if !reflect.DeepEqual(got, start) {
		t.Fatalf("expected model unchanged, got %+v", got)
	}
}

func TestNew_LoadsFakeWorkbenchData(t *testing.T) {
	got, ok := New().(model)
	if !ok {
		t.Fatalf("expected model from New")
	}
	if len(got.repos) == 0 {
		t.Fatalf("expected fake repositories")
	}
	if len(got.workItems) == 0 {
		t.Fatalf("expected fake work items")
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

func TestUpdate_MoveSelection(t *testing.T) {
	start := newModel()
	if start.selectedItem != 0 {
		t.Fatalf("expected initial selection at 0, got %d", start.selectedItem)
	}

	got, cmd := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if cmd != nil {
		t.Fatalf("expected nil cmd for movement, got %T", cmd)
	}
	mm := got.(model)
	if mm.selectedItem != 1 {
		t.Fatalf("expected j to move selection to 1, got %d", mm.selectedItem)
	}

	got, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	mm = got.(model)
	if mm.selectedItem != 0 {
		t.Fatalf("expected k to move selection back to 0, got %d", mm.selectedItem)
	}

	got, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	mm = got.(model)
	if mm.selectedItem != len(mm.workItems)-1 {
		t.Fatalf("expected G to move selection to end, got %d", mm.selectedItem)
	}

	got, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	mm = got.(model)
	if mm.selectedItem != 0 {
		t.Fatalf("expected g to move selection to start, got %d", mm.selectedItem)
	}
}

func TestUpdate_MoveSelection_ClampsAtEdges(t *testing.T) {
	start := newModel()
	got, _ := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	mm := got.(model)
	if mm.selectedItem != 0 {
		t.Fatalf("expected selection to stay at start, got %d", mm.selectedItem)
	}

	mm.selectedItem = len(mm.workItems) - 1
	got, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	mm = got.(model)
	if mm.selectedItem != len(mm.workItems)-1 {
		t.Fatalf("expected selection to stay at end, got %d", mm.selectedItem)
	}
}

func TestUpdate_TabChangesFocusedPane(t *testing.T) {
	start := newModel()
	got, cmd := start.Update(tea.KeyMsg{Type: tea.KeyTab})
	if cmd != nil {
		t.Fatalf("expected nil cmd for tab, got %T", cmd)
	}
	mm := got.(model)
	if mm.focusedPane != panePreview {
		t.Fatalf("expected tab to focus preview pane, got %v", mm.focusedPane)
	}

	got, _ = mm.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	mm = got.(model)
	if mm.focusedPane != paneWorkItems {
		t.Fatalf("expected shift+tab to focus work items pane, got %v", mm.focusedPane)
	}

	got, _ = mm.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	mm = got.(model)
	if mm.focusedPane != paneRepositories {
		t.Fatalf("expected shift+tab to focus repositories pane, got %v", mm.focusedPane)
	}
}

func TestUpdate_NumberKeysFocusVisiblePanes(t *testing.T) {
	start := newModel()

	got, cmd := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	if cmd != nil {
		t.Fatalf("expected nil cmd for pane number key, got %T", cmd)
	}
	mm := got.(model)
	if mm.focusedPane != paneRepositories {
		t.Fatalf("expected 1 to focus repositories pane, got %v", mm.focusedPane)
	}

	got, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	mm = got.(model)
	if mm.focusedPane != paneWorkItems {
		t.Fatalf("expected 2 to focus work items pane, got %v", mm.focusedPane)
	}

	got, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	mm = got.(model)
	if mm.focusedPane != panePreview {
		t.Fatalf("expected 3 to focus preview pane, got %v", mm.focusedPane)
	}
}

func TestUpdate_NumberKeysFollowCompactVisiblePanes(t *testing.T) {
	start := newModel()
	start.width = 50

	got, _ := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	mm := got.(model)
	if mm.focusedPane != panePreview {
		t.Fatalf("expected compact 2 to focus preview pane, got %v", mm.focusedPane)
	}

	got, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	mm = got.(model)
	if mm.focusedPane != panePreview {
		t.Fatalf("expected compact 3 to leave focus unchanged, got %v", mm.focusedPane)
	}
}

func TestUpdate_MovementTargetsFocusedPane(t *testing.T) {
	start := newModel()
	got, _ := start.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	mm := got.(model)

	got, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	mm = got.(model)
	if mm.selectedRepo != 1 {
		t.Fatalf("expected j to move repository selection when repo pane is focused, got %d", mm.selectedRepo)
	}
	if mm.selectedItem != 0 {
		t.Fatalf("expected work item selection to stay unchanged, got %d", mm.selectedItem)
	}
}

func TestUpdate_RepositoryViewsFilterWorkItems(t *testing.T) {
	start := newModel()
	start.focusedPane = paneRepositories
	start.selectedItem = 2

	got, _ := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	mm := got.(model)
	if mm.viewSelected {
		t.Fatalf("expected first repository movement to select another repo, got view selected")
	}
	if items := mm.visibleWorkItems(); len(items) != 0 {
		t.Fatalf("expected dotfiles repo to have no fake work items, got %d", len(items))
	}

	got, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	mm = got.(model)
	if !mm.viewSelected || mm.selectedView != int(repoViewActiveWorktrees) {
		t.Fatalf("expected active worktrees view, got viewSelected=%v selectedView=%d", mm.viewSelected, mm.selectedView)
	}
	if mm.selectedItem != 0 {
		t.Fatalf("expected filter change to reset selected work item, got %d", mm.selectedItem)
	}
	active := mm.visibleWorkItems()
	if len(active) == 0 {
		t.Fatalf("expected active worktrees view to include work items")
	}
	for _, item := range active {
		if item.Worktree == nil {
			t.Fatalf("expected active worktrees view to exclude item without worktree: %+v", item)
		}
	}

	got, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	mm = got.(model)
	reviewRequested := mm.visibleWorkItems()
	if len(reviewRequested) != 1 || reviewRequested[0].PullRequest == nil || reviewRequested[0].PullRequest.ReviewState != "review requested" {
		t.Fatalf("expected review requested view to include only review-requested PR work, got %+v", reviewRequested)
	}

	got, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	mm = got.(model)
	failedChecks := mm.visibleWorkItems()
	if len(failedChecks) != 1 || failedChecks[0].Checks.State != workbench.CheckFailing {
		t.Fatalf("expected failed checks view to include only failing-check work, got %+v", failedChecks)
	}
}

func TestUpdate_PreviewPaneIgnoresMovementKeys(t *testing.T) {
	start := newModel()
	got, _ := start.Update(tea.KeyMsg{Type: tea.KeyTab})
	mm := got.(model)

	got, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	mm = got.(model)
	if mm.selectedRepo != 0 || mm.selectedItem != 0 {
		t.Fatalf("expected preview pane movement to leave selections unchanged, got repo=%d item=%d", mm.selectedRepo, mm.selectedItem)
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

func TestPad_UsesTerminalDisplayWidth(t *testing.T) {
	got := pad("日本", 6)
	if width := lipgloss.Width(got); width != 6 {
		t.Fatalf("expected display width 6, got %d for %q", width, got)
	}
}

func TestTruncate_UsesTerminalDisplayWidth(t *testing.T) {
	got := truncate("日本語", 5)
	if width := lipgloss.Width(got); width > 5 {
		t.Fatalf("expected display width at most 5, got %d for %q", width, got)
	}
	if got != "日本~" {
		t.Fatalf("expected wide text to truncate with tail, got %q", got)
	}
}

func TestRenderPane_UsesLipGlossBorderWidth(t *testing.T) {
	got := newModel().renderPane("Demo[1]", []string{"header", "row"}, 10, 3, true)
	lines := strings.Split(got, "\n")
	if len(lines) != 5 {
		t.Fatalf("expected bordered pane height 5, got %d in %q", len(lines), got)
	}
	if !strings.Contains(lines[0], "Demo[1]") {
		t.Fatalf("expected pane title in top border, got %q", lines[0])
	}
	for _, line := range lines {
		if width := lipgloss.Width(line); width != 10+paneBorderWidth {
			t.Fatalf("expected bordered pane width %d, got %d for %q", 10+paneBorderWidth, width, line)
		}
	}
	if !strings.Contains(got, paneBorderGlyph) {
		t.Fatalf("expected Lip Gloss pane border, got %q", got)
	}
}

func TestShortRemoteLabel_ShowsIssueNumberWhenBranchHasNoPullRequest(t *testing.T) {
	item := workbench.WorkItem{
		Branch: &workbench.BranchRef{Name: "feat/issue-linked-work"},
		Issue:  &workbench.IssueRef{Number: 34, Title: "Branch preview UX", Certain: true},
	}
	if got := shortRemoteLabel(item); got != "#34" {
		t.Fatalf("expected issue number for branch without PR, got %q", got)
	}
}
