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

func (r *fakeRunner) Run(_ context.Context, args ...string) ([]byte, error) {
	r.args = append([]string(nil), args...)
	return r.output, r.err
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
	runner := &fakeRunner{output: []byte(`[{"number":12,"title":"Add feature","state":"OPEN","url":"https://example.test/pr/12","reviewDecision":"REVIEW_REQUIRED"}]`)}
	service := CLIService{Runner: runner}

	got, err := service.PullRequests(context.Background(), "0maru/gh-zen")
	if err != nil {
		t.Fatalf("expected pull requests to parse, got %v", err)
	}
	want := []workbench.PullRequestRef{{
		Number:      12,
		Title:       "Add feature",
		State:       "open",
		URL:         "https://example.test/pr/12",
		ReviewState: "review required",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %+v, got %+v", want, got)
	}
	if !reflect.DeepEqual(runner.args[:4], []string{"pr", "list", "--repo", "0maru/gh-zen"}) {
		t.Fatalf("expected gh pr list args, got %#v", runner.args)
	}
}

func TestCLIService_IssuesParsesGHOutput(t *testing.T) {
	runner := &fakeRunner{output: []byte(`[{"number":9,"title":"Config","state":"OPEN","url":"https://example.test/issues/9"}]`)}
	service := CLIService{Runner: runner}

	got, err := service.Issues(context.Background(), "0maru/gh-zen")
	if err != nil {
		t.Fatalf("expected issues to parse, got %v", err)
	}
	want := []workbench.IssueRef{{
		Number:  9,
		Title:   "Config",
		State:   "open",
		URL:     "https://example.test/issues/9",
		Certain: true,
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %+v, got %+v", want, got)
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
