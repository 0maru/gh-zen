package app

import (
	"context"

	"github.com/0maru/gh-zen/internal/github"
	"github.com/0maru/gh-zen/internal/workbench"
)

const defaultWorkflowRunLimit = 30

// ActionsLoader loads GitHub Actions data for the selected repository.
type ActionsLoader interface {
	LoadRuns(ctx context.Context, repo workbench.RepoRef) ([]workbench.WorkflowRunRef, error)
	LoadRunPreview(ctx context.Context, repo workbench.RepoRef, run workbench.WorkflowRunRef) (ActionsRunPreview, error)
	LoadRunLogs(ctx context.Context, repo workbench.RepoRef, run workbench.WorkflowRunRef, opts github.LogFetchOptions) (workbench.WorkflowLog, error)
}

// ActionsRunPreview contains the lightweight details shown when a run is focused.
type ActionsRunPreview struct {
	Run         workbench.WorkflowRunRef
	Jobs        []workbench.WorkflowJobRef
	Annotations map[int64][]workbench.AnnotationRef
}

type githubActionsLoader struct {
	service github.Service
	limit   int
}

// NewGitHubActionsLoader adapts the GitHub service boundary for the TUI.
func NewGitHubActionsLoader(service github.Service) ActionsLoader {
	return githubActionsLoader{service: service, limit: defaultWorkflowRunLimit}
}

func (l githubActionsLoader) LoadRuns(ctx context.Context, repo workbench.RepoRef) ([]workbench.WorkflowRunRef, error) {
	return l.service.WorkflowRuns(ctx, repo.FullName(), github.WorkflowRunListOptions{Limit: l.limit})
}

func (l githubActionsLoader) LoadRunPreview(ctx context.Context, repo workbench.RepoRef, run workbench.WorkflowRunRef) (ActionsRunPreview, error) {
	detail, err := l.service.WorkflowRun(ctx, repo.FullName(), run.ID)
	if err != nil {
		return ActionsRunPreview{}, err
	}
	if detail.ID == 0 {
		detail = run
	}
	jobs, err := l.service.WorkflowRunJobs(ctx, repo.FullName(), run.ID)
	if err != nil {
		return ActionsRunPreview{Run: detail}, err
	}
	annotations := map[int64][]workbench.AnnotationRef{}
	for _, job := range jobs {
		jobAnnotations, err := l.service.JobAnnotations(ctx, repo.FullName(), job.ID)
		if err != nil {
			return ActionsRunPreview{Run: detail, Jobs: jobs, Annotations: annotations}, err
		}
		if len(jobAnnotations) > 0 {
			annotations[job.ID] = jobAnnotations
		}
	}
	return ActionsRunPreview{
		Run:         detail,
		Jobs:        jobs,
		Annotations: annotations,
	}, nil
}

func (l githubActionsLoader) LoadRunLogs(ctx context.Context, repo workbench.RepoRef, run workbench.WorkflowRunRef, opts github.LogFetchOptions) (workbench.WorkflowLog, error) {
	return l.service.WorkflowRunLogs(ctx, repo.FullName(), run.ID, opts)
}

type fakeActionsLoader struct {
	runs        []workbench.WorkflowRunRef
	jobs        map[int64][]workbench.WorkflowJobRef
	annotations map[int64][]workbench.AnnotationRef
	logs        map[int64]workbench.WorkflowLog
}

func newFakeActionsLoader() ActionsLoader {
	return fakeActionsLoader{
		runs:        workbench.FakeWorkflowRuns(),
		jobs:        workbench.FakeWorkflowJobs(),
		annotations: workbench.FakeWorkflowAnnotations(),
		logs:        workbench.FakeWorkflowLogs(),
	}
}

func (l fakeActionsLoader) LoadRuns(context.Context, workbench.RepoRef) ([]workbench.WorkflowRunRef, error) {
	return append([]workbench.WorkflowRunRef(nil), l.runs...), nil
}

func (l fakeActionsLoader) LoadRunPreview(_ context.Context, _ workbench.RepoRef, run workbench.WorkflowRunRef) (ActionsRunPreview, error) {
	detail := run
	for _, candidate := range l.runs {
		if candidate.ID == run.ID {
			detail = candidate
			break
		}
	}
	jobs := cloneActionJobs(l.jobs[run.ID])
	annotations := map[int64][]workbench.AnnotationRef{}
	for _, job := range jobs {
		if jobAnnotations := l.annotations[job.ID]; len(jobAnnotations) > 0 {
			annotations[job.ID] = append([]workbench.AnnotationRef(nil), jobAnnotations...)
		}
	}
	return ActionsRunPreview{Run: detail, Jobs: jobs, Annotations: annotations}, nil
}

func (l fakeActionsLoader) LoadRunLogs(_ context.Context, _ workbench.RepoRef, run workbench.WorkflowRunRef, opts github.LogFetchOptions) (workbench.WorkflowLog, error) {
	if log, ok := l.logs[run.ID]; ok {
		log.Lines = append([]string(nil), log.Lines...)
		return log, nil
	}
	return workbench.WorkflowLog{RunID: run.ID, Failed: opts.FailedOnly}, nil
}

func cloneActionJobs(jobs []workbench.WorkflowJobRef) []workbench.WorkflowJobRef {
	out := append([]workbench.WorkflowJobRef(nil), jobs...)
	for i := range out {
		out[i].Steps = append([]workbench.WorkflowStepRef(nil), out[i].Steps...)
	}
	return out
}
