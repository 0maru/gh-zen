package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	cfgpkg "github.com/0maru/gh-zen/internal/config"
	"github.com/0maru/gh-zen/internal/workbench"
)

func hasWorkItem(items []workbench.WorkItem, match func(workbench.WorkItem) bool) bool {
	for _, item := range items {
		if match(item) {
			return true
		}
	}
	return false
}

func TestIssueFiltering_CoversStateAssigneeLabelMilestoneAndSearch(t *testing.T) {
	issues := []workbench.IssueRef{
		{
			Number:    1,
			Title:     "Add issue browser",
			State:     "open",
			Body:      "Show labels and assignees in the terminal.",
			Labels:    []string{"enhancement"},
			Assignees: []string{"0maru"},
			Milestone: "v1",
		},
		{
			Number: 2,
			Title:  "設定を検索する",
			State:  "closed",
			Body:   "Unicode search should preserve Japanese text.",
			Labels: []string{"bug"},
		},
	}

	assertNumbers := func(name string, filter issueFilterState, want ...int) {
		t.Helper()
		got := filterIssues(issues, filter, "0maru")
		if len(got) != len(want) {
			t.Fatalf("%s: expected %v, got %+v", name, want, got)
		}
		for i, issue := range got {
			if issue.Number != want[i] {
				t.Fatalf("%s: expected %v, got %+v", name, want, got)
			}
		}
	}

	assertNumbers("default open", defaultIssueFilterState(), 1)
	assertNumbers("closed", issueFilterState{State: issueStateClosed}, 2)
	assertNumbers("all", issueFilterState{State: issueStateAll}, 1, 2)
	assertNumbers("me", issueFilterState{State: issueStateAll, Assignee: issueAssigneeMe}, 1)
	assertNumbers("unassigned", issueFilterState{State: issueStateAll, Assignee: issueAssigneeUnassigned}, 2)
	assertNumbers("label", issueFilterState{State: issueStateAll, Label: "bug"}, 2)
	assertNumbers("milestone", issueFilterState{State: issueStateAll, Milestone: "v1"}, 1)
	assertNumbers("unicode search", issueFilterState{State: issueStateAll, Search: "設定"}, 2)
}

func TestIssueFiltering_SearchesFullIssueBody(t *testing.T) {
	body := strings.Repeat("setup details ", 30) + "final diagnostic marker"
	got := filterIssues([]workbench.IssueRef{{
		Number: 1,
		Title:  "Long issue template",
		State:  "open",
		Body:   body,
	}}, issueFilterState{State: issueStateAll, Search: "diagnostic marker"}, "")

	if len(got) != 1 || got[0].Number != 1 {
		t.Fatalf("expected search to match full issue body, got %+v", got)
	}
}

func TestIssuePreviewLines_ShowsLinkedPRsAndComments(t *testing.T) {
	issue := workbench.IssueRef{
		Number:        75,
		Title:         "Add first-class issue browsing",
		State:         "open",
		URL:           "https://example.test/issues/75",
		Body:          "Render issue title, body, labels, linked pull requests, and comments count.",
		Labels:        []string{"enhancement", "ux"},
		Assignees:     []string{"0maru"},
		Milestone:     "v1",
		AuthorLogin:   "alice",
		CommentsCount: 2,
		UpdatedAt:     "2026-06-21T12:00:00Z",
	}
	m := newModel()
	m.screen = screenIssues
	m.issueRepo = workbench.RepoRef{Owner: "0maru", Name: "gh-zen"}
	m.issues = []workbench.IssueRef{issue}
	m.prsByIssueNumber = pullRequestsByIssueNumber([]workbench.PullRequestRef{{
		Number: 24,
		Title:  "Add issue view",
		LinkedIssues: []workbench.IssueRef{
			{Number: 75},
		},
	}})

	got := strings.Join(m.issuePreviewLines(120), "\n")
	for _, want := range []string{
		"Issue: #75 Add first-class issue browsing",
		"State: open",
		"Author: alice",
		"Labels: enhancement, ux",
		"Assignees: 0maru",
		"Milestone: v1",
		"Updated: 2026-06-21T12:00:00Z",
		"Comments: 2 comments",
		"Linked PRs:",
		"#24 Add issue view",
		"Body: Render issue title",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected preview to contain %q, got:\n%s", want, got)
		}
	}
}

func TestIssuePreviewLines_OmitsUnavailableLinkedPRsAndZeroComments(t *testing.T) {
	m := newModel()
	m.screen = screenIssues
	m.issues = []workbench.IssueRef{{Number: 1, Title: "No comments", State: "open"}}
	m.prsByIssueNumber = map[int][]workbench.PullRequestRef{}

	got := strings.Join(m.issuePreviewLines(120), "\n")
	if strings.Contains(got, "Comments:") || strings.Contains(got, "Linked PRs:") {
		t.Fatalf("expected unavailable summary rows to be omitted, got:\n%s", got)
	}
}

func TestUpdate_IssueViewUsesSelectedWorkItemRepo(t *testing.T) {
	repoA := workbench.RepoRef{Owner: "0maru", Name: "gh-zen"}
	repoB := workbench.RepoRef{Owner: "0maru", Name: "dotfiles"}
	repoAItem := workbench.WorkItem{
		ID:       "branch:repo-a",
		Repo:     repoA,
		Branch:   &workbench.BranchRef{Name: "repo-a"},
		Worktree: &workbench.WorktreeRef{Path: "/tmp/repo-a"},
	}
	repoBItem := workbench.WorkItem{
		ID:       "branch:repo-b",
		Repo:     repoB,
		Branch:   &workbench.BranchRef{Name: "repo-b"},
		Worktree: &workbench.WorktreeRef{Path: "/tmp/repo-b"},
		Issue:    &workbench.IssueRef{Number: 22, Title: "Repo B issue", State: "open"},
	}
	start := newModelWithRuntimeData(cfgpkg.Defaults(), repoA.FullName(), WorkbenchData{
		Repos:     []workbench.RepoRef{repoA, repoB},
		WorkItems: []workbench.WorkItem{repoAItem, repoBItem},
	}, fakeDelayedPreviewLoader(0))
	start.setRepoPaneIndex(len(start.repos))
	start.selectedItem = 1
	start.focusedWorkItemID = repoBItem.ID

	got, cmd := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	if cmd != nil {
		t.Fatalf("expected issue view transition without command, got %T", cmd)
	}
	mm := got.(model)
	if mm.issueRepo != repoB {
		t.Fatalf("expected issue view repo %+v, got %+v", repoB, mm.issueRepo)
	}
	issue, ok := mm.selectedIssueRef()
	if !ok || issue.Number != 22 {
		t.Fatalf("expected selected issue #22 from repo B, got %+v ok=%v", issue, ok)
	}
}

func TestUpdate_IssueViewRefreshUsesIssueRepo(t *testing.T) {
	repoA := workbench.RepoRef{Owner: "0maru", Name: "gh-zen"}
	repoB := workbench.RepoRef{Owner: "0maru", Name: "dotfiles"}
	repoAItem := workbench.WorkItem{
		ID:       "branch:repo-a",
		Repo:     repoA,
		Branch:   &workbench.BranchRef{Name: "repo-a"},
		Worktree: &workbench.WorktreeRef{Path: "/tmp/repo-a"},
	}
	repoBItem := workbench.WorkItem{
		ID:       "branch:repo-b",
		Repo:     repoB,
		Branch:   &workbench.BranchRef{Name: "repo-b"},
		Worktree: &workbench.WorktreeRef{Path: "/tmp/repo-b"},
		Issue:    &workbench.IssueRef{Number: 22, Title: "Repo B issue", State: "open"},
	}
	reloader := &fakeWorkbenchReloader{
		results: map[string]workbench.RuntimeLoadResult{
			repoB.FullName(): {Repo: repoB, Items: []workbench.WorkItem{repoBItem}},
		},
	}
	start := newModelWithRuntimeData(cfgpkg.Defaults(), repoA.FullName(), WorkbenchData{
		Repos:     []workbench.RepoRef{repoA, repoB},
		WorkItems: []workbench.WorkItem{repoAItem, repoBItem},
		Reloader:  reloader,
	}, fakeDelayedPreviewLoader(0))
	start.setRepoPaneIndex(len(start.repos))
	start.selectedItem = 1
	start.focusedWorkItemID = repoBItem.ID

	got, _ := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	got, cmd := got.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	msg := requireWorkbenchReloadMsg(t, cmd)

	if msg.request.repo != repoB {
		t.Fatalf("expected issue refresh request for %+v, got %+v", repoB, msg.request.repo)
	}
	if len(reloader.calls) != 1 || reloader.calls[0] != repoB {
		t.Fatalf("expected reload call for %+v, got %+v", repoB, reloader.calls)
	}
}

func TestUpdate_IssueViewBackRestoresWorkbenchSelection(t *testing.T) {
	start := newModel()
	start.selectedItem = 2
	start.focusedPane = paneWorkItems
	start.focusedWorkItemID = start.visibleWorkItems()[2].ID

	got, _ := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	mm := got.(model)
	if mm.screen != screenIssues {
		t.Fatalf("expected issue screen, got %v", mm.screen)
	}
	got, cmd := mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd != nil {
		t.Fatalf("expected back to workbench without preview reload, got %T", cmd)
	}
	mm = got.(model)
	if mm.screen != screenWorkbench || mm.selectedItem != 2 || mm.focusedPane != paneWorkItems {
		t.Fatalf("expected workbench selection to be restored, got screen=%v item=%d focus=%v", mm.screen, mm.selectedItem, mm.focusedPane)
	}
}

func TestUpdate_IssueViewRefreshUpdatesReturnSelection(t *testing.T) {
	repo := workbench.RepoRef{Owner: "0maru", Name: "gh-zen"}
	first := workbench.WorkItem{ID: "branch:first", Repo: repo, Branch: &workbench.BranchRef{Name: "first"}}
	second := workbench.WorkItem{ID: "branch:second", Repo: repo, Branch: &workbench.BranchRef{Name: "second"}}
	reloader := &fakeWorkbenchReloader{
		results: map[string]workbench.RuntimeLoadResult{
			repo.FullName(): {
				Repo:  repo,
				Items: []workbench.WorkItem{second, first},
			},
		},
	}
	start := newModelWithRuntimeData(cfgpkg.Defaults(), repo.FullName(), WorkbenchData{
		Repos:     []workbench.RepoRef{repo},
		WorkItems: []workbench.WorkItem{first, second},
		Reloader:  reloader,
	}, fakeDelayedPreviewLoader(0))
	start.selectedItem = 1
	start.focusedWorkItemID = second.ID

	got, _ := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	got, cmd := got.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Fatalf("expected issue refresh command")
	}
	got, _ = got.(model).Update(requireWorkbenchReloadMsg(t, cmd))
	got, _ = got.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	mm := got.(model)

	if mm.selectedItem != 0 {
		t.Fatalf("expected return selection to follow reloaded item to index 0, got %d", mm.selectedItem)
	}
	if item, ok := mm.selectedWorkItem(); !ok || item.ID != second.ID {
		t.Fatalf("expected selected work item %q, got %+v ok=%v", second.ID, item, ok)
	}
}

func TestUpdate_IssueViewRefreshPreservesReturnRepo(t *testing.T) {
	repoA := workbench.RepoRef{Owner: "0maru", Name: "gh-zen"}
	repoB := workbench.RepoRef{Owner: "0maru", Name: "dotfiles"}
	repoAItem := workbench.WorkItem{
		ID:       "branch:repo-a",
		Repo:     repoA,
		Branch:   &workbench.BranchRef{Name: "repo-a"},
		Worktree: &workbench.WorktreeRef{Path: "/tmp/repo-a"},
	}
	repoBItem := workbench.WorkItem{
		ID:       "branch:repo-b",
		Repo:     repoB,
		Branch:   &workbench.BranchRef{Name: "repo-b"},
		Worktree: &workbench.WorktreeRef{Path: "/tmp/repo-b"},
		Issue:    &workbench.IssueRef{Number: 22, Title: "Repo B issue", State: "open"},
	}
	reloader := &fakeWorkbenchReloader{
		results: map[string]workbench.RuntimeLoadResult{
			repoB.FullName(): {
				Repo: repoB,
				Repositories: []workbench.RepositorySummary{
					{Repo: repoB, Path: "/repos/dotfiles"},
					{Repo: repoA, Path: "/repos/gh-zen"},
				},
				Items: []workbench.WorkItem{repoBItem, repoAItem},
			},
		},
	}
	start := newModelWithRuntimeData(cfgpkg.Defaults(), repoA.FullName(), WorkbenchData{
		Repos:     []workbench.RepoRef{repoA, repoB},
		WorkItems: []workbench.WorkItem{repoAItem, repoBItem},
		Reloader:  reloader,
	}, fakeDelayedPreviewLoader(0))
	start.setRepoPaneIndex(len(start.repos))
	start.selectedItem = 1
	start.focusedWorkItemID = repoBItem.ID

	got, _ := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	got, cmd := got.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	got, _ = got.(model).Update(requireWorkbenchReloadMsg(t, cmd))
	got, _ = got.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	mm := got.(model)

	if mm.screen != screenWorkbench {
		t.Fatalf("expected workbench screen, got %v", mm.screen)
	}
	if repo, ok := mm.selectedRepoRef(); !ok || repo != repoA {
		t.Fatalf("expected return repo %+v, got %+v ok=%v", repoA, repo, ok)
	}
}

func TestUpdate_IssueRefreshAppliesAfterReturningToWorkbench(t *testing.T) {
	repoA := workbench.RepoRef{Owner: "0maru", Name: "gh-zen"}
	repoB := workbench.RepoRef{Owner: "0maru", Name: "dotfiles"}
	repoAItem := workbench.WorkItem{
		ID:       "branch:repo-a",
		Repo:     repoA,
		Branch:   &workbench.BranchRef{Name: "repo-a"},
		Worktree: &workbench.WorktreeRef{Path: "/tmp/repo-a"},
	}
	repoBItem := workbench.WorkItem{
		ID:       "branch:repo-b",
		Repo:     repoB,
		Branch:   &workbench.BranchRef{Name: "repo-b"},
		Worktree: &workbench.WorktreeRef{Path: "/tmp/repo-b"},
		Issue:    &workbench.IssueRef{Number: 22, Title: "Repo B issue", State: "open"},
	}
	repoBReloaded := repoBItem
	repoBReloaded.Issue = &workbench.IssueRef{Number: 23, Title: "Reloaded repo B issue", State: "open"}
	reloader := &fakeWorkbenchReloader{
		results: map[string]workbench.RuntimeLoadResult{
			repoB.FullName(): {
				Repo: repoB,
				Repositories: []workbench.RepositorySummary{
					{Repo: repoA, Path: "/repos/gh-zen"},
					{Repo: repoB, Path: "/repos/dotfiles"},
				},
				Items: []workbench.WorkItem{repoAItem, repoBReloaded},
			},
		},
	}
	start := newModelWithRuntimeData(cfgpkg.Defaults(), repoA.FullName(), WorkbenchData{
		Repos:     []workbench.RepoRef{repoA, repoB},
		WorkItems: []workbench.WorkItem{repoAItem, repoBItem},
		Reloader:  reloader,
	}, fakeDelayedPreviewLoader(0))
	start.setRepoPaneIndex(len(start.repos))
	start.selectedItem = 1
	start.focusedWorkItemID = repoBItem.ID

	got, _ := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	got, cmd := got.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	msg := requireWorkbenchReloadMsg(t, cmd)
	got, _ = got.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	got, _ = got.(model).Update(msg)
	mm := got.(model)

	if repo, ok := mm.selectedRepoRef(); !ok || repo != repoA {
		t.Fatalf("expected current workbench repo %+v to be preserved, got %+v ok=%v", repoA, repo, ok)
	}
	if !hasWorkItem(mm.workItems, func(item workbench.WorkItem) bool {
		return item.Repo == repoB && item.Issue != nil && item.Issue.Number == 23
	}) {
		t.Fatalf("expected repo B refresh result to be applied, got %+v", mm.workItems)
	}
}

func TestUpdate_IssueSearchPreservesUnicodeInput(t *testing.T) {
	repo := workbench.RepoRef{Owner: "0maru", Name: "gh-zen"}
	start := newModelWithRuntimeData(cfgpkg.Defaults(), repo.FullName(), WorkbenchData{
		Repos: []workbench.RepoRef{repo},
		Issues: []workbench.IssueRef{
			{Number: 1, Title: "English", State: "open"},
			{Number: 2, Title: "設定を検索する", State: "open"},
		},
	}, fakeDelayedPreviewLoader(0))
	start.screen = screenIssues
	start.issueRepo = repo

	got, _ := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	got, _ = got.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("設定")})
	got, _ = got.(model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := got.(model)

	if mm.issueFilter.Search != "設定" {
		t.Fatalf("expected unicode query to be preserved, got %q", mm.issueFilter.Search)
	}
	issues := mm.visibleIssues()
	if len(issues) != 1 || issues[0].Number != 2 {
		t.Fatalf("expected search to select issue #2, got %+v", issues)
	}
}

func TestUpdate_IssueViewActionsCopyAndOpenSelectedIssue(t *testing.T) {
	runner := &fakeActionRunner{}
	start := newModel()
	start.actionRunner = runner
	start.selectedItem = 1

	got, _ := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	got, cmd := got.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if cmd == nil {
		t.Fatalf("expected copy issue URL command")
	}
	got, _ = got.(model).Update(cmd())
	if len(runner.copied) != 1 || runner.copied[0] != "https://github.com/0maru/gh-zen/issues/9" {
		t.Fatalf("expected issue URL to copy, got %#v", runner.copied)
	}

	got, cmd = got.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if cmd == nil {
		t.Fatalf("expected copy issue number command")
	}
	got, _ = got.(model).Update(cmd())
	if len(runner.copied) != 2 || runner.copied[1] != "#9" {
		t.Fatalf("expected issue number to copy, got %#v", runner.copied)
	}

	got, cmd = got.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if cmd == nil {
		t.Fatalf("expected open issue command")
	}
	got, _ = got.(model).Update(cmd())
	if len(runner.opened) != 1 || runner.opened[0] != "https://github.com/0maru/gh-zen/issues/9" {
		t.Fatalf("expected issue URL to open, got %#v", runner.opened)
	}
}

func TestUpdate_IssueRefreshUpdatesIssueData(t *testing.T) {
	repo := workbench.RepoRef{Owner: "0maru", Name: "gh-zen"}
	reloader := &fakeWorkbenchReloader{
		results: map[string]workbench.RuntimeLoadResult{
			repo.FullName(): {
				Repo:         repo,
				IssuesLoaded: true,
				Issues: []workbench.IssueRef{
					{Number: 2, Title: "Reloaded issue", State: "open"},
				},
				PullRequestsLoaded: true,
				PullRequests: []workbench.PullRequestRef{
					{Number: 24, Title: "Linked PR", LinkedIssues: []workbench.IssueRef{{Number: 2}}},
				},
			},
		},
	}
	linkedItem := workbench.WorkItem{
		ID:    "pull-request:missing-linked-issue",
		Repo:  repo,
		Issue: &workbench.IssueRef{Number: 99, Title: "Linked issue outside list", State: "open"},
	}
	start := newModelWithRuntimeData(cfgpkg.Defaults(), repo.FullName(), WorkbenchData{
		Repos:    []workbench.RepoRef{repo},
		Reloader: reloader,
		Issues:   []workbench.IssueRef{{Number: 1, Title: "Old issue", State: "open"}},
	}, fakeDelayedPreviewLoader(0))
	reloader.results[repo.FullName()] = workbench.RuntimeLoadResult{
		Repo: repo,
		Items: []workbench.WorkItem{
			linkedItem,
		},
		IssuesLoaded: true,
		Issues: []workbench.IssueRef{
			{Number: 2, Title: "Reloaded issue", State: "open"},
		},
		PullRequestsLoaded: true,
		PullRequests: []workbench.PullRequestRef{
			{Number: 24, Title: "Linked PR", LinkedIssues: []workbench.IssueRef{{Number: 2}}},
		},
	}
	start.screen = screenIssues

	got, cmd := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Fatalf("expected issue refresh command")
	}
	mm := got.(model)
	if !mm.issuesLoading {
		t.Fatalf("expected issue loading state during refresh")
	}
	got, cmd = mm.Update(requireWorkbenchReloadMsg(t, cmd))
	if cmd != nil {
		t.Fatalf("expected issue reload not to start workbench preview command, got %T", cmd)
	}
	mm = got.(model)
	if len(mm.issues) != 2 || mm.issues[0].Number != 2 || mm.issues[1].Number != 99 {
		t.Fatalf("expected reloaded issue data, got %+v", mm.issues)
	}
	if prs := mm.prsByIssueNumber[2]; len(prs) != 1 || prs[0].Number != 24 {
		t.Fatalf("expected linked PR index from reload, got %+v", mm.prsByIssueNumber)
	}
}
