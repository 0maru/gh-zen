package app

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0maru/gh-zen/internal/config"
	"github.com/0maru/gh-zen/internal/github"
	"github.com/0maru/gh-zen/internal/workbench"
)

type recordingActionsLoader struct {
	runs         []workbench.WorkflowRunRef
	previews     map[int64]ActionsRunPreview
	logs         map[int64]workbench.WorkflowLog
	runErr       error
	previewErr   error
	logErr       error
	runCalls     []workbench.RepoRef
	previewCalls []int64
	logCalls     []github.LogFetchOptions
}

func (l *recordingActionsLoader) LoadRuns(_ context.Context, repo workbench.RepoRef) ([]workbench.WorkflowRunRef, error) {
	l.runCalls = append(l.runCalls, repo)
	if l.runErr != nil {
		return nil, l.runErr
	}
	return append([]workbench.WorkflowRunRef(nil), l.runs...), nil
}

func (l *recordingActionsLoader) LoadRunPreview(_ context.Context, _ workbench.RepoRef, run workbench.WorkflowRunRef) (ActionsRunPreview, error) {
	l.previewCalls = append(l.previewCalls, run.ID)
	if l.previewErr != nil {
		return ActionsRunPreview{}, l.previewErr
	}
	if preview, ok := l.previews[run.ID]; ok {
		return preview, nil
	}
	return ActionsRunPreview{Run: run}, nil
}

func (l *recordingActionsLoader) LoadRunLogs(_ context.Context, _ workbench.RepoRef, run workbench.WorkflowRunRef, opts github.LogFetchOptions) (workbench.WorkflowLog, error) {
	l.logCalls = append(l.logCalls, opts)
	if l.logErr != nil {
		return workbench.WorkflowLog{}, l.logErr
	}
	if log, ok := l.logs[run.ID]; ok {
		log.Lines = append([]string(nil), log.Lines...)
		return log, nil
	}
	return workbench.WorkflowLog{RunID: run.ID, Failed: opts.FailedOnly}, nil
}

func requireActionsLoadMsg(t *testing.T, cmd tea.Cmd) actionsLoadMsg {
	t.Helper()
	if cmd == nil {
		t.Fatalf("expected actions load command, got nil")
	}
	msg := cmd()
	result, ok := msg.(actionsLoadMsg)
	if !ok {
		t.Fatalf("expected actionsLoadMsg, got %T", msg)
	}
	return result
}

func requireActionsPreviewMsg(t *testing.T, cmd tea.Cmd) actionsPreviewMsg {
	t.Helper()
	if cmd == nil {
		t.Fatalf("expected actions preview command, got nil")
	}
	msg := cmd()
	result, ok := msg.(actionsPreviewMsg)
	if !ok {
		t.Fatalf("expected actionsPreviewMsg, got %T", msg)
	}
	return result
}

func requireActionsLogMsg(t *testing.T, cmd tea.Cmd) actionsLogMsg {
	t.Helper()
	if cmd == nil {
		t.Fatalf("expected actions log command, got nil")
	}
	msg := cmd()
	result, ok := msg.(actionsLogMsg)
	if !ok {
		t.Fatalf("expected actionsLogMsg, got %T", msg)
	}
	return result
}

func testActionsModel(loader ActionsLoader) model {
	repo := workbench.RepoRef{Owner: "0maru", Name: "gh-zen"}
	return newModelWithRuntimeData(config.Defaults(), repo.FullName(), WorkbenchData{
		Repos:         []workbench.RepoRef{repo},
		ActionsLoader: loader,
	}, fakeDelayedPreviewLoader(0))
}

func TestActionsFilter_MatchesAllDimensions(t *testing.T) {
	runs := []workbench.WorkflowRunRef{
		{ID: 1, Status: "completed", Conclusion: "success", Branch: "main", WorkflowName: "CI", Event: "push", Actor: "0maru"},
		{ID: 2, Status: "completed", Conclusion: "failure", Branch: "feature", WorkflowName: "CI", Event: "pull_request", Actor: "teammate"},
		{ID: 3, Status: "in_progress", Branch: "main", WorkflowName: "Release", Event: "workflow_dispatch", Actor: "0maru"},
	}
	cases := []struct {
		name   string
		filter actionsFilter
		want   []int64
	}{
		{name: "empty", filter: actionsFilter{}, want: []int64{1, 2, 3}},
		{name: "status", filter: actionsFilter{Status: "completed"}, want: []int64{1, 2}},
		{name: "conclusion", filter: actionsFilter{Conclusion: "failure"}, want: []int64{2}},
		{name: "branch", filter: actionsFilter{Branch: "main"}, want: []int64{1, 3}},
		{name: "workflow", filter: actionsFilter{Workflow: "Release"}, want: []int64{3}},
		{name: "event", filter: actionsFilter{Event: "pull_request"}, want: []int64{2}},
		{name: "actor", filter: actionsFilter{Actor: "0maru"}, want: []int64{1, 3}},
		{name: "combined", filter: actionsFilter{Status: "completed", Conclusion: "success", Branch: "main", Workflow: "CI", Event: "push", Actor: "0maru"}, want: []int64{1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := filterWorkflowRuns(runs, tc.filter)
			gotIDs := make([]int64, 0, len(got))
			for _, run := range got {
				gotIDs = append(gotIDs, run.ID)
			}
			if !reflect.DeepEqual(gotIDs, tc.want) {
				t.Fatalf("expected run IDs %+v, got %+v", tc.want, gotIDs)
			}
		})
	}
}

func TestActionsMode_LoadsRunsAndPreview(t *testing.T) {
	runs := workbench.FakeWorkflowRuns()
	loader := &recordingActionsLoader{
		runs: runs,
		previews: map[int64]ActionsRunPreview{
			runs[0].ID: {
				Run:  runs[0],
				Jobs: workbench.FakeWorkflowJobs()[runs[0].ID],
			},
		},
	}
	start := testActionsModel(loader)

	got, cmd := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if cmd == nil {
		t.Fatalf("expected actions load command")
	}
	msg := requireActionsLoadMsg(t, cmd)
	got, cmd = got.(model).Update(msg)
	if cmd == nil {
		t.Fatalf("expected run preview command after load")
	}
	got, _ = got.(model).Update(requireActionsPreviewMsg(t, cmd))
	mm := got.(model)

	if mm.mode != modeActions {
		t.Fatalf("expected actions mode, got %v", mm.mode)
	}
	if len(loader.runCalls) != 1 || loader.runCalls[0].FullName() != "0maru/gh-zen" {
		t.Fatalf("expected one run load for repo, got %+v", loader.runCalls)
	}
	if len(loader.previewCalls) != 1 || loader.previewCalls[0] != runs[0].ID {
		t.Fatalf("expected preview load for first run, got %+v", loader.previewCalls)
	}
	if len(loader.logCalls) != 0 {
		t.Fatalf("expected focus preview not to fetch logs, got %+v", loader.logCalls)
	}
	if lines := strings.Join(mm.actionsPreviewLines(120), "\n"); !strings.Contains(lines, "Workflow: CI") || !strings.Contains(lines, "Failure summary: none") {
		t.Fatalf("expected run preview lines, got:\n%s", lines)
	}
}

func TestActionsMode_MovingRunLoadsPreviewButNotLogs(t *testing.T) {
	runs := workbench.FakeWorkflowRuns()
	jobs := workbench.FakeWorkflowJobs()
	loader := &recordingActionsLoader{
		runs: runs,
		previews: map[int64]ActionsRunPreview{
			runs[0].ID: {Run: runs[0], Jobs: jobs[runs[0].ID]},
			runs[1].ID: {Run: runs[1], Jobs: jobs[runs[1].ID], Annotations: workbench.FakeWorkflowAnnotations()},
		},
	}
	start := testActionsModel(loader)
	got, cmd := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	got, cmd = got.(model).Update(requireActionsLoadMsg(t, cmd))
	got, _ = got.(model).Update(requireActionsPreviewMsg(t, cmd))

	got, cmd = got.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if cmd == nil {
		t.Fatalf("expected preview command after moving to next run")
	}
	got, _ = got.(model).Update(requireActionsPreviewMsg(t, cmd))
	mm := got.(model)

	if len(loader.previewCalls) != 2 || loader.previewCalls[1] != runs[1].ID {
		t.Fatalf("expected second preview load for run %d, got %+v", runs[1].ID, loader.previewCalls)
	}
	if len(loader.logCalls) != 0 {
		t.Fatalf("expected movement not to fetch logs, got %+v", loader.logCalls)
	}
	lines := strings.Join(mm.actionsPreviewLines(120), "\n")
	if !strings.Contains(lines, "Failure summary: 1 failed job, 1 failed step, 1 annotation") {
		t.Fatalf("expected failure summary, got:\n%s", lines)
	}
}

func TestActionsMode_FetchLogsIsExplicit(t *testing.T) {
	runs := workbench.FakeWorkflowRuns()
	loader := &recordingActionsLoader{
		runs: runs,
		previews: map[int64]ActionsRunPreview{
			runs[0].ID: {Run: runs[0]},
		},
		logs: map[int64]workbench.WorkflowLog{
			runs[0].ID: {RunID: runs[0].ID, Failed: true, Lines: []string{"job\tstep\tfailed line"}},
		},
	}
	start := testActionsModel(loader)
	got, cmd := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	got, cmd = got.(model).Update(requireActionsLoadMsg(t, cmd))
	got, _ = got.(model).Update(requireActionsPreviewMsg(t, cmd))

	got, cmd = got.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	if cmd == nil {
		t.Fatalf("expected explicit log command")
	}
	got, _ = got.(model).Update(requireActionsLogMsg(t, cmd))
	mm := got.(model)

	if len(loader.logCalls) != 1 || !loader.logCalls[0].FailedOnly {
		t.Fatalf("expected one failed-only log call, got %+v", loader.logCalls)
	}
	if lines := strings.Join(mm.actionsPreviewLines(120), "\n"); !strings.Contains(lines, "Failed logs (1 lines):") || !strings.Contains(lines, "failed line") {
		t.Fatalf("expected loaded logs in preview, got:\n%s", lines)
	}
}

func TestActionsMode_StaleLoadResultIsDiscarded(t *testing.T) {
	repo := workbench.RepoRef{Owner: "0maru", Name: "gh-zen"}
	runs := workbench.FakeWorkflowRuns()
	loader := &recordingActionsLoader{runs: runs}
	start := newModelWithRuntimeData(config.Defaults(), repo.FullName(), WorkbenchData{
		Repos:         []workbench.RepoRef{repo},
		ActionsLoader: loader,
	}, fakeDelayedPreviewLoader(0))
	got, firstCmd := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	first := requireActionsLoadMsg(t, firstCmd)
	got, secondCmd := got.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if secondCmd == nil {
		t.Fatalf("expected second actions load command")
	}

	got, cmd := got.(model).Update(first)
	if cmd != nil {
		t.Fatalf("expected stale actions load to skip preview command, got %T", cmd)
	}
	mm := got.(model)
	if len(mm.actions.runs) != 0 {
		t.Fatalf("expected stale result not to set runs, got %+v", mm.actions.runs)
	}
}

func TestActionsMode_OpenCopyRunActions(t *testing.T) {
	run := workbench.WorkflowRunRef{
		ID:        12345,
		RunNumber: 88,
		URL:       "https://github.com/0maru/gh-zen/actions/runs/12345",
		CreatedAt: time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC),
	}
	loader := &recordingActionsLoader{runs: []workbench.WorkflowRunRef{run}}
	start := testActionsModel(loader)
	got, cmd := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	got, cmd = got.(model).Update(requireActionsLoadMsg(t, cmd))
	got, _ = got.(model).Update(requireActionsPreviewMsg(t, cmd))
	runner := &fakeActionRunner{}
	mm := got.(model)
	mm.actionRunner = runner

	for _, key := range []rune{'o', 'y', 'Y'} {
		updated, actionCmd := mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}})
		if actionCmd == nil {
			t.Fatalf("expected action command for %q", string(key))
		}
		updated, _ = updated.(model).Update(actionCmd())
		mm = updated.(model)
	}
	if !reflect.DeepEqual(runner.opened, []string{run.URL}) {
		t.Fatalf("expected opened run URL, got %+v", runner.opened)
	}
	if !reflect.DeepEqual(runner.copied, []string{run.URL, "12345"}) {
		t.Fatalf("expected copied run URL and ID, got %+v", runner.copied)
	}
}

func TestActionsMode_LoadErrorIsNonFatal(t *testing.T) {
	loader := &recordingActionsLoader{runErr: errors.New("gh auth failed")}
	start := testActionsModel(loader)

	got, cmd := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	got, _ = got.(model).Update(requireActionsLoadMsg(t, cmd))
	mm := got.(model)

	if mm.mode != modeActions {
		t.Fatalf("expected to remain in actions mode")
	}
	if !strings.Contains(strings.Join(mm.actionsRunLines(120, true), "\n"), "gh auth failed") {
		t.Fatalf("expected load error in actions list, got %+v", mm.actionsRunLines(120, true))
	}
}
