package app

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/0maru/gh-zen/internal/workbench"
)

func requirePreviewResultMsg(t *testing.T, cmd tea.Cmd) previewResultMsg {
	t.Helper()
	if cmd == nil {
		t.Fatalf("expected preview command, got nil")
	}
	msg := cmd()
	result, ok := msg.(previewResultMsg)
	if !ok {
		t.Fatalf("expected previewResultMsg, got %T", msg)
	}
	return result
}

func requireModelEqualIgnoringPreviewLoader(t *testing.T, got tea.Model, want model) {
	t.Helper()
	mm, ok := got.(model)
	if !ok {
		t.Fatalf("expected model, got %T", got)
	}
	mm.previewLoader = nil
	want.previewLoader = nil
	if !reflect.DeepEqual(mm, want) {
		t.Fatalf("expected model unchanged, got %+v", mm)
	}
}

func emptyPreviewLoader(req previewRequest) tea.Cmd {
	return func() tea.Msg {
		return previewResultMsg{
			requestID:  req.requestID,
			workItemID: req.workItemID,
			empty:      true,
		}
	}
}

func errorPreviewLoader(err error) previewLoader {
	return func(req previewRequest) tea.Cmd {
		return func() tea.Msg {
			return previewResultMsg{
				requestID:  req.requestID,
				workItemID: req.workItemID,
				err:        err,
			}
		}
	}
}

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
			_, cmd := newModel().Update(tc.msg)
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
	requireModelEqualIgnoringPreviewLoader(t, got, start)
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
	if got.preview.status != previewLoading {
		t.Fatalf("expected initial preview to be loading, got %v", got.preview.status)
	}
	if got.focusedWorkItemID != got.workItems[0].ID {
		t.Fatalf("expected focused item %q, got %q", got.workItems[0].ID, got.focusedWorkItemID)
	}
}

func TestInit_LoadsInitialPreview(t *testing.T) {
	start := newModelWithPreviewLoader(fakeDelayedPreviewLoader(0))
	msg := requirePreviewResultMsg(t, start.Init())
	if msg.requestID != start.preview.requestID {
		t.Fatalf("expected request ID %d, got %d", start.preview.requestID, msg.requestID)
	}
	if msg.workItemID != start.focusedWorkItemID {
		t.Fatalf("expected work item ID %q, got %q", start.focusedWorkItemID, msg.workItemID)
	}

	got, cmd := start.Update(msg)
	if cmd != nil {
		t.Fatalf("expected nil command after preview result, got %T", cmd)
	}
	mm := got.(model)
	if mm.preview.status != previewLoaded {
		t.Fatalf("expected loaded preview, got %v", mm.preview.status)
	}
	if mm.preview.loaded.item.ID != start.focusedWorkItemID {
		t.Fatalf("expected loaded item %q, got %q", start.focusedWorkItemID, mm.preview.loaded.item.ID)
	}
}

func TestInit_BuildsInitialPreviewCommand(t *testing.T) {
	loaderCalls := 0
	loader := func(req previewRequest) tea.Cmd {
		loaderCalls++
		return fakeDelayedPreviewLoader(0)(req)
	}

	start := newModelWithPreviewLoader(loader)
	if loaderCalls != 0 {
		t.Fatalf("expected constructor to avoid building preview command, got %d loader calls", loaderCalls)
	}

	cmd := start.Init()
	if cmd == nil {
		t.Fatalf("expected Init to build preview command")
	}
	if loaderCalls != 1 {
		t.Fatalf("expected Init to build one preview command, got %d loader calls", loaderCalls)
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
	if cmd == nil {
		t.Fatalf("expected preview load command for movement")
	}
	mm := got.(model)
	if mm.selectedItem != 1 {
		t.Fatalf("expected j to move selection to 1, got %d", mm.selectedItem)
	}

	got, cmd = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if cmd == nil {
		t.Fatalf("expected preview load command for movement back to first item")
	}
	mm = got.(model)
	if mm.selectedItem != 0 {
		t.Fatalf("expected k to move selection back to 0, got %d", mm.selectedItem)
	}

	got, cmd = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	if cmd == nil {
		t.Fatalf("expected preview load command for jump to end")
	}
	mm = got.(model)
	if mm.selectedItem != len(mm.workItems)-1 {
		t.Fatalf("expected G to move selection to end, got %d", mm.selectedItem)
	}

	got, cmd = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	if cmd == nil {
		t.Fatalf("expected preview load command for jump to start")
	}
	mm = got.(model)
	if mm.selectedItem != 0 {
		t.Fatalf("expected g to move selection to start, got %d", mm.selectedItem)
	}
}

func TestUpdate_ArrowKeysMoveSelection(t *testing.T) {
	start := newModel()
	got, cmd := start.Update(tea.KeyMsg{Type: tea.KeyDown})
	if cmd == nil {
		t.Fatalf("expected preview load command for arrow movement")
	}
	mm := got.(model)
	if mm.selectedItem != 1 {
		t.Fatalf("expected down arrow to move selection to 1, got %d", mm.selectedItem)
	}

	got, _ = mm.Update(tea.KeyMsg{Type: tea.KeyUp})
	mm = got.(model)
	if mm.selectedItem != 0 {
		t.Fatalf("expected up arrow to move selection back to 0, got %d", mm.selectedItem)
	}
}

func TestUpdate_MoveSelection_ClampsAtEdges(t *testing.T) {
	start := newModel()
	got, cmd := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if cmd != nil {
		t.Fatalf("expected nil cmd when clamped at start, got %T", cmd)
	}
	mm := got.(model)
	if mm.selectedItem != 0 {
		t.Fatalf("expected selection to stay at start, got %d", mm.selectedItem)
	}

	items := mm.visibleWorkItems()
	mm.selectedItem = len(items) - 1
	mm.focusedWorkItemID = items[len(items)-1].ID
	mm.preview = previewState{status: previewLoading, focusedWorkItemID: mm.focusedWorkItemID}
	got, cmd = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if cmd != nil {
		t.Fatalf("expected nil cmd when clamped at end, got %T", cmd)
	}
	mm = got.(model)
	if mm.selectedItem != len(items)-1 {
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

func TestUpdate_HAndLChangeFocusedPane(t *testing.T) {
	start := newModel()

	got, cmd := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if cmd != nil {
		t.Fatalf("expected nil cmd for pane focus, got %T", cmd)
	}
	mm := got.(model)
	if mm.focusedPane != paneRepositories {
		t.Fatalf("expected h to focus repositories pane, got %v", mm.focusedPane)
	}

	got, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	mm = got.(model)
	if mm.focusedPane != paneWorkItems {
		t.Fatalf("expected l to focus work items pane, got %v", mm.focusedPane)
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

	got, cmd := mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if cmd != nil {
		t.Fatalf("expected nil cmd when repository movement selects no work item, got %T", cmd)
	}
	mm = got.(model)
	if mm.selectedRepo != 1 {
		t.Fatalf("expected j to move repository selection when repo pane is focused, got %d", mm.selectedRepo)
	}
	if mm.selectedItem != 0 {
		t.Fatalf("expected work item selection to stay unchanged, got %d", mm.selectedItem)
	}
	if mm.preview.status != previewEmpty {
		t.Fatalf("expected empty preview after selecting repo with no work items, got %v", mm.preview.status)
	}
}

func TestUpdate_ToggleHelpPreservesFocusedWorkItem(t *testing.T) {
	start := newModel()
	start.selectedItem = 2
	start.focusedPane = paneWorkItems

	got, cmd := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if cmd != nil {
		t.Fatalf("expected nil cmd for help toggle, got %T", cmd)
	}
	mm := got.(model)
	if !mm.help.ShowAll {
		t.Fatalf("expected ? to show full help")
	}
	if mm.selectedItem != 2 || mm.focusedPane != paneWorkItems {
		t.Fatalf("expected help toggle to preserve focus and selected item, got focus=%v item=%d", mm.focusedPane, mm.selectedItem)
	}

	got, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	mm = got.(model)
	if mm.help.ShowAll {
		t.Fatalf("expected second ? to hide full help")
	}
	if mm.selectedItem != 2 || mm.focusedPane != paneWorkItems {
		t.Fatalf("expected help hide to preserve focus and selected item, got focus=%v item=%d", mm.focusedPane, mm.selectedItem)
	}
}

func TestUpdate_KeyOverrideChangesActionAndHelp(t *testing.T) {
	start := newModel()
	start.keys.MoveDown.SetKeys("n")
	start.keys.MoveDown.SetHelp("n", "down")

	got, cmd := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if cmd == nil {
		t.Fatalf("expected preview load command for remapped movement")
	}
	mm := got.(model)
	if mm.selectedItem != 1 {
		t.Fatalf("expected remapped n key to move selection to 1, got %d", mm.selectedItem)
	}

	got, _ = start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	mm = got.(model)
	if mm.selectedItem != 0 {
		t.Fatalf("expected original j key to stop moving after override, got %d", mm.selectedItem)
	}
	if !strings.Contains(start.keymapLine(defaultWidth), "n/k move") {
		t.Fatalf("expected help to reflect remapped key, got %q", start.keymapLine(defaultWidth))
	}
}

func TestUpdate_ActionKeysAreBound(t *testing.T) {
	cases := []struct {
		name string
		msg  tea.KeyMsg
		want actionID
	}{
		{"refresh", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}, actionRefresh},
		{"open", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}}, actionOpen},
		{"enter", tea.KeyMsg{Type: tea.KeyEnter}, actionOpen},
		{"copy", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}, actionCopy},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := newModel().matchedAction(tc.msg)
			if !ok {
				t.Fatalf("expected key to be bound to %s", tc.want)
			}
			if got != tc.want {
				t.Fatalf("expected action %s, got %s", tc.want, got)
			}
		})
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

	got, cmd := mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if cmd != nil {
		t.Fatalf("expected preview pane movement to skip preview load, got %T", cmd)
	}
	mm = got.(model)
	if mm.selectedRepo != 0 || mm.selectedItem != 0 {
		t.Fatalf("expected preview pane movement to leave selections unchanged, got repo=%d item=%d", mm.selectedRepo, mm.selectedItem)
	}
}

func TestUpdate_FocusChangeStartsPreviewLoad(t *testing.T) {
	start := newModelWithPreviewLoader(fakeDelayedPreviewLoader(0))
	initialRequestID := start.preview.requestID
	second := start.visibleWorkItems()[1]

	got, cmd := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if cmd == nil {
		t.Fatalf("expected preview load command")
	}
	mm := got.(model)
	if mm.focusedWorkItemID != second.ID {
		t.Fatalf("expected focused item %q, got %q", second.ID, mm.focusedWorkItemID)
	}
	if mm.preview.status != previewLoading {
		t.Fatalf("expected loading preview, got %v", mm.preview.status)
	}
	if mm.preview.requestID != initialRequestID+1 {
		t.Fatalf("expected request ID to increment to %d, got %d", initialRequestID+1, mm.preview.requestID)
	}
}

func TestUpdate_StalePreviewResultIsDiscarded(t *testing.T) {
	start := newModelWithPreviewLoader(fakeDelayedPreviewLoader(0))
	firstResult := requirePreviewResultMsg(t, start.Init())

	got, secondCmd := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	mm := got.(model)
	secondID := mm.focusedWorkItemID

	got, cmd := mm.Update(firstResult)
	if cmd != nil {
		t.Fatalf("expected nil cmd for stale preview result, got %T", cmd)
	}
	mm = got.(model)
	if mm.focusedWorkItemID != secondID {
		t.Fatalf("expected focus to stay on %q, got %q", secondID, mm.focusedWorkItemID)
	}
	if mm.preview.status != previewLoading {
		t.Fatalf("expected stale result to leave preview loading, got %v", mm.preview.status)
	}
	if mm.preview.loaded.item.ID == firstResult.workItemID {
		t.Fatalf("expected stale item %q not to be loaded", firstResult.workItemID)
	}

	wrongIdentity := previewResultMsg{
		requestID:  mm.preview.requestID,
		workItemID: firstResult.workItemID,
		data: previewData{
			workItemID: firstResult.workItemID,
			item:       firstResult.data.item,
		},
	}
	got, _ = mm.Update(wrongIdentity)
	mm = got.(model)
	if mm.preview.status != previewLoading {
		t.Fatalf("expected wrong work item identity to be discarded, got %v", mm.preview.status)
	}

	secondResult := requirePreviewResultMsg(t, secondCmd)
	got, _ = mm.Update(secondResult)
	mm = got.(model)
	if mm.preview.status != previewLoaded {
		t.Fatalf("expected current result to load preview, got %v", mm.preview.status)
	}
	if mm.preview.loaded.item.ID != secondID {
		t.Fatalf("expected loaded item %q, got %q", secondID, mm.preview.loaded.item.ID)
	}
}

func TestUpdate_EmptyPreviewResultRendersClearly(t *testing.T) {
	start := newModelWithPreviewLoader(emptyPreviewLoader)
	got, _ := start.Update(requirePreviewResultMsg(t, start.Init()))
	mm := got.(model)
	if mm.preview.status != previewEmpty {
		t.Fatalf("expected empty preview, got %v", mm.preview.status)
	}
	if got := strings.Join(mm.previewLines(80), "\n"); !strings.Contains(got, "No preview data") {
		t.Fatalf("expected empty preview copy, got %q", got)
	}
}

func TestUpdate_ErrorPreviewResultRendersClearly(t *testing.T) {
	start := newModelWithPreviewLoader(errorPreviewLoader(errors.New("fake preview failed")))
	got, _ := start.Update(requirePreviewResultMsg(t, start.Init()))
	mm := got.(model)
	if mm.preview.status != previewError {
		t.Fatalf("expected error preview, got %v", mm.preview.status)
	}
	lines := strings.Join(mm.previewLines(80), "\n")
	if !strings.Contains(lines, "Preview failed") || !strings.Contains(lines, "fake preview failed") {
		t.Fatalf("expected error preview copy, got %q", lines)
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
