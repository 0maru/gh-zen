package app

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	cfgpkg "github.com/0maru/gh-zen/internal/config"
	"github.com/0maru/gh-zen/internal/pullrequests"
	"github.com/0maru/gh-zen/internal/workbench"
)

const (
	defaultWidth            = 100
	repoPaneWidth           = 23
	workItemPaneWidth       = 41
	pullRequestPaneMinWidth = 96
	paneGapWidth            = 1
	paneContentPaddingLeft  = 1
	paneBorderGlyph         = "│"
	paneBorderWidth         = 2
	frameBorderLines        = 2
	horizontalLineGlyph     = "─"
	frameTopLeftGlyph       = "┌"
	frameTopRightGlyph      = "┐"
	frameBottomLeftGlyph    = "└"
	frameBottomRightGlyph   = "┘"
	previewPaneMinWidth     = 28
	fullLayoutMinWidth      = repoPaneWidth + workItemPaneWidth + previewPaneMinWidth + paneBorderWidth*3 + paneGapWidth*2
)

// paneFocus tracks the pane that owns pane-scoped key handling.
type paneFocus int

const (
	paneWorkItems paneFocus = iota
	paneRepositories
	panePreview
	panePullRequests
)

func (p paneFocus) label() string {
	switch p {
	case paneRepositories:
		return "Repositories"
	case panePullRequests:
		return "Pull Requests"
	case panePreview:
		return "Review"
	default:
		return "Work Items"
	}
}

func (p paneFocus) borderLabel() string {
	switch p {
	case paneRepositories:
		return "Repositories"
	case panePullRequests:
		return "PullRequests"
	case panePreview:
		return "Review"
	default:
		return "WorkItems"
	}
}

type model struct {
	width                   int
	height                  int
	activeView              appView
	repos                   []workbench.RepoRef
	repoSummaries           []workbench.RepositorySummary
	selectedRepo            int
	selectedView            int
	viewSelected            bool
	workItems               []workbench.WorkItem
	selectedItem            int
	workbenchSource         workbenchDataSource
	workbenchLoading        bool
	focusedPane             paneFocus
	focusedWorkItemRepo     workbench.RepoRef
	focusedWorkItemID       string
	preview                 previewState
	nextPreviewRequestID    int
	previewLoader           previewLoader
	pullRequests            []pullrequests.PullRequest
	pullRequestRepo         workbench.RepoRef
	selectedPR              int
	pendingPullRequest      int
	pullRequestsLoading     bool
	pullRequestFilter       pullrequests.PullRequestFilter
	pullRequestSearch       bool
	pullRequestSearchIn     string
	pullRequestFilterUI     bool
	pullRequestPreview      pullRequestPreviewState
	nextPRPreviewRequest    int
	prPreviewLoader         pullRequestPreviewLoader
	pullRequestPreviewWidth float64
	pullRequestService      pullrequests.Service
	nextPRLoadRequestID     int
	activePRLoadRequest     pullRequestLoadRequest
	workbenchReloader       WorkbenchReloader
	nextReloadRequestID     int
	activeReloadRequest     workbenchReloadRequest
	workbenchFilter         cfgpkg.WorkbenchFilter
	actionRunner            actionRunner
	statusMessage           string
	styles                  Styles
	keys                    workbenchKeyMap
	help                    help.Model
}

// WorkbenchData contains resolved repository workbench state for app startup.
type WorkbenchData struct {
	Repos               []workbench.RepoRef
	RepositorySummaries []workbench.RepositorySummary
	WorkItems           []workbench.WorkItem
	PullRequests        []pullrequests.PullRequest
	PullRequestsAPI     pullrequests.Service
	Reloader            WorkbenchReloader
	InitialLoading      bool
	Demo                bool
}

// WorkbenchReloader reloads runtime workbench data using the selected repository
// as the selection anchor.
type WorkbenchReloader interface {
	Load(ctx context.Context, repo workbench.RepoRef) workbench.RuntimeLoadResult
}

type repoViewFilter int

type workbenchDataSource int

type appView int

const (
	repoViewActiveWorktrees repoViewFilter = iota
	repoViewNeedsMyReview
	repoViewWaitingOnReview
	repoViewFailedChecks
)

const (
	workbenchDataLive workbenchDataSource = iota
	workbenchDataDemo
)

const (
	appViewWorkbench appView = iota
	appViewPullRequests
)

type repoView struct {
	label  string
	filter repoViewFilter
}

var repoViews = []repoView{
	{label: "Active worktrees", filter: repoViewActiveWorktrees},
	{label: "Needs my review", filter: repoViewNeedsMyReview},
	{label: "Waiting on review", filter: repoViewWaitingOnReview},
	{label: "Failed checks", filter: repoViewFailedChecks},
}

var workbenchErrorIDPrefixes = []string{
	"local-discovery-error:",
	"pull-request-discovery-error:",
	"issue-check-discovery-error:",
	"repository-path-error:",
}

func New() tea.Model {
	return newModel()
}

func NewWithConfig(cfg cfgpkg.Config, startupRepo string) tea.Model {
	return newModelWithRuntimeConfig(cfg, startupRepo, fakeDelayedPreviewLoader(defaultPreviewDelay))
}

// NewWithWorkbenchData builds the app model from already resolved workbench data.
func NewWithWorkbenchData(cfg cfgpkg.Config, startupRepo string, data WorkbenchData) tea.Model {
	return newModelWithRuntimeData(cfg, startupRepo, data, fakeDelayedPreviewLoader(defaultPreviewDelay))
}

func newModel() model {
	return newModelWithPreviewLoader(fakeDelayedPreviewLoader(defaultPreviewDelay))
}

func newModelWithPreviewLoader(loader previewLoader) model {
	return newModelWithRuntimeConfig(cfgpkg.Defaults(), "", loader)
}

func newModelWithRuntimeConfig(cfg cfgpkg.Config, startupRepo string, loader previewLoader) model {
	return newModelWithRuntimeConfigLoaders(cfg, startupRepo, loader, fakeDelayedPullRequestPreviewLoader(defaultPreviewDelay))
}

func newModelWithRuntimeConfigLoaders(cfg cfgpkg.Config, startupRepo string, loader previewLoader, prLoader pullRequestPreviewLoader) model {
	return newModelWithRuntimeDataLoaders(cfg, startupRepo, WorkbenchData{
		Repos:     workbench.FakeRepos(),
		WorkItems: workbench.FakeWorkItems(),
		Demo:      true,
	}, loader, prLoader)
}

func newModelWithRuntimeData(cfg cfgpkg.Config, startupRepo string, data WorkbenchData, loader previewLoader) model {
	return newModelWithRuntimeDataLoaders(cfg, startupRepo, data, loader, fakeDelayedPullRequestPreviewLoader(defaultPreviewDelay))
}

func newModelWithRuntimeDataLoaders(cfg cfgpkg.Config, startupRepo string, data WorkbenchData, loader previewLoader, prLoader pullRequestPreviewLoader) model {
	source := workbenchDataLive
	if data.Demo {
		source = workbenchDataDemo
	}
	prs := append([]pullrequests.PullRequest(nil), data.PullRequests...)
	if len(prs) == 0 && data.Demo {
		prs = pullrequests.FakePullRequests()
	}
	pullrequests.SortByUpdatedDesc(prs)
	repoSummaries := normalizeRepositorySummaries(data.RepositorySummaries, data.Repos)
	m := model{
		repos:                   repoRefsFromSummaries(repoSummaries),
		repoSummaries:           cloneRepositorySummaries(repoSummaries),
		workItems:               cloneWorkItems(data.WorkItems),
		workbenchSource:         source,
		previewLoader:           loader,
		prPreviewLoader:         prLoader,
		pullRequests:            prs,
		pullRequestFilter:       pullRequestFilterFromConfig(cfg.PullRequests.Filter),
		pullRequestPreviewWidth: cfg.PullRequests.PreviewWidth,
		pullRequestService:      data.PullRequestsAPI,
		workbenchReloader:       data.Reloader,
		workbenchFilter:         cfg.Workbench.Filter,
		actionRunner:            systemActionRunner{},
		styles:                  DefaultStyles(),
		keys:                    DefaultKeyMap(),
		help:                    newHelpModel(),
	}
	m.applyStartupView(cfg.Startup.View)
	if startupRepo == "" {
		startupRepo = cfg.Startup.Repo
	}
	m.applyStartupRepo(startupRepo)
	if data.InitialLoading {
		m.beginWorkbenchReload("Loading workbench data...")
	}
	if repo, ok := m.selectedRepoRef(); ok {
		m.pullRequestRepo = repo
	}
	_ = m.startPreviewLoadForCurrentItem()
	return m
}

type workbenchReloadRequest struct {
	requestID int
	repo      workbench.RepoRef
	status    string
}

type workbenchReloadMsg struct {
	request workbenchReloadRequest
	result  workbench.RuntimeLoadResult
}

type pullRequestLoadRequest struct {
	requestID int
	repo      workbench.RepoRef
	status    string
}

type pullRequestLoadMsg struct {
	request pullRequestLoadRequest
	prs     []pullrequests.PullRequest
	err     error
}

func (m model) Init() tea.Cmd {
	var cmds []tea.Cmd
	if m.workbenchLoading && m.activeReloadRequest.requestID != 0 {
		cmds = append(cmds, m.workbenchReloadCommand(m.activeReloadRequest))
	}
	if m.pullRequestsLoading && m.activePRLoadRequest.requestID != 0 {
		cmds = append(cmds, m.pullRequestLoadCommand(m.activePRLoadRequest))
	}
	if m.preview.status != previewLoading || m.previewLoader == nil {
		if m.pullRequestPreview.status == previewLoading && m.prPreviewLoader != nil {
			if pr, ok := m.selectedPullRequest(); ok && pr.Key() == m.pullRequestPreview.focusedPullRequestKey {
				cmds = append(cmds, m.prPreviewLoader(pullRequestPreviewRequest{
					requestID:      m.pullRequestPreview.requestID,
					pullRequestKey: pr.Key(),
					pr:             pr,
				}))
			}
		}
		return batchCommands(cmds...)
	}
	item, ok := m.selectedWorkItem()
	if !ok || !m.focusedWorkItemMatches(item) {
		return batchCommands(cmds...)
	}
	cmds = append(cmds, m.previewLoader(previewRequest{
		requestID:    m.preview.requestID,
		workItemRepo: item.Repo,
		workItemID:   item.ID,
		item:         item,
	}))
	return batchCommands(cmds...)
}

func batchCommands(cmds ...tea.Cmd) tea.Cmd {
	filtered := make([]tea.Cmd, 0, len(cmds))
	for _, cmd := range cmds {
		if cmd != nil {
			filtered = append(filtered, cmd)
		}
	}
	switch len(filtered) {
	case 0:
		return nil
	case 1:
		return filtered[0]
	default:
		return tea.Batch(filtered...)
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case previewResultMsg:
		m.handlePreviewResult(msg)
		return m, nil
	case pullRequestPreviewResultMsg:
		m.handlePullRequestPreviewResult(msg)
		return m, nil
	case actionResultMsg:
		m.handleActionResult(msg)
		return m, nil
	case workbenchReloadMsg:
		return m, m.handleWorkbenchReload(msg)
	case pullRequestLoadMsg:
		return m, m.handlePullRequestLoad(msg)
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, m.handleAction(actionQuit)
		}
		if m.pullRequestSearch {
			return m, m.handlePullRequestSearchKey(msg)
		}
		if m.pullRequestFilterUI {
			return m, m.handlePullRequestFilterKey(msg)
		}
		if action, ok := m.matchedAction(msg); ok {
			return m, m.handleAction(action)
		}
	}
	return m, nil
}

func (m *model) startPreviewLoadIfFocusedItemChanged() tea.Cmd {
	item, ok := m.selectedWorkItem()
	if !ok {
		m.clearFocusedWorkItem()
		m.preview = previewState{status: previewEmpty}
		return nil
	}
	if item.ID == "" {
		m.clearFocusedWorkItem()
		m.preview = previewState{status: previewEmpty}
		return nil
	}
	if m.focusedWorkItemMatches(item) {
		return nil
	}
	return m.startPreviewLoadForCurrentItem()
}

func (m *model) startPreviewLoadForCurrentItem() tea.Cmd {
	item, ok := m.selectedWorkItem()
	if !ok || item.ID == "" {
		m.clearFocusedWorkItem()
		m.preview = previewState{status: previewEmpty}
		return nil
	}

	m.nextPreviewRequestID++
	requestID := m.nextPreviewRequestID
	m.focusedWorkItemRepo = item.Repo
	m.focusedWorkItemID = item.ID
	m.preview = previewState{
		status:              previewLoading,
		requestID:           requestID,
		focusedWorkItemRepo: item.Repo,
		focusedWorkItemID:   item.ID,
	}
	if m.previewLoader == nil {
		return nil
	}
	return m.previewLoader(previewRequest{
		requestID:    requestID,
		workItemRepo: item.Repo,
		workItemID:   item.ID,
		item:         item,
	})
}

func (m *model) handlePreviewResult(msg previewResultMsg) {
	if msg.requestID != m.preview.requestID || msg.workItemID != m.focusedWorkItemID || msg.workItemRepo != m.focusedWorkItemRepo {
		return
	}

	next := previewState{
		requestID:           msg.requestID,
		focusedWorkItemRepo: msg.workItemRepo,
		focusedWorkItemID:   msg.workItemID,
	}
	switch {
	case msg.err != nil:
		next.status = previewError
		next.errorMessage = msg.err.Error()
	case msg.empty:
		next.status = previewEmpty
	default:
		next.status = previewLoaded
		next.loaded = msg.data
		if next.loaded.workItemID == "" {
			next.loaded.workItemID = msg.workItemID
		}
		if next.loaded.workItemRepo == (workbench.RepoRef{}) {
			next.loaded.workItemRepo = msg.workItemRepo
		}
	}
	m.preview = next
}

func (m *model) startPullRequestPreviewLoadIfFocusedChanged() tea.Cmd {
	pr, ok := m.selectedPullRequest()
	if !ok {
		m.pullRequestPreview = pullRequestPreviewState{status: previewEmpty}
		return nil
	}
	if pr.Key() == m.pullRequestPreview.focusedPullRequestKey {
		return nil
	}
	return m.startPullRequestPreviewLoadForCurrent()
}

func (m *model) startPullRequestPreviewLoadForCurrent() tea.Cmd {
	pr, ok := m.selectedPullRequest()
	if !ok {
		m.pullRequestPreview = pullRequestPreviewState{status: previewEmpty}
		return nil
	}

	m.nextPRPreviewRequest++
	requestID := m.nextPRPreviewRequest
	key := pr.Key()
	m.pullRequestPreview = pullRequestPreviewState{
		status:                previewLoading,
		requestID:             requestID,
		focusedPullRequestKey: key,
	}
	if m.prPreviewLoader == nil {
		return nil
	}
	return m.prPreviewLoader(pullRequestPreviewRequest{
		requestID:      requestID,
		pullRequestKey: key,
		pr:             pr,
	})
}

func (m *model) handlePullRequestPreviewResult(msg pullRequestPreviewResultMsg) {
	if msg.requestID != m.pullRequestPreview.requestID || msg.pullRequestKey != m.pullRequestPreview.focusedPullRequestKey {
		return
	}

	next := pullRequestPreviewState{
		requestID:             msg.requestID,
		focusedPullRequestKey: msg.pullRequestKey,
	}
	switch {
	case msg.err != nil:
		next.status = previewError
		next.errorMessage = msg.err.Error()
	case msg.empty:
		next.status = previewEmpty
	default:
		next.status = previewLoaded
		next.loaded = msg.data
		if next.loaded.pullRequestKey == "" {
			next.loaded.pullRequestKey = msg.pullRequestKey
		}
	}
	m.pullRequestPreview = next
}

func (m model) matchedAction(msg tea.KeyMsg) (actionID, bool) {
	for _, binding := range m.keys.actionBindings(m.activeView) {
		if key.Matches(msg, binding.binding) {
			return binding.id, true
		}
	}
	return "", false
}

func (m *model) handleAction(action actionID) tea.Cmd {
	switch action {
	case actionQuit:
		return tea.Quit
	case actionToggleHelp:
		m.help.ShowAll = !m.help.ShowAll
	case actionFocusNextPane:
		m.focusNextPane()
	case actionFocusPreviousPane:
		m.focusPreviousPane()
	case actionFocusPane1:
		m.focusPaneByNumber(1)
	case actionFocusPane2:
		m.focusPaneByNumber(2)
	case actionFocusPane3:
		m.focusPaneByNumber(3)
	case actionMoveDown:
		m.moveFocusedSelection(1)
		return m.startPreviewLoadAfterSelectionChange()
	case actionMoveUp:
		m.moveFocusedSelection(-1)
		return m.startPreviewLoadAfterSelectionChange()
	case actionJumpTop:
		m.jumpFocusedSelection(false)
		return m.startPreviewLoadAfterSelectionChange()
	case actionJumpBottom:
		m.jumpFocusedSelection(true)
		return m.startPreviewLoadAfterSelectionChange()
	case actionRefresh:
		cmd := m.refreshActiveData()
		if cmd == nil {
			m.statusMessage = "Refresh unavailable"
			return nil
		}
		return cmd
	case actionOpenPullRequest:
		return m.openPullRequest()
	case actionOpenSelected:
		return m.openSelected()
	case actionOpenIssue:
		return m.openIssue()
	case actionCopyURL:
		return m.copyURL()
	case actionCopyWorktreePath:
		return m.copyWorktreePath()
	case actionCopyPullRequestNumber:
		return m.copyPullRequestNumber()
	case actionCopyPullRequestHead:
		return m.copyPullRequestHead()
	case actionShowPullRequests:
		return m.showPullRequests()
	case actionShowWorkbench:
		return m.showWorkbench()
	case actionSearchPullRequests:
		m.startPullRequestSearch()
	case actionFilterPullRequests:
		m.togglePullRequestFilterUI()
	}
	return nil
}

func (m *model) startPreviewLoadAfterSelectionChange() tea.Cmd {
	if m.activeView == appViewPullRequests {
		if m.activePane() == paneRepositories {
			return m.startPullRequestLoad("Loading pull requests...")
		}
		return m.startPullRequestPreviewLoadIfFocusedChanged()
	}
	return m.startPreviewLoadIfFocusedItemChanged()
}

func (m *model) openPullRequest() tea.Cmd {
	if m.activeView == appViewPullRequests {
		return m.openSelectedPullRequest()
	}
	item, ok := m.selectedWorkItem()
	if !ok {
		m.statusMessage = "No work item selected"
		return nil
	}
	if item.PullRequest == nil || item.PullRequest.URL == "" {
		m.statusMessage = "No PR URL for selected work item"
		return nil
	}
	label := item.PullRequest.NumberLabel()
	m.statusMessage = "Opening " + label + "..."
	return m.actionCommand("Opened "+label, "Open PR failed", func(ctx context.Context) error {
		return m.runner().Open(ctx, item.PullRequest.URL)
	})
}

func (m *model) openSelected() tea.Cmd {
	if m.activeView == appViewPullRequests {
		return m.openSelectedPullRequest()
	}
	return m.openPullRequest()
}

func (m *model) openSelectedPullRequest() tea.Cmd {
	pr, ok := m.selectedPullRequest()
	if !ok {
		m.statusMessage = "No pull request selected"
		return nil
	}
	if pr.URL == "" {
		m.statusMessage = "No PR URL for selected pull request"
		return nil
	}
	label := pr.NumberLabel()
	m.statusMessage = "Opening " + label + "..."
	return m.actionCommand("Opened "+label, "Open PR failed", func(ctx context.Context) error {
		return m.runner().Open(ctx, pr.URL)
	})
}

func (m *model) openIssue() tea.Cmd {
	item, ok := m.selectedWorkItem()
	if !ok {
		m.statusMessage = "No work item selected"
		return nil
	}
	if item.Issue == nil || item.Issue.URL == "" {
		m.statusMessage = "No issue URL for selected work item"
		return nil
	}
	label := item.Issue.Label()
	m.statusMessage = "Opening " + label + "..."
	return m.actionCommand("Opened "+label, "Open issue failed", func(ctx context.Context) error {
		return m.runner().Open(ctx, item.Issue.URL)
	})
}

func (m *model) copyURL() tea.Cmd {
	if m.activeView == appViewPullRequests {
		return m.copyPullRequestURL()
	}
	item, ok := m.selectedWorkItem()
	if !ok {
		m.statusMessage = "No work item selected"
		return nil
	}
	label, target, ok := bestWorkItemURL(item)
	if !ok {
		m.statusMessage = "No URL for selected work item"
		return nil
	}
	m.statusMessage = "Copying " + label + " URL..."
	return m.actionCommand("Copied "+label+" URL", "Copy URL failed", func(ctx context.Context) error {
		return m.runner().Copy(ctx, target)
	})
}

func (m *model) copyPullRequestURL() tea.Cmd {
	pr, ok := m.selectedPullRequest()
	if !ok {
		m.statusMessage = "No pull request selected"
		return nil
	}
	if pr.URL == "" {
		m.statusMessage = "No PR URL for selected pull request"
		return nil
	}
	m.statusMessage = "Copying " + pr.NumberLabel() + " URL..."
	return m.actionCommand("Copied "+pr.NumberLabel()+" URL", "Copy URL failed", func(ctx context.Context) error {
		return m.runner().Copy(ctx, pr.URL)
	})
}

func (m *model) copyWorktreePath() tea.Cmd {
	item, ok := m.selectedWorkItem()
	if !ok {
		m.statusMessage = "No work item selected"
		return nil
	}
	if item.Worktree == nil || item.Worktree.Path == "" {
		m.statusMessage = "No worktree path for selected work item"
		return nil
	}
	m.statusMessage = "Copying worktree path..."
	return m.actionCommand("Copied worktree path", "Copy worktree path failed", func(ctx context.Context) error {
		return m.runner().Copy(ctx, item.Worktree.Path)
	})
}

func (m *model) copyPullRequestNumber() tea.Cmd {
	pr, ok := m.selectedPullRequest()
	if !ok {
		m.statusMessage = "No pull request selected"
		return nil
	}
	number := pr.ShortNumberLabel()
	m.statusMessage = "Copying " + number + "..."
	return m.actionCommand("Copied "+number, "Copy PR number failed", func(ctx context.Context) error {
		return m.runner().Copy(ctx, number)
	})
}

func (m *model) copyPullRequestHead() tea.Cmd {
	pr, ok := m.selectedPullRequest()
	if !ok {
		m.statusMessage = "No pull request selected"
		return nil
	}
	head := pr.HeadLabel()
	if head == "" {
		m.statusMessage = "No head ref for selected pull request"
		return nil
	}
	m.statusMessage = "Copying head ref..."
	return m.actionCommand("Copied head ref", "Copy head ref failed", func(ctx context.Context) error {
		return m.runner().Copy(ctx, head)
	})
}

func (m *model) actionCommand(success string, failure string, run func(context.Context) error) tea.Cmd {
	return func() tea.Msg {
		return actionResultMsg{
			success: success,
			failure: failure,
			err:     run(context.Background()),
		}
	}
}

func (m *model) handleActionResult(msg actionResultMsg) {
	if msg.err != nil {
		m.statusMessage = fmt.Sprintf("%s: %v", msg.failure, msg.err)
		return
	}
	m.statusMessage = msg.success
}

func (m model) runner() actionRunner {
	if m.actionRunner != nil {
		return m.actionRunner
	}
	return systemActionRunner{}
}

func bestWorkItemURL(item workbench.WorkItem) (string, string, bool) {
	if item.PullRequest != nil && item.PullRequest.URL != "" {
		return "PR", item.PullRequest.URL, true
	}
	if item.Issue != nil && item.Issue.URL != "" {
		return "issue", item.Issue.URL, true
	}
	return "", "", false
}

func (m *model) refreshWorkbenchData() tea.Cmd {
	return m.startWorkbenchReload("Reloading workbench data...")
}

func (m *model) refreshActiveData() tea.Cmd {
	if m.activeView == appViewPullRequests {
		return m.startPullRequestLoad("Reloading pull requests...")
	}
	return m.refreshWorkbenchData()
}

func (m *model) showPullRequests() tea.Cmd {
	selectedWorkItemRepo := workbench.RepoRef{}
	selectedWorkItemID := ""
	m.pendingPullRequest = 0
	if item, ok := m.selectedWorkItem(); ok {
		selectedWorkItemRepo = item.Repo
		selectedWorkItemID = item.ID
		if item.Repo != (workbench.RepoRef{}) {
			m.restoreSelectedRepo(item.Repo)
		}
		if item.PullRequest != nil {
			m.pendingPullRequest = item.PullRequest.Number
			m.ensurePullRequestFromWorkItem(item)
		}
	}
	m.activeView = appViewPullRequests
	m.focusedPane = panePullRequests
	m.viewSelected = false
	m.restoreSelectedWorkItem(selectedWorkItemRepo, selectedWorkItemID)
	if m.pendingPullRequest > 0 {
		m.restoreSelectedPullRequest(m.pendingPullRequest)
	}
	loadCmd := m.startPullRequestLoad("Loading pull requests...")
	if m.pullRequestsLoading {
		return batchCommands(loadCmd, m.startPullRequestPreviewLoadForCurrent())
	}
	return loadCmd
}

func (m *model) showWorkbench() tea.Cmd {
	m.activeView = appViewWorkbench
	m.focusedPane = paneWorkItems
	m.pullRequestSearch = false
	m.pullRequestFilterUI = false
	return m.startPreviewLoadIfFocusedItemChanged()
}

func (m *model) ensurePullRequestFromWorkItem(item workbench.WorkItem) {
	if item.PullRequest == nil {
		return
	}
	if item.Repo != (workbench.RepoRef{}) {
		if m.pullRequestRepo != (workbench.RepoRef{}) && m.pullRequestRepo != item.Repo {
			m.pullRequests = nil
			m.selectedPR = 0
		}
		m.pullRequestRepo = item.Repo
	}
	pr := pullRequestFromWorkItem(item)
	for i, existing := range m.pullRequests {
		if existing.Number == pr.Number {
			m.pullRequests[i] = pr
			return
		}
	}
	m.pullRequests = append(m.pullRequests, pr)
	pullrequests.SortByUpdatedDesc(m.pullRequests)
}

func pullRequestFromWorkItem(item workbench.WorkItem) pullrequests.PullRequest {
	pr := item.PullRequest
	return pullrequests.PullRequest{
		Number:                pr.Number,
		Title:                 pr.Title,
		State:                 pr.State,
		IsDraft:               pr.IsDraft,
		Author:                pr.AuthorLogin,
		HeadRef:               pr.HeadLabel(),
		BaseRef:               pr.BaseBranch,
		ReviewDecision:        pr.ReviewState,
		ReviewRequests:        pullRequestReviewRequestsFromWorkbench(pr.ReviewRequests),
		LatestReviews:         pullRequestLatestReviewsFromWorkbench(pr.LatestReviews),
		LinkedIssues:          pullRequestLinkedIssuesFromWorkbench(pr.LinkedIssues),
		Checks:                pullRequestChecksFromWorkbench(item.Checks),
		Mergeability:          pr.Mergeability,
		UpdatedAt:             pr.UpdatedAt,
		URL:                   pr.URL,
		BodyExcerpt:           pr.BodyExcerpt,
		ViewerReviewRequested: pr.ViewerReviewRequested,
		WaitingOnReview:       pr.WaitingOnReview,
	}
}

func pullRequestReviewRequestsFromWorkbench(requests []workbench.ReviewRequestRef) []pullrequests.ReviewRequest {
	out := make([]pullrequests.ReviewRequest, 0, len(requests))
	for _, request := range requests {
		out = append(out, pullrequests.ReviewRequest{
			Kind:  request.Kind,
			Login: request.Login,
			Name:  request.Name,
			Slug:  request.Slug,
		})
	}
	return out
}

func pullRequestLatestReviewsFromWorkbench(reviews []workbench.PullRequestReviewRef) []pullrequests.Review {
	out := make([]pullrequests.Review, 0, len(reviews))
	for _, review := range reviews {
		out = append(out, pullrequests.Review{
			Author: review.AuthorLogin,
			State:  review.State,
		})
	}
	return out
}

func pullRequestLinkedIssuesFromWorkbench(issues []workbench.IssueRef) []pullrequests.LinkedIssue {
	out := make([]pullrequests.LinkedIssue, 0, len(issues))
	for _, issue := range issues {
		out = append(out, pullrequests.LinkedIssue{
			Number: issue.Number,
			Title:  issue.Title,
			State:  issue.State,
			URL:    issue.URL,
		})
	}
	return out
}

func pullRequestChecksFromWorkbench(checks workbench.CheckSummary) pullrequests.CheckSummary {
	return pullrequests.CheckSummary{
		State:   pullrequests.CheckState(checks.State),
		Passing: checks.Passing,
		Failing: checks.Failing,
		Pending: checks.Pending,
	}
}

func (m *model) startPullRequestSearch() {
	if m.activeView != appViewPullRequests {
		return
	}
	m.pullRequestSearch = true
	m.pullRequestFilterUI = false
	m.pullRequestSearchIn = m.pullRequestFilter.TextQuery
}

func (m *model) togglePullRequestFilterUI() {
	if m.activeView != appViewPullRequests {
		return
	}
	m.pullRequestFilterUI = !m.pullRequestFilterUI
	m.pullRequestSearch = false
}

func (m *model) handlePullRequestSearchKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyEnter:
		m.pullRequestSearch = false
		m.pullRequestFilter.TextQuery = strings.TrimSpace(m.pullRequestSearchIn)
		m.selectedPR = 0
		return m.startPullRequestPreviewLoadForCurrent()
	case tea.KeyEsc:
		m.pullRequestSearch = false
		return nil
	case tea.KeyBackspace:
		runes := []rune(m.pullRequestSearchIn)
		if len(runes) > 0 {
			m.pullRequestSearchIn = string(runes[:len(runes)-1])
		}
	case tea.KeyRunes:
		m.pullRequestSearchIn += string(msg.Runes)
	}
	return nil
}

func (m *model) handlePullRequestFilterKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyEnter, tea.KeyEsc:
		m.pullRequestFilterUI = false
		return nil
	case tea.KeyRunes:
		if len(msg.Runes) == 0 {
			return nil
		}
		switch msg.Runes[0] {
		case 'f':
			m.pullRequestFilterUI = false
			return nil
		case 's':
			m.pullRequestFilter = m.pullRequestFilter.NextState()
		case 'a':
			m.togglePullRequestAuthorFilter()
		case 'r':
			m.pullRequestFilter.ReviewRequested = !m.pullRequestFilter.ReviewRequested
		case 'w':
			m.pullRequestFilter.WaitingOnReview = !m.pullRequestFilter.WaitingOnReview
		case 'c':
			m.pullRequestFilter.FailedChecks = !m.pullRequestFilter.FailedChecks
		case 'd':
			m.pullRequestFilter = m.pullRequestFilter.NextDraft()
		case 'x':
			m.pullRequestFilter = pullrequests.PullRequestFilter{}
		default:
			return nil
		}
		m.selectedPR = 0
		return m.startPullRequestPreviewLoadForCurrent()
	}
	return nil
}

func (m *model) togglePullRequestAuthorFilter() {
	pr, ok := m.selectedPullRequest()
	if !ok || pr.Author == "" {
		m.pullRequestFilter.Author = ""
		return
	}
	if strings.EqualFold(m.pullRequestFilter.Author, pr.Author) {
		m.pullRequestFilter.Author = ""
		return
	}
	m.pullRequestFilter.Author = pr.Author
}

func (m *model) startWorkbenchReload(status string) tea.Cmd {
	if !m.beginWorkbenchReload(status) {
		return nil
	}
	return m.workbenchReloadCommand(m.activeReloadRequest)
}

func (m *model) beginWorkbenchReload(status string) bool {
	if m.workbenchReloader == nil {
		return false
	}
	repo, _ := m.selectedRepoRef()
	m.nextReloadRequestID++
	request := workbenchReloadRequest{
		requestID: m.nextReloadRequestID,
		repo:      repo,
		status:    status,
	}
	m.activeReloadRequest = request
	m.workbenchLoading = true
	m.statusMessage = status
	return true
}

func (m model) workbenchReloadCommand(request workbenchReloadRequest) tea.Cmd {
	return func() tea.Msg {
		return workbenchReloadMsg{
			request: request,
			result:  m.workbenchReloader.Load(context.Background(), request.repo),
		}
	}
}

func (m *model) handleWorkbenchReload(msg workbenchReloadMsg) tea.Cmd {
	if msg.request != m.activeReloadRequest {
		return nil
	}
	repo, ok := m.selectedRepoRef()
	if msg.request.repo != (workbench.RepoRef{}) && (!ok || repo != msg.request.repo) {
		m.workbenchLoading = false
		if m.statusMessage == msg.request.status {
			m.statusMessage = ""
		}
		return nil
	}

	selectedWorkItemRepo := workbench.RepoRef{}
	selectedWorkItemID := ""
	if item, ok := m.selectedWorkItem(); ok {
		selectedWorkItemRepo = item.Repo
		selectedWorkItemID = item.ID
	}
	if len(msg.result.Repositories) > 0 {
		m.replaceWorkbenchData(msg.result, msg.request.repo)
	} else {
		m.workItems = replaceRepoWorkItems(m.workItems, msg.request.repo, msg.result.Items)
	}
	m.restoreSelectedWorkItem(selectedWorkItemRepo, selectedWorkItemID)
	m.workbenchLoading = false
	if hasWorkbenchErrorItems(msg.result.Items) {
		m.statusMessage = "Workbench loaded with partial errors"
	} else {
		m.statusMessage = ""
	}
	return m.startPreviewLoadForCurrentItem()
}

func (m *model) startPullRequestLoad(status string) tea.Cmd {
	if !m.beginPullRequestLoad(status) {
		return m.startPullRequestPreviewLoadForCurrent()
	}
	return m.pullRequestLoadCommand(m.activePRLoadRequest)
}

func (m *model) beginPullRequestLoad(status string) bool {
	if m.pullRequestService == nil {
		if repo, ok := m.selectedRepoRef(); ok {
			if m.pullRequestRepo != (workbench.RepoRef{}) && m.pullRequestRepo != repo {
				m.pullRequests = nil
				m.selectedPR = 0
				m.pullRequestPreview = pullRequestPreviewState{status: previewEmpty}
			}
			m.pullRequestRepo = repo
		}
		m.pullRequestsLoading = false
		return false
	}
	repo, ok := m.selectedRepoRef()
	if !ok {
		return false
	}
	if m.pullRequestRepo != (workbench.RepoRef{}) && m.pullRequestRepo != repo {
		m.pullRequests = nil
		m.selectedPR = 0
		m.pullRequestPreview = pullRequestPreviewState{status: previewEmpty}
	}
	m.nextPRLoadRequestID++
	request := pullRequestLoadRequest{
		requestID: m.nextPRLoadRequestID,
		repo:      repo,
		status:    status,
	}
	m.activePRLoadRequest = request
	m.pullRequestsLoading = true
	m.statusMessage = status
	return true
}

func (m model) pullRequestLoadCommand(request pullRequestLoadRequest) tea.Cmd {
	return func() tea.Msg {
		prs, err := m.pullRequestService.List(context.Background(), request.repo.FullName(), pullrequests.PullRequestFilter{})
		return pullRequestLoadMsg{
			request: request,
			prs:     prs,
			err:     err,
		}
	}
}

func (m *model) handlePullRequestLoad(msg pullRequestLoadMsg) tea.Cmd {
	if msg.request != m.activePRLoadRequest {
		return nil
	}
	repo, ok := m.selectedRepoRef()
	if !ok || repo != msg.request.repo || m.activeView != appViewPullRequests {
		m.pullRequestsLoading = false
		m.pendingPullRequest = 0
		if m.statusMessage == msg.request.status {
			m.statusMessage = ""
		}
		return nil
	}

	selectedNumber := m.pendingPullRequest
	if selectedNumber == 0 {
		if pr, ok := m.selectedPullRequest(); ok {
			selectedNumber = pr.Number
		}
	}
	if msg.err != nil {
		m.pullRequestsLoading = false
		m.pendingPullRequest = 0
		m.statusMessage = "Pull requests failed: " + msg.err.Error()
		return nil
	}
	m.pullRequests = append([]pullrequests.PullRequest(nil), msg.prs...)
	pullrequests.SortByUpdatedDesc(m.pullRequests)
	m.pullRequestRepo = msg.request.repo
	m.restoreSelectedPullRequest(selectedNumber)
	m.pendingPullRequest = 0
	m.pullRequestsLoading = false
	if m.statusMessage == msg.request.status {
		m.statusMessage = ""
	}
	return m.startPullRequestPreviewLoadForCurrent()
}

func (m *model) focusNextPane() {
	m.focusedPane = nextPane(m.activePane(), m.paneOrder())
}

func (m *model) focusPreviousPane() {
	m.focusedPane = previousPane(m.activePane(), m.paneOrder())
}

func (m *model) focusPaneByNumber(number int) {
	order := m.paneOrder()
	index := number - 1
	if index < 0 || index >= len(order) {
		return
	}
	m.focusedPane = order[index]
}

// paneOrder is the visible pane traversal order for tab navigation.
func (m model) paneOrder() []paneFocus {
	listPane := paneWorkItems
	if m.activeView == appViewPullRequests {
		listPane = panePullRequests
	}
	if m.isCompact() {
		return []paneFocus{listPane, panePreview}
	}
	return []paneFocus{paneRepositories, listPane, panePreview}
}

// activePane normalizes focus when the current layout hides a pane.
func (m model) activePane() paneFocus {
	focus := m.focusedPane
	for _, pane := range m.paneOrder() {
		if focus == pane {
			return focus
		}
	}
	if m.activeView == appViewPullRequests {
		return panePullRequests
	}
	return paneWorkItems
}

func (m model) isCompact() bool {
	return m.effectiveWidth() < fullLayoutMinWidth
}

func (m model) effectiveWidth() int {
	if m.width <= 0 {
		return defaultWidth
	}
	return m.width
}

func nextPane(current paneFocus, order []paneFocus) paneFocus {
	for i, pane := range order {
		if pane == current {
			return order[(i+1)%len(order)]
		}
	}
	return order[0]
}

func previousPane(current paneFocus, order []paneFocus) paneFocus {
	for i, pane := range order {
		if pane == current {
			return order[(i+len(order)-1)%len(order)]
		}
	}
	return order[0]
}

// moveFocusedSelection keeps j/k scoped to the active pane.
func (m *model) moveFocusedSelection(delta int) {
	switch m.activePane() {
	case paneRepositories:
		m.moveRepoSelection(delta)
	case paneWorkItems:
		m.moveWorkItemSelection(delta)
	case panePullRequests:
		m.movePullRequestSelection(delta)
	}
}

// jumpFocusedSelection keeps g/G behavior aligned with the active pane.
func (m *model) jumpFocusedSelection(toEnd bool) {
	switch m.activePane() {
	case paneRepositories:
		if toEnd {
			m.setRepoPaneIndex(m.repoPaneEntryCount() - 1)
			return
		}
		m.setRepoPaneIndex(0)
	case paneWorkItems:
		items := m.visibleWorkItems()
		if toEnd {
			if len(items) > 0 {
				m.selectedItem = len(items) - 1
			}
			return
		}
		m.selectedItem = 0
	case panePullRequests:
		prs := m.visiblePullRequests()
		if toEnd {
			if len(prs) > 0 {
				m.selectedPR = len(prs) - 1
			}
			return
		}
		m.selectedPR = 0
	}
}

func (m *model) moveRepoSelection(delta int) {
	count := m.repoPaneEntryCount()
	if count == 0 {
		m.selectedRepo = 0
		m.selectedView = 0
		m.viewSelected = false
		return
	}

	m.setRepoPaneIndex(clamp(m.repoPaneIndex()+delta, 0, count-1))
}

func (m *model) moveWorkItemSelection(delta int) {
	items := m.visibleWorkItems()
	if len(items) == 0 {
		m.selectedItem = 0
		return
	}

	m.selectedItem += delta
	if m.selectedItem < 0 {
		m.selectedItem = 0
	}
	if m.selectedItem >= len(items) {
		m.selectedItem = len(items) - 1
	}
}

func (m *model) movePullRequestSelection(delta int) {
	prs := m.visiblePullRequests()
	if len(prs) == 0 {
		m.selectedPR = 0
		return
	}

	m.selectedPR += delta
	if m.selectedPR < 0 {
		m.selectedPR = 0
	}
	if m.selectedPR >= len(prs) {
		m.selectedPR = len(prs) - 1
	}
}

func (m model) repoPaneEntryCount() int {
	if m.activeView == appViewPullRequests {
		return len(m.repos)
	}
	return len(m.repos) + len(repoViews)
}

func (m model) repoPaneIndex() int {
	if m.viewSelected {
		return len(m.repos) + clamp(m.selectedView, 0, max(len(repoViews)-1, 0))
	}
	return clamp(m.selectedRepo, 0, max(len(m.repos)-1, 0))
}

func (m *model) setRepoPaneIndex(index int) {
	count := m.repoPaneEntryCount()
	if count == 0 {
		m.selectedRepo = 0
		m.selectedView = 0
		m.viewSelected = false
		m.selectedItem = 0
		return
	}

	index = clamp(index, 0, count-1)
	if m.activeView == appViewPullRequests || index < len(m.repos) {
		m.selectedRepo = index
		m.viewSelected = false
	} else {
		m.selectedView = index - len(m.repos)
		m.viewSelected = true
	}
	m.selectedItem = 0
}

func (m model) visibleWorkItems() []workbench.WorkItem {
	if m.viewSelected {
		view, ok := m.selectedRepoView()
		if !ok {
			return nil
		}
		return filterWorkItems(m.workItems, func(item workbench.WorkItem) bool {
			return view.matches(item) && matchesWorkbenchFilter(item, m.workbenchFilter)
		})
	}

	repo, ok := m.selectedRepoRef()
	if !ok {
		return nil
	}
	return filterWorkItems(m.workItems, func(item workbench.WorkItem) bool {
		return item.Repo == repo && matchesWorkbenchFilter(item, m.workbenchFilter)
	})
}

func (m model) selectedRepoRef() (workbench.RepoRef, bool) {
	if len(m.repos) == 0 || m.selectedRepo < 0 || m.selectedRepo >= len(m.repos) {
		return workbench.RepoRef{}, false
	}
	return m.repos[m.selectedRepo], true
}

func (m model) selectedRepoSummary() (workbench.RepositorySummary, bool) {
	repo, ok := m.selectedRepoRef()
	if !ok {
		return workbench.RepositorySummary{}, false
	}
	return m.repoSummary(repo)
}

func (m model) repoSummary(repo workbench.RepoRef) (workbench.RepositorySummary, bool) {
	for _, summary := range m.repoSummaries {
		if summary.Repo == repo {
			return summary, true
		}
	}
	return workbench.RepositorySummary{}, false
}

func (m model) selectedRepoView() (repoView, bool) {
	if m.selectedView < 0 || m.selectedView >= len(repoViews) {
		return repoView{}, false
	}
	return repoViews[m.selectedView], true
}

func filterWorkItems(items []workbench.WorkItem, keep func(workbench.WorkItem) bool) []workbench.WorkItem {
	out := make([]workbench.WorkItem, 0, len(items))
	for _, item := range items {
		if keep(item) {
			out = append(out, item)
		}
	}
	return out
}

func replaceRepoWorkItems(items []workbench.WorkItem, repo workbench.RepoRef, replacement []workbench.WorkItem) []workbench.WorkItem {
	out := make([]workbench.WorkItem, 0, len(items)+len(replacement))
	replaced := false
	for _, item := range items {
		if item.Repo == repo {
			if !replaced {
				out = append(out, replacement...)
				replaced = true
			}
			continue
		}
		out = append(out, item)
	}
	if !replaced {
		out = append(out, replacement...)
	}
	return out
}

func (m *model) replaceWorkbenchData(result workbench.RuntimeLoadResult, selectedRepo workbench.RepoRef) {
	m.repoSummaries = cloneRepositorySummaries(result.Repositories)
	m.repos = repoRefsFromSummaries(m.repoSummaries)
	m.workItems = cloneWorkItems(result.Items)
	m.restoreSelectedRepo(selectedRepo)
}

func (m *model) restoreSelectedRepo(repo workbench.RepoRef) {
	if repo != (workbench.RepoRef{}) {
		for i, candidate := range m.repos {
			if candidate == repo {
				m.selectedRepo = i
				return
			}
		}
	}
	if len(m.repos) == 0 {
		m.selectedRepo = 0
		m.viewSelected = false
		return
	}
	m.selectedRepo = clamp(m.selectedRepo, 0, len(m.repos)-1)
	if m.viewSelected {
		m.selectedView = clamp(m.selectedView, 0, max(len(repoViews)-1, 0))
	}
}

func hasWorkbenchErrorItems(items []workbench.WorkItem) bool {
	for _, item := range items {
		for _, prefix := range workbenchErrorIDPrefixes {
			if strings.HasPrefix(item.ID, prefix) {
				return true
			}
		}
	}
	return false
}

func (m *model) restoreSelectedWorkItem(repo workbench.RepoRef, workItemID string) {
	items := m.visibleWorkItems()
	if len(items) == 0 {
		m.selectedItem = 0
		return
	}
	if workItemID != "" {
		for i, item := range items {
			if item.Repo == repo && item.ID == workItemID {
				m.selectedItem = i
				return
			}
		}
	}
	m.selectedItem = clamp(m.selectedItem, 0, len(items)-1)
}

func cloneRepoRefs(repos []workbench.RepoRef) []workbench.RepoRef {
	return append([]workbench.RepoRef(nil), repos...)
}

func normalizeRepositorySummaries(summaries []workbench.RepositorySummary, repos []workbench.RepoRef) []workbench.RepositorySummary {
	if len(summaries) > 0 {
		return cloneRepositorySummaries(summaries)
	}
	out := make([]workbench.RepositorySummary, 0, len(repos))
	for _, repo := range repos {
		out = append(out, workbench.RepositorySummary{Repo: repo})
	}
	return out
}

func repoRefsFromSummaries(summaries []workbench.RepositorySummary) []workbench.RepoRef {
	repos := make([]workbench.RepoRef, 0, len(summaries))
	for _, summary := range summaries {
		repos = append(repos, summary.Repo)
	}
	return repos
}

func cloneRepositorySummaries(summaries []workbench.RepositorySummary) []workbench.RepositorySummary {
	out := append([]workbench.RepositorySummary(nil), summaries...)
	for i := range out {
		out[i].Remotes = append([]string(nil), summaries[i].Remotes...)
	}
	return out
}

func cloneWorkItems(items []workbench.WorkItem) []workbench.WorkItem {
	return append([]workbench.WorkItem(nil), items...)
}

func (v repoView) matches(item workbench.WorkItem) bool {
	switch v.filter {
	case repoViewActiveWorktrees:
		return item.Worktree != nil
	case repoViewNeedsMyReview:
		return item.PullRequest != nil && item.PullRequest.ViewerReviewRequested
	case repoViewWaitingOnReview:
		return item.PullRequest != nil && item.PullRequest.WaitingOnReview
	case repoViewFailedChecks:
		return item.Checks.State == workbench.CheckFailing
	default:
		return false
	}
}

func (m *model) applyStartupView(view cfgpkg.StartupView) {
	if view == cfgpkg.StartupViewWorkbench {
		m.viewSelected = false
	}
}

func (m *model) applyStartupRepo(repoName string) {
	owner, name, ok := strings.Cut(repoName, "/")
	if !ok || owner == "" || name == "" {
		return
	}
	repo := workbench.RepoRef{Owner: owner, Name: name}
	for i, existing := range m.repos {
		if existing == repo {
			m.selectedRepo = i
			m.viewSelected = false
			m.selectedItem = 0
			m.ensureRepositorySummary(repo)
			return
		}
	}
	m.repos = append(m.repos, repo)
	m.repoSummaries = append(m.repoSummaries, workbench.RepositorySummary{Repo: repo})
	m.selectedRepo = len(m.repos) - 1
	m.viewSelected = false
	m.selectedItem = 0
}

func (m *model) ensureRepositorySummary(repo workbench.RepoRef) {
	for _, summary := range m.repoSummaries {
		if summary.Repo == repo {
			return
		}
	}
	m.repoSummaries = append(m.repoSummaries, workbench.RepositorySummary{Repo: repo})
}

func matchesWorkbenchFilter(item workbench.WorkItem, filter cfgpkg.WorkbenchFilter) bool {
	if filter.Worktree != "" {
		if item.Worktree == nil || !matchPathFilter(filter.Worktree, item.Worktree.Path) {
			return false
		}
	}
	if filter.BranchPattern != "" {
		if item.Branch == nil || !matchBranchFilter(filter.BranchPattern, item.Branch.Name) {
			return false
		}
	}
	switch filter.PullRequest {
	case "", cfgpkg.PullRequestAny:
	case cfgpkg.PullRequestPresent:
		if item.PullRequest == nil {
			return false
		}
	case cfgpkg.PullRequestAbsent:
		if item.PullRequest != nil {
			return false
		}
	}
	switch filter.LocalStatus {
	case "", cfgpkg.LocalStatusAny:
	default:
		if item.Local == nil || string(item.Local.State) != string(filter.LocalStatus) {
			return false
		}
	}
	return true
}

func workbenchFilterActive(filter cfgpkg.WorkbenchFilter) bool {
	return filter.Worktree != "" ||
		filter.BranchPattern != "" ||
		(filter.PullRequest != "" && filter.PullRequest != cfgpkg.PullRequestAny) ||
		(filter.LocalStatus != "" && filter.LocalStatus != cfgpkg.LocalStatusAny)
}

func pullRequestFilterFromConfig(filter cfgpkg.PullRequestsFilter) pullrequests.PullRequestFilter {
	return pullrequests.PullRequestFilter{
		State:           pullrequests.StateFilter(filter.State),
		Author:          filter.Author,
		ReviewRequested: filter.ReviewRequested,
		WaitingOnReview: filter.WaitingOnReview,
		FailedChecks:    filter.FailedChecks,
		Draft:           pullrequests.DraftFilter(filter.Draft),
		TextQuery:       filter.TextQuery,
	}.Normalize()
}

func matchBranchFilter(pattern string, branch string) bool {
	return matchFilterPattern(pattern, branch, path.Match)
}

func matchPathFilter(pattern string, value string) bool {
	return matchFilterPattern(pattern, value, filepath.Match)
}

func matchFilterPattern(pattern string, value string, match func(pattern string, name string) (bool, error)) bool {
	if pattern == value {
		return true
	}
	matched, err := match(pattern, value)
	return err == nil && matched
}

func (m model) View() string {
	width := m.effectiveWidth()

	if width < fullLayoutMinWidth {
		return m.renderCompact(width)
	}
	return m.renderFull(width)
}

func (m model) renderFull(width int) string {
	repoWidth, listWidth, rightWidth := m.fullPaneWidths(width)
	focus := m.activePane()
	listPane := m.listPane()

	left := m.repoLines(paneTextWidth(repoWidth), focus == paneRepositories)
	middle := m.listLines(paneTextWidth(listWidth), focus == listPane)
	right := m.previewLines(paneTextWidth(rightWidth))
	out := m.headerLines(m.viewTitle(), width)
	bodyHeight := m.frameBodyHeight(max(len(left), max(len(middle), len(right))), len(out))

	body := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.renderPane(m.paneHeading(paneRepositories), left, repoWidth, bodyHeight, focus == paneRepositories),
		paneGap(bodyHeight+frameBorderLines),
		m.renderPane(m.paneHeading(listPane), middle, listWidth, bodyHeight, focus == listPane),
		paneGap(bodyHeight+frameBorderLines),
		m.renderPane(m.paneHeading(panePreview), right, rightWidth, bodyHeight, focus == panePreview),
	)
	out = append(out, body)
	return strings.Join(out, "\n") + "\n"
}

func (m model) fullPaneWidths(width int) (int, int, int) {
	listWidth := workItemPaneWidth
	rightWidth := width - repoPaneWidth - listWidth - paneBorderWidth*3 - paneGapWidth*2
	if m.activeView != appViewPullRequests {
		return repoPaneWidth, listWidth, max(rightWidth, 0)
	}

	available := width - repoPaneWidth - paneBorderWidth*3 - paneGapWidth*2
	if available <= 0 {
		return repoPaneWidth, 0, 0
	}
	previewRatio := m.pullRequestPreviewWidth
	if previewRatio <= 0 || previewRatio >= 1 {
		previewRatio = cfgpkg.Defaults().PullRequests.PreviewWidth
	}
	rightWidth = max(int(float64(available)*previewRatio), previewPaneMinWidth)
	listWidth = max(available-rightWidth, 0)
	if listWidth < pullRequestPaneMinWidth {
		listWidth = min(max(available-previewPaneMinWidth, 0), pullRequestPaneMinWidth)
		rightWidth = max(available-listWidth, 0)
	}
	return repoPaneWidth, listWidth, rightWidth
}

func (m model) renderCompact(width int) string {
	contentWidth := max(width-paneBorderWidth, 0)
	focus := m.activePane()
	listPane := m.listPane()
	out := m.headerLines(m.viewTitle(), width)

	workLines := m.listLines(paneTextWidth(contentWidth), focus == listPane)
	previewLines := m.previewLines(paneTextWidth(contentWidth))
	workHeight := len(workLines)
	previewHeight := len(previewLines)
	if m.height > 0 {
		availableContentHeight := m.height - len(out) - frameBorderLines*2
		if availableContentHeight > workHeight+previewHeight {
			previewHeight += availableContentHeight - workHeight - previewHeight
		}
	}

	out = append(out,
		m.renderPane(m.paneHeading(listPane), workLines, contentWidth, workHeight, focus == listPane),
		m.renderPane(m.paneHeading(panePreview), previewLines, contentWidth, previewHeight, focus == panePreview),
	)
	return strings.Join(out, "\n") + "\n"
}

func (m model) listPane() paneFocus {
	if m.activeView == appViewPullRequests {
		return panePullRequests
	}
	return paneWorkItems
}

func (m model) viewTitle() string {
	if m.activeView == appViewPullRequests {
		return "gh-zen  pull requests"
	}
	if m.isCompact() {
		return "gh-zen workbench"
	}
	return "gh-zen  repository workbench"
}

func (m model) listLines(width int, focused bool) []string {
	if m.activeView == appViewPullRequests {
		return m.pullRequestLines(width, focused)
	}
	return m.workItemLines(width, focused)
}

func (m model) repoLines(width int, focused bool) []string {
	lines := []string{}
	if len(m.repos) == 0 {
		lines = append(lines, "  none")
	} else {
		for i, repo := range m.repos {
			marker := selectionMarker(!m.viewSelected && i == m.selectedRepo, focused)
			label := repo.FullName()
			if counts := m.repositoryCountLabel(repo); counts != "" {
				label += " " + counts
			}
			lines = append(lines, truncate(fmt.Sprintf("%s %s", marker, label), width))
		}
	}
	if m.activeView == appViewPullRequests {
		return lines
	}
	lines = append(lines, "", "Views")
	for i, view := range repoViews {
		marker := selectionMarker(m.viewSelected && i == m.selectedView, focused)
		lines = append(lines, truncate(fmt.Sprintf("%s %s", marker, view.label), width))
	}
	return lines
}

func (m model) workItemLines(width int, focused bool) []string {
	items := m.visibleWorkItems()
	if len(items) == 0 {
		return []string{m.emptyWorkItemLine()}
	}
	lines := []string{}
	for i, item := range items {
		marker := selectionMarker(i == m.selectedItem, focused)
		row := fmt.Sprintf("%s %-22s %-7s %s", marker, item.Title(), item.LocalLabel(), shortRemoteLabel(item))
		lines = append(lines, truncate(row, width))
	}
	return lines
}

func (m model) pullRequestLines(width int, focused bool) []string {
	prs := m.visiblePullRequests()
	if len(prs) == 0 {
		return []string{m.emptyPullRequestLine()}
	}
	lines := []string{}
	for i, pr := range prs {
		marker := selectionMarker(i == m.selectedPR, focused)
		row := fmt.Sprintf("%s %-5s %-10s %-18s %-10s %-16s %-18s %s",
			marker,
			pr.ShortNumberLabel(),
			truncate(pr.StateLabel(), 10),
			truncate(pr.Title, 18),
			truncate(authorLabel(pr.Author), 10),
			truncate(shortReviewLabel(pr), 16),
			truncate(pr.Checks.Label(), 18),
			shortUpdatedAt(pr.UpdatedAt),
		)
		lines = append(lines, truncate(row, width))
	}
	return lines
}

func (m model) emptyWorkItemLine() string {
	if m.workbenchLoading {
		return "  loading workbench data..."
	}
	if workbenchFilterActive(m.workbenchFilter) {
		return "  no work items match filters"
	}
	if m.workbenchSource == workbenchDataLive {
		return "  no live work items"
	}
	return "  no work items"
}

func (m model) emptyPullRequestLine() string {
	if m.pullRequestsLoading {
		return "  loading pull requests..."
	}
	if m.pullRequestFilter.Active() {
		return "  no pull requests match filters"
	}
	if m.pullRequestRepo != (workbench.RepoRef{}) {
		return "  no pull requests"
	}
	return "  no pull request data"
}

func authorLabel(author string) string {
	if author == "" {
		return "@unknown"
	}
	return "@" + author
}

func shortReviewLabel(pr pullrequests.PullRequest) string {
	switch {
	case pr.ReviewDecision != "":
		return pr.ReviewDecision
	case len(pr.ReviewRequests) > 0:
		return "review requested"
	default:
		return "no review"
	}
}

func shortUpdatedAt(value string) string {
	if len(value) >= len("2006-01-02") {
		return value[:len("2006-01-02")]
	}
	if value == "" {
		return "unknown"
	}
	return value
}

func (m model) previewLines(width int) []string {
	if m.activeView == appViewPullRequests {
		return m.pullRequestPreviewLines(width)
	}
	if m.activePane() == paneRepositories && !m.viewSelected {
		if summary, ok := m.selectedRepoSummary(); ok {
			return repositoryPreviewLines(summary, width)
		}
	}
	switch m.preview.status {
	case previewLoading:
		return m.previewStatusLines(width, "Loading preview...")
	case previewLoaded:
		if m.preview.loaded.workItemID != m.focusedWorkItemID || m.preview.loaded.workItemRepo != m.focusedWorkItemRepo {
			return m.previewStatusLines(width, "Loading preview...")
		}
		return workItemPreviewLines(m.preview.loaded.item, width)
	case previewEmpty:
		if _, ok := m.selectedWorkItem(); !ok {
			return []string{"  no work item selected"}
		}
		return m.previewStatusLines(width, "No preview data")
	case previewError:
		lines := m.previewStatusLines(width, "Preview failed")
		if m.preview.errorMessage != "" {
			lines = append(lines, truncate("Error: "+m.preview.errorMessage, width))
		}
		return lines
	default:
		if _, ok := m.selectedWorkItem(); !ok {
			return []string{"  no work item selected"}
		}
		return m.previewStatusLines(width, "Preview idle")
	}
}

func repositoryPreviewLines(summary workbench.RepositorySummary, width int) []string {
	path := summary.Path
	if path == "" {
		path = "unknown"
	}
	defaultBranch := summary.DefaultBranch
	if defaultBranch == "" {
		defaultBranch = "unknown"
	}
	remotes := strings.Join(summary.Remotes, ", ")
	if remotes == "" {
		remotes = "none"
	}
	return []string{
		truncate("Repo: "+summary.Repo.FullName(), width),
		truncate("Path: "+path, width),
		truncate("Default branch: "+defaultBranch, width),
		truncate("Remotes: "+remotes, width),
		truncate(fmt.Sprintf("Active worktrees: %d", summary.ActiveWorktreeCount), width),
		truncate(fmt.Sprintf("Open PRs: %d", summary.OpenPullRequestCount), width),
		truncate(fmt.Sprintf("Open issues: %d", summary.OpenIssueCount), width),
		truncate(fmt.Sprintf("Failing checks: %d", summary.FailingCheckCount), width),
	}
}

func (m model) repositoryCountLabel(repo workbench.RepoRef) string {
	summary, ok := m.repoSummary(repo)
	if !ok || emptyRepositorySummary(summary) {
		return ""
	}
	return fmt.Sprintf("wt%d pr%d issue%d fail%d", summary.ActiveWorktreeCount, summary.OpenPullRequestCount, summary.OpenIssueCount, summary.FailingCheckCount)
}

func emptyRepositorySummary(summary workbench.RepositorySummary) bool {
	return summary.Path == "" &&
		summary.DefaultBranch == "" &&
		len(summary.Remotes) == 0 &&
		summary.ActiveWorktreeCount == 0 &&
		summary.OpenPullRequestCount == 0 &&
		summary.OpenIssueCount == 0 &&
		summary.FailingCheckCount == 0
}

func (m model) previewStatusLines(width int, status string) []string {
	lines := []string{truncate(status, width)}
	if item, ok := m.selectedWorkItem(); ok {
		lines = append(lines, truncate("Item: "+item.Title(), width))
	}
	return lines
}

func (m model) pullRequestPreviewLines(width int) []string {
	switch m.pullRequestPreview.status {
	case previewLoading:
		return m.pullRequestPreviewStatusLines(width, "Loading preview...")
	case previewLoaded:
		if m.pullRequestPreview.loaded.pullRequestKey != m.pullRequestPreview.focusedPullRequestKey {
			return m.pullRequestPreviewStatusLines(width, "Loading preview...")
		}
		return pullRequestPreviewLines(m.pullRequestPreview.loaded.pr, width)
	case previewEmpty:
		if _, ok := m.selectedPullRequest(); !ok {
			return []string{"  no pull request selected"}
		}
		return m.pullRequestPreviewStatusLines(width, "No preview data")
	case previewError:
		lines := m.pullRequestPreviewStatusLines(width, "Preview failed")
		if m.pullRequestPreview.errorMessage != "" {
			lines = append(lines, truncate("Error: "+m.pullRequestPreview.errorMessage, width))
		}
		return lines
	default:
		if _, ok := m.selectedPullRequest(); !ok {
			return []string{"  no pull request selected"}
		}
		return m.pullRequestPreviewStatusLines(width, "Preview idle")
	}
}

func (m model) pullRequestPreviewStatusLines(width int, status string) []string {
	lines := []string{truncate(status, width)}
	if pr, ok := m.selectedPullRequest(); ok {
		lines = append(lines, truncate("PR: "+pr.NumberLabel()+" "+pr.Title, width))
	}
	return lines
}

func (m model) headerLines(title string, width int) []string {
	out := []string{title}
	out = append(out, m.keymapLines(width)...)
	if m.activeView == appViewPullRequests {
		out = append(out, m.pullRequestHeaderLines(width)...)
	}
	if m.statusMessage != "" {
		out = append(out, truncate("Status: "+m.statusMessage, width))
	}
	return out
}

func (m model) pullRequestHeaderLines(width int) []string {
	if m.pullRequestSearch {
		return []string{truncate("Search PRs: "+m.pullRequestSearchIn, width)}
	}
	if m.pullRequestFilterUI {
		return []string{truncate("PR filters: s state  a author  r requested  w waiting  c checks  d draft  x clear", width)}
	}
	labels := m.pullRequestFilter.ActiveLabels()
	if len(labels) == 0 {
		return []string{truncate("PR filters: none", width)}
	}
	return []string{truncate("PR filters: "+strings.Join(labels, ", "), width)}
}

// keymapLines keeps the active pane affordances visible near the title.
func (m model) keymapLines(width int) []string {
	focus := m.activePane()
	prefix := focus.label() + " keys: "
	helpWidth := max(width-lipgloss.Width(prefix), 0)
	helpView := m.styledHelp(helpWidth).View(m.keys.contextualHelp(m.activeView, focus, m.paneOrder()))
	lines := strings.Split(helpView, "\n")
	indent := strings.Repeat(" ", lipgloss.Width(prefix))

	for i := range lines {
		if i == 0 {
			lines[i] = truncate(prefix+lines[i], width)
			continue
		}
		lines[i] = truncate(indent+lines[i], width)
	}
	return lines
}

func (m model) keymapLine(width int) string {
	return m.keymapLines(width)[0]
}

func newHelpModel() help.Model {
	helpModel := help.New()
	helpModel.ShortSeparator = "  "
	return helpModel
}

func (m model) styledHelp(width int) help.Model {
	helpModel := m.help
	helpModel.Width = width
	helpModel.ShortSeparator = "  "
	helpModel.Styles.ShortKey = m.styles.Key
	helpModel.Styles.FullKey = m.styles.Key
	helpModel.Styles.ShortDesc = m.styles.KeyDescription
	helpModel.Styles.FullDesc = m.styles.KeyDescription
	helpModel.Styles.ShortSeparator = m.styles.Divider
	helpModel.Styles.FullSeparator = m.styles.Divider
	helpModel.Styles.Ellipsis = m.styles.Muted
	return helpModel
}

// selectionMarker keeps the retained selection visible outside the active pane.
func selectionMarker(selected, focused bool) string {
	if !selected {
		return " "
	}
	if focused {
		return ">"
	}
	return "*"
}

func (m model) frameBodyHeight(contentHeight int, headerHeight int) int {
	if m.height <= 0 {
		return contentHeight
	}
	available := m.height - headerHeight - frameBorderLines
	if available > contentHeight {
		return available
	}
	return contentHeight
}

// renderPane draws a lazygit-style independent pane box.
func (m model) renderPane(title string, lines []string, width int, height int, focused bool) string {
	content := renderPaneContent(lines, width, height)
	contentLines := strings.Split(content, "\n")
	border := m.styles.PaneBorder.GetForeground()
	if focused {
		border = m.styles.FrameBorder.GetForeground()
	}
	borderStyle := lipgloss.NewStyle().Foreground(border)
	leftBorder := borderStyle.Render(paneBorderGlyph)
	rightBorder := borderStyle.Render(paneBorderGlyph)

	out := make([]string, 0, height+frameBorderLines)
	out = append(out, m.paneTopBorder(width, title, borderStyle))
	for i := range height {
		out = append(out, leftBorder+pad(lineAt(contentLines, i), width)+rightBorder)
	}
	out = append(out, borderStyle.Render(frameBottomLeftGlyph+strings.Repeat(horizontalLineGlyph, width)+frameBottomRightGlyph))
	return strings.Join(out, "\n")
}

func (m model) paneTopBorder(width int, title string, borderStyle lipgloss.Style) string {
	title = truncate(horizontalLineGlyph+title, width)
	line := title + strings.Repeat(horizontalLineGlyph, max(width-lipgloss.Width(title), 0))
	return borderStyle.Render(frameTopLeftGlyph + line + frameTopRightGlyph)
}

func paneGap(height int) string {
	lines := make([]string, height)
	for i := range lines {
		lines[i] = strings.Repeat(" ", paneGapWidth)
	}
	return strings.Join(lines, "\n")
}

func paneTextWidth(width int) int {
	return max(width-paneContentPaddingLeft, 0)
}

func (m model) paneHeading(pane paneFocus) string {
	number, ok := m.paneNumber(pane)
	if !ok {
		return pane.borderLabel()
	}
	return fmt.Sprintf("%s[%d]", pane.borderLabel(), number)
}

func (m model) paneNumber(pane paneFocus) (int, bool) {
	for i, visiblePane := range m.paneOrder() {
		if visiblePane == pane {
			return i + 1, true
		}
	}
	return 0, false
}

// renderPaneContent keeps each pane block rectangular before borders are added.
func renderPaneContent(lines []string, width int, height int) string {
	out := make([]string, height)
	for i := range out {
		out[i] = pad(strings.Repeat(" ", paneContentPaddingLeft)+lineAt(lines, i), width)
	}
	return strings.Join(out, "\n")
}

func (m model) selectedWorkItem() (workbench.WorkItem, bool) {
	items := m.visibleWorkItems()
	if len(items) == 0 || m.selectedItem < 0 || m.selectedItem >= len(items) {
		return workbench.WorkItem{}, false
	}
	return items[m.selectedItem], true
}

func (m model) visiblePullRequests() []pullrequests.PullRequest {
	repo, ok := m.selectedRepoRef()
	if !ok {
		return nil
	}
	if m.pullRequestRepo != (workbench.RepoRef{}) && m.pullRequestRepo != repo {
		return nil
	}
	return pullrequests.Filter(m.pullRequests, m.pullRequestFilter)
}

func (m model) selectedPullRequest() (pullrequests.PullRequest, bool) {
	prs := m.visiblePullRequests()
	if len(prs) == 0 || m.selectedPR < 0 || m.selectedPR >= len(prs) {
		return pullrequests.PullRequest{}, false
	}
	return prs[m.selectedPR], true
}

func (m *model) restoreSelectedPullRequest(number int) {
	prs := m.visiblePullRequests()
	if len(prs) == 0 {
		m.selectedPR = 0
		return
	}
	if number > 0 {
		for i, pr := range prs {
			if pr.Number == number {
				m.selectedPR = i
				return
			}
		}
	}
	m.selectedPR = clamp(m.selectedPR, 0, len(prs)-1)
}

func (m model) focusedWorkItemMatches(item workbench.WorkItem) bool {
	return item.Repo == m.focusedWorkItemRepo && item.ID == m.focusedWorkItemID
}

func (m *model) clearFocusedWorkItem() {
	m.focusedWorkItemRepo = workbench.RepoRef{}
	m.focusedWorkItemID = ""
}

func shortRemoteLabel(item workbench.WorkItem) string {
	if item.PullRequest != nil {
		if item.PullRequest.Number == 0 {
			return "PR"
		}
		return fmt.Sprintf("PR #%d", item.PullRequest.Number)
	}
	if item.Issue != nil {
		if item.Issue.Number == 0 {
			return "issue"
		}
		return fmt.Sprintf("#%d", item.Issue.Number)
	}
	if item.Branch != nil && item.Branch.RemoteOnly {
		return "remote"
	}
	return "no PR"
}

func lineAt(lines []string, i int) string {
	if i < 0 || i >= len(lines) {
		return ""
	}
	return lines[i]
}

func pad(s string, width int) string {
	s = truncate(s, width)
	padWidth := width - lipgloss.Width(s)
	if padWidth <= 0 {
		return s
	}
	return s + strings.Repeat(" ", padWidth)
}

func clamp(value int, minValue int, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

// truncate uses terminal display width so wide characters keep columns aligned.
func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	return ansi.Truncate(s, width, "~")
}
