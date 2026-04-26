package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/0maru/gh-zen/internal/workbench"
)

const (
	defaultWidth        = 100
	repoPaneWidth       = 23
	workItemPaneWidth   = 39
	paneSeparator       = " | "
	paneSeparatorWidth  = len(paneSeparator)
	previewPaneMinWidth = 28
	dividerMaxWidth     = 72
	compactDividerWidth = 48
	fullLayoutMinWidth  = repoPaneWidth + workItemPaneWidth + paneSeparatorWidth*2 + previewPaneMinWidth
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
		return "Preview"
	default:
		return "Work Items"
	}
}

type model struct {
	width        int
	height       int
	repos        []workbench.RepoRef
	selectedRepo int
	workItems    []workbench.WorkItem
	selectedItem int
	focusedPane  paneFocus
}

func New() tea.Model {
	return newModel()
}

func newModel() model {
	return model{
		repos:     workbench.FakeRepos(),
		workItems: workbench.FakeWorkItems(),
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
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "tab":
			m.focusNextPane()
			return m, nil
		case "shift+tab":
			m.focusPreviousPane()
			return m, nil
		case "j", "down":
			m.moveFocusedSelection(1)
			return m, nil
		case "k", "up":
			m.moveFocusedSelection(-1)
			return m, nil
		case "g":
			m.jumpFocusedSelection(false)
			return m, nil
		case "G":
			m.jumpFocusedSelection(true)
			return m, nil
		}
	}
	return m, nil
}

func (m *model) focusNextPane() {
	m.focusedPane = nextPane(m.activePane(), m.paneOrder())
}

func (m *model) focusPreviousPane() {
	m.focusedPane = previousPane(m.activePane(), m.paneOrder())
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
			if len(m.repos) > 0 {
				m.selectedRepo = len(m.repos) - 1
			}
			return
		}
		m.selectedRepo = 0
	case paneWorkItems:
		if toEnd {
			if len(m.workItems) > 0 {
				m.selectedItem = len(m.workItems) - 1
			}
			return
		}
		m.selectedItem = 0
	}
}

func (m *model) moveRepoSelection(delta int) {
	if len(m.repos) == 0 {
		m.selectedRepo = 0
		return
	}

	m.selectedRepo += delta
	if m.selectedRepo < 0 {
		m.selectedRepo = 0
	}
	if m.selectedRepo >= len(m.repos) {
		m.selectedRepo = len(m.repos) - 1
	}
}

func (m *model) moveWorkItemSelection(delta int) {
	if len(m.workItems) == 0 {
		m.selectedItem = 0
		return
	}

	m.selectedItem += delta
	if m.selectedItem < 0 {
		m.selectedItem = 0
	}
	if m.selectedItem >= len(m.workItems) {
		m.selectedItem = len(m.workItems) - 1
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
	rightWidth := width - repoPaneWidth - workItemPaneWidth - paneSeparatorWidth*2
	focus := m.activePane()

	left := m.repoLines(repoPaneWidth, focus == paneRepositories)
	middle := m.workItemLines(workItemPaneWidth, focus == paneWorkItems)
	right := m.previewLines(rightWidth, focus == panePreview)
	lines := max(len(left), max(len(middle), len(right)))

	out := []string{
		"gh-zen  repository workbench",
		m.keymapLine(width),
		strings.Repeat("-", min(width, dividerMaxWidth)),
	}
	for i := 0; i < lines; i++ {
		row := paneRow(lineAt(left, i), lineAt(middle, i), lineAt(right, i), rightWidth)
		out = append(out, strings.TrimRight(row, " "))
	}
	return strings.Join(out, "\n") + "\n"
}

func (m model) renderCompact(width int) string {
	focus := m.activePane()
	lines := []string{
		"gh-zen workbench",
		m.keymapLine(width),
		strings.Repeat("-", min(width, compactDividerWidth)),
	}
	lines = append(lines, m.workItemLines(width, focus == paneWorkItems)...)
	lines = append(lines, "")
	lines = append(lines, strings.Repeat("-", min(width, compactDividerWidth)))
	lines = append(lines, m.previewLines(width, focus == panePreview)...)
	return strings.Join(lines, "\n") + "\n"
}

func (m model) repoLines(width int, focused bool) []string {
	lines := []string{paneTitle("Repositories", focused)}
	if len(m.repos) == 0 {
		lines = append(lines, "  none")
	} else {
		for i, repo := range m.repos {
			marker := selectionMarker(i == m.selectedRepo, focused)
			lines = append(lines, truncate(fmt.Sprintf("%s %s", marker, repo.FullName()), width))
		}
	}
	lines = append(lines, "", "Views", "  Active worktrees", "  Review requested", "  Failed checks")
	return lines
}

func (m model) workItemLines(width int, focused bool) []string {
	lines := []string{paneTitle("Work Items", focused)}
	if len(m.workItems) == 0 {
		return append(lines, "  no work items")
	}
	for i, item := range m.workItems {
		marker := selectionMarker(i == m.selectedItem, focused)
		row := fmt.Sprintf("%s %-22s %-7s %s", marker, item.Title(), item.LocalLabel(), shortRemoteLabel(item))
		lines = append(lines, truncate(row, width))
	}
	return lines
}

func (m model) previewLines(width int, focused bool) []string {
	item, ok := m.selectedWorkItem()
	if !ok {
		return []string{paneTitle("Preview", focused), "  no work item selected"}
	}

	lines := []string{
		paneTitle("Preview", focused),
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

// keymapLine keeps the active pane affordances visible near the title.
func (m model) keymapLine(width int) string {
	focus := m.activePane()
	prefix := focus.label() + " keys: "
	switch focus {
	case paneRepositories, paneWorkItems:
		return truncate(prefix+"j/k move  g/G jump  tab/S-tab pane  q quit", width)
	case panePreview:
		return truncate(prefix+"tab/S-tab pane  q quit", width)
	}
	return truncate(prefix+"tab/S-tab pane  q quit", width)
}

func paneTitle(label string, focused bool) string {
	if focused {
		return label + " [active]"
	}
	return label
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

// paneRow renders full-layout columns with visible pane separators.
func paneRow(left, middle, right string, rightWidth int) string {
	return pad(left, repoPaneWidth) + paneSeparator + pad(middle, workItemPaneWidth) + paneSeparator + pad(right, rightWidth)
}

func (m model) selectedWorkItem() (workbench.WorkItem, bool) {
	if len(m.workItems) == 0 || m.selectedItem < 0 || m.selectedItem >= len(m.workItems) {
		return workbench.WorkItem{}, false
	}
	return m.workItems[m.selectedItem], true
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
