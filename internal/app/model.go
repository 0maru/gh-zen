package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/0maru/gh-zen/internal/workbench"
)

const (
	defaultWidth           = 100
	repoPaneWidth          = 23
	workItemPaneWidth      = 41
	paneGapWidth           = 1
	paneContentPaddingLeft = 1
	paneBorderGlyph        = "│"
	paneBorderWidth        = 2
	frameBorderLines       = 2
	horizontalLineGlyph    = "─"
	frameTopLeftGlyph      = "┌"
	frameTopRightGlyph     = "┐"
	frameBottomLeftGlyph   = "└"
	frameBottomRightGlyph  = "┘"
	previewPaneMinWidth    = 28
	fullLayoutMinWidth     = repoPaneWidth + workItemPaneWidth + previewPaneMinWidth + paneBorderWidth*3 + paneGapWidth*2
)

// paneFocus tracks the pane that owns pane-scoped key handling.
type paneFocus int

const (
	paneWorkItems paneFocus = iota
	paneRepositories
	panePreview
)

func (p paneFocus) label() string {
	switch p {
	case paneRepositories:
		return "Repositories"
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
	case panePreview:
		return "Review"
	default:
		return "WorkItems"
	}
}

type model struct {
	width        int
	height       int
	repos        []workbench.RepoRef
	selectedRepo int
	selectedView int
	viewSelected bool
	workItems    []workbench.WorkItem
	selectedItem int
	focusedPane  paneFocus
	styles       Styles
	keys         workbenchKeyMap
	help         help.Model
}

type repoViewFilter int

const (
	repoViewActiveWorktrees repoViewFilter = iota
	repoViewReviewRequested
	repoViewFailedChecks
)

type repoView struct {
	label  string
	filter repoViewFilter
}

var repoViews = []repoView{
	{label: "Active worktrees", filter: repoViewActiveWorktrees},
	{label: "Review requested", filter: repoViewReviewRequested},
	{label: "Failed checks", filter: repoViewFailedChecks},
}

func New() tea.Model {
	return newModel()
}

func newModel() model {
	return model{
		repos:     workbench.FakeRepos(),
		workItems: workbench.FakeWorkItems(),
		styles:    DefaultStyles(),
		keys:      DefaultKeyMap(),
		help:      newHelpModel(),
	}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		if action, ok := m.matchedAction(msg); ok {
			return m, m.handleAction(action)
		}
	}
	return m, nil
}

func (m model) matchedAction(msg tea.KeyMsg) (actionID, bool) {
	for _, binding := range m.keys.actionBindings() {
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
	case actionMoveUp:
		m.moveFocusedSelection(-1)
	case actionJumpTop:
		m.jumpFocusedSelection(false)
	case actionJumpBottom:
		m.jumpFocusedSelection(true)
	case actionRefresh, actionOpen, actionCopy:
		return nil
	}
	return nil
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
	if m.isCompact() {
		return []paneFocus{paneWorkItems, panePreview}
	}
	return []paneFocus{paneRepositories, paneWorkItems, panePreview}
}

// activePane normalizes focus when the current layout hides a pane.
func (m model) activePane() paneFocus {
	focus := m.focusedPane
	for _, pane := range m.paneOrder() {
		if focus == pane {
			return focus
		}
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

func (m model) repoPaneEntryCount() int {
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
	if index < len(m.repos) {
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
		return filterWorkItems(m.workItems, view.matches)
	}

	repo, ok := m.selectedRepoRef()
	if !ok {
		return nil
	}
	return filterWorkItems(m.workItems, func(item workbench.WorkItem) bool {
		return item.Repo == repo
	})
}

func (m model) selectedRepoRef() (workbench.RepoRef, bool) {
	if len(m.repos) == 0 || m.selectedRepo < 0 || m.selectedRepo >= len(m.repos) {
		return workbench.RepoRef{}, false
	}
	return m.repos[m.selectedRepo], true
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

func (v repoView) matches(item workbench.WorkItem) bool {
	switch v.filter {
	case repoViewActiveWorktrees:
		return item.Worktree != nil
	case repoViewReviewRequested:
		return item.PullRequest != nil && item.PullRequest.ReviewState == "review requested"
	case repoViewFailedChecks:
		return item.Checks.State == workbench.CheckFailing
	default:
		return false
	}
}

func (m model) View() string {
	width := m.effectiveWidth()

	if width < fullLayoutMinWidth {
		return m.renderCompact(width)
	}
	return m.renderFull(width)
}

func (m model) renderFull(width int) string {
	rightWidth := width - repoPaneWidth - workItemPaneWidth - paneBorderWidth*3 - paneGapWidth*2
	focus := m.activePane()

	left := m.repoLines(paneTextWidth(repoPaneWidth), focus == paneRepositories)
	middle := m.workItemLines(paneTextWidth(workItemPaneWidth), focus == paneWorkItems)
	right := m.previewLines(paneTextWidth(rightWidth))
	out := m.headerLines("gh-zen  repository workbench", width)
	bodyHeight := m.frameBodyHeight(max(len(left), max(len(middle), len(right))), len(out))

	body := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.renderPane(m.paneHeading(paneRepositories), left, repoPaneWidth, bodyHeight, focus == paneRepositories),
		paneGap(bodyHeight+frameBorderLines),
		m.renderPane(m.paneHeading(paneWorkItems), middle, workItemPaneWidth, bodyHeight, focus == paneWorkItems),
		paneGap(bodyHeight+frameBorderLines),
		m.renderPane(m.paneHeading(panePreview), right, rightWidth, bodyHeight, focus == panePreview),
	)
	out = append(out, body)
	return strings.Join(out, "\n") + "\n"
}

func (m model) renderCompact(width int) string {
	contentWidth := max(width-paneBorderWidth, 0)
	focus := m.activePane()
	out := m.headerLines("gh-zen workbench", width)

	workLines := m.workItemLines(paneTextWidth(contentWidth), focus == paneWorkItems)
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
		m.renderPane(m.paneHeading(paneWorkItems), workLines, contentWidth, workHeight, focus == paneWorkItems),
		m.renderPane(m.paneHeading(panePreview), previewLines, contentWidth, previewHeight, focus == panePreview),
	)
	return strings.Join(out, "\n") + "\n"
}

func (m model) repoLines(width int, focused bool) []string {
	lines := []string{}
	if len(m.repos) == 0 {
		lines = append(lines, "  none")
	} else {
		for i, repo := range m.repos {
			marker := selectionMarker(!m.viewSelected && i == m.selectedRepo, focused)
			lines = append(lines, truncate(fmt.Sprintf("%s %s", marker, repo.FullName()), width))
		}
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
		return []string{"  no work items"}
	}
	lines := []string{}
	for i, item := range items {
		marker := selectionMarker(i == m.selectedItem, focused)
		row := fmt.Sprintf("%s %-22s %-7s %s", marker, item.Title(), item.LocalLabel(), shortRemoteLabel(item))
		lines = append(lines, truncate(row, width))
	}
	return lines
}

func (m model) previewLines(width int) []string {
	item, ok := m.selectedWorkItem()
	if !ok {
		return []string{"  no work item selected"}
	}

	lines := []string{
		truncate("Repo: "+item.Repo.FullName(), width),
		truncate("Item: "+item.Title(), width),
		truncate("Where: "+item.Location(), width),
	}
	if item.Branch != nil {
		base := item.Branch.Base
		if base == "" {
			base = "unknown base"
		}
		lines = append(lines, truncate("Branch: "+item.Branch.Name+" -> "+base, width))
	}
	lines = append(lines, truncate("Local: "+item.LocalLabel(), width))
	lines = append(lines, truncate("Issue: "+item.IssueLabel(), width))
	lines = append(lines, truncate("PR: "+item.PullRequestLabel(), width))
	if item.PullRequest != nil && item.PullRequest.ReviewState != "" {
		lines = append(lines, truncate("Review: "+item.PullRequest.ReviewState, width))
	}
	lines = append(lines, truncate("Checks: "+item.Checks.Label(), width))
	if len(item.Commits) > 0 {
		lines = append(lines, "Commits:")
		for _, commit := range item.Commits {
			lines = append(lines, truncate("  "+commit.ShortSHA+" "+commit.Subject, width))
		}
	}
	return lines
}

func (m model) headerLines(title string, width int) []string {
	out := []string{title}
	return append(out, m.keymapLines(width)...)
}

// keymapLines keeps the active pane affordances visible near the title.
func (m model) keymapLines(width int) []string {
	focus := m.activePane()
	prefix := focus.label() + " keys: "
	helpWidth := max(width-lipgloss.Width(prefix), 0)
	helpView := m.styledHelp(helpWidth).View(m.keys.contextualHelp(focus, m.paneOrder()))
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
