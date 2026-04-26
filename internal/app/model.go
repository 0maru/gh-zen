package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0maru/gh-zen/internal/workbench"
)

const defaultWidth = 100

type model struct {
	width        int
	height       int
	repos        []workbench.RepoRef
	selectedRepo int
	workItems    []workbench.WorkItem
	selectedItem int
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
		case "j", "down":
			m.moveSelection(1)
			return m, nil
		case "k", "up":
			m.moveSelection(-1)
			return m, nil
		case "g":
			m.selectedItem = 0
			return m, nil
		case "G":
			if len(m.workItems) > 0 {
				m.selectedItem = len(m.workItems) - 1
			}
			return m, nil
		}
	}
	return m, nil
}

func (m *model) moveSelection(delta int) {
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
	width := m.width
	if width <= 0 {
		width = defaultWidth
	}

	if width < 94 {
		return m.renderCompact(width)
	}
	return m.renderFull(width)
}

func (m model) renderFull(width int) string {
	leftWidth := 23
	middleWidth := 39
	gapWidth := 2
	rightWidth := width - leftWidth - middleWidth - gapWidth*2
	if rightWidth < 28 {
		rightWidth = 28
	}

	left := m.repoLines(leftWidth)
	middle := m.workItemLines(middleWidth)
	right := m.previewLines(rightWidth)
	lines := maxInt(len(left), maxInt(len(middle), len(right)))

	out := []string{
		"gh-zen  repository workbench",
		strings.Repeat("-", minInt(width, 72)),
	}
	for i := 0; i < lines; i++ {
		row := pad(lineAt(left, i), leftWidth) + strings.Repeat(" ", gapWidth) + pad(lineAt(middle, i), middleWidth) + strings.Repeat(" ", gapWidth) + pad(lineAt(right, i), rightWidth)
		out = append(out, strings.TrimRight(row, " "))
	}
	out = append(out, "", "j/k move  g/G jump  q quit")
	return strings.Join(out, "\n") + "\n"
}

func (m model) renderCompact(width int) string {
	lines := []string{
		"gh-zen workbench",
		strings.Repeat("-", minInt(width, 48)),
	}
	lines = append(lines, m.workItemLines(width)...)
	lines = append(lines, "")
	lines = append(lines, m.previewLines(width)...)
	lines = append(lines, "", "j/k move  q quit")
	return strings.Join(lines, "\n") + "\n"
}

func (m model) repoLines(width int) []string {
	lines := []string{"Repositories"}
	if len(m.repos) == 0 {
		lines = append(lines, "  none")
	} else {
		for i, repo := range m.repos {
			marker := " "
			if i == m.selectedRepo {
				marker = ">"
			}
			lines = append(lines, truncate(fmt.Sprintf("%s %s", marker, repo.FullName()), width))
		}
	}
	lines = append(lines, "", "Views", "  Active worktrees", "  Review requested", "  Failed checks")
	return lines
}

func (m model) workItemLines(width int) []string {
	lines := []string{"Work Items"}
	if len(m.workItems) == 0 {
		return append(lines, "  no work items")
	}
	for i, item := range m.workItems {
		marker := " "
		if i == m.selectedItem {
			marker = ">"
		}
		row := fmt.Sprintf("%s %-22s %-7s %s", marker, item.Title(), item.LocalLabel(), shortRemoteLabel(item))
		lines = append(lines, truncate(row, width))
	}
	return lines
}

func (m model) previewLines(width int) []string {
	item, ok := m.selectedWorkItem()
	if !ok {
		return []string{"Preview", "  no work item selected"}
	}

	lines := []string{
		"Preview",
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
	lines = append(lines, truncate("Checks: "+item.Checks.Label(), width))
	if len(item.Commits) > 0 {
		lines = append(lines, "Commits:")
		for _, commit := range item.Commits {
			lines = append(lines, truncate("  "+commit.ShortSHA+" "+commit.Subject, width))
		}
	}
	return lines
}

func (m model) selectedWorkItem() (workbench.WorkItem, bool) {
	if len(m.workItems) == 0 || m.selectedItem < 0 || m.selectedItem >= len(m.workItems) {
		return workbench.WorkItem{}, false
	}
	return m.workItems[m.selectedItem], true
}

func shortRemoteLabel(item workbench.WorkItem) string {
	if item.PullRequest != nil {
		return fmt.Sprintf("PR #%d", item.PullRequest.Number)
	}
	if item.Branch != nil && item.Branch.RemoteOnly {
		return "remote"
	}
	if item.Issue != nil && item.Branch == nil {
		return fmt.Sprintf("#%d", item.Issue.Number)
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
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	if width == 1 {
		return string(runes[:1])
	}
	return string(runes[:width-1]) + "~"
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
