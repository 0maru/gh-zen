package app

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/0maru/gh-zen/internal/workbench"
)

const (
	issueAssigneeMe         = "@me"
	issueAssigneeUnassigned = "(unassigned)"
)

type issueStateFilter int

const (
	issueStateOpen issueStateFilter = iota
	issueStateClosed
	issueStateAll
)

type issueFilterState struct {
	State     issueStateFilter
	Assignee  string
	Label     string
	Milestone string
	Search    string
}

func defaultIssueFilterState() issueFilterState {
	return issueFilterState{State: issueStateOpen}
}

func (f issueStateFilter) String() string {
	switch f {
	case issueStateClosed:
		return "closed"
	case issueStateAll:
		return "all"
	default:
		return "open"
	}
}

func (f issueStateFilter) next() issueStateFilter {
	switch f {
	case issueStateOpen:
		return issueStateClosed
	case issueStateClosed:
		return issueStateAll
	default:
		return issueStateOpen
	}
}

func (m *model) enterIssueView() tea.Cmd {
	item, hasItem := m.selectedWorkItem()
	repo, ok := m.selectedRepoRef()
	if hasItem && hasRepoRef(item.Repo) {
		repo = item.Repo
		ok = true
	}
	if !ok {
		m.statusMessage = "No repository selected"
		return nil
	}

	targetIssueNumber := 0
	if hasItem && item.Issue != nil {
		targetIssueNumber = item.Issue.Number
	}

	returnRepo := workbench.RepoRef{}
	if selectedRepo, ok := m.selectedRepoRef(); ok {
		returnRepo = selectedRepo
	}
	m.prepareIssueDataForRepo(repo)
	m.workbenchReturn = workbenchReturnState{
		valid:           true,
		selectedRepo:    m.selectedRepo,
		selectedRepoRef: returnRepo,
		selectedView:    m.selectedView,
		viewSelected:    m.viewSelected,
		selectedItem:    m.selectedItem,
		focusedPane:     m.focusedPane,
	}
	m.screen = screenIssues
	m.focusedPane = paneWorkItems
	m.issueSearchEditing = false
	if m.workbenchLoading {
		m.issuesLoading = true
	}

	if targetIssueNumber > 0 {
		m.ensureIssueVisible(targetIssueNumber)
		if m.selectIssueNumber(targetIssueNumber) {
			m.statusMessage = ""
			return m.startIssueViewReload()
		}
		m.statusMessage = fmt.Sprintf("Issue #%d is not in the loaded issue list", targetIssueNumber)
		return m.startIssueViewReload()
	}
	m.clampIssueSelection()
	m.statusMessage = ""
	return m.startIssueViewReload()
}

func (m *model) startIssueViewReload() tea.Cmd {
	if m.workbenchReloader == nil || !hasRepoRef(m.issueRepo) {
		return nil
	}
	return m.startWorkbenchReload("Loading issues...")
}

func (m *model) backToWorkbench() tea.Cmd {
	m.screen = screenWorkbench
	m.issueSearchEditing = false
	if m.workbenchReturn.valid {
		m.selectedRepo = m.workbenchReturn.selectedRepo
		m.selectedView = m.workbenchReturn.selectedView
		m.viewSelected = m.workbenchReturn.viewSelected
		m.selectedItem = m.workbenchReturn.selectedItem
		m.focusedPane = m.workbenchReturn.focusedPane
	}
	m.workbenchReturn = workbenchReturnState{}
	return m.startPreviewLoadIfFocusedItemChanged()
}

func (m *model) prepareIssueDataForRepo(repo workbench.RepoRef) {
	if strings.EqualFold(m.issueRepo.FullName(), repo.FullName()) {
		return
	}
	m.issueRepo = repo
	m.issues = issuesFromWorkItems(m.workItems, repo)
	m.prsByIssueNumber = pullRequestsByIssueNumber(pullRequestsFromWorkItems(m.workItems, repo))
	m.issuesError = issueDiscoveryErrorFromWorkItems(m.workItems, repo)
	m.selectedIssue = 0
}

func (m *model) updateIssueDataFromRuntimeResult(result workbench.RuntimeLoadResult) {
	selectedNumber := 0
	if issue, ok := m.selectedIssueRef(); ok {
		selectedNumber = issue.Number
	}
	issueRepo := result.IssuesRepo
	if !hasRepoRef(issueRepo) {
		issueRepo = result.Repo
	}
	m.issueRepo = issueRepo
	workItemIssues := issuesFromWorkItems(result.Items, issueRepo)
	if result.IssuesLoaded {
		m.issues = mergeIssueRefs(result.Issues, workItemIssues)
	} else {
		m.issues = workItemIssues
	}
	if result.PullRequestsLoaded {
		m.prsByIssueNumber = pullRequestsByIssueNumber(result.PullRequests)
	} else {
		m.prsByIssueNumber = pullRequestsByIssueNumber(pullRequestsFromWorkItems(result.Items, issueRepo))
	}
	if result.ViewerSubject.Login != "" {
		m.viewerLogin = result.ViewerSubject.Login
	}
	m.issuesError = issueDiscoveryErrorFromWorkItems(result.Items, issueRepo)
	if selectedNumber > 0 && m.selectIssueNumber(selectedNumber) {
		return
	}
	m.clampIssueSelection()
}

func (m model) visibleIssues() []workbench.IssueRef {
	return filterIssues(m.issues, m.issueFilter, m.viewerLogin)
}

func filterIssues(issues []workbench.IssueRef, filter issueFilterState, viewerLogin string) []workbench.IssueRef {
	out := make([]workbench.IssueRef, 0, len(issues))
	for _, issue := range issues {
		if issueMatchesFilter(issue, filter, viewerLogin) {
			out = append(out, issue)
		}
	}
	return out
}

func issueMatchesFilter(issue workbench.IssueRef, filter issueFilterState, viewerLogin string) bool {
	switch filter.State {
	case issueStateClosed:
		if !strings.EqualFold(issue.State, "closed") {
			return false
		}
	case issueStateAll:
	default:
		if issue.State != "" && !strings.EqualFold(issue.State, "open") {
			return false
		}
	}
	if filter.Assignee != "" {
		switch filter.Assignee {
		case issueAssigneeMe:
			if viewerLogin == "" || !hasCaseFolded(issue.Assignees, viewerLogin) {
				return false
			}
		case issueAssigneeUnassigned:
			if len(issue.Assignees) != 0 {
				return false
			}
		default:
			if !hasCaseFolded(issue.Assignees, filter.Assignee) {
				return false
			}
		}
	}
	if filter.Label != "" && !hasCaseFolded(issue.Labels, filter.Label) {
		return false
	}
	if filter.Milestone != "" && !strings.EqualFold(issue.Milestone, filter.Milestone) {
		return false
	}
	query := strings.TrimSpace(filter.Search)
	if query != "" {
		haystack := strings.ToLower(issue.Title + "\n" + issue.Body)
		if !strings.Contains(haystack, strings.ToLower(query)) {
			return false
		}
	}
	return true
}

func (m *model) ensureIssueVisible(number int) {
	for _, issue := range m.issues {
		if issue.Number == number && !issueMatchesFilter(issue, m.issueFilter, m.viewerLogin) {
			m.issueFilter = defaultIssueFilterState()
			if !issueMatchesFilter(issue, m.issueFilter, m.viewerLogin) {
				m.issueFilter.State = issueStateAll
			}
			return
		}
	}
}

func (m *model) selectIssueNumber(number int) bool {
	for i, issue := range m.visibleIssues() {
		if issue.Number == number {
			m.selectedIssue = i
			return true
		}
	}
	return false
}

func (m model) selectedIssueRef() (workbench.IssueRef, bool) {
	issues := m.visibleIssues()
	if len(issues) == 0 || m.selectedIssue < 0 || m.selectedIssue >= len(issues) {
		return workbench.IssueRef{}, false
	}
	return issues[m.selectedIssue], true
}

func (m *model) moveIssueSelection(delta int) {
	issues := m.visibleIssues()
	if len(issues) == 0 {
		m.selectedIssue = 0
		return
	}
	m.selectedIssue = clamp(m.selectedIssue+delta, 0, len(issues)-1)
}

func (m *model) jumpIssueSelection(toEnd bool) {
	issues := m.visibleIssues()
	if len(issues) == 0 {
		m.selectedIssue = 0
		return
	}
	if toEnd {
		m.selectedIssue = len(issues) - 1
		return
	}
	m.selectedIssue = 0
}

func (m *model) clampIssueSelection() {
	issues := m.visibleIssues()
	if len(issues) == 0 {
		m.selectedIssue = 0
		return
	}
	m.selectedIssue = clamp(m.selectedIssue, 0, len(issues)-1)
}

func (m *model) cycleIssueStateFilter() {
	m.issueFilter.State = m.issueFilter.State.next()
	m.clampIssueSelection()
	m.statusMessage = "Issue state filter: " + m.issueFilter.State.String()
}

func (m *model) cycleIssueAssigneeFilter() {
	values := issueAssigneeFilterValues(m.issues, m.viewerLogin)
	m.issueFilter.Assignee = nextFilterValue(values, m.issueFilter.Assignee)
	m.clampIssueSelection()
	m.statusMessage = "Issue assignee filter: " + issueAssigneeFilterLabel(m.issueFilter.Assignee)
}

func (m *model) cycleIssueLabelFilter() {
	values := append([]string{""}, uniqueSortedIssueValues(m.issues, func(issue workbench.IssueRef) []string {
		return issue.Labels
	})...)
	m.issueFilter.Label = nextFilterValue(values, m.issueFilter.Label)
	m.clampIssueSelection()
	m.statusMessage = "Issue label filter: " + issueOptionalFilterLabel(m.issueFilter.Label)
}

func (m *model) cycleIssueMilestoneFilter() {
	values := append([]string{""}, uniqueSortedIssueValues(m.issues, func(issue workbench.IssueRef) []string {
		if issue.Milestone == "" {
			return nil
		}
		return []string{issue.Milestone}
	})...)
	m.issueFilter.Milestone = nextFilterValue(values, m.issueFilter.Milestone)
	m.clampIssueSelection()
	m.statusMessage = "Issue milestone filter: " + issueOptionalFilterLabel(m.issueFilter.Milestone)
}

func (m *model) startIssueSearch() {
	m.issueSearchEditing = true
	m.statusMessage = "Editing issue search"
}

func (m *model) clearIssueFilters() {
	m.issueFilter = defaultIssueFilterState()
	m.issueSearchEditing = false
	m.clampIssueSelection()
	m.statusMessage = "Issue filters cleared"
}

func (m *model) handleIssueSearchKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		m.issueSearchEditing = false
		m.clampIssueSelection()
		m.statusMessage = "Issue search applied"
		return true, nil
	case tea.KeyEsc:
		m.issueSearchEditing = false
		m.statusMessage = "Issue search kept"
		return true, nil
	case tea.KeyCtrlC:
		return true, tea.Quit
	case tea.KeyBackspace, tea.KeyCtrlH:
		m.issueFilter.Search = dropLastRune(m.issueFilter.Search)
		m.clampIssueSelection()
		return true, nil
	case tea.KeyCtrlU:
		m.issueFilter.Search = ""
		m.clampIssueSelection()
		return true, nil
	case tea.KeySpace:
		m.issueFilter.Search += " "
		m.clampIssueSelection()
		return true, nil
	case tea.KeyRunes:
		m.issueFilter.Search += string(msg.Runes)
		m.clampIssueSelection()
		return true, nil
	default:
		return true, nil
	}
}

func (m *model) openSelectedIssueInBrowser() tea.Cmd {
	issue, ok := m.selectedIssueRef()
	if !ok {
		m.statusMessage = "No issue selected"
		return nil
	}
	if issue.URL == "" {
		m.statusMessage = "No issue URL for selected issue"
		return nil
	}
	label := issue.Label()
	m.statusMessage = "Opening " + label + "..."
	return m.actionCommand("Opened "+label, "Open issue failed", func(ctx context.Context) error {
		return m.runner().Open(ctx, issue.URL)
	})
}

func (m model) renderIssueFull(width int) string {
	leftWidth := issueListPaneWidth
	rightWidth := width - leftWidth - paneBorderWidth*2 - paneGapWidth
	focus := m.activePane()

	left := m.issueLines(paneTextWidth(leftWidth), focus == paneWorkItems)
	right := m.issuePreviewLines(paneTextWidth(rightWidth))
	out := m.headerLines("gh-zen  issues: "+m.issueRepo.FullName(), width)
	bodyHeight := m.frameBodyHeight(max(len(left), len(right)), len(out))

	body := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.renderPane(m.paneHeading(paneWorkItems), left, leftWidth, bodyHeight, focus == paneWorkItems),
		paneGap(bodyHeight+frameBorderLines),
		m.renderPane(m.paneHeading(panePreview), right, rightWidth, bodyHeight, focus == panePreview),
	)
	out = append(out, body)
	return strings.Join(out, "\n") + "\n"
}

func (m model) renderIssueCompact(width int) string {
	contentWidth := max(width-paneBorderWidth, 0)
	focus := m.activePane()
	out := m.headerLines("gh-zen issues: "+m.issueRepo.FullName(), width)

	issueLines := m.issueLines(paneTextWidth(contentWidth), focus == paneWorkItems)
	previewLines := m.issuePreviewLines(paneTextWidth(contentWidth))
	issueHeight := len(issueLines)
	previewHeight := len(previewLines)
	if m.height > 0 {
		availableContentHeight := m.height - len(out) - frameBorderLines*2
		if availableContentHeight > issueHeight+previewHeight {
			previewHeight += availableContentHeight - issueHeight - previewHeight
		}
	}

	out = append(out,
		m.renderPane(m.paneHeading(paneWorkItems), issueLines, contentWidth, issueHeight, focus == paneWorkItems),
		m.renderPane(m.paneHeading(panePreview), previewLines, contentWidth, previewHeight, focus == panePreview),
	)
	return strings.Join(out, "\n") + "\n"
}

func (m model) issueLines(width int, focused bool) []string {
	lines := []string{truncate(m.issueFilterLine(), width)}
	if m.issueSearchEditing {
		lines = append(lines, truncate("Search: "+m.issueFilter.Search+"_", width))
	}
	issues := m.visibleIssues()
	if len(issues) == 0 {
		return append(lines, m.emptyIssueLine())
	}
	for i, issue := range issues {
		marker := selectionMarker(i == m.selectedIssue, focused)
		row := fmt.Sprintf("%s %-7s %-7s %4s %-12s %s",
			marker,
			issueNumberLabel(issue.Number),
			issueStateLabel(issue.State),
			issueCommentShortLabel(issue.CommentsCount),
			emptyFallback(issue.AuthorLogin, "-"),
			issue.Title,
		)
		lines = append(lines, truncate(row, width))
		if meta := issueListMeta(issue); meta != "" {
			lines = append(lines, truncate("  "+meta, width))
		}
	}
	return lines
}

func (m model) issueFilterLine() string {
	parts := []string{
		"state=" + m.issueFilter.State.String(),
		"assignee=" + issueAssigneeFilterLabel(m.issueFilter.Assignee),
		"label=" + issueOptionalFilterLabel(m.issueFilter.Label),
		"milestone=" + issueOptionalFilterLabel(m.issueFilter.Milestone),
	}
	search := strings.TrimSpace(m.issueFilter.Search)
	if search == "" {
		search = "none"
	}
	parts = append(parts, "search="+search)
	return "Filters: " + strings.Join(parts, " ")
}

func (m model) emptyIssueLine() string {
	switch {
	case m.issuesLoading || m.workbenchLoading:
		return "  loading issues..."
	case m.issuesError != "":
		return "  issue loading failed: " + m.issuesError
	case len(m.issues) == 0:
		return "  no issues"
	default:
		return "  no issues match filters"
	}
}

func (m model) issuePreviewLines(width int) []string {
	issue, ok := m.selectedIssueRef()
	if !ok {
		return []string{"  no issue selected"}
	}
	lines := []string{
		truncate("Issue: "+issue.Label(), width),
		truncate("State: "+issueStateLabel(issue.State), width),
	}
	if issue.AuthorLogin != "" {
		lines = append(lines, truncate("Author: "+issue.AuthorLogin, width))
	}
	if labels := strings.Join(issue.Labels, ", "); labels != "" {
		lines = append(lines, truncate("Labels: "+labels, width))
	}
	if assignees := strings.Join(issue.Assignees, ", "); assignees != "" {
		lines = append(lines, truncate("Assignees: "+assignees, width))
	}
	if issue.Milestone != "" {
		lines = append(lines, truncate("Milestone: "+issue.Milestone, width))
	}
	if issue.UpdatedAt != "" {
		lines = append(lines, truncate("Updated: "+issue.UpdatedAt, width))
	}
	if issue.CommentsCount > 0 {
		lines = append(lines, truncate("Comments: "+issueCommentsLabel(issue.CommentsCount), width))
	}
	if prs := m.prsByIssueNumber[issue.Number]; len(prs) > 0 {
		lines = append(lines, "Linked PRs:")
		for _, pr := range prs {
			lines = append(lines, truncate("  "+pullRequestIssueLinkLabel(pr), width))
		}
	}
	if excerpt := issueBodyExcerpt(issue.Body); excerpt != "" {
		lines = append(lines, truncate("Body: "+excerpt, width))
	}
	if issue.URL != "" {
		lines = append(lines, truncate("URL: "+issue.URL, width))
	}
	return lines
}

func issueListMeta(issue workbench.IssueRef) string {
	parts := []string{}
	if labels := strings.Join(issue.Labels, ","); labels != "" {
		parts = append(parts, "labels:"+labels)
	}
	if assignees := strings.Join(issue.Assignees, ","); assignees != "" {
		parts = append(parts, "assignees:"+assignees)
	}
	if issue.Milestone != "" {
		parts = append(parts, "milestone:"+issue.Milestone)
	}
	if issue.UpdatedAt != "" {
		parts = append(parts, "updated:"+issue.UpdatedAt)
	}
	return strings.Join(parts, "  ")
}

func issueNumberLabel(number int) string {
	if number == 0 {
		return "#?"
	}
	return fmt.Sprintf("#%d", number)
}

func issueStateLabel(state string) string {
	if state == "" {
		return "unknown"
	}
	return strings.ToLower(state)
}

func issueCommentShortLabel(count int) string {
	if count <= 0 {
		return "-"
	}
	return fmt.Sprintf("%dc", count)
}

func issueCommentsLabel(count int) string {
	if count == 1 {
		return "1 comment"
	}
	return fmt.Sprintf("%d comments", count)
}

func pullRequestIssueLinkLabel(pr workbench.PullRequestRef) string {
	number := "#?"
	if pr.Number > 0 {
		number = fmt.Sprintf("#%d", pr.Number)
	}
	if pr.Title == "" {
		return number
	}
	return number + " " + pr.Title
}

func issueAssigneeFilterValues(issues []workbench.IssueRef, viewerLogin string) []string {
	values := []string{""}
	if viewerLogin != "" {
		values = append(values, issueAssigneeMe)
	}
	values = append(values, uniqueSortedIssueValues(issues, func(issue workbench.IssueRef) []string {
		return issue.Assignees
	})...)
	values = append(values, issueAssigneeUnassigned)
	return compactUniqueStrings(values)
}

func issueAssigneeFilterLabel(value string) string {
	switch value {
	case "":
		return "any"
	case issueAssigneeUnassigned:
		return "unassigned"
	default:
		return value
	}
}

func issueOptionalFilterLabel(value string) string {
	if value == "" {
		return "any"
	}
	return value
}

func nextFilterValue(values []string, current string) string {
	if len(values) == 0 {
		return ""
	}
	for i, value := range values {
		if value == current {
			return values[(i+1)%len(values)]
		}
	}
	return values[0]
}

func uniqueSortedIssueValues(issues []workbench.IssueRef, values func(workbench.IssueRef) []string) []string {
	seen := map[string]string{}
	for _, issue := range issues {
		for _, value := range values(issue) {
			if value == "" {
				continue
			}
			key := strings.ToLower(value)
			if _, ok := seen[key]; !ok {
				seen[key] = value
			}
		}
	}
	out := make([]string, 0, len(seen))
	for _, value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func compactUniqueStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		out = append(out, value)
		seen[key] = true
	}
	return out
}

func hasCaseFolded(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(value, target) {
			return true
		}
	}
	return false
}

func issuesFromWorkItems(items []workbench.WorkItem, repo workbench.RepoRef) []workbench.IssueRef {
	byNumber := map[int]workbench.IssueRef{}
	ordered := []workbench.IssueRef{}
	for _, item := range items {
		if item.Repo != repo || item.Issue == nil || item.Issue.Number == 0 {
			continue
		}
		if _, ok := byNumber[item.Issue.Number]; ok {
			continue
		}
		issue := *item.Issue
		byNumber[issue.Number] = issue
		ordered = append(ordered, issue)
	}
	return ordered
}

func mergeIssueRefs(primary []workbench.IssueRef, extras []workbench.IssueRef) []workbench.IssueRef {
	out := make([]workbench.IssueRef, 0, len(primary)+len(extras))
	seen := map[int]bool{}
	for _, issue := range primary {
		if issue.Number == 0 || seen[issue.Number] {
			continue
		}
		seen[issue.Number] = true
		out = append(out, issue)
	}
	for _, issue := range extras {
		if issue.Number == 0 || seen[issue.Number] {
			continue
		}
		seen[issue.Number] = true
		out = append(out, issue)
	}
	return out
}

func pullRequestsFromWorkItems(items []workbench.WorkItem, repo workbench.RepoRef) []workbench.PullRequestRef {
	seen := map[int]bool{}
	prs := []workbench.PullRequestRef{}
	for _, item := range items {
		if item.Repo != repo || item.PullRequest == nil {
			continue
		}
		if item.PullRequest.Number > 0 {
			if seen[item.PullRequest.Number] {
				continue
			}
			seen[item.PullRequest.Number] = true
		}
		prs = append(prs, *item.PullRequest)
	}
	return prs
}

func pullRequestsByIssueNumber(prs []workbench.PullRequestRef) map[int][]workbench.PullRequestRef {
	out := map[int][]workbench.PullRequestRef{}
	for _, pr := range prs {
		seenIssues := map[int]bool{}
		for _, issue := range pr.LinkedIssues {
			if issue.Number == 0 || seenIssues[issue.Number] {
				continue
			}
			out[issue.Number] = append(out[issue.Number], pr)
			seenIssues[issue.Number] = true
		}
	}
	return out
}

func issueDiscoveryErrorFromWorkItems(items []workbench.WorkItem, repo workbench.RepoRef) string {
	for _, item := range items {
		if item.Repo != repo || !strings.HasPrefix(item.ID, "issue-check-discovery-error:") || item.Local == nil {
			continue
		}
		return item.Local.Summary
	}
	return ""
}

func cloneIssueRefs(issues []workbench.IssueRef) []workbench.IssueRef {
	return append([]workbench.IssueRef(nil), issues...)
}

func emptyFallback(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func dropLastRune(value string) string {
	if value == "" {
		return ""
	}
	_, size := utf8.DecodeLastRuneInString(value)
	if size <= 0 {
		return ""
	}
	return value[:len(value)-size]
}
