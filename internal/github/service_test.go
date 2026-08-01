package github

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/0maru/gh-zen/internal/workbench"
)

type fakeRunner struct {
	output []byte
	err    error
	args   []string
}

type fakeRunnerByCommand struct {
	outputs map[string][]byte
	calls   [][]string
}

type fakeExitError struct {
	code int
}

func (e fakeExitError) Error() string {
	return "exit status"
}

func (e fakeExitError) ExitCode() int {
	return e.code
}

func (r *fakeRunner) Run(_ context.Context, args ...string) ([]byte, error) {
	r.args = append([]string(nil), args...)
	return r.output, r.err
}

func (r *fakeRunnerByCommand) Run(_ context.Context, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string(nil), args...))
	key := commandKey(args...)
	output, ok := r.outputs[key]
	if !ok {
		return nil, errors.New("unexpected gh command: " + key)
	}
	return output, nil
}

func TestFakeService_ReturnsRepositorySummary(t *testing.T) {
	service := FakeService{
		PullRequestsByRepo: map[string][]workbench.PullRequestRef{
			"0maru/gh-zen": {{Number: 1, Title: "PR", State: "open"}},
		},
		IssuesByRepo: map[string][]workbench.IssueRef{
			"0maru/gh-zen": {{Number: 2, Title: "Issue", State: "open", Certain: true}},
		},
		Checks: map[string]workbench.CheckSummary{
			"0maru/gh-zen": {State: workbench.CheckPassing, Passing: 2},
		},
	}

	got, err := service.RepositorySummary(context.Background(), "0maru/gh-zen")
	if err != nil {
		t.Fatalf("expected fake summary, got %v", err)
	}
	if len(got.PullRequests) != 1 || len(got.Issues) != 1 || got.Checks.State != workbench.CheckPassing {
		t.Fatalf("unexpected fake summary: %+v", got)
	}
}

func TestCLIService_PullRequestsParsesGHOutput(t *testing.T) {
	repo := "0maru/gh-zen"
	runner := &fakeRunnerByCommand{outputs: map[string][]byte{
		commandKey("pr", "list", "--repo", repo, "--state", "all", "--limit", listLimit, "--json", prListFields):                             []byte(`[{"number":12,"title":"Add feature","state":"OPEN","url":"https://example.test/pr/12","author":{"login":"0maru"},"headRefName":"feature","headRepositoryOwner":{"login":"0maru"},"baseRefName":"main","isDraft":false,"updatedAt":"2026-05-03T12:00:00Z","reviewDecision":"REVIEW_REQUIRED","reviewRequests":[{"__typename":"User","login":"alice","name":"Alice"},{"__typename":"Team","slug":"core","name":"Core"}],"latestReviews":[{"author":{"login":"bob"},"state":"APPROVED"}],"body":"No closing keyword"}]`),
		commandKey("api", "graphql", "-f", "owner=0maru", "-f", "name=gh-zen", "-f", "after=", "-f", "query="+pullRequestClosingIssuesQuery): []byte(`{"data":{"repository":{"pullRequests":{"nodes":[{"number":12,"closingIssuesReferences":{"nodes":[{"number":9,"title":"Issue","state":"OPEN","url":"https://example.test/issues/9"}]}}],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}}`),
	}}
	service := CLIService{Runner: runner}

	got, err := service.PullRequests(context.Background(), repo)
	if err != nil {
		t.Fatalf("expected pull requests to parse, got %v", err)
	}
	want := []workbench.PullRequestRef{{
		Number:      12,
		Title:       "Add feature",
		State:       "open",
		URL:         "https://example.test/pr/12",
		AuthorLogin: "0maru",
		HeadOwner:   "0maru",
		HeadBranch:  "feature",
		BaseBranch:  "main",
		UpdatedAt:   "2026-05-03T12:00:00Z",
		LinkedIssues: []workbench.IssueRef{
			{Number: 9, Repository: repo, Title: "Issue", State: "open", URL: "https://example.test/issues/9", Certain: true},
		},
		ReviewState: "review required",
		ReviewRequests: []workbench.ReviewRequestRef{
			{Kind: "User", Login: "alice", Name: "Alice"},
			{Kind: "Team", Name: "Core", Slug: "core"},
		},
		LatestReviews: []workbench.PullRequestReviewRef{
			{AuthorLogin: "bob", State: "approved"},
		},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %+v, got %+v", want, got)
	}
	if !reflect.DeepEqual(runner.calls[0][:4], []string{"pr", "list", "--repo", repo}) {
		t.Fatalf("expected gh pr list args, got %#v", runner.calls)
	}
	if !hasArgPair(runner.calls[0], "--limit", listLimit) {
		t.Fatalf("expected gh pr list limit, got %#v", runner.calls)
	}
	if !hasArgValue(runner.calls[0], prListFields) {
		t.Fatalf("expected gh pr list to request head repository owner, got %#v", runner.calls)
	}
}

func TestCLIService_PullRequestsPreservesClosingIssueRepository(t *testing.T) {
	repo := "0maru/gh-zen"
	runner := &fakeRunnerByCommand{outputs: map[string][]byte{
		commandKey("pr", "list", "--repo", repo, "--state", "all", "--limit", listLimit, "--json", prListFields):                             []byte(`[{"number":12,"title":"Add feature","state":"OPEN","url":"https://example.test/pr/12","author":{"login":"0maru"},"headRefName":"feature","headRepositoryOwner":{"login":"0maru"},"baseRefName":"main","isDraft":false,"updatedAt":"2026-05-03T12:00:00Z","reviewRequests":[],"latestReviews":[],"body":"No fallback"}]`),
		commandKey("api", "graphql", "-f", "owner=0maru", "-f", "name=gh-zen", "-f", "after=", "-f", "query="+pullRequestClosingIssuesQuery): []byte(`{"data":{"repository":{"pullRequests":{"nodes":[{"number":12,"closingIssuesReferences":{"nodes":[{"number":9,"title":"Local","state":"OPEN","url":"https://example.test/0maru/gh-zen/issues/9","repository":{"nameWithOwner":"0maru/gh-zen"}},{"number":9,"title":"External","state":"OPEN","url":"https://example.test/other/repo/issues/9","repository":{"nameWithOwner":"other/repo"}}]}}],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}}`),
	}}

	got, err := (CLIService{Runner: runner}).PullRequests(context.Background(), repo)
	if err != nil {
		t.Fatalf("expected pull requests to parse, got %v", err)
	}
	if len(got) != 1 || len(got[0].LinkedIssues) != 2 {
		t.Fatalf("expected both repository-qualified closing issues, got %+v", got)
	}
	if got[0].LinkedIssues[0].Repository != repo || got[0].LinkedIssues[1].Repository != "other/repo" {
		t.Fatalf("expected closing issue repositories to be preserved, got %+v", got[0].LinkedIssues)
	}
	if !strings.Contains(pullRequestClosingIssuesQuery, "nameWithOwner") {
		t.Fatalf("expected closing issue query to request repository identity, got %q", pullRequestClosingIssuesQuery)
	}
}

func TestLinkedIssuesFromBodyRequiresClosingKeywordForEachIssue(t *testing.T) {
	body := "Fixes #1 and see #2. Resolves: #3, closes #1. Mentions #4."

	got := linkedIssuesFromBody("0maru/gh-zen", body)
	want := []workbench.IssueRef{
		{Number: 1, Repository: "0maru/gh-zen", Certain: true},
		{Number: 3, Repository: "0maru/gh-zen", Certain: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected only explicit closing references %+v, got %+v", want, got)
	}
}

func TestLinkedIssuesFromBodyAcceptsQualifiedReferencesForSameRepository(t *testing.T) {
	body := strings.Join([]string{
		"Fixes 0maru/gh-zen#1.",
		"Resolves https://github.com/0MARU/GH-ZEN/issues/2.",
		"Closes other/repo#3.",
		"Fixes https://github.com/other/repo/issues/4.",
	}, " ")

	got := linkedIssuesFromBody("0maru/gh-zen", body)
	want := []workbench.IssueRef{
		{Number: 1, Repository: "0maru/gh-zen", Certain: true},
		{Number: 2, Repository: "0maru/gh-zen", Certain: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected only same-repository closing references %+v, got %+v", want, got)
	}
}

func TestCLIService_IssuesParsesGHOutput(t *testing.T) {
	repo := "0maru/gh-zen"
	runner := &fakeRunnerByCommand{outputs: map[string][]byte{
		commandKey("issue", "list", "--repo", repo, "--state", "all", "--limit", listLimit, "--json", issueListFields):                 []byte(`[{"number":9,"title":"Config","state":"OPEN","url":"https://example.test/issues/9","body":"Issue details","labels":[{"name":"enhancement"}],"assignees":[{"login":"0maru"}],"milestone":{"title":"v1"},"author":{"login":"alice"},"updatedAt":"2026-05-03T12:00:00Z"}]`),
		commandKey("api", "graphql", "-f", "owner=0maru", "-f", "name=gh-zen", "-f", "after=", "-f", "query="+issueCommentCountsQuery): []byte(`{"data":{"repository":{"issues":{"nodes":[{"number":9,"comments":{"totalCount":3}}],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}}`),
	}}
	service := CLIService{Runner: runner}

	got, err := service.Issues(context.Background(), repo)
	if err != nil {
		t.Fatalf("expected issues to parse, got %v", err)
	}
	want := []workbench.IssueRef{{
		Number:        9,
		Repository:    repo,
		Title:         "Config",
		State:         "open",
		URL:           "https://example.test/issues/9",
		Body:          "Issue details",
		Labels:        []string{"enhancement"},
		Assignees:     []string{"0maru"},
		Milestone:     "v1",
		AuthorLogin:   "alice",
		CommentsCount: 3,
		UpdatedAt:     "2026-05-03T12:00:00Z",
		Certain:       true,
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %+v, got %+v", want, got)
	}
	if !hasArgPair(runner.calls[0], "--limit", listLimit) {
		t.Fatalf("expected gh issue list limit, got %#v", runner.calls)
	}
	if !hasArgValue(runner.calls[0], issueListFields) {
		t.Fatalf("expected gh issue list to request issue detail fields, got %#v", runner.calls)
	}
	if strings.Contains(issueListFields, "comments") {
		t.Fatalf("expected gh issue list not to request comment bodies, got %q", issueListFields)
	}
	if !strings.Contains(issueCommentCountsQuery, "field:CREATED_AT") || strings.Contains(issueCommentCountsQuery, "field:UPDATED_AT") {
		t.Fatalf("expected issue comment count query to match gh issue list ordering, got %q", issueCommentCountsQuery)
	}
}

func TestCLIService_IssuesWithOptionsDefersCommentCounts(t *testing.T) {
	repo := "0maru/gh-zen"
	runner := &fakeRunnerByCommand{outputs: map[string][]byte{
		commandKey("issue", "list", "--repo", repo, "--state", "all", "--limit", listLimit, "--json", issueListFields): []byte(`[{"number":9,"title":"Config","state":"OPEN","url":"https://example.test/issues/9","body":"Issue details","labels":[],"assignees":[],"milestone":null,"author":{"login":"alice"},"updatedAt":"2026-05-03T12:00:00Z"}]`),
	}}
	service := CLIService{Runner: runner}

	got, err := service.IssuesWithOptions(context.Background(), repo, workbench.IssueListOptions{})
	if err != nil {
		t.Fatalf("expected lightweight issues to parse, got %v", err)
	}
	if len(got) != 1 || got[0].Number != 9 || got[0].CommentsCount != 0 {
		t.Fatalf("expected issue without comment count, got %+v", got)
	}
	if len(runner.calls) != 1 || runner.calls[0][0] != "issue" {
		t.Fatalf("expected only gh issue list without GraphQL comment pagination, got %#v", runner.calls)
	}
}

func TestCLIService_IssuesWithOptionsReportsCommentCountFailure(t *testing.T) {
	repo := "0maru/gh-zen"
	runner := &fakeRunnerByCommand{outputs: map[string][]byte{
		commandKey("issue", "list", "--repo", repo, "--state", "all", "--limit", listLimit, "--json", issueListFields): []byte(`[{"number":9,"title":"Config","state":"OPEN","url":"https://example.test/issues/9","body":"Issue details","labels":[],"assignees":[],"milestone":null,"author":{"login":"alice"},"updatedAt":"2026-05-03T12:00:00Z"}]`),
	}}
	service := CLIService{Runner: runner}

	_, err := service.IssuesWithOptions(context.Background(), repo, workbench.IssueListOptions{IncludeCommentsCount: true})
	if err == nil || !strings.Contains(err.Error(), "load issue comment counts") {
		t.Fatalf("expected comment count failure to be reported, got %v", err)
	}
}

func TestCLIService_CheckSummaryParsesGHOutput(t *testing.T) {
	runner := &fakeRunner{output: []byte(`[{"name":"test","state":"SUCCESS"},{"name":"lint","state":"FAILURE"},{"name":"build","state":"PENDING"}]`)}
	service := CLIService{Runner: runner}

	got, err := service.CheckSummary(context.Background(), "0maru/gh-zen", "feature")
	if err != nil {
		t.Fatalf("expected checks to parse, got %v", err)
	}
	if got.State != workbench.CheckFailing || got.Passing != 1 || got.Failing != 1 || got.Pending != 1 {
		t.Fatalf("unexpected check summary: %+v", got)
	}
}

func TestCLIService_ViewerReviewSubjectsParsesGHOutput(t *testing.T) {
	runner := &fakeRunnerByCommand{outputs: map[string][]byte{
		commandKey("api", "user", "--jq", ".login"):         []byte("0maru\n"),
		commandKey("api", "user/teams", "--jq", ".[].slug"): []byte("frontend\nplatform\n"),
	}}
	service := CLIService{Runner: runner}

	got, err := service.ViewerReviewSubjects(context.Background())
	if err != nil {
		t.Fatalf("expected viewer subjects to parse, got %v", err)
	}
	want := workbench.ReviewSubjects{Login: "0maru", TeamSlugs: []string{"frontend", "platform"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %+v, got %+v", want, got)
	}
}

func TestCLIService_WorkflowRunsParsesGHOutputAndActorFallback(t *testing.T) {
	repo := "0maru/gh-zen"
	runner := &fakeRunnerByCommand{outputs: map[string][]byte{
		commandKey("run", "list", "--repo", repo, "--limit", "2", "--json", runListFields):      []byte(`[{"databaseId":101,"number":77,"workflowName":"CI","headBranch":"main","event":"push","status":"completed","conclusion":"success","headSha":"abcdef1234567890","displayTitle":"Build main","url":"https://example.test/runs/101","createdAt":"2026-06-20T12:00:00Z","updatedAt":"2026-06-20T12:04:00Z"},{"databaseId":102,"number":78,"name":"Fallback name","headBranch":"feature","event":"pull_request","status":"completed","conclusion":"failure","headSha":"1234567890abcdef","displayTitle":"Build feature","url":"https://example.test/runs/102","createdAt":"2026-06-20T13:00:00Z","updatedAt":"2026-06-20T13:08:00Z"}]`),
		commandKey("api", "repos/"+repo+"/actions/runs", "--method", "GET", "-f", "per_page=2"): []byte(`{"workflow_runs":[{"id":101,"triggering_actor":{"login":"0maru"}},{"id":102,"actor":{"login":"teammate"}}]}`),
	}}
	service := CLIService{Runner: runner}

	got, err := service.WorkflowRuns(context.Background(), repo, WorkflowRunListOptions{Limit: 2})
	if err != nil {
		t.Fatalf("expected workflow runs to parse, got %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected two runs, got %+v", got)
	}
	if got[0].ID != 101 || got[0].RunNumber != 77 || got[0].WorkflowName != "CI" || got[0].Actor != "0maru" || got[0].ShortSHA() != "abcdef1" {
		t.Fatalf("unexpected first run: %+v", got[0])
	}
	if got[1].WorkflowName != "Fallback name" || got[1].Conclusion != "failure" || got[1].Actor != "teammate" {
		t.Fatalf("unexpected second run: %+v", got[1])
	}
}

func TestCLIService_WorkflowRunParsesSingleRunWithActorFallback(t *testing.T) {
	repo := "0maru/gh-zen"
	runner := &fakeRunnerByCommand{outputs: map[string][]byte{
		commandKey("run", "view", "101", "--repo", repo, "--json", runViewFields): []byte(`{"databaseId":101,"number":77,"workflowName":"CI","headBranch":"main","event":"push","status":"completed","conclusion":"success","headSha":"abcdef1234567890","displayTitle":"Build main","url":"https://example.test/runs/101","createdAt":"2026-06-20T12:00:00Z","updatedAt":"2026-06-20T12:04:00Z"}`),
		commandKey("api", "repos/"+repo+"/actions/runs/101"):                      []byte(`{"id":101,"triggering_actor":{"login":"0maru"}}`),
	}}
	service := CLIService{Runner: runner}

	got, err := service.WorkflowRun(context.Background(), repo, 101)
	if err != nil {
		t.Fatalf("expected workflow run to parse, got %v", err)
	}
	if got.ID != 101 || got.Actor != "0maru" || got.StatusLabel() != "success" {
		t.Fatalf("unexpected workflow run: %+v", got)
	}
}

func TestCLIService_WorkflowRunJobsParsesGHOutput(t *testing.T) {
	repo := "0maru/gh-zen"
	runner := &fakeRunnerByCommand{outputs: map[string][]byte{
		commandKey("run", "view", "101", "--repo", repo, "--json", runJobsFields): []byte(`{"jobs":[{"databaseId":201,"name":"test","status":"completed","conclusion":"failure","startedAt":"2026-06-20T12:00:00Z","completedAt":"2026-06-20T12:04:00Z","url":"https://example.test/jobs/201","steps":[{"number":1,"name":"Checkout","status":"completed","conclusion":"success"},{"number":2,"name":"Go test","status":"completed","conclusion":"failure"}]}]}`),
	}}
	service := CLIService{Runner: runner}

	got, err := service.WorkflowRunJobs(context.Background(), repo, 101)
	if err != nil {
		t.Fatalf("expected jobs to parse, got %v", err)
	}
	if len(got) != 1 || got[0].ID != 201 || got[0].Conclusion != "failure" || len(got[0].Steps) != 2 || got[0].Steps[1].Conclusion != "failure" {
		t.Fatalf("unexpected jobs: %+v", got)
	}
}

func TestCLIService_JobAnnotationsParsesGHOutput(t *testing.T) {
	repo := "0maru/gh-zen"
	runner := &fakeRunnerByCommand{outputs: map[string][]byte{
		commandKey("api", "repos/"+repo+"/check-runs/201/annotations", "--paginate", "--slurp"): []byte(`[[{"path":"internal/app/model.go","start_line":42,"end_line":42,"annotation_level":"failure","title":"Test failure","message":"expected preview"}],[{"path":"internal/app/view.go","start_line":7,"end_line":8,"annotation_level":"warning","title":"Lint","message":"second page"}]]`),
	}}
	service := CLIService{Runner: runner}

	got, err := service.JobAnnotations(context.Background(), repo, 201)
	if err != nil {
		t.Fatalf("expected annotations to parse, got %v", err)
	}
	want := []workbench.AnnotationRef{
		{Path: "internal/app/model.go", StartLine: 42, EndLine: 42, Level: "failure", Title: "Test failure", Message: "expected preview"},
		{Path: "internal/app/view.go", StartLine: 7, EndLine: 8, Level: "warning", Title: "Lint", Message: "second page"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %+v, got %+v", want, got)
	}
}

func TestCLIService_WorkflowRunLogsFetchesRunAndJobScopedLogs(t *testing.T) {
	repo := "0maru/gh-zen"
	jobID := int64(201)
	runner := &fakeRunnerByCommand{outputs: map[string][]byte{
		commandKey("run", "view", "101", "--repo", repo, "--log-failed"):                 []byte("line 1\nline 2\n"),
		commandKey("run", "view", "101", "--repo", repo, "--job", "201", "--log"):        []byte("job line\n"),
		commandKey("run", "view", "101", "--repo", repo, "--job", "201", "--log-failed"): []byte("failed job line\n"),
	}}
	service := CLIService{Runner: runner}

	runLog, err := service.WorkflowRunLogs(context.Background(), repo, 101, LogFetchOptions{FailedOnly: true})
	if err != nil {
		t.Fatalf("expected run log, got %v", err)
	}
	if !reflect.DeepEqual(runLog.Lines, []string{"line 1", "line 2"}) || !runLog.Failed || runLog.JobID != 0 {
		t.Fatalf("unexpected run log: %+v", runLog)
	}

	jobLog, err := service.WorkflowRunLogs(context.Background(), repo, 101, LogFetchOptions{JobID: &jobID})
	if err != nil {
		t.Fatalf("expected job log, got %v", err)
	}
	if !reflect.DeepEqual(jobLog.Lines, []string{"job line"}) || jobLog.JobID != jobID || jobLog.Failed {
		t.Fatalf("unexpected job log: %+v", jobLog)
	}

	failedJobLog, err := service.WorkflowRunLogs(context.Background(), repo, 101, LogFetchOptions{JobID: &jobID, FailedOnly: true})
	if err != nil {
		t.Fatalf("expected failed job log, got %v", err)
	}
	if !reflect.DeepEqual(failedJobLog.Lines, []string{"failed job line"}) || failedJobLog.JobID != jobID || !failedJobLog.Failed {
		t.Fatalf("unexpected failed job log: %+v", failedJobLog)
	}
}

func TestCLIService_WorkflowRunLogsTailsLargeOutput(t *testing.T) {
	repo := "0maru/gh-zen"
	runner := &fakeRunnerByCommand{outputs: map[string][]byte{
		commandKey("run", "view", "101", "--repo", repo, "--log"): []byte("one\ntwo\nthree\n"),
	}}
	service := CLIService{Runner: runner}

	got, err := service.WorkflowRunLogs(context.Background(), repo, 101, LogFetchOptions{TailLines: 2})
	if err != nil {
		t.Fatalf("expected log tail, got %v", err)
	}
	if !reflect.DeepEqual(got.Lines, []string{"two", "three"}) {
		t.Fatalf("expected last two lines, got %+v", got.Lines)
	}
}

func TestCLIService_ProvidesDataForWorkbenchEnrichment(t *testing.T) {
	repo := workbench.RepoRef{Owner: "0maru", Name: "gh-zen"}
	runner := &fakeRunnerByCommand{outputs: map[string][]byte{
		commandKey("pr", "list", "--repo", repo.FullName(), "--state", "all", "--limit", listLimit, "--json", prListFields):                  []byte(`[{"number":24,"title":"Runtime pipeline","state":"OPEN","url":"https://example.test/pull/24","author":{"login":"0maru"},"headRefName":"feature/issue-123-runtime","headRepositoryOwner":{"login":"0maru"},"baseRefName":"main","isDraft":false,"updatedAt":"2026-05-03T12:00:00Z","reviewDecision":"APPROVED","reviewRequests":[],"latestReviews":[],"body":"Closes #123"}]`),
		commandKey("api", "graphql", "-f", "owner=0maru", "-f", "name=gh-zen", "-f", "after=", "-f", "query="+pullRequestClosingIssuesQuery): []byte(`{"data":{"repository":{"pullRequests":{"nodes":[{"number":24,"closingIssuesReferences":{"nodes":[{"number":123,"title":"Runtime pipeline","state":"OPEN","url":"https://example.test/issues/123"}]}}],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}}`),
		commandKey("issue", "list", "--repo", repo.FullName(), "--state", "all", "--limit", listLimit, "--json", issueListFields):            []byte(`[{"number":123,"title":"Runtime pipeline","state":"OPEN","url":"https://example.test/issues/123","body":"Runtime issue","labels":[],"assignees":[],"milestone":null,"updatedAt":"2026-05-03T12:00:00Z"}]`),
		commandKey("api", "graphql", "-f", "owner=0maru", "-f", "name=gh-zen", "-f", "after=", "-f", "query="+issueCommentCountsQuery):       []byte(`{"data":{"repository":{"issues":{"nodes":[{"number":123,"comments":{"totalCount":2}}],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}}`),
		commandKey("pr", "checks", "feature/issue-123-runtime", "--repo", repo.FullName(), "--json", "name,state"):                           []byte(`[{"name":"test","state":"SUCCESS"},{"name":"lint","state":"SUCCESS"}]`),
	}}
	service := CLIService{Runner: runner}

	prs, err := service.PullRequests(context.Background(), repo.FullName())
	if err != nil {
		t.Fatalf("expected pull requests to parse, got %v", err)
	}
	issues, err := service.Issues(context.Background(), repo.FullName())
	if err != nil {
		t.Fatalf("expected issues to parse, got %v", err)
	}
	checks, err := service.CheckSummary(context.Background(), repo.FullName(), "feature/issue-123-runtime")
	if err != nil {
		t.Fatalf("expected checks to parse, got %v", err)
	}

	items := workbench.LinkPullRequests([]workbench.WorkItem{{
		ID:     "feature",
		Repo:   repo,
		Branch: &workbench.BranchRef{Name: "feature/issue-123-runtime"},
	}}, prs)
	items = workbench.LinkIssues(items, issues)
	items[0].Checks = checks

	if items[0].PullRequest == nil || items[0].PullRequest.Number != 24 || items[0].PullRequest.ReviewState != "approved" {
		t.Fatalf("expected CLI PR data to enrich work item, got %+v", items[0])
	}
	if items[0].Issue == nil || items[0].Issue.Number != 123 || items[0].Issue.Title != "Runtime pipeline" {
		t.Fatalf("expected CLI issue data to enrich work item, got %+v", items[0])
	}
	if items[0].Checks.State != workbench.CheckPassing || items[0].Checks.Passing != 2 {
		t.Fatalf("expected CLI check data to enrich work item, got %+v", items[0])
	}
	if len(runner.calls) != 5 {
		t.Fatalf("expected five gh calls, got %#v", runner.calls)
	}
}

func TestClassifyError(t *testing.T) {
	err := classifyError("gh pr list", []byte("run gh auth login"), errors.New("exit status 1"))
	if err.Kind != ErrorAuth {
		t.Fatalf("expected auth error, got %+v", err)
	}
	if !strings.Contains(err.Error(), "auth") {
		t.Fatalf("expected classified error text, got %q", err.Error())
	}

	err = classifyError("gh pr list", []byte("could not resolve host"), errors.New("exit status 1"))
	if err.Kind != ErrorNetwork {
		t.Fatalf("expected network error, got %+v", err)
	}
}

func TestIsPendingChecksExit(t *testing.T) {
	if !isPendingChecksExit([]string{"pr", "checks", "feature"}, fakeExitError{code: 8}) {
		t.Fatal("expected gh pr checks exit 8 to be pending")
	}
	if isPendingChecksExit([]string{"pr", "checks", "feature"}, fakeExitError{code: 1}) {
		t.Fatal("expected non-pending check exit to remain an error")
	}
	if isPendingChecksExit([]string{"pr", "list"}, fakeExitError{code: 8}) {
		t.Fatal("expected exit 8 from a different gh command to remain an error")
	}
}

func commandKey(args ...string) string {
	return strings.Join(args, "\x00")
}

func hasArgPair(args []string, key string, value string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == key && args[i+1] == value {
			return true
		}
	}
	return false
}

func hasArgValue(args []string, value string) bool {
	for _, arg := range args {
		if arg == value {
			return true
		}
	}
	return false
}
