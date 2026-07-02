package github

import (
	"context"

	"github.com/0maru/gh-zen/internal/workbench"
)

// FakeService is an in-memory GitHub service for small tests.
type FakeService struct {
	Summaries          map[string]RepositorySummary
	PullRequestsByRepo map[string][]workbench.PullRequestRef
	IssuesByRepo       map[string][]workbench.IssueRef
	Checks             map[string]workbench.CheckSummary
	ReviewSubjects     workbench.ReviewSubjects
	WorkflowRunsByRepo map[string][]workbench.WorkflowRunRef
	WorkflowRunsByID   map[int64]workbench.WorkflowRunRef
	JobsByRunID        map[int64][]workbench.WorkflowJobRef
	AnnotationsByJobID map[int64][]workbench.AnnotationRef
	LogsByRunID        map[int64]workbench.WorkflowLog
	LogsByJobID        map[int64]workbench.WorkflowLog
	Err                error
}

func (f FakeService) RepositorySummary(_ context.Context, repo string) (RepositorySummary, error) {
	if f.Err != nil {
		return RepositorySummary{}, f.Err
	}
	if summary, ok := f.Summaries[repo]; ok {
		return summary, nil
	}
	prs, err := f.PullRequestsForRepo(repo)
	if err != nil {
		return RepositorySummary{}, err
	}
	issues, err := f.IssuesForRepo(repo)
	if err != nil {
		return RepositorySummary{}, err
	}
	return RepositorySummary{
		Repo:         repo,
		PullRequests: prs,
		Issues:       issues,
		Checks:       fakeCheckSummary(f.Checks, repo),
	}, nil
}

func (f FakeService) PullRequests(_ context.Context, repo string) ([]workbench.PullRequestRef, error) {
	return f.PullRequestsForRepo(repo)
}

func (f FakeService) Issues(_ context.Context, repo string) ([]workbench.IssueRef, error) {
	return f.IssuesForRepo(repo)
}

func (f FakeService) CheckSummary(_ context.Context, repo string, ref string) (workbench.CheckSummary, error) {
	if f.Err != nil {
		return workbench.CheckSummary{}, f.Err
	}
	if summary, ok := f.Checks[repo+"@"+ref]; ok {
		return summary, nil
	}
	if summary, ok := f.Checks[repo]; ok {
		return summary, nil
	}
	return workbench.CheckSummary{State: workbench.CheckUnknown}, nil
}

func (f FakeService) ViewerReviewSubjects(context.Context) (workbench.ReviewSubjects, error) {
	if f.Err != nil {
		return workbench.ReviewSubjects{}, f.Err
	}
	return f.ReviewSubjects, nil
}

func (f FakeService) WorkflowRuns(_ context.Context, repo string, opts WorkflowRunListOptions) ([]workbench.WorkflowRunRef, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	runs := append([]workbench.WorkflowRunRef(nil), f.WorkflowRunsByRepo[repo]...)
	if opts.Limit > 0 && len(runs) > opts.Limit {
		runs = runs[:opts.Limit]
	}
	return runs, nil
}

func (f FakeService) WorkflowRun(_ context.Context, repo string, runID int64) (workbench.WorkflowRunRef, error) {
	if f.Err != nil {
		return workbench.WorkflowRunRef{}, f.Err
	}
	if run, ok := f.WorkflowRunsByID[runID]; ok {
		return run, nil
	}
	for _, run := range f.WorkflowRunsByRepo[repo] {
		if run.ID == runID {
			return run, nil
		}
	}
	return workbench.WorkflowRunRef{}, nil
}

func (f FakeService) WorkflowRunJobs(_ context.Context, _ string, runID int64) ([]workbench.WorkflowJobRef, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	return cloneWorkflowJobs(f.JobsByRunID[runID]), nil
}

func (f FakeService) JobAnnotations(_ context.Context, _ string, jobID int64) ([]workbench.AnnotationRef, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	return append([]workbench.AnnotationRef(nil), f.AnnotationsByJobID[jobID]...), nil
}

func (f FakeService) WorkflowRunLogs(_ context.Context, _ string, runID int64, opts LogFetchOptions) (workbench.WorkflowLog, error) {
	if f.Err != nil {
		return workbench.WorkflowLog{}, f.Err
	}
	if opts.JobID != nil {
		if log, ok := f.LogsByJobID[*opts.JobID]; ok {
			return cloneWorkflowLog(log), nil
		}
	}
	if log, ok := f.LogsByRunID[runID]; ok {
		return cloneWorkflowLog(log), nil
	}
	return workbench.WorkflowLog{RunID: runID, Failed: opts.FailedOnly}, nil
}

func (f FakeService) PullRequestsForRepo(repo string) ([]workbench.PullRequestRef, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	return append([]workbench.PullRequestRef(nil), f.PullRequestsByRepo[repo]...), nil
}

func (f FakeService) IssuesForRepo(repo string) ([]workbench.IssueRef, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	return append([]workbench.IssueRef(nil), f.IssuesByRepo[repo]...), nil
}

func fakeCheckSummary(checks map[string]workbench.CheckSummary, key string) workbench.CheckSummary {
	if summary, ok := checks[key]; ok {
		return summary
	}
	return workbench.CheckSummary{State: workbench.CheckUnknown}
}

func cloneWorkflowJobs(jobs []workbench.WorkflowJobRef) []workbench.WorkflowJobRef {
	out := append([]workbench.WorkflowJobRef(nil), jobs...)
	for i := range out {
		out[i].Steps = append([]workbench.WorkflowStepRef(nil), out[i].Steps...)
	}
	return out
}

func cloneWorkflowLog(log workbench.WorkflowLog) workbench.WorkflowLog {
	log.Lines = append([]string(nil), log.Lines...)
	return log
}
