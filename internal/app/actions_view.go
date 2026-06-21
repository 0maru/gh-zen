package app

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0maru/gh-zen/internal/github"
	"github.com/0maru/gh-zen/internal/workbench"
)

const maxPreviewLogLines = 20

type appMode int

const (
	modeWorkbench appMode = iota
	modeActions
)

type actionsState struct {
	repo                 workbench.RepoRef
	runs                 []workbench.WorkflowRunRef
	selectedRun          int
	loading              bool
	loadError            string
	filter               actionsFilter
	activeLoadRequest    actionsLoadRequest
	nextLoadRequestID    int
	preview              actionsPreviewState
	nextPreviewRequestID int
	activeLogRequest     actionsLogRequest
	nextLogRequestID     int
	logLoading           bool
	logsByRunID          map[int64]workbench.WorkflowLog
}

type actionsPreviewState struct {
	status       previewStatus
	requestID    int
	focusedRunID int64
	loaded       ActionsRunPreview
	errorMessage string
}

type actionsLoadRequest struct {
	requestID int
	repo      workbench.RepoRef
	status    string
}

type actionsLoadMsg struct {
	request actionsLoadRequest
	runs    []workbench.WorkflowRunRef
	err     error
}

type actionsPreviewRequest struct {
	requestID int
	repo      workbench.RepoRef
	run       workbench.WorkflowRunRef
}

type actionsPreviewMsg struct {
	request actionsPreviewRequest
	preview ActionsRunPreview
	err     error
}

type actionsLogRequest struct {
	requestID int
	repo      workbench.RepoRef
	run       workbench.WorkflowRunRef
	opts      github.LogFetchOptions
	status    string
}

type actionsLogMsg struct {
	request actionsLogRequest
	log     workbench.WorkflowLog
	err     error
}

func (m *model) switchMode(mode appMode) tea.Cmd {
	if m.mode == mode {
		return nil
	}
	m.mode = mode
	switch mode {
	case modeActions:
		m.focusedPane = paneWorkItems
		m.viewSelected = false
		m.statusMessage = "Loading workflow runs..."
		return m.startActionsLoadForSelectedRepo("Loading workflow runs...")
	default:
		m.focusedPane = paneWorkItems
		return m.startPreviewLoadForCurrentItem()
	}
}

func (m *model) refreshActionsData() tea.Cmd {
	cmd := m.startActionsLoadForSelectedRepo("Reloading workflow runs...")
	if cmd == nil {
		m.statusMessage = "Actions refresh unavailable"
	}
	return cmd
}

func (m *model) startActionsLoadForSelectedRepo(status string) tea.Cmd {
	if m.actionsLoader == nil {
		return nil
	}
	repo, ok := m.selectedRepoRef()
	if !ok {
		m.actions.runs = nil
		m.actions.loading = false
		m.actions.preview = actionsPreviewState{status: previewEmpty}
		return nil
	}
	m.actions.nextLoadRequestID++
	request := actionsLoadRequest{
		requestID: m.actions.nextLoadRequestID,
		repo:      repo,
		status:    status,
	}
	m.actions.activeLoadRequest = request
	m.actions.loading = true
	m.actions.loadError = ""
	m.statusMessage = status
	return m.actionsLoadCommand(request)
}

func (m model) actionsLoadCommand(request actionsLoadRequest) tea.Cmd {
	return func() tea.Msg {
		runs, err := m.actionsLoader.LoadRuns(context.Background(), request.repo)
		return actionsLoadMsg{request: request, runs: runs, err: err}
	}
}

func (m *model) handleActionsLoad(msg actionsLoadMsg) tea.Cmd {
	if msg.request != m.actions.activeLoadRequest {
		return nil
	}
	repo, ok := m.selectedRepoRef()
	if !ok || repo != msg.request.repo {
		m.actions.loading = false
		if m.statusMessage == msg.request.status {
			m.statusMessage = ""
		}
		return nil
	}
	m.actions.loading = false
	if msg.err != nil {
		m.actions.loadError = msg.err.Error()
		m.statusMessage = "Workflow runs load failed"
		return nil
	}

	selectedRunID := int64(0)
	if run, ok := m.selectedWorkflowRun(); ok {
		selectedRunID = run.ID
	}
	m.actions.runs = append([]workbench.WorkflowRunRef(nil), msg.runs...)
	m.actions.repo = msg.request.repo
	m.actions.loadError = ""
	m.restoreSelectedWorkflowRun(selectedRunID)
	if m.statusMessage == msg.request.status {
		m.statusMessage = ""
	}
	return m.startActionsPreviewForCurrentRun()
}

func (m *model) startActionsPreviewIfFocusedRunChanged() tea.Cmd {
	run, ok := m.selectedWorkflowRun()
	if !ok || run.ID == 0 {
		m.actions.preview = actionsPreviewState{status: previewEmpty}
		return nil
	}
	if run.ID == m.actions.preview.focusedRunID {
		return nil
	}
	return m.startActionsPreviewForCurrentRun()
}

func (m *model) startActionsPreviewForCurrentRun() tea.Cmd {
	run, ok := m.selectedWorkflowRun()
	if !ok || run.ID == 0 {
		m.actions.preview = actionsPreviewState{status: previewEmpty}
		return nil
	}
	if m.actionsLoader == nil {
		m.actions.preview = actionsPreviewState{status: previewEmpty}
		return nil
	}
	repo, ok := m.selectedRepoRef()
	if !ok {
		m.actions.preview = actionsPreviewState{status: previewEmpty}
		return nil
	}
	m.actions.nextPreviewRequestID++
	request := actionsPreviewRequest{
		requestID: m.actions.nextPreviewRequestID,
		repo:      repo,
		run:       run,
	}
	m.actions.preview = actionsPreviewState{
		status:       previewLoading,
		requestID:    request.requestID,
		focusedRunID: run.ID,
	}
	return m.actionsPreviewCommand(request)
}

func (m model) actionsPreviewCommand(request actionsPreviewRequest) tea.Cmd {
	return func() tea.Msg {
		preview, err := m.actionsLoader.LoadRunPreview(context.Background(), request.repo, request.run)
		return actionsPreviewMsg{request: request, preview: preview, err: err}
	}
}

func (m *model) handleActionsPreview(msg actionsPreviewMsg) {
	if msg.request.requestID != m.actions.preview.requestID || msg.request.run.ID != m.actions.preview.focusedRunID {
		return
	}
	next := actionsPreviewState{
		requestID:    msg.request.requestID,
		focusedRunID: msg.request.run.ID,
	}
	if msg.err != nil {
		next.status = previewError
		next.errorMessage = msg.err.Error()
	} else {
		next.status = previewLoaded
		next.loaded = msg.preview
		if next.loaded.Run.ID == 0 {
			next.loaded.Run = msg.request.run
		}
	}
	m.actions.preview = next
}

func (m *model) fetchActionsLogs() tea.Cmd {
	if m.actionsLoader == nil {
		m.statusMessage = "Actions logs unavailable"
		return nil
	}
	repo, ok := m.selectedRepoRef()
	if !ok {
		m.statusMessage = "No repository selected"
		return nil
	}
	run, ok := m.selectedWorkflowRun()
	if !ok {
		m.statusMessage = "No workflow run selected"
		return nil
	}
	m.actions.nextLogRequestID++
	request := actionsLogRequest{
		requestID: m.actions.nextLogRequestID,
		repo:      repo,
		run:       run,
		opts:      github.LogFetchOptions{FailedOnly: true},
		status:    "Loading failed logs for " + run.NumberLabel() + "...",
	}
	m.actions.activeLogRequest = request
	m.actions.logLoading = true
	m.statusMessage = request.status
	return m.actionsLogCommand(request)
}

func (m model) actionsLogCommand(request actionsLogRequest) tea.Cmd {
	return func() tea.Msg {
		log, err := m.actionsLoader.LoadRunLogs(context.Background(), request.repo, request.run, request.opts)
		return actionsLogMsg{request: request, log: log, err: err}
	}
}

func (m *model) handleActionsLog(msg actionsLogMsg) {
	if msg.request != m.actions.activeLogRequest {
		return
	}
	m.actions.logLoading = false
	if msg.err != nil {
		m.statusMessage = fmt.Sprintf("Workflow log load failed: %v", msg.err)
		return
	}
	if m.actions.logsByRunID == nil {
		m.actions.logsByRunID = map[int64]workbench.WorkflowLog{}
	}
	m.actions.logsByRunID[msg.request.run.ID] = msg.log
	m.statusMessage = "Loaded failed logs for " + msg.request.run.NumberLabel()
}

func (m model) visibleWorkflowRuns() []workbench.WorkflowRunRef {
	repo, ok := m.selectedRepoRef()
	if !ok || repo != m.actions.repo {
		return nil
	}
	return filterWorkflowRuns(m.actions.runs, m.actions.filter)
}

func (m model) selectedWorkflowRun() (workbench.WorkflowRunRef, bool) {
	runs := m.visibleWorkflowRuns()
	if len(runs) == 0 || m.actions.selectedRun < 0 || m.actions.selectedRun >= len(runs) {
		return workbench.WorkflowRunRef{}, false
	}
	return runs[m.actions.selectedRun], true
}

func (m *model) moveWorkflowRunSelection(delta int) {
	runs := m.visibleWorkflowRuns()
	if len(runs) == 0 {
		m.actions.selectedRun = 0
		return
	}
	m.actions.selectedRun = clamp(m.actions.selectedRun+delta, 0, len(runs)-1)
}

func (m *model) jumpWorkflowRunSelection(toEnd bool) {
	runs := m.visibleWorkflowRuns()
	if len(runs) == 0 {
		m.actions.selectedRun = 0
		return
	}
	if toEnd {
		m.actions.selectedRun = len(runs) - 1
		return
	}
	m.actions.selectedRun = 0
}

func (m *model) restoreSelectedWorkflowRun(runID int64) {
	runs := m.visibleWorkflowRuns()
	if len(runs) == 0 {
		m.actions.selectedRun = 0
		return
	}
	if runID != 0 {
		for i, run := range runs {
			if run.ID == runID {
				m.actions.selectedRun = i
				return
			}
		}
	}
	m.actions.selectedRun = clamp(m.actions.selectedRun, 0, len(runs)-1)
}

func (m model) actionsRunLines(width int, focused bool) []string {
	runs := m.visibleWorkflowRuns()
	if len(runs) == 0 {
		return []string{m.emptyActionsRunLine()}
	}
	lines := []string{}
	if m.actions.filter.active() {
		lines = append(lines, truncate("  filters: "+m.actions.filter.describe(), width))
	}
	for i, run := range runs {
		marker := selectionMarker(i == m.actions.selectedRun, focused)
		workflow := run.WorkflowName
		if workflow == "" {
			workflow = "workflow"
		}
		branch := run.Branch
		if branch == "" {
			branch = "unknown"
		}
		row := fmt.Sprintf("%s %-11s %-12s %-14s %s", marker, run.StatusLabel(), workflow, branch, run.Title)
		lines = append(lines, truncate(row, width))
	}
	return lines
}

func (m model) emptyActionsRunLine() string {
	if m.actions.loading {
		return "  loading workflow runs..."
	}
	if m.actions.loadError != "" {
		return "  workflow runs failed: " + m.actions.loadError
	}
	if m.actions.filter.active() {
		return "  no workflow runs match filters"
	}
	return "  no workflow runs"
}

func (m model) actionsPreviewLines(width int) []string {
	switch m.actions.preview.status {
	case previewLoading:
		return m.actionsPreviewStatusLines(width, "Loading run details...")
	case previewLoaded:
		if m.actions.preview.loaded.Run.ID != m.actions.preview.focusedRunID {
			return m.actionsPreviewStatusLines(width, "Loading run details...")
		}
		log, logLoaded := m.actions.logsByRunID[m.actions.preview.focusedRunID]
		return workflowRunPreviewLines(m.actions.preview.loaded, log, logLoaded, m.actions.logLoading, width)
	case previewEmpty:
		if _, ok := m.selectedWorkflowRun(); !ok {
			return []string{"  no workflow run selected"}
		}
		return m.actionsPreviewStatusLines(width, "No run details")
	case previewError:
		lines := m.actionsPreviewStatusLines(width, "Run details failed")
		if m.actions.preview.errorMessage != "" {
			lines = append(lines, truncate("Error: "+m.actions.preview.errorMessage, width))
		}
		return lines
	default:
		if _, ok := m.selectedWorkflowRun(); !ok {
			return []string{"  no workflow run selected"}
		}
		return m.actionsPreviewStatusLines(width, "Run details idle")
	}
}

func (m model) actionsPreviewStatusLines(width int, status string) []string {
	lines := []string{truncate(status, width)}
	if run, ok := m.selectedWorkflowRun(); ok {
		lines = append(lines, truncate("Run: "+run.Label(), width))
	}
	return lines
}

func workflowRunPreviewLines(preview ActionsRunPreview, log workbench.WorkflowLog, logLoaded bool, logLoading bool, width int) []string {
	run := preview.Run
	lines := []string{
		truncate("Workflow: "+valueOrUnknown(run.WorkflowName), width),
		truncate("Title: "+valueOrUnknown(run.Title), width),
		truncate("Run: "+run.NumberLabel()+" ("+strconvFormatInt(run.ID)+")", width),
		truncate("Status: "+run.StatusLabel(), width),
		truncate("Branch: "+valueOrUnknown(run.Branch), width),
		truncate("Event: "+valueOrUnknown(run.Event), width),
		truncate("Actor: "+valueOrUnknown(run.Actor), width),
		truncate("Commit: "+valueOrUnknown(run.ShortSHA()), width),
		truncate("Created: "+formatActionTime(run.CreatedAt), width),
		truncate("Updated: "+formatActionTime(run.UpdatedAt), width),
	}
	if run.URL != "" {
		lines = append(lines, truncate("URL: "+run.URL, width))
	}
	lines = append(lines, truncate("Failure summary: "+workflowFailureSummary(preview), width))
	if len(preview.Jobs) > 0 {
		lines = append(lines, "Jobs:")
		for _, job := range preview.Jobs {
			line := fmt.Sprintf("  %s %s", job.StatusLabel(), job.Label())
			if duration := actionDuration(job.StartedAt, job.CompletedAt); duration != "" {
				line += " " + duration
			}
			lines = append(lines, truncate(line, width))
			for _, step := range job.Steps {
				if !isFailureConclusion(step.Conclusion) {
					continue
				}
				lines = append(lines, truncate(fmt.Sprintf("    failed step: %s", valueOrUnknown(step.Name)), width))
			}
		}
	}
	annotationLines := workflowAnnotationLines(preview.Annotations, width)
	if len(annotationLines) > 0 {
		lines = append(lines, "Annotations:")
		lines = append(lines, annotationLines...)
	}
	if logLoading {
		lines = append(lines, "Logs: loading failed logs...")
	}
	if logLoaded {
		lines = append(lines, workflowLogPreviewLines(log, width)...)
	}
	return lines
}

func workflowFailureSummary(preview ActionsRunPreview) string {
	failedJobs := 0
	failedSteps := 0
	annotations := 0
	for _, job := range preview.Jobs {
		if isFailureConclusion(job.Conclusion) {
			failedJobs++
		}
		for _, step := range job.Steps {
			if isFailureConclusion(step.Conclusion) {
				failedSteps++
			}
		}
		annotations += len(preview.Annotations[job.ID])
	}
	parts := []string{}
	if failedJobs > 0 {
		parts = append(parts, pluralize(failedJobs, "failed job"))
	}
	if failedSteps > 0 {
		parts = append(parts, pluralize(failedSteps, "failed step"))
	}
	if annotations > 0 {
		parts = append(parts, pluralize(annotations, "annotation"))
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}

func workflowAnnotationLines(annotationsByJob map[int64][]workbench.AnnotationRef, width int) []string {
	lines := []string{}
	jobIDs := make([]int64, 0, len(annotationsByJob))
	for jobID := range annotationsByJob {
		jobIDs = append(jobIDs, jobID)
	}
	sort.Slice(jobIDs, func(i, j int) bool {
		return jobIDs[i] < jobIDs[j]
	})
	for _, jobID := range jobIDs {
		annotations := annotationsByJob[jobID]
		for _, annotation := range annotations {
			line := "  " + annotation.Label()
			if annotation.Level != "" {
				line += " [" + annotation.Level + "]"
			}
			lines = append(lines, truncate(line, width))
			if annotation.Message != "" {
				lines = append(lines, truncate("    "+annotation.Message, width))
			}
		}
	}
	return lines
}

func workflowLogPreviewLines(log workbench.WorkflowLog, width int) []string {
	label := "Logs"
	if log.Failed {
		label = "Failed logs"
	}
	lines := []string{fmt.Sprintf("%s (%d lines):", label, len(log.Lines))}
	start := 0
	if len(log.Lines) > maxPreviewLogLines {
		start = len(log.Lines) - maxPreviewLogLines
		lines[0] = fmt.Sprintf("%s (%d lines, showing last %d):", label, len(log.Lines), maxPreviewLogLines)
	}
	for _, line := range log.Lines[start:] {
		lines = append(lines, truncate("  "+line, width))
	}
	return lines
}

func isFailureConclusion(value string) bool {
	value = strings.ToLower(value)
	return strings.Contains(value, "fail") ||
		strings.Contains(value, "error") ||
		strings.Contains(value, "cancel") ||
		strings.Contains(value, "timed_out") ||
		strings.Contains(value, "startup_failure")
}

func actionDuration(start time.Time, end time.Time) string {
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return ""
	}
	return "(" + end.Sub(start).Round(time.Second).String() + ")"
}

func formatActionTime(value time.Time) string {
	if value.IsZero() {
		return "unknown"
	}
	return value.UTC().Format("2006-01-02 15:04 UTC")
}

func valueOrUnknown(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}

func strconvFormatInt(value int64) string {
	if value == 0 {
		return "unknown"
	}
	return fmt.Sprintf("%d", value)
}

func pluralize(count int, label string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, label)
	}
	return fmt.Sprintf("%d %ss", count, label)
}
