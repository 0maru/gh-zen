package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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

	got, cmd := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	got, _ = got.(model).Update(requireWorkbenchReloadMsg(t, cmd))
	got, cmd = got.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	msg := requireWorkbenchReloadMsg(t, cmd)

	if msg.request.repo != repoB {
		t.Fatalf("expected issue refresh request for %+v, got %+v", repoB, msg.request.repo)
	}
	if len(reloader.calls) != 2 || reloader.calls[0] != repoB || reloader.calls[1] != repoB {
		t.Fatalf("expected reload call for %+v, got %+v", repoB, reloader.calls)
	}
}

func TestUpdate_IssueViewLoadsRawIssuesForSelectedRepo(t *testing.T) {
	repoA := workbench.RepoRef{Owner: "0maru", Name: "gh-zen"}
	repoB := workbench.RepoRef{Owner: "0maru", Name: "dotfiles"}
	reloader := &fakeWorkbenchReloader{
		results: map[string]workbench.RuntimeLoadResult{
			repoB.FullName(): {
				Repo:         repoB,
				IssuesLoaded: true,
				Issues: []workbench.IssueRef{{
					Number:  44,
					Title:   "Unlinked repo B issue",
					State:   "open",
					Certain: true,
				}},
			},
		},
	}
	start := newModelWithRuntimeData(cfgpkg.Defaults(), repoA.FullName(), WorkbenchData{
		Repos: []workbench.RepoRef{repoA, repoB},
		WorkItems: []workbench.WorkItem{{
			ID:     "branch:repo-a",
			Repo:   repoA,
			Branch: &workbench.BranchRef{Name: "repo-a"},
		}},
		Reloader: reloader,
	}, fakeDelayedPreviewLoader(0))
	start.setRepoPaneIndex(1)

	got, cmd := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	msg := requireWorkbenchReloadMsg(t, cmd)
	if msg.request.repo != repoB {
		t.Fatalf("expected issue view reload request for %+v, got %+v", repoB, msg.request.repo)
	}
	got, _ = got.(model).Update(msg)
	mm := got.(model)

	if len(reloader.calls) != 1 || reloader.calls[0] != repoB {
		t.Fatalf("expected reload call for %+v, got %+v", repoB, reloader.calls)
	}
	if issues := mm.visibleIssues(); len(issues) != 1 || issues[0].Number != 44 {
		t.Fatalf("expected raw repo B issue after issue view reload, got %+v", issues)
	}
}

func TestUpdate_IssueViewUsesRepoScopedReloader(t *testing.T) {
	repo := workbench.RepoRef{Owner: "0maru", Name: "gh-zen"}
	workbenchReloader := &fakeWorkbenchReloader{}
	issueReloader := &fakeIssueReloader{
		results: map[string]workbench.RuntimeLoadResult{
			repo.FullName(): {
				Repo:         repo,
				IssuesRepo:   repo,
				IssuesLoaded: true,
				Issues:       []workbench.IssueRef{{Number: 75, Title: "Issue browsing", State: "open"}},
			},
		},
	}
	start := newModelWithRuntimeData(cfgpkg.Defaults(), repo.FullName(), WorkbenchData{
		Repos:         []workbench.RepoRef{repo},
		Reloader:      workbenchReloader,
		IssueReloader: issueReloader,
	}, fakeDelayedPreviewLoader(0))

	got, cmd := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	msg := requireWorkbenchReloadMsg(t, cmd)
	got, _ = got.(model).Update(msg)
	mm := got.(model)

	if len(workbenchReloader.calls) != 0 {
		t.Fatalf("expected issue entry not to scan the full workbench, got %+v", workbenchReloader.calls)
	}
	if len(issueReloader.calls) != 1 || issueReloader.calls[0] != repo {
		t.Fatalf("expected one repo-scoped issue reload for %+v, got %+v", repo, issueReloader.calls)
	}
	if issues := mm.visibleIssues(); len(issues) != 1 || issues[0].Number != 75 {
		t.Fatalf("expected repo-scoped issues to be applied, got %+v", issues)
	}
}

func TestUpdate_IssueEntryWaitsForActiveWorkbenchReload(t *testing.T) {
	repo := workbench.RepoRef{Owner: "0maru", Name: "gh-zen"}
	workbenchReloader := &fakeWorkbenchReloader{}
	issueReloader := &fakeIssueReloader{
		results: map[string]workbench.RuntimeLoadResult{
			repo.FullName(): {
				Repo:         repo,
				IssuesRepo:   repo,
				IssuesLoaded: true,
				Issues:       []workbench.IssueRef{{Number: 75, Title: "Issue browsing", State: "open"}},
			},
		},
	}
	start := newModelWithRuntimeData(cfgpkg.Defaults(), repo.FullName(), WorkbenchData{
		Repos:          []workbench.RepoRef{repo},
		Reloader:       workbenchReloader,
		IssueReloader:  issueReloader,
		InitialLoading: true,
	}, fakeDelayedPreviewLoader(0))
	initialRequest := start.activeReloadRequest

	got, cmd := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	if cmd != nil {
		t.Fatalf("expected issue entry to keep the active workbench reload, got a new command")
	}
	mm := got.(model)
	if mm.activeReloadRequest != initialRequest || !mm.issueReloadPending {
		t.Fatalf("expected active reload to remain pending before issue reload, got active=%+v pending=%v", mm.activeReloadRequest, mm.issueReloadPending)
	}

	got, cmd = mm.Update(workbenchReloadMsg{
		request: initialRequest,
		result: workbench.RuntimeLoadResult{
			Repo:         repo,
			Repositories: []workbench.RepositorySummary{{Repo: repo}},
			IssuesRepo:   repo,
			IssuesLoaded: true,
		},
	})
	if cmd == nil {
		t.Fatalf("expected a repo-scoped issue reload after the workbench reload")
	}
	mm = got.(model)
	if !mm.activeReloadRequest.issueScoped || mm.issueReloadPending {
		t.Fatalf("expected chained issue reload, got active=%+v pending=%v", mm.activeReloadRequest, mm.issueReloadPending)
	}
	requireWorkbenchReloadMsg(t, cmd)
	if len(issueReloader.calls) != 1 || issueReloader.calls[0] != repo {
		t.Fatalf("expected one delayed repo-scoped reload for %+v, got %+v", repo, issueReloader.calls)
	}
}

func TestUpdate_IssueRefreshWaitsForActiveWorkbenchReload(t *testing.T) {
	repo := workbench.RepoRef{Owner: "0maru", Name: "gh-zen"}
	start := newModelWithRuntimeData(cfgpkg.Defaults(), repo.FullName(), WorkbenchData{
		Repos:          []workbench.RepoRef{repo},
		Reloader:       &fakeWorkbenchReloader{},
		IssueReloader:  &fakeIssueReloader{},
		InitialLoading: true,
	}, fakeDelayedPreviewLoader(0))
	initialRequest := start.activeReloadRequest
	start.screen = screenIssues
	start.issueRepo = repo

	got, cmd := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd != nil {
		t.Fatalf("expected issue refresh to keep the active workbench reload, got a new command")
	}
	mm := got.(model)
	if mm.activeReloadRequest != initialRequest || !mm.issueReloadPending || mm.pendingIssueRepo != repo {
		t.Fatalf("expected issue refresh to wait for the active reload, got active=%+v pending=%v repo=%+v", mm.activeReloadRequest, mm.issueReloadPending, mm.pendingIssueRepo)
	}
}

func TestUpdate_IssueRefreshWithFullReloaderDoesNotDuplicateOtherRepositories(t *testing.T) {
	repoA := workbench.RepoRef{Owner: "0maru", Name: "repo-a"}
	repoB := workbench.RepoRef{Owner: "0maru", Name: "repo-b"}
	oldA := workbench.WorkItem{ID: "branch:old-a", Repo: repoA, Branch: &workbench.BranchRef{Name: "old-a"}}
	oldB := workbench.WorkItem{ID: "branch:old-b", Repo: repoB, Branch: &workbench.BranchRef{Name: "old-b"}}
	newA := workbench.WorkItem{ID: "branch:new-a", Repo: repoA, Branch: &workbench.BranchRef{Name: "new-a"}}
	newB := workbench.WorkItem{ID: "branch:new-b", Repo: repoB, Branch: &workbench.BranchRef{Name: "new-b"}}
	reloader := &fakeWorkbenchReloader{results: map[string]workbench.RuntimeLoadResult{
		repoA.FullName(): {Repo: repoA, Items: []workbench.WorkItem{newA, newB}},
	}}
	start := newModelWithRuntimeData(cfgpkg.Defaults(), repoA.FullName(), WorkbenchData{
		Repos:     []workbench.RepoRef{repoA, repoB},
		WorkItems: []workbench.WorkItem{oldA, oldB},
		Reloader:  reloader,
	}, fakeDelayedPreviewLoader(0))
	start.screen = screenIssues
	start.issueRepo = repoA

	got, cmd := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	msg := requireWorkbenchReloadMsg(t, cmd)
	if !msg.request.issueScoped || !msg.request.fullResult {
		t.Fatalf("expected issue lifecycle with full-result semantics, got %+v", msg.request)
	}
	got, _ = got.(model).Update(msg)
	mm := got.(model)

	if hasWorkItem(mm.workItems, func(item workbench.WorkItem) bool { return item.ID == oldA.ID || item.ID == newB.ID }) {
		t.Fatalf("expected only repo A items from the full-reloader result to replace repo A, got %+v", mm.workItems)
	}
	for _, id := range []string{newA.ID, oldB.ID} {
		count := 0
		for _, item := range mm.workItems {
			if item.ID == id {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("expected exactly one %q item, got %d in %+v", id, count, mm.workItems)
		}
	}
}

func TestUpdate_IssueEntryStartsPendingReloadAfterDifferentRepoWorkbenchResponse(t *testing.T) {
	repoA := workbench.RepoRef{Owner: "0maru", Name: "repo-a"}
	repoB := workbench.RepoRef{Owner: "0maru", Name: "repo-b"}
	issueReloader := &fakeIssueReloader{results: map[string]workbench.RuntimeLoadResult{
		repoB.FullName(): {
			Repo:         repoB,
			IssuesRepo:   repoB,
			IssuesLoaded: true,
			Issues:       []workbench.IssueRef{{Number: 75, Title: "Repo B issue", State: "open"}},
		},
	}}
	start := newModelWithRuntimeData(cfgpkg.Defaults(), repoA.FullName(), WorkbenchData{
		Repos:          []workbench.RepoRef{repoA, repoB},
		Reloader:       &fakeWorkbenchReloader{},
		IssueReloader:  issueReloader,
		InitialLoading: true,
	}, fakeDelayedPreviewLoader(0))
	initialRequest := start.activeReloadRequest
	start.screen = screenIssues
	start.issueRepo = repoB
	start.issueReloadPending = true
	start.pendingIssueRepo = repoB
	start.issuesLoading = true

	got, cmd := start.Update(workbenchReloadMsg{
		request: initialRequest,
		result: workbench.RuntimeLoadResult{
			Repo:         repoA,
			Repositories: []workbench.RepositorySummary{{Repo: repoA}, {Repo: repoB}},
			Items: []workbench.WorkItem{{
				ID:   "repo-a-refreshed-item",
				Repo: repoA,
			}},
		},
	})
	if cmd == nil {
		t.Fatalf("expected pending repo B reload after stale repo A response")
	}
	mm := got.(model)
	if !mm.activeReloadRequest.issueScoped || mm.activeReloadRequest.repo != repoB {
		t.Fatalf("expected repo B scoped reload, got %+v", mm.activeReloadRequest)
	}
	if !hasWorkItem(mm.workItems, func(item workbench.WorkItem) bool { return item.ID == "repo-a-refreshed-item" }) {
		t.Fatalf("expected the full workbench response to be applied before the pending reload, got %+v", mm.workItems)
	}
	requireWorkbenchReloadMsg(t, cmd)
	if len(issueReloader.calls) != 1 || issueReloader.calls[0] != repoB {
		t.Fatalf("expected pending reload for %+v, got %+v", repoB, issueReloader.calls)
	}
}

func TestUpdate_IssueReentryQueuesDifferentRepoBehindScopedReload(t *testing.T) {
	repoB := workbench.RepoRef{Owner: "0maru", Name: "repo-b"}
	repoC := workbench.RepoRef{Owner: "0maru", Name: "repo-c"}
	issueReloader := &fakeIssueReloader{results: map[string]workbench.RuntimeLoadResult{
		repoC.FullName(): {
			Repo:         repoC,
			IssuesRepo:   repoC,
			IssuesLoaded: true,
			Issues:       []workbench.IssueRef{{Number: 3, Title: "Repo C issue", State: "open"}},
		},
	}}
	start := newModel()
	start.screen = screenIssues
	start.issueRepo = repoC
	start.issues = []workbench.IssueRef{{Number: 3, Title: "Cached repo C issue", State: "open"}}
	start.issueReloader = issueReloader
	start.workbenchLoading = true
	start.issuesLoading = true
	start.nextReloadRequestID = 1
	start.activeReloadRequest = workbenchReloadRequest{requestID: 1, repo: repoB, status: "Loading issues...", issueScoped: true}

	if cmd := start.startIssueViewReload(); cmd != nil {
		t.Fatalf("expected repo C reload to wait for active repo B reload")
	}
	if !start.issueReloadPending || start.pendingIssueRepo != repoC {
		t.Fatalf("expected repo C to be recorded as pending, got pending=%v repo=%+v", start.issueReloadPending, start.pendingIssueRepo)
	}

	got, cmd := start.Update(workbenchReloadMsg{
		request: workbenchReloadRequest{requestID: 1, repo: repoB, status: "Loading issues...", issueScoped: true},
		result: workbench.RuntimeLoadResult{
			Repo:         repoB,
			IssuesRepo:   repoB,
			IssuesLoaded: true,
			Issues:       []workbench.IssueRef{{Number: 2, Title: "Repo B issue", State: "open"}},
		},
	})
	if cmd == nil {
		t.Fatalf("expected queued repo C reload after repo B completes")
	}
	mm := got.(model)
	if mm.issueRepo != repoC || len(mm.issues) != 1 || mm.issues[0].Number != 3 {
		t.Fatalf("expected visible repo C issue data to stay intact, got repo=%+v issues=%+v", mm.issueRepo, mm.issues)
	}
	if !mm.activeReloadRequest.issueScoped || mm.activeReloadRequest.repo != repoC {
		t.Fatalf("expected active repo C reload, got %+v", mm.activeReloadRequest)
	}
	requireWorkbenchReloadMsg(t, cmd)
}

func TestUpdate_IssueViewKeepsRawIssueSourceRepo(t *testing.T) {
	repo := workbench.RepoRef{Owner: "Owner", Name: "Repo"}
	reloader := &fakeWorkbenchReloader{
		results: map[string]workbench.RuntimeLoadResult{
			repo.FullName(): {
				Repo:         workbench.RepoRef{},
				IssuesRepo:   repo,
				IssuesLoaded: true,
				Issues: []workbench.IssueRef{{
					Number:  44,
					Title:   "Unlinked issue",
					State:   "open",
					Certain: true,
				}},
			},
		},
	}
	start := newModelWithRuntimeData(cfgpkg.Defaults(), repo.FullName(), WorkbenchData{
		Repos:    []workbench.RepoRef{repo},
		Reloader: reloader,
	}, fakeDelayedPreviewLoader(0))
	start.screen = screenIssues
	start.issueRepo = repo

	got, cmd := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Fatalf("expected issue refresh command")
	}
	got, _ = got.(model).Update(requireWorkbenchReloadMsg(t, cmd))
	mm := got.(model)

	if mm.issueRepo != repo {
		t.Fatalf("expected issue repo to remain %+v, got %+v", repo, mm.issueRepo)
	}
	if issues := mm.visibleIssues(); len(issues) != 1 || issues[0].Number != 44 {
		t.Fatalf("expected raw issue to remain visible, got %+v", issues)
	}
	mm.prepareIssueDataForRepo(workbench.RepoRef{Owner: "owner", Name: "repo"})
	if issues := mm.visibleIssues(); len(issues) != 1 || issues[0].Number != 44 {
		t.Fatalf("expected raw issue cache to keep its case-insensitive source repo, got %+v", issues)
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

	got, cmd := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	got, _ = got.(model).Update(requireWorkbenchReloadMsg(t, cmd))
	got, cmd = got.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
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

func TestUpdate_IssueViewReloadRefreshesPreviewOnReturn(t *testing.T) {
	repo := workbench.RepoRef{Owner: "0maru", Name: "gh-zen"}
	original := workbench.WorkItem{
		ID:     "branch:feature",
		Repo:   repo,
		Branch: &workbench.BranchRef{Name: "feature"},
		Issue:  &workbench.IssueRef{Number: 75, Title: "Old issue", State: "open"},
	}
	reloaded := original
	reloaded.Issue = &workbench.IssueRef{Number: 75, Title: "Reloaded issue", State: "open"}
	reloader := &fakeWorkbenchReloader{
		results: map[string]workbench.RuntimeLoadResult{
			repo.FullName(): {
				Repo:               repo,
				Items:              []workbench.WorkItem{reloaded},
				PullRequestsLoaded: true,
				IssuesRepo:         repo,
				IssuesLoaded:       true,
			},
		},
	}
	start := newModelWithRuntimeData(cfgpkg.Defaults(), repo.FullName(), WorkbenchData{
		Repos:     []workbench.RepoRef{repo},
		WorkItems: []workbench.WorkItem{original},
		Reloader:  reloader,
	}, fakeDelayedPreviewLoader(0))
	start.focusedWorkItemRepo = repo
	start.focusedWorkItemID = original.ID
	start.preview = previewState{
		status:              previewLoaded,
		focusedWorkItemRepo: repo,
		focusedWorkItemID:   original.ID,
		loaded: previewData{
			workItemRepo: repo,
			workItemID:   original.ID,
			item:         original,
		},
	}

	got, cmd := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	got, _ = got.(model).Update(requireWorkbenchReloadMsg(t, cmd))
	got, cmd = got.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	got, _ = got.(model).Update(requireWorkbenchReloadMsg(t, cmd))
	got, cmd = got.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatalf("expected preview reload command after issue reload")
	}
	previewMsg := requirePreviewResultMsg(t, cmd)
	if previewMsg.data.item.Issue == nil || previewMsg.data.item.Issue.Title != "Reloaded issue" {
		t.Fatalf("expected preview reload to use reloaded work item, got %+v", previewMsg.data.item)
	}
	got, _ = got.(model).Update(previewMsg)
	mm := got.(model)
	if mm.preview.loaded.item.Issue == nil || mm.preview.loaded.item.Issue.Title != "Reloaded issue" {
		t.Fatalf("expected refreshed preview item, got %+v", mm.preview.loaded.item)
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

	got, cmd := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	got, _ = got.(model).Update(requireWorkbenchReloadMsg(t, cmd))
	got, cmd = got.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
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
	issueReloader := &fakeIssueReloader{
		results: map[string]workbench.RuntimeLoadResult{
			repoB.FullName(): {
				Repo: repoB,
				Repositories: []workbench.RepositorySummary{
					{Repo: repoB, Path: "/repos/dotfiles"},
				},
				Items:              []workbench.WorkItem{repoBReloaded},
				PullRequestsLoaded: true,
				IssuesRepo:         repoB,
				IssuesLoaded:       true,
			},
		},
	}
	start := newModelWithRuntimeData(cfgpkg.Defaults(), repoA.FullName(), WorkbenchData{
		Repos:         []workbench.RepoRef{repoA, repoB},
		WorkItems:     []workbench.WorkItem{repoAItem, repoBItem},
		IssueReloader: issueReloader,
	}, fakeDelayedPreviewLoader(0))
	start.setRepoPaneIndex(len(start.repos))
	start.selectedItem = 1
	start.focusedWorkItemID = repoBItem.ID

	got, cmd := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
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

func TestUpdate_IssueRefreshFailureKeepsRawIssues(t *testing.T) {
	repo := workbench.RepoRef{Owner: "0maru", Name: "gh-zen"}
	start := newModelWithRuntimeData(cfgpkg.Defaults(), repo.FullName(), WorkbenchData{
		Repos:  []workbench.RepoRef{repo},
		Issues: []workbench.IssueRef{{Number: 1, Title: "Previously loaded", State: "open"}},
	}, fakeDelayedPreviewLoader(0))
	start.screen = screenIssues
	start.issueRepo = repo

	start.updateIssueDataFromRuntimeResult(workbench.RuntimeLoadResult{
		Repo: repo,
		Items: []workbench.WorkItem{
			{
				ID:    "linked-issue",
				Repo:  repo,
				Issue: &workbench.IssueRef{Number: 2, Title: "Linked issue", State: "open"},
			},
			{
				ID:     "issue-check-discovery-error:" + repo.FullName(),
				Repo:   repo,
				Local:  &workbench.LocalStatus{Summary: "issue and check discovery failed: network unavailable"},
				Checks: workbench.CheckSummary{State: workbench.CheckUnknown},
			},
		},
	})

	if len(start.issues) != 2 || start.issues[0].Number != 1 || start.issues[1].Number != 2 {
		t.Fatalf("expected the prior raw issue and new linked issue to remain, got %+v", start.issues)
	}
	if !strings.Contains(start.issuesError, "network unavailable") {
		t.Fatalf("expected issue refresh error state, got %q", start.issuesError)
	}
}

func TestUpdate_IssueRefreshFailureKeepsLinkedPullRequests(t *testing.T) {
	repo := workbench.RepoRef{Owner: "0maru", Name: "gh-zen"}
	start := newModel()
	start.issueRepo = repo
	start.prsByIssueNumber = pullRequestsByIssueNumber([]workbench.PullRequestRef{{
		Number:       10,
		Title:        "Previously loaded PR",
		LinkedIssues: []workbench.IssueRef{{Number: 1}},
	}})

	start.updateIssueDataFromRuntimeResult(workbench.RuntimeLoadResult{
		Repo: repo,
		Items: []workbench.WorkItem{{
			Repo: repo,
			PullRequest: &workbench.PullRequestRef{
				Number:       11,
				Title:        "Work item PR",
				LinkedIssues: []workbench.IssueRef{{Number: 2}},
			},
		}},
	})

	if prs := start.prsByIssueNumber[1]; len(prs) != 1 || prs[0].Number != 10 {
		t.Fatalf("expected previously loaded linked PR to remain, got %+v", start.prsByIssueNumber)
	}
	if prs := start.prsByIssueNumber[2]; len(prs) != 1 || prs[0].Number != 11 {
		t.Fatalf("expected work item linked PR to be merged, got %+v", start.prsByIssueNumber)
	}
}

func TestUpdate_IssueRefreshPullRequestFailureKeepsWorkbenchEnrichment(t *testing.T) {
	repo := workbench.RepoRef{Owner: "0maru", Name: "gh-zen"}
	localItem := workbench.WorkItem{
		ID:          "branch:issue-browser",
		Repo:        repo,
		Branch:      &workbench.BranchRef{Name: "issue-browser"},
		PullRequest: &workbench.PullRequestRef{Number: 10, Title: "Existing PR", State: "open"},
		Issue:       &workbench.IssueRef{Number: 75, Title: "Existing issue", State: "open"},
		Checks:      workbench.CheckSummary{State: workbench.CheckFailing, Failing: 1},
	}
	pullRequestOnlyItem := workbench.WorkItem{
		ID:          "pull-request:0maru/gh-zen:#11",
		Repo:        repo,
		PullRequest: &workbench.PullRequestRef{Number: 11, Title: "Remote PR", State: "open"},
		Checks:      workbench.CheckSummary{State: workbench.CheckPassing},
	}
	start := newModelWithRuntimeData(cfgpkg.Defaults(), repo.FullName(), WorkbenchData{
		RepositorySummaries: []workbench.RepositorySummary{{Repo: repo, Path: "/repos/gh-zen"}},
		WorkItems:           []workbench.WorkItem{localItem, pullRequestOnlyItem},
	}, fakeDelayedPreviewLoader(0))
	request := workbenchReloadRequest{requestID: 1, repo: repo, status: "Loading issues...", issueScoped: true}
	start.screen = screenIssues
	start.issueRepo = repo
	start.workbenchLoading = true
	start.activeReloadRequest = request

	got, _ := start.Update(workbenchReloadMsg{
		request: request,
		result: workbench.RuntimeLoadResult{
			Repo: repo,
			Repositories: []workbench.RepositorySummary{{
				Repo: repo,
				Path: "/repos/gh-zen",
			}},
			Items: []workbench.WorkItem{
				{ID: localItem.ID, Repo: repo, Branch: localItem.Branch},
				{
					ID:    "pull-request-discovery-error:" + repo.FullName(),
					Repo:  repo,
					Local: &workbench.LocalStatus{Summary: "pull request discovery failed"},
				},
			},
			IssuesRepo:   repo,
			IssuesLoaded: true,
			Issues: []workbench.IssueRef{{
				Number: 75,
				Title:  "Refreshed issue",
				State:  "open",
			}},
		},
	})
	mm := got.(model)

	var reloadedLocal workbench.WorkItem
	if !hasWorkItem(mm.workItems, func(item workbench.WorkItem) bool {
		if item.ID != localItem.ID {
			return false
		}
		reloadedLocal = item
		return true
	}) {
		t.Fatalf("expected local item to remain, got %+v", mm.workItems)
	}
	if reloadedLocal.PullRequest == nil || reloadedLocal.PullRequest.Number != 10 {
		t.Fatalf("expected existing PR enrichment to remain, got %+v", reloadedLocal)
	}
	if reloadedLocal.Checks.State != workbench.CheckFailing {
		t.Fatalf("expected existing check result to remain, got %+v", reloadedLocal.Checks)
	}
	if reloadedLocal.Issue == nil || reloadedLocal.Issue.Title != "Refreshed issue" {
		t.Fatalf("expected retained PR link to use refreshed issue metadata, got %+v", reloadedLocal.Issue)
	}
	if !hasWorkItem(mm.workItems, func(item workbench.WorkItem) bool {
		return item.ID == pullRequestOnlyItem.ID && item.PullRequest != nil && item.PullRequest.Number == 11
	}) {
		t.Fatalf("expected PR-only item to remain, got %+v", mm.workItems)
	}
	if !hasWorkItem(mm.workItems, func(item workbench.WorkItem) bool {
		return strings.HasPrefix(item.ID, "pull-request-discovery-error:")
	}) {
		t.Fatalf("expected current discovery error item to remain, got %+v", mm.workItems)
	}
	summary, ok := mm.repoSummary(repo)
	if !ok || summary.OpenPullRequestCount != 2 || summary.OpenIssueCount != 1 || summary.FailingCheckCount != 1 {
		t.Fatalf("expected summary from merged work items, got %+v ok=%v", summary, ok)
	}
}

func TestUpdate_IssueRefreshCheckFailureKeepsOnlyFailedCheckState(t *testing.T) {
	repo := workbench.RepoRef{Owner: "0maru", Name: "gh-zen"}
	failedItem := workbench.WorkItem{
		ID:          "branch:first",
		Repo:        repo,
		Branch:      &workbench.BranchRef{Name: "first"},
		PullRequest: &workbench.PullRequestRef{Number: 10, HeadBranch: "first"},
		Checks:      workbench.CheckSummary{State: workbench.CheckFailing, Failing: 1},
	}
	loadedItem := workbench.WorkItem{
		ID:          "branch:second",
		Repo:        repo,
		Branch:      &workbench.BranchRef{Name: "second"},
		PullRequest: &workbench.PullRequestRef{Number: 11, HeadBranch: "second"},
		Checks:      workbench.CheckSummary{State: workbench.CheckFailing, Failing: 1},
	}
	start := newModelWithRuntimeData(cfgpkg.Defaults(), repo.FullName(), WorkbenchData{
		RepositorySummaries: []workbench.RepositorySummary{{Repo: repo}},
		WorkItems:           []workbench.WorkItem{failedItem, loadedItem},
	}, fakeDelayedPreviewLoader(0))
	request := workbenchReloadRequest{requestID: 1, repo: repo, status: "Loading issues...", issueScoped: true}
	start.screen = screenIssues
	start.issueRepo = repo
	start.workbenchLoading = true
	start.activeReloadRequest = request

	got, _ := start.Update(workbenchReloadMsg{
		request: request,
		result: workbench.RuntimeLoadResult{
			Repo:               repo,
			Repositories:       []workbench.RepositorySummary{{Repo: repo}},
			PullRequestsLoaded: true,
			IssuesRepo:         repo,
			IssuesLoaded:       true,
			FailedCheckRefs:    []string{"first"},
			Items: []workbench.WorkItem{
				{
					ID:          failedItem.ID,
					Repo:        repo,
					Branch:      failedItem.Branch,
					PullRequest: failedItem.PullRequest,
					Checks:      workbench.CheckSummary{State: workbench.CheckUnknown},
				},
				{
					ID:          loadedItem.ID,
					Repo:        repo,
					Branch:      loadedItem.Branch,
					PullRequest: loadedItem.PullRequest,
					Checks:      workbench.CheckSummary{State: workbench.CheckUnknown},
				},
			},
		},
	})
	mm := got.(model)

	for _, item := range mm.workItems {
		switch item.ID {
		case failedItem.ID:
			if item.Checks.State != workbench.CheckFailing || item.Checks.Failing != 1 {
				t.Fatalf("expected failed check load to preserve previous state, got %+v", item.Checks)
			}
		case loadedItem.ID:
			if item.Checks.State != workbench.CheckUnknown {
				t.Fatalf("expected successful unknown check load to replace previous state, got %+v", item.Checks)
			}
		}
	}
	summary, ok := mm.repoSummary(repo)
	if !ok || summary.FailingCheckCount != 1 {
		t.Fatalf("expected summary to retain one failed check, got %+v ok=%v", summary, ok)
	}
}

func TestUpdate_IssueRefreshViewerFailureKeepsReviewPerspective(t *testing.T) {
	repo := workbench.RepoRef{Owner: "0maru", Name: "gh-zen"}
	previous := workbench.WorkItem{
		ID:     "branch:feature",
		Repo:   repo,
		Branch: &workbench.BranchRef{Name: "feature"},
		PullRequest: &workbench.PullRequestRef{
			Number:                10,
			Title:                 "Old title",
			HeadBranch:            "feature",
			ViewerReviewRequested: true,
			ViewerAuthored:        true,
			WaitingOnReview:       true,
		},
	}
	refreshed := previous
	refreshed.PullRequest = &workbench.PullRequestRef{Number: 10, Title: "New title", HeadBranch: "feature"}

	got := mergeIssueScopedWorkItems([]workbench.WorkItem{previous}, repo, workbench.RuntimeLoadResult{
		Repo:               repo,
		Items:              []workbench.WorkItem{refreshed},
		PullRequestsLoaded: true,
		ViewerSubjectError: "viewer unavailable",
	})
	if len(got) != 1 || got[0].PullRequest == nil {
		t.Fatalf("expected one refreshed pull request item, got %+v", got)
	}
	pr := got[0].PullRequest
	if pr.Title != "New title" || !pr.ViewerReviewRequested || !pr.ViewerAuthored || !pr.WaitingOnReview {
		t.Fatalf("expected refreshed PR data with preserved review perspective, got %+v", pr)
	}
}

func TestSyncWorkbenchReturnAfterReloadMatchesRepositoryCaseInsensitively(t *testing.T) {
	start := newModel()
	start.repos = []workbench.RepoRef{
		{Owner: "Other", Name: "Repo"},
		{Owner: "Owner", Name: "Repo"},
	}
	start.selectedRepo = 0
	start.workbenchReturn = workbenchReturnState{
		valid:           true,
		selectedRepoRef: workbench.RepoRef{Owner: "owner", Name: "repo"},
	}

	start.syncWorkbenchReturnAfterReload()

	if start.workbenchReturn.selectedRepo != 1 {
		t.Fatalf("expected case-insensitive return repo index 1, got %+v", start.workbenchReturn)
	}
}

func TestUpdate_IssueRefreshReplacesRepositoryCaseInsensitively(t *testing.T) {
	canonicalRepo := workbench.RepoRef{Owner: "Owner", Name: "Repo"}
	requestRepo := workbench.RepoRef{Owner: "owner", Name: "repo"}
	start := newModelWithRuntimeData(cfgpkg.Defaults(), canonicalRepo.FullName(), WorkbenchData{
		RepositorySummaries: []workbench.RepositorySummary{{Repo: canonicalRepo, Path: "/old"}},
		WorkItems: []workbench.WorkItem{{
			ID:     "branch:old",
			Repo:   canonicalRepo,
			Branch: &workbench.BranchRef{Name: "old"},
		}},
	}, fakeDelayedPreviewLoader(0))
	request := workbenchReloadRequest{requestID: 1, repo: requestRepo, status: "Loading issues...", issueScoped: true}
	start.screen = screenIssues
	start.issueRepo = requestRepo
	start.workbenchLoading = true
	start.activeReloadRequest = request

	got, _ := start.Update(workbenchReloadMsg{
		request: request,
		result: workbench.RuntimeLoadResult{
			Repo: requestRepo,
			Repositories: []workbench.RepositorySummary{{
				Repo: requestRepo,
				Path: "/new",
			}},
			Items: []workbench.WorkItem{{
				ID:     "branch:new",
				Repo:   requestRepo,
				Branch: &workbench.BranchRef{Name: "new"},
			}},
			PullRequestsLoaded: true,
			IssuesRepo:         requestRepo,
			IssuesLoaded:       true,
		},
	})
	mm := got.(model)

	if len(mm.workItems) != 1 || mm.workItems[0].ID != "branch:new" || mm.workItems[0].Repo != canonicalRepo {
		t.Fatalf("expected one canonical-cased replacement item, got %+v", mm.workItems)
	}
	if summary, ok := mm.repoSummary(canonicalRepo); !ok || summary.Path != "/new" {
		t.Fatalf("expected canonical summary to be replaced, got %+v ok=%v", summary, ok)
	}
}

func TestUpdate_LeavingIssueViewRestoresFullReloadStaleCheck(t *testing.T) {
	repoA := workbench.RepoRef{Owner: "0maru", Name: "repo-a"}
	repoB := workbench.RepoRef{Owner: "0maru", Name: "repo-b"}
	repoC := workbench.RepoRef{Owner: "0maru", Name: "repo-c"}
	original := workbench.WorkItem{ID: "branch:original", Repo: repoC}
	start := newModelWithRuntimeData(cfgpkg.Defaults(), repoA.FullName(), WorkbenchData{
		Repos:     []workbench.RepoRef{repoA, repoB, repoC},
		WorkItems: []workbench.WorkItem{original},
	}, fakeDelayedPreviewLoader(0))
	request := workbenchReloadRequest{requestID: 1, repo: repoA, status: "Reloading workbench data..."}
	start.screen = screenIssues
	start.issueRepo = repoB
	start.issueReloadPending = true
	start.pendingIssueRepo = repoB
	start.workbenchLoading = true
	start.activeReloadRequest = request

	start.backToWorkbench()
	start.selectedRepo = 2
	if start.issueReloadPending || hasRepoRef(start.pendingIssueRepo) {
		t.Fatalf("expected leaving issues to clear pending reload state, got pending=%v repo=%+v", start.issueReloadPending, start.pendingIssueRepo)
	}

	got, cmd := start.Update(workbenchReloadMsg{
		request: request,
		result: workbench.RuntimeLoadResult{
			Repo:         repoA,
			Repositories: []workbench.RepositorySummary{{Repo: repoA}},
			Items:        []workbench.WorkItem{{ID: "branch:stale", Repo: repoA}},
		},
	})
	if cmd != nil {
		t.Fatalf("expected stale full reload to be discarded without a command, got %T", cmd)
	}
	mm := got.(model)
	if repo, ok := mm.selectedRepoRef(); !ok || repo != repoC {
		t.Fatalf("expected current repo C selection to remain, got %+v ok=%v", repo, ok)
	}
	if len(mm.workItems) != 1 || mm.workItems[0].ID != original.ID {
		t.Fatalf("expected stale response not to replace work items, got %+v", mm.workItems)
	}
}

func TestUpdate_IssueRefreshCheckFailureDoesNotReportIssueFailure(t *testing.T) {
	repo := workbench.RepoRef{Owner: "0maru", Name: "gh-zen"}
	start := newModel()
	start.issueRepo = repo

	start.updateIssueDataFromRuntimeResult(workbench.RuntimeLoadResult{
		Repo:         repo,
		IssuesRepo:   repo,
		IssuesLoaded: true,
		Items: []workbench.WorkItem{{
			ID:     "issue-check-discovery-error:" + repo.FullName(),
			Repo:   repo,
			Local:  &workbench.LocalStatus{Summary: "issue and check discovery failed: checks unavailable"},
			Checks: workbench.CheckSummary{State: workbench.CheckUnknown},
		}},
	})

	if start.issuesError != "" {
		t.Fatalf("expected successful issue load not to report a check failure, got %q", start.issuesError)
	}
}

func TestIssuesFromWorkItemsExcludesUncertainBranchGuess(t *testing.T) {
	repo := workbench.RepoRef{Owner: "0maru", Name: "gh-zen"}
	issues := issuesFromWorkItems([]workbench.WorkItem{
		{Repo: repo, Issue: &workbench.IssueRef{Number: 2026, Certain: false, Source: workbench.IssueLinkSourceBranch}},
		{Repo: repo, Issue: &workbench.IssueRef{Number: 75, Certain: true, Source: workbench.IssueLinkSourceBranch}},
		{
			Repo:  repo,
			Issue: &workbench.IssueRef{Number: 76, Certain: false, Source: workbench.IssueLinkSourcePullRequest},
			PullRequest: &workbench.PullRequestRef{LinkedIssues: []workbench.IssueRef{
				{Number: 76, Title: "First linked issue"},
				{Number: 77, Title: "Second linked issue"},
			}},
		},
	}, repo)

	if len(issues) != 3 || issues[0].Number != 75 || issues[1].Number != 76 || issues[2].Number != 77 {
		t.Fatalf("expected only verified or PR-linked issues, got %+v", issues)
	}
}

func TestIssueLinesKeepLongAuthorInsideFixedColumn(t *testing.T) {
	start := newModel()
	start.issues = []workbench.IssueRef{
		{Number: 75, Title: "ASCII title", State: "open", AuthorLogin: "author-name-that-is-far-too-long"},
		{Number: 76, Title: "Unicode title", State: "open", AuthorLogin: "長いユーザー名です"},
	}

	lines := start.issueLines(61, 0, true)
	if len(lines) != 3 {
		t.Fatalf("expected filter plus two issue rows, got %#v", lines)
	}
	titleColumns := make([]int, 2)
	for i, title := range []string{"ASCII title", "Unicode title"} {
		byteIndex := strings.Index(lines[i+1], title)
		if byteIndex < 0 {
			t.Fatalf("expected %q to remain visible, got %q", title, lines[i+1])
		}
		titleColumns[i] = lipgloss.Width(lines[i+1][:byteIndex])
	}
	if titleColumns[0] != titleColumns[1] {
		t.Fatalf("expected author columns to keep titles aligned, got columns %v", titleColumns)
	}
}

func TestIssuePreviewCanScrollToBodyAndURL(t *testing.T) {
	start := newModel()
	start.screen = screenIssues
	start.focusedPane = panePreview
	start.width = issueLayoutMinWidth
	start.height = 10
	start.issueRepo = workbench.RepoRef{Owner: "0maru", Name: "gh-zen"}
	start.issues = []workbench.IssueRef{{
		Number: 75,
		Title:  "Issue with many linked pull requests",
		State:  "open",
		Body:   "The body remains reachable.",
		URL:    "https://example.test/issues/75",
	}}
	prs := make([]workbench.PullRequestRef, 10)
	for i := range prs {
		prs[i] = workbench.PullRequestRef{Number: i + 1, Title: "Linked pull request"}
	}
	start.prsByIssueNumber = map[int][]workbench.PullRequestRef{75: prs}

	if got := start.renderIssueFull(start.width); strings.Contains(got, "Body:") {
		t.Fatalf("expected body to begin below the initial preview viewport, got %q", got)
	}
	start.moveFocusedSelection(1)
	if start.issuePreviewOffset != 1 {
		t.Fatalf("expected preview j/k movement to advance one line, got offset %d", start.issuePreviewOffset)
	}
	start.jumpFocusedSelection(true)
	got := start.renderIssueFull(start.width)
	for _, want := range []string{"Body:", "URL:"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected scrolled preview to contain %q, got %q", want, got)
		}
	}
}

func TestUpdate_IssueRefreshReplacesOnlyTargetRepositorySummary(t *testing.T) {
	repoA := workbench.RepoRef{Owner: "0maru", Name: "gh-zen"}
	repoB := workbench.RepoRef{Owner: "0maru", Name: "dotfiles"}
	refreshedItems := []workbench.WorkItem{
		{ID: "issue:1", Repo: repoA, Issue: &workbench.IssueRef{Number: 1, State: "open"}},
		{ID: "issue:2", Repo: repoA, Issue: &workbench.IssueRef{Number: 2, State: "open"}},
		{ID: "issue:3", Repo: repoA, Issue: &workbench.IssueRef{Number: 3, State: "open"}},
	}
	issueReloader := &fakeIssueReloader{results: map[string]workbench.RuntimeLoadResult{
		repoA.FullName(): {
			Repo:               repoA,
			Repositories:       []workbench.RepositorySummary{{Repo: repoA, Path: "/new/a"}},
			Items:              refreshedItems,
			PullRequestsLoaded: true,
			IssuesRepo:         repoA,
			IssuesLoaded:       true,
		},
	}}
	start := newModelWithRuntimeData(cfgpkg.Defaults(), repoA.FullName(), WorkbenchData{
		RepositorySummaries: []workbench.RepositorySummary{
			{Repo: repoA, Path: "/old/a", OpenIssueCount: 1},
			{Repo: repoB, Path: "/old/b", OpenIssueCount: 2},
		},
		IssueReloader: issueReloader,
	}, fakeDelayedPreviewLoader(0))
	start.screen = screenIssues

	got, cmd := start.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	got, _ = got.(model).Update(requireWorkbenchReloadMsg(t, cmd))
	mm := got.(model)

	if len(mm.repoSummaries) != 2 {
		t.Fatalf("expected both repository summaries to remain, got %+v", mm.repoSummaries)
	}
	if summary, ok := mm.repoSummary(repoA); !ok || summary.Path != "/new/a" || summary.OpenIssueCount != 3 {
		t.Fatalf("expected target summary to be refreshed, got %+v ok=%v", summary, ok)
	}
	if summary, ok := mm.repoSummary(repoB); !ok || summary.Path != "/old/b" || summary.OpenIssueCount != 2 {
		t.Fatalf("expected unrelated summary to remain unchanged, got %+v ok=%v", summary, ok)
	}
}

func TestIssueLinesKeepSelectedIssueInsideViewport(t *testing.T) {
	start := newModel()
	start.issues = make([]workbench.IssueRef, 20)
	for i := range start.issues {
		start.issues[i] = workbench.IssueRef{Number: i + 1, Title: "Issue", State: "open"}
	}
	start.selectedIssue = 15

	lines := start.issueLines(80, 6, true)
	if len(lines) > 6 {
		t.Fatalf("expected at most six issue lines, got %d: %#v", len(lines), lines)
	}
	if got := strings.Join(lines, "\n"); !strings.Contains(got, "> #16") {
		t.Fatalf("expected selected issue to remain in the viewport, got %q", got)
	}
}

func TestRenderIssueFullFitsTerminalHeight(t *testing.T) {
	start := newModel()
	start.screen = screenIssues
	start.width = fullLayoutMinWidth
	start.height = 12
	start.issues = make([]workbench.IssueRef, 20)
	for i := range start.issues {
		start.issues[i] = workbench.IssueRef{Number: i + 1, Title: "Issue", State: "open"}
	}
	start.selectedIssue = 19

	rendered := strings.TrimSuffix(start.renderIssueFull(start.width), "\n")
	if lines := strings.Count(rendered, "\n") + 1; lines > start.height {
		t.Fatalf("expected issue view to fit height %d, got %d lines", start.height, lines)
	}
	if !strings.Contains(rendered, "#20") {
		t.Fatalf("expected selected issue to remain visible, got %q", rendered)
	}
}
