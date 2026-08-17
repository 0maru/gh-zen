package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/0maru/gh-zen/internal/pullrequests"
	"github.com/0maru/gh-zen/internal/workbench"
)

// Service is the GitHub data boundary consumed by application commands.
type Service interface {
	RepositorySummary(ctx context.Context, repo string) (RepositorySummary, error)
	PullRequests(ctx context.Context, repo string) ([]workbench.PullRequestRef, error)
	Issues(ctx context.Context, repo string) ([]workbench.IssueRef, error)
	CheckSummary(ctx context.Context, repo string, ref string) (workbench.CheckSummary, error)
	ViewerReviewSubjects(ctx context.Context) (workbench.ReviewSubjects, error)
	WorkflowRuns(ctx context.Context, repo string, opts WorkflowRunListOptions) ([]workbench.WorkflowRunRef, error)
	WorkflowRun(ctx context.Context, repo string, runID int64) (workbench.WorkflowRunRef, error)
	WorkflowRunJobs(ctx context.Context, repo string, runID int64) ([]workbench.WorkflowJobRef, error)
	JobAnnotations(ctx context.Context, repo string, jobID int64) ([]workbench.AnnotationRef, error)
	WorkflowRunLogs(ctx context.Context, repo string, runID int64, opts LogFetchOptions) (workbench.WorkflowLog, error)
}

// RepositorySummary contains lightweight GitHub data for a repository refresh.
type RepositorySummary struct {
	Repo         string
	PullRequests []workbench.PullRequestRef
	Issues       []workbench.IssueRef
	Checks       workbench.CheckSummary
}

// Runner executes gh commands.
type Runner interface {
	Run(ctx context.Context, args ...string) ([]byte, error)
}

// CLIService is a gh-backed GitHub service skeleton.
type CLIService struct {
	Runner Runner
}

// WorkflowRunListOptions controls workflow run list size.
type WorkflowRunListOptions struct {
	Limit int
}

// LogFetchOptions controls explicit workflow log fetches.
type LogFetchOptions struct {
	FailedOnly bool
	JobID      *int64
	TailLines  int
}

// GHRunner executes the gh binary.
type GHRunner struct{}

// ErrorKind classifies gh failures into user-actionable categories.
type ErrorKind string

const (
	ErrorAuth    ErrorKind = "auth"
	ErrorNetwork ErrorKind = "network"
	ErrorCommand ErrorKind = "command"

	issueListFields  = "number,title,state,url,body,labels,assignees,milestone,author,updatedAt"
	listLimit        = "1000"
	listLimitCount   = 1000
	prListFields     = "number,title,state,url,headRefName,headRepositoryOwner,baseRefName,isDraft,updatedAt,author,reviewRequests,latestReviews,reviewDecision,body"
	runListFields    = "databaseId,number,name,workflowName,headBranch,event,status,conclusion,headSha,displayTitle,url,createdAt,updatedAt"
	runViewFields    = "databaseId,number,name,workflowName,headBranch,event,status,conclusion,headSha,displayTitle,url,createdAt,updatedAt"
	runJobsFields    = "jobs"
	runListLimit     = 30
	logTailLineLimit = 500

	repositoryPullRequestsQuery = `
query($owner:String!, $name:String!, $after:String) {
  viewer {
    login
  }
  repository(owner:$owner, name:$name) {
    pullRequests(first:100, after:$after, states:[OPEN, CLOSED, MERGED], orderBy:{field:UPDATED_AT, direction:DESC}) {
      nodes {
        number
        title
        state
        isDraft
        url
        bodyText
        headRefName
        baseRefName
        reviewDecision
        mergeable
        updatedAt
        author {
          login
        }
        headRepositoryOwner {
          login
        }
        reviewRequests(first:20) {
          nodes {
            requestedReviewer {
              __typename
              ... on User {
                login
                name
              }
              ... on Team {
                slug
                name
              }
            }
          }
        }
        latestReviews(first:20) {
          nodes {
            author {
              login
            }
            state
          }
        }
        closingIssuesReferences(first:20) {
          nodes {
            number
            title
            state
            url
          }
        }
        commits(last:1) {
          nodes {
            commit {
              oid
              statusCheckRollup {
                contexts(first:100) {
                  pageInfo {
                    hasNextPage
                    endCursor
                  }
                  nodes {
                    __typename
                    ... on CheckRun {
                      name
                      status
                      conclusion
                    }
                    ... on StatusContext {
                      context
                      state
                    }
                  }
                }
              }
            }
          }
        }
      }
      pageInfo {
        hasNextPage
        endCursor
      }
    }
  }
}`

	repositoryPullRequestDetailQuery = `
query($owner:String!, $name:String!, $number:Int!) {
  repository(owner:$owner, name:$name) {
    pullRequest(number:$number) {
      number
      title
      state
      isDraft
      url
      bodyText
      headRefName
      baseRefName
      reviewDecision
      mergeable
      updatedAt
      author {
        login
      }
      headRepositoryOwner {
        login
      }
      reviewRequests(first:20) {
        nodes {
          requestedReviewer {
            __typename
            ... on User {
              login
              name
            }
            ... on Team {
              slug
              name
            }
          }
        }
      }
      latestReviews(first:20) {
        nodes {
          author {
            login
          }
          state
        }
      }
      closingIssuesReferences(first:20) {
        nodes {
          number
          title
          state
          url
        }
      }
      commits(last:1) {
        nodes {
          commit {
            oid
            statusCheckRollup {
              contexts(first:100) {
                pageInfo {
                  hasNextPage
                  endCursor
                }
                nodes {
                  __typename
                  ... on CheckRun {
                    name
                    status
                    conclusion
                  }
                  ... on StatusContext {
                    context
                    state
                  }
                }
              }
            }
          }
        }
      }
    }
  }
	}`

	repositoryPullRequestCheckContextsQuery = `
query($owner:String!, $name:String!, $oid:GitObjectID!, $after:String!) {
  repository(owner:$owner, name:$name) {
    object(oid:$oid) {
      ... on Commit {
        statusCheckRollup {
          contexts(first:100, after:$after) {
            pageInfo {
              hasNextPage
              endCursor
            }
            nodes {
              __typename
              ... on CheckRun {
                name
                status
                conclusion
              }
              ... on StatusContext {
                context
                state
              }
            }
          }
        }
      }
    }
  }
}`

	pullRequestClosingIssuesQuery = `
query($owner:String!, $name:String!, $after:String) {
  repository(owner:$owner, name:$name) {
    pullRequests(first:100, after:$after, states:[OPEN, CLOSED, MERGED], orderBy:{field:CREATED_AT, direction:DESC}) {
      nodes {
        number
        closingIssuesReferences(first:100) {
          nodes {
            number
            title
            state
            url
            repository {
              nameWithOwner
            }
          }
          pageInfo {
            hasNextPage
            endCursor
          }
        }
      }
      pageInfo {
        hasNextPage
        endCursor
      }
    }
  }
}`
	pullRequestClosingIssuePageQuery = `
query($owner:String!, $name:String!, $number:Int!, $after:String) {
  repository(owner:$owner, name:$name) {
    pullRequest(number:$number) {
      closingIssuesReferences(first:100, after:$after) {
        nodes {
          number
          title
          state
          url
          repository {
            nameWithOwner
          }
        }
        pageInfo {
          hasNextPage
          endCursor
        }
      }
    }
  }
}`

	issueCommentCountsQuery = `
query($owner:String!, $name:String!, $after:String) {
  repository(owner:$owner, name:$name) {
    issues(first:100, after:$after, states:[OPEN, CLOSED], orderBy:{field:CREATED_AT, direction:DESC}) {
      nodes {
        number
        comments {
          totalCount
        }
      }
      pageInfo {
        hasNextPage
        endCursor
      }
    }
  }
}`
)

var (
	closingIssueReferencePattern = regexp.MustCompile(`(?i)\b(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?)\b:?\s+(?:#(\d+)\b|([[:alnum:]_.-]+)/([[:alnum:]_.-]+)#(\d+)\b|https://github\.com/([[:alnum:]_.-]+)/([[:alnum:]_.-]+)/issues/(\d+)\b)`)
)

// Error describes a gh-backed service failure.
type Error struct {
	Op     string
	Kind   ErrorKind
	Output string
	Err    error
}

func (e Error) Error() string {
	output := strings.TrimSpace(e.Output)
	if output == "" {
		return fmt.Sprintf("%s failed (%s): %v", e.Op, e.Kind, e.Err)
	}
	return fmt.Sprintf("%s failed (%s): %s", e.Op, e.Kind, output)
}

func (e Error) Unwrap() error {
	return e.Err
}

// Run executes gh and returns raw command output.
func (GHRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if isPendingChecksExit(args, err) {
			return output, nil
		}
		return nil, classifyError("gh "+strings.Join(args, " "), output, err)
	}
	return output, nil
}

// RepositorySummary loads lightweight PR and issue summaries.
func (s CLIService) RepositorySummary(ctx context.Context, repo string) (RepositorySummary, error) {
	prs, err := s.PullRequests(ctx, repo)
	if err != nil {
		return RepositorySummary{}, err
	}
	issues, err := s.Issues(ctx, repo)
	if err != nil {
		return RepositorySummary{}, err
	}
	return RepositorySummary{
		Repo:         repo,
		PullRequests: prs,
		Issues:       issues,
		Checks:       workbench.CheckSummary{State: workbench.CheckUnknown},
	}, nil
}

// PullRequests loads pull request summaries through gh.
func (s CLIService) PullRequests(ctx context.Context, repo string) ([]workbench.PullRequestRef, error) {
	output, err := s.runner().Run(ctx, "pr", "list", "--repo", repo, "--state", "all", "--limit", listLimit, "--json", prListFields)
	if err != nil {
		return nil, err
	}
	var payload []struct {
		Number              int    `json:"number"`
		Title               string `json:"title"`
		State               string `json:"state"`
		URL                 string `json:"url"`
		HeadRefName         string `json:"headRefName"`
		BaseRefName         string `json:"baseRefName"`
		IsDraft             bool   `json:"isDraft"`
		UpdatedAt           string `json:"updatedAt"`
		Body                string `json:"body"`
		Author              ghUser `json:"author"`
		HeadRepositoryOwner struct {
			Login string `json:"login"`
		} `json:"headRepositoryOwner"`
		ReviewDecision string             `json:"reviewDecision"`
		ReviewRequests []ghReviewRequest  `json:"reviewRequests"`
		LatestReviews  []ghPullReviewItem `json:"latestReviews"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		return nil, fmt.Errorf("parse gh pr list output: %w", err)
	}
	closingIssuesByPR, closingIssuesErr := s.pullRequestClosingIssues(ctx, repo)

	prs := make([]workbench.PullRequestRef, 0, len(payload))
	for _, pr := range payload {
		prs = append(prs, workbench.PullRequestRef{
			Number:         pr.Number,
			Title:          pr.Title,
			State:          strings.ToLower(pr.State),
			URL:            pr.URL,
			AuthorLogin:    pr.Author.Login,
			HeadOwner:      pr.HeadRepositoryOwner.Login,
			HeadBranch:     pr.HeadRefName,
			BaseBranch:     pr.BaseRefName,
			IsDraft:        pr.IsDraft,
			UpdatedAt:      pr.UpdatedAt,
			LinkedIssues:   linkedIssues(closingIssuesByPR[pr.Number], repo, pr.Body, closingIssuesErr != nil),
			BodyExcerpt:    textExcerpt(pr.Body),
			ReviewDecision: reviewState(pr.ReviewDecision),
			ReviewState:    reviewState(pr.ReviewDecision),
			ReviewRequests: reviewRequests(pr.ReviewRequests),
			LatestReviews:  latestReviews(pr.LatestReviews),
		})
	}
	return prs, nil
}

// List loads first-class pull request browser data through gh GraphQL.
func (s CLIService) List(ctx context.Context, repo string, filter pullrequests.PullRequestFilter) ([]pullrequests.PullRequest, error) {
	prs, err := s.RepositoryPullRequests(ctx, repo)
	if err != nil {
		return nil, err
	}
	return pullrequests.Filter(prs, filter), nil
}

// Detail loads one pull request for the PR browser.
func (s CLIService) Detail(ctx context.Context, repo string, number int) (pullrequests.PullRequest, error) {
	if number <= 0 {
		return pullrequests.PullRequest{}, fmt.Errorf("pull request number must be positive")
	}
	owner, name, ok := strings.Cut(repo, "/")
	if !ok || owner == "" || name == "" {
		return pullrequests.PullRequest{}, fmt.Errorf("repo must use owner/repo format")
	}
	output, err := s.runner().Run(ctx, "api", "graphql", "-f", "owner="+owner, "-f", "name="+name, "-F", "number="+strconv.Itoa(number), "-f", "query="+repositoryPullRequestDetailQuery)
	if err != nil {
		return pullrequests.PullRequest{}, err
	}
	var payload struct {
		Data struct {
			Repository struct {
				PullRequest *ghPullRequestBrowserNode `json:"pullRequest"`
			} `json:"repository"`
		} `json:"data"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		return pullrequests.PullRequest{}, fmt.Errorf("parse gh pull request detail output: %w", err)
	}
	if payload.Data.Repository.PullRequest == nil {
		return pullrequests.PullRequest{}, fmt.Errorf("pull request #%d not found", number)
	}
	if err := s.loadRemainingPullRequestCheckContexts(ctx, owner, name, payload.Data.Repository.PullRequest); err != nil {
		return pullrequests.PullRequest{}, err
	}
	return pullRequestFromGraphQL(*payload.Data.Repository.PullRequest), nil
}

// RepositoryPullRequests loads all selected-repository pull requests for the PR browser.
func (s CLIService) RepositoryPullRequests(ctx context.Context, repo string) ([]pullrequests.PullRequest, error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok || owner == "" || name == "" {
		return nil, fmt.Errorf("repo must use owner/repo format")
	}

	prs := []pullrequests.PullRequest{}
	after := ""
	for {
		args := []string{"api", "graphql", "-f", "owner=" + owner, "-f", "name=" + name}
		if after != "" {
			args = append(args, "-f", "after="+after)
		}
		args = append(args, "-f", "query="+repositoryPullRequestsQuery)
		output, err := s.runner().Run(ctx, args...)
		if err != nil {
			return prs, err
		}
		var payload struct {
			Data struct {
				Viewer     ghUser `json:"viewer"`
				Repository struct {
					PullRequests struct {
						Nodes    []ghPullRequestBrowserNode `json:"nodes"`
						PageInfo struct {
							HasNextPage bool   `json:"hasNextPage"`
							EndCursor   string `json:"endCursor"`
						} `json:"pageInfo"`
					} `json:"pullRequests"`
				} `json:"repository"`
			} `json:"data"`
		}
		if err := json.Unmarshal(output, &payload); err != nil {
			return prs, fmt.Errorf("parse gh repository pull requests output: %w", err)
		}
		for i := range payload.Data.Repository.PullRequests.Nodes {
			node := &payload.Data.Repository.PullRequests.Nodes[i]
			if err := s.loadRemainingPullRequestCheckContexts(ctx, owner, name, node); err != nil {
				return prs, err
			}
			prs = append(prs, pullRequestWithViewerPerspective(pullRequestFromGraphQL(*node), payload.Data.Viewer.Login))
		}
		if !payload.Data.Repository.PullRequests.PageInfo.HasNextPage {
			pullrequests.SortByUpdatedDesc(prs)
			return prs, nil
		}
		after = payload.Data.Repository.PullRequests.PageInfo.EndCursor
		if after == "" {
			pullrequests.SortByUpdatedDesc(prs)
			return prs, nil
		}
	}
}

func (s CLIService) loadRemainingPullRequestCheckContexts(ctx context.Context, owner string, name string, node *ghPullRequestBrowserNode) error {
	for i := range node.Commits.Nodes {
		commit := &node.Commits.Nodes[i].Commit
		if commit.StatusCheckRollup == nil {
			continue
		}
		contexts := &commit.StatusCheckRollup.Contexts
		for contexts.PageInfo.HasNextPage {
			if commit.OID == "" || contexts.PageInfo.EndCursor == "" {
				return fmt.Errorf("paginate pull request #%d check contexts: missing commit oid or cursor", node.Number)
			}
			args := []string{
				"api", "graphql",
				"-f", "owner=" + owner,
				"-f", "name=" + name,
				"-f", "oid=" + commit.OID,
				"-f", "after=" + contexts.PageInfo.EndCursor,
				"-f", "query=" + repositoryPullRequestCheckContextsQuery,
			}
			output, err := s.runner().Run(ctx, args...)
			if err != nil {
				return fmt.Errorf("paginate pull request #%d check contexts: %w", node.Number, err)
			}
			var payload struct {
				Data struct {
					Repository struct {
						Object *struct {
							StatusCheckRollup *struct {
								Contexts ghCheckContexts `json:"contexts"`
							} `json:"statusCheckRollup"`
						} `json:"object"`
					} `json:"repository"`
				} `json:"data"`
			}
			if err := json.Unmarshal(output, &payload); err != nil {
				return fmt.Errorf("parse pull request #%d check contexts: %w", node.Number, err)
			}
			if payload.Data.Repository.Object == nil || payload.Data.Repository.Object.StatusCheckRollup == nil {
				return fmt.Errorf("paginate pull request #%d check contexts: commit not found", node.Number)
			}
			page := payload.Data.Repository.Object.StatusCheckRollup.Contexts
			contexts.Nodes = append(contexts.Nodes, page.Nodes...)
			contexts.PageInfo = page.PageInfo
		}
	}
	return nil
}

// Issues loads issue summaries and browser-only metadata through gh.
func (s CLIService) Issues(ctx context.Context, repo string) ([]workbench.IssueRef, error) {
	return s.IssuesWithOptions(ctx, repo, workbench.IssueListOptions{IncludeCommentsCount: true})
}

// IssuesWithOptions loads issue summaries while allowing workbench refreshes to defer comment counts.
func (s CLIService) IssuesWithOptions(ctx context.Context, repo string, opts workbench.IssueListOptions) ([]workbench.IssueRef, error) {
	output, err := s.runner().Run(ctx, "issue", "list", "--repo", repo, "--state", "all", "--limit", listLimit, "--json", issueListFields)
	if err != nil {
		return nil, err
	}
	var payload []struct {
		Number    int       `json:"number"`
		Title     string    `json:"title"`
		State     string    `json:"state"`
		URL       string    `json:"url"`
		Body      string    `json:"body"`
		Labels    []ghLabel `json:"labels"`
		Assignees []ghUser  `json:"assignees"`
		Milestone *struct {
			Title string `json:"title"`
		} `json:"milestone"`
		Author    ghUser `json:"author"`
		UpdatedAt string `json:"updatedAt"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		return nil, fmt.Errorf("parse gh issue list output: %w", err)
	}
	commentCounts := map[int]int{}
	var commentCountsErr error
	if opts.IncludeCommentsCount {
		commentCounts, commentCountsErr = s.issueCommentCounts(ctx, repo)
	}
	issues := make([]workbench.IssueRef, 0, len(payload))
	for _, issue := range payload {
		issues = append(issues, workbench.IssueRef{
			Number:        issue.Number,
			Repository:    repo,
			Title:         issue.Title,
			State:         strings.ToLower(issue.State),
			URL:           issue.URL,
			Body:          issue.Body,
			Labels:        labelNames(issue.Labels),
			Assignees:     userLogins(issue.Assignees),
			Milestone:     milestoneTitle(issue.Milestone),
			AuthorLogin:   issue.Author.Login,
			CommentsCount: commentCounts[issue.Number],
			UpdatedAt:     issue.UpdatedAt,
			Certain:       true,
		})
	}
	if commentCountsErr != nil {
		return issues, fmt.Errorf("load issue comment counts: %w", commentCountsErr)
	}
	return issues, nil
}

// CheckSummary loads a check summary for a pull request or branch ref through gh.
func (s CLIService) CheckSummary(ctx context.Context, repo string, ref string) (workbench.CheckSummary, error) {
	if ref == "" {
		return workbench.CheckSummary{State: workbench.CheckUnknown}, nil
	}
	output, err := s.runner().Run(ctx, "pr", "checks", ref, "--repo", repo, "--json", "name,state")
	if err != nil {
		return workbench.CheckSummary{}, err
	}
	var payload []struct {
		Name  string `json:"name"`
		State string `json:"state"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		return workbench.CheckSummary{}, fmt.Errorf("parse gh pr checks output: %w", err)
	}
	states := make([]string, 0, len(payload))
	for _, check := range payload {
		states = append(states, check.State)
	}
	return summarizeCheckStates(states), nil
}

// ViewerReviewSubjects loads the authenticated viewer and team slugs used for review request matching.
func (s CLIService) ViewerReviewSubjects(ctx context.Context) (workbench.ReviewSubjects, error) {
	output, err := s.runner().Run(ctx, "api", "user", "--jq", ".login")
	if err != nil {
		return workbench.ReviewSubjects{}, err
	}
	subjects := workbench.ReviewSubjects{Login: strings.TrimSpace(string(output))}

	output, err = s.runner().Run(ctx, "api", "user/teams", "--jq", ".[].slug")
	if err != nil {
		return subjects, err
	}
	subjects.TeamSlugs = nonEmptyLines(string(output))
	return subjects, nil
}

// WorkflowRuns loads recent GitHub Actions workflow runs through gh.
func (s CLIService) WorkflowRuns(ctx context.Context, repo string, opts WorkflowRunListOptions) ([]workbench.WorkflowRunRef, error) {
	limit := workflowRunLimit(opts)
	output, err := s.runner().Run(ctx, "run", "list", "--repo", repo, "--limit", strconv.Itoa(limit), "--json", runListFields)
	if err != nil {
		return nil, err
	}
	var payload []ghWorkflowRun
	if err := json.Unmarshal(output, &payload); err != nil {
		return nil, fmt.Errorf("parse gh run list output: %w", err)
	}
	runs := make([]workbench.WorkflowRunRef, 0, len(payload))
	for _, run := range payload {
		runs = append(runs, workflowRunRef(run))
	}
	actors := workflowRunActors(ctx, s, repo, limit)
	for i := range runs {
		if runs[i].Actor == "" {
			runs[i].Actor = actors[runs[i].ID]
		}
	}
	return runs, nil
}

// WorkflowRun loads one workflow run summary through gh.
func (s CLIService) WorkflowRun(ctx context.Context, repo string, runID int64) (workbench.WorkflowRunRef, error) {
	output, err := s.runner().Run(ctx, "run", "view", strconv.FormatInt(runID, 10), "--repo", repo, "--json", runViewFields)
	if err != nil {
		return workbench.WorkflowRunRef{}, err
	}
	var payload ghWorkflowRun
	if err := json.Unmarshal(output, &payload); err != nil {
		return workbench.WorkflowRunRef{}, fmt.Errorf("parse gh run view output: %w", err)
	}
	run := workflowRunRef(payload)
	if run.Actor == "" {
		run.Actor = workflowRunActor(ctx, s, repo, run.ID)
	}
	return run, nil
}

// WorkflowRunJobs loads jobs for one workflow run through gh.
func (s CLIService) WorkflowRunJobs(ctx context.Context, repo string, runID int64) ([]workbench.WorkflowJobRef, error) {
	output, err := s.runner().Run(ctx, "run", "view", strconv.FormatInt(runID, 10), "--repo", repo, "--json", runJobsFields)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Jobs []ghWorkflowJob `json:"jobs"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		return nil, fmt.Errorf("parse gh run jobs output: %w", err)
	}
	jobs := make([]workbench.WorkflowJobRef, 0, len(payload.Jobs))
	for _, job := range payload.Jobs {
		jobs = append(jobs, workflowJobRef(job))
	}
	return jobs, nil
}

// JobAnnotations loads check-run annotations for one workflow job through gh api.
func (s CLIService) JobAnnotations(ctx context.Context, repo string, jobID int64) ([]workbench.AnnotationRef, error) {
	output, err := s.runner().Run(ctx, "api", fmt.Sprintf("repos/%s/check-runs/%d/annotations", repo, jobID), "--paginate", "--slurp")
	if err != nil {
		return nil, err
	}
	payload, err := parseAnnotationPages(output)
	if err != nil {
		return nil, fmt.Errorf("parse gh job annotations output: %w", err)
	}
	annotations := make([]workbench.AnnotationRef, 0, len(payload))
	for _, annotation := range payload {
		annotations = append(annotations, workbench.AnnotationRef{
			Path:      annotation.Path,
			StartLine: annotation.StartLine,
			EndLine:   annotation.EndLine,
			Level:     strings.ToLower(annotation.Level),
			Title:     annotation.Title,
			Message:   annotation.Message,
		})
	}
	return annotations, nil
}

func parseAnnotationPages(output []byte) ([]ghAnnotation, error) {
	var pages [][]ghAnnotation
	if err := json.Unmarshal(output, &pages); err == nil {
		count := 0
		for _, page := range pages {
			count += len(page)
		}
		annotations := make([]ghAnnotation, 0, count)
		for _, page := range pages {
			annotations = append(annotations, page...)
		}
		return annotations, nil
	}
	var annotations []ghAnnotation
	if err := json.Unmarshal(output, &annotations); err != nil {
		return nil, err
	}
	return annotations, nil
}

// WorkflowRunLogs fetches logs only when the caller explicitly requests them.
func (s CLIService) WorkflowRunLogs(ctx context.Context, repo string, runID int64, opts LogFetchOptions) (workbench.WorkflowLog, error) {
	args := []string{"run", "view", strconv.FormatInt(runID, 10), "--repo", repo}
	if opts.JobID != nil {
		args = append(args, "--job", strconv.FormatInt(*opts.JobID, 10))
	}
	if opts.FailedOnly {
		args = append(args, "--log-failed")
	} else {
		args = append(args, "--log")
	}
	output, err := s.runner().Run(ctx, args...)
	if err != nil {
		return workbench.WorkflowLog{}, err
	}
	lines := logLines(string(output))
	tailLimit := opts.TailLines
	if tailLimit <= 0 {
		tailLimit = logTailLineLimit
	}
	if len(lines) > tailLimit {
		lines = lines[len(lines)-tailLimit:]
	}
	log := workbench.WorkflowLog{
		RunID:  runID,
		Failed: opts.FailedOnly,
		Lines:  lines,
	}
	if opts.JobID != nil {
		log.JobID = *opts.JobID
	}
	return log, nil
}

func (s CLIService) pullRequestClosingIssues(ctx context.Context, repo string) (map[int][]workbench.IssueRef, error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok || owner == "" || name == "" {
		return nil, nil
	}

	issuesByPR := map[int][]workbench.IssueRef{}
	after := ""
	fetched := 0
	for {
		output, err := s.runner().Run(ctx, "api", "graphql", "-f", "owner="+owner, "-f", "name="+name, "-f", "after="+after, "-f", "query="+pullRequestClosingIssuesQuery)
		if err != nil {
			return issuesByPR, err
		}
		var payload struct {
			Data struct {
				Repository struct {
					PullRequests struct {
						Nodes []struct {
							Number        int `json:"number"`
							ClosingIssues struct {
								Nodes    []ghIssue `json:"nodes"`
								PageInfo struct {
									HasNextPage bool   `json:"hasNextPage"`
									EndCursor   string `json:"endCursor"`
								} `json:"pageInfo"`
							} `json:"closingIssuesReferences"`
						} `json:"nodes"`
						PageInfo struct {
							HasNextPage bool   `json:"hasNextPage"`
							EndCursor   string `json:"endCursor"`
						} `json:"pageInfo"`
					} `json:"pullRequests"`
				} `json:"repository"`
			} `json:"data"`
		}
		if err := json.Unmarshal(output, &payload); err != nil {
			return issuesByPR, fmt.Errorf("parse gh pull request closing issues output: %w", err)
		}
		for _, pr := range payload.Data.Repository.PullRequests.Nodes {
			if pr.Number <= 0 {
				continue
			}
			closingIssues := append([]ghIssue(nil), pr.ClosingIssues.Nodes...)
			if pr.ClosingIssues.PageInfo.HasNextPage && pr.ClosingIssues.PageInfo.EndCursor != "" {
				additional, err := s.pullRequestClosingIssuePages(ctx, owner, name, pr.Number, pr.ClosingIssues.PageInfo.EndCursor)
				closingIssues = append(closingIssues, additional...)
				if err != nil {
					issuesByPR[pr.Number] = issuesFromGraphQL(closingIssues, repo)
					return issuesByPR, err
				}
			}
			issuesByPR[pr.Number] = issuesFromGraphQL(closingIssues, repo)
		}
		fetched += len(payload.Data.Repository.PullRequests.Nodes)
		if !payload.Data.Repository.PullRequests.PageInfo.HasNextPage || fetched >= listLimitCount {
			return issuesByPR, nil
		}
		after = payload.Data.Repository.PullRequests.PageInfo.EndCursor
		if after == "" {
			return issuesByPR, nil
		}
	}
}

func (s CLIService) pullRequestClosingIssuePages(ctx context.Context, owner string, name string, number int, after string) ([]ghIssue, error) {
	var issues []ghIssue
	for after != "" {
		output, err := s.runner().Run(ctx, "api", "graphql", "-f", "owner="+owner, "-f", "name="+name, "-F", "number="+strconv.Itoa(number), "-f", "after="+after, "-f", "query="+pullRequestClosingIssuePageQuery)
		if err != nil {
			return issues, err
		}
		var payload struct {
			Data struct {
				Repository struct {
					PullRequest struct {
						ClosingIssues struct {
							Nodes    []ghIssue `json:"nodes"`
							PageInfo struct {
								HasNextPage bool   `json:"hasNextPage"`
								EndCursor   string `json:"endCursor"`
							} `json:"pageInfo"`
						} `json:"closingIssuesReferences"`
					} `json:"pullRequest"`
				} `json:"repository"`
			} `json:"data"`
		}
		if err := json.Unmarshal(output, &payload); err != nil {
			return issues, fmt.Errorf("parse gh pull request closing issue page output: %w", err)
		}
		connection := payload.Data.Repository.PullRequest.ClosingIssues
		issues = append(issues, connection.Nodes...)
		if !connection.PageInfo.HasNextPage || connection.PageInfo.EndCursor == "" {
			return issues, nil
		}
		after = connection.PageInfo.EndCursor
	}
	return issues, nil
}

func (s CLIService) issueCommentCounts(ctx context.Context, repo string) (map[int]int, error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok || owner == "" || name == "" {
		return nil, nil
	}

	counts := map[int]int{}
	after := ""
	fetched := 0
	for {
		output, err := s.runner().Run(ctx, "api", "graphql", "-f", "owner="+owner, "-f", "name="+name, "-f", "after="+after, "-f", "query="+issueCommentCountsQuery)
		if err != nil {
			return counts, err
		}
		var payload struct {
			Data struct {
				Repository struct {
					Issues struct {
						Nodes []struct {
							Number   int `json:"number"`
							Comments struct {
								TotalCount int `json:"totalCount"`
							} `json:"comments"`
						} `json:"nodes"`
						PageInfo struct {
							HasNextPage bool   `json:"hasNextPage"`
							EndCursor   string `json:"endCursor"`
						} `json:"pageInfo"`
					} `json:"issues"`
				} `json:"repository"`
			} `json:"data"`
		}
		if err := json.Unmarshal(output, &payload); err != nil {
			return counts, fmt.Errorf("parse gh issue comment counts output: %w", err)
		}
		for _, issue := range payload.Data.Repository.Issues.Nodes {
			if issue.Number > 0 {
				counts[issue.Number] = issue.Comments.TotalCount
			}
		}
		fetched += len(payload.Data.Repository.Issues.Nodes)
		if !payload.Data.Repository.Issues.PageInfo.HasNextPage || fetched >= listLimitCount {
			return counts, nil
		}
		after = payload.Data.Repository.Issues.PageInfo.EndCursor
		if after == "" {
			return counts, nil
		}
	}
}

func (s CLIService) runner() Runner {
	if s.Runner != nil {
		return s.Runner
	}
	return GHRunner{}
}

type ghUser struct {
	Login string `json:"login"`
}

type ghLabel struct {
	Name string `json:"name"`
}

type ghIssue struct {
	Number     int    `json:"number"`
	Title      string `json:"title"`
	State      string `json:"state"`
	URL        string `json:"url"`
	Repository struct {
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"repository"`
}

type ghReviewRequest struct {
	TypeName string `json:"__typename"`
	Login    string `json:"login"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
}

type ghPullReviewItem struct {
	Author ghUser `json:"author"`
	State  string `json:"state"`
}

type ghPullRequestBrowserNode struct {
	Number              int    `json:"number"`
	Title               string `json:"title"`
	State               string `json:"state"`
	IsDraft             bool   `json:"isDraft"`
	URL                 string `json:"url"`
	BodyText            string `json:"bodyText"`
	HeadRefName         string `json:"headRefName"`
	BaseRefName         string `json:"baseRefName"`
	ReviewDecision      string `json:"reviewDecision"`
	Mergeable           string `json:"mergeable"`
	UpdatedAt           string `json:"updatedAt"`
	Author              ghUser `json:"author"`
	HeadRepositoryOwner struct {
		Login string `json:"login"`
	} `json:"headRepositoryOwner"`
	ReviewRequests struct {
		Nodes []struct {
			RequestedReviewer ghReviewRequest `json:"requestedReviewer"`
		} `json:"nodes"`
	} `json:"reviewRequests"`
	LatestReviews struct {
		Nodes []ghPullReviewItem `json:"nodes"`
	} `json:"latestReviews"`
	ClosingIssues struct {
		Nodes []ghIssue `json:"nodes"`
	} `json:"closingIssuesReferences"`
	Commits struct {
		Nodes []struct {
			Commit struct {
				OID               string `json:"oid"`
				StatusCheckRollup *struct {
					Contexts ghCheckContexts `json:"contexts"`
				} `json:"statusCheckRollup"`
			} `json:"commit"`
		} `json:"nodes"`
	} `json:"commits"`
}

type ghCheckContexts struct {
	Nodes    []ghCheckContext `json:"nodes"`
	PageInfo struct {
		HasNextPage bool   `json:"hasNextPage"`
		EndCursor   string `json:"endCursor"`
	} `json:"pageInfo"`
}

type ghCheckContext struct {
	TypeName   string `json:"__typename"`
	Name       string `json:"name"`
	Context    string `json:"context"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	State      string `json:"state"`
}

func pullRequestFromGraphQL(node ghPullRequestBrowserNode) pullrequests.PullRequest {
	return pullrequests.PullRequest{
		Number:         node.Number,
		Title:          node.Title,
		State:          strings.ToLower(node.State),
		IsDraft:        node.IsDraft,
		Author:         node.Author.Login,
		HeadRef:        pullRequestHeadRef(node.HeadRepositoryOwner.Login, node.HeadRefName),
		BaseRef:        node.BaseRefName,
		ReviewDecision: reviewState(node.ReviewDecision),
		ReviewRequests: pullRequestReviewRequests(node.ReviewRequests.Nodes),
		LatestReviews:  pullRequestLatestReviews(node.LatestReviews.Nodes),
		LinkedIssues:   pullRequestLinkedIssues(node.ClosingIssues.Nodes, node.BodyText),
		Checks:         pullRequestCheckSummary(node.Commits.Nodes),
		Mergeability:   reviewState(node.Mergeable),
		UpdatedAt:      node.UpdatedAt,
		URL:            node.URL,
		BodyExcerpt:    textExcerpt(node.BodyText),
	}
}

func pullRequestWithViewerPerspective(pr pullrequests.PullRequest, viewer string) pullrequests.PullRequest {
	if viewer == "" {
		return pr
	}
	pr.ViewerReviewRequested = pullRequestNeedsViewerReview(pr, viewer)
	pr.WaitingOnReview = pullRequestWaitingOnViewerAuthoredReview(pr, viewer)
	return pr
}

func pullRequestNeedsViewerReview(pr pullrequests.PullRequest, viewer string) bool {
	if !pullRequestOpenForReview(pr) {
		return false
	}
	for _, request := range pr.ReviewRequests {
		if strings.EqualFold(request.Login, viewer) {
			return true
		}
	}
	return false
}

func pullRequestWaitingOnViewerAuthoredReview(pr pullrequests.PullRequest, viewer string) bool {
	if !pullRequestOpenForReview(pr) || !strings.EqualFold(pr.Author, viewer) {
		return false
	}
	return len(pr.ReviewRequests) > 0 || strings.EqualFold(pr.ReviewDecision, "review required")
}

func pullRequestOpenForReview(pr pullrequests.PullRequest) bool {
	return strings.EqualFold(pr.State, "open") && !pr.IsDraft
}

func pullRequestHeadRef(owner string, branch string) string {
	switch {
	case owner != "" && branch != "":
		return owner + "/" + branch
	case branch != "":
		return branch
	default:
		return owner
	}
}

func pullRequestReviewRequests(nodes []struct {
	RequestedReviewer ghReviewRequest `json:"requestedReviewer"`
}) []pullrequests.ReviewRequest {
	requests := make([]pullrequests.ReviewRequest, 0, len(nodes))
	for _, node := range nodes {
		request := node.RequestedReviewer
		requests = append(requests, pullrequests.ReviewRequest{
			Kind:  request.TypeName,
			Login: request.Login,
			Name:  reviewRequestName(request),
			Slug:  request.Slug,
		})
	}
	return requests
}

func pullRequestLatestReviews(payload []ghPullReviewItem) []pullrequests.Review {
	reviews := make([]pullrequests.Review, 0, len(payload))
	for _, review := range payload {
		reviews = append(reviews, pullrequests.Review{
			Author: review.Author.Login,
			State:  reviewState(review.State),
		})
	}
	return reviews
}

func pullRequestLinkedIssues(closingIssues []ghIssue, body string) []pullrequests.LinkedIssue {
	issues := make([]pullrequests.LinkedIssue, 0, len(closingIssues))
	seen := map[int]bool{}
	for _, issue := range closingIssues {
		if issue.Number == 0 {
			continue
		}
		issues = append(issues, pullrequests.LinkedIssue{
			Number: issue.Number,
			Title:  issue.Title,
			State:  strings.ToLower(issue.State),
			URL:    issue.URL,
		})
		seen[issue.Number] = true
	}
	for _, issue := range linkedIssuesFromBody("", body) {
		if issue.Number == 0 || seen[issue.Number] {
			continue
		}
		issues = append(issues, pullrequests.LinkedIssue{
			Number: issue.Number,
			Title:  issue.Title,
			State:  issue.State,
			URL:    issue.URL,
		})
		seen[issue.Number] = true
	}
	return issues
}

func pullRequestCheckSummary(nodes []struct {
	Commit struct {
		OID               string `json:"oid"`
		StatusCheckRollup *struct {
			Contexts ghCheckContexts `json:"contexts"`
		} `json:"statusCheckRollup"`
	} `json:"commit"`
}) pullrequests.CheckSummary {
	states := []string{}
	for _, node := range nodes {
		if node.Commit.StatusCheckRollup == nil {
			continue
		}
		for _, context := range node.Commit.StatusCheckRollup.Contexts.Nodes {
			states = append(states, pullRequestCheckContextState(context))
		}
	}
	return summarizePullRequestCheckStates(states)
}

func pullRequestCheckContextState(context ghCheckContext) string {
	if context.Conclusion != "" {
		return context.Conclusion
	}
	if context.State != "" {
		return context.State
	}
	return context.Status
}

func summarizePullRequestCheckStates(states []string) pullrequests.CheckSummary {
	summary := pullrequests.CheckSummary{State: pullrequests.CheckUnknown}
	for _, state := range states {
		switch normalizedPullRequestCheckState(state) {
		case pullrequests.CheckPassing:
			summary.Passing++
		case pullrequests.CheckFailing:
			summary.Failing++
		case pullrequests.CheckPending:
			summary.Pending++
		}
	}
	switch {
	case summary.Failing > 0:
		summary.State = pullrequests.CheckFailing
	case summary.Pending > 0:
		summary.State = pullrequests.CheckPending
	case summary.Passing > 0:
		summary.State = pullrequests.CheckPassing
	}
	return summary
}

func normalizedPullRequestCheckState(value string) pullrequests.CheckState {
	switch normalizedCheckState(value) {
	case workbench.CheckPassing:
		return pullrequests.CheckPassing
	case workbench.CheckFailing:
		return pullrequests.CheckFailing
	case workbench.CheckPending:
		return pullrequests.CheckPending
	default:
		return pullrequests.CheckUnknown
	}
}

func textExcerpt(body string) string {
	fields := strings.Fields(body)
	if len(fields) == 0 {
		return ""
	}
	const maxWords = 60
	if len(fields) > maxWords {
		fields = fields[:maxWords]
		return strings.Join(fields, " ") + "..."
	}
	return strings.Join(fields, " ")
}

type ghWorkflowRun struct {
	DatabaseID      int64     `json:"databaseId"`
	ID              int64     `json:"id"`
	Number          int       `json:"number"`
	Name            string    `json:"name"`
	WorkflowName    string    `json:"workflowName"`
	HeadBranch      string    `json:"headBranch"`
	Event           string    `json:"event"`
	Status          string    `json:"status"`
	Conclusion      string    `json:"conclusion"`
	Actor           ghUser    `json:"actor"`
	TriggeringActor ghUser    `json:"triggering_actor"`
	HeadSHA         string    `json:"headSha"`
	DisplayTitle    string    `json:"displayTitle"`
	URL             string    `json:"url"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type ghWorkflowJob struct {
	DatabaseID  int64            `json:"databaseId"`
	ID          int64            `json:"id"`
	Name        string           `json:"name"`
	Status      string           `json:"status"`
	Conclusion  string           `json:"conclusion"`
	StartedAt   time.Time        `json:"startedAt"`
	CompletedAt time.Time        `json:"completedAt"`
	Steps       []ghWorkflowStep `json:"steps"`
	URL         string           `json:"url"`
}

type ghWorkflowStep struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	Number     int    `json:"number"`
}

type ghAnnotation struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Level     string `json:"annotation_level"`
	Title     string `json:"title"`
	Message   string `json:"message"`
}

type ghActionsRunsResponse struct {
	WorkflowRuns []ghWorkflowRun `json:"workflow_runs"`
}

func workflowRunLimit(opts WorkflowRunListOptions) int {
	if opts.Limit > 0 {
		return opts.Limit
	}
	return runListLimit
}

func workflowRunRef(run ghWorkflowRun) workbench.WorkflowRunRef {
	id := run.DatabaseID
	if id == 0 {
		id = run.ID
	}
	workflowName := run.WorkflowName
	if workflowName == "" {
		workflowName = run.Name
	}
	actor := run.Actor.Login
	if actor == "" {
		actor = run.TriggeringActor.Login
	}
	return workbench.WorkflowRunRef{
		ID:           id,
		RunNumber:    run.Number,
		WorkflowName: workflowName,
		Branch:       run.HeadBranch,
		Event:        strings.ToLower(run.Event),
		Status:       strings.ToLower(run.Status),
		Conclusion:   strings.ToLower(run.Conclusion),
		Actor:        actor,
		HeadSHA:      run.HeadSHA,
		Title:        run.DisplayTitle,
		URL:          run.URL,
		CreatedAt:    run.CreatedAt,
		UpdatedAt:    run.UpdatedAt,
	}
}

func workflowJobRef(job ghWorkflowJob) workbench.WorkflowJobRef {
	id := job.DatabaseID
	if id == 0 {
		id = job.ID
	}
	steps := make([]workbench.WorkflowStepRef, 0, len(job.Steps))
	for _, step := range job.Steps {
		steps = append(steps, workbench.WorkflowStepRef{
			Name:       step.Name,
			Status:     strings.ToLower(step.Status),
			Conclusion: strings.ToLower(step.Conclusion),
			Number:     step.Number,
		})
	}
	return workbench.WorkflowJobRef{
		ID:          id,
		Name:        job.Name,
		Status:      strings.ToLower(job.Status),
		Conclusion:  strings.ToLower(job.Conclusion),
		StartedAt:   job.StartedAt,
		CompletedAt: job.CompletedAt,
		Steps:       steps,
		URL:         job.URL,
	}
}

func workflowRunActors(ctx context.Context, s CLIService, repo string, limit int) map[int64]string {
	actors := map[int64]string{}
	if repo == "" {
		return actors
	}
	output, err := s.runner().Run(ctx, "api", "repos/"+repo+"/actions/runs", "--method", "GET", "-f", fmt.Sprintf("per_page=%d", limit))
	if err != nil {
		return actors
	}
	var payload ghActionsRunsResponse
	if err := json.Unmarshal(output, &payload); err != nil {
		return actors
	}
	for _, run := range payload.WorkflowRuns {
		ref := workflowRunRef(run)
		if ref.ID == 0 || ref.Actor == "" {
			continue
		}
		actors[ref.ID] = ref.Actor
	}
	return actors
}

func workflowRunActor(ctx context.Context, s CLIService, repo string, runID int64) string {
	if repo == "" || runID == 0 {
		return ""
	}
	output, err := s.runner().Run(ctx, "api", fmt.Sprintf("repos/%s/actions/runs/%d", repo, runID))
	if err != nil {
		return ""
	}
	var payload ghWorkflowRun
	if err := json.Unmarshal(output, &payload); err != nil {
		return ""
	}
	return workflowRunRef(payload).Actor
}

func logLines(value string) []string {
	value = strings.TrimRight(value, "\n")
	if value == "" {
		return nil
	}
	return strings.Split(value, "\n")
}

func linkedIssues(closingIssues []workbench.IssueRef, repo string, body string, useBodyFallback bool) []workbench.IssueRef {
	issues := append([]workbench.IssueRef(nil), closingIssues...)
	if !useBodyFallback {
		return issues
	}
	seen := map[string]bool{}
	for _, issue := range issues {
		if issue.Number > 0 {
			seen[issueIdentity(issue, repo)] = true
		}
	}
	for _, issue := range linkedIssuesFromBody(repo, body) {
		identity := issueIdentity(issue, repo)
		if issue.Number == 0 || seen[identity] {
			continue
		}
		issues = append(issues, issue)
		seen[identity] = true
	}
	return issues
}

func issuesFromGraphQL(payload []ghIssue, fallbackRepo string) []workbench.IssueRef {
	issues := make([]workbench.IssueRef, 0, len(payload))
	for _, issue := range payload {
		repository := issue.Repository.NameWithOwner
		if repository == "" {
			repository = fallbackRepo
		}
		issues = append(issues, workbench.IssueRef{
			Number:     issue.Number,
			Repository: repository,
			Title:      issue.Title,
			State:      strings.ToLower(issue.State),
			URL:        issue.URL,
			Certain:    true,
		})
	}
	return issues
}

func issueIdentity(issue workbench.IssueRef, fallbackRepo string) string {
	repository := issue.Repository
	if repository == "" {
		repository = fallbackRepo
	}
	return strings.ToLower(repository) + "#" + strconv.Itoa(issue.Number)
}

func linkedIssuesFromBody(repo string, body string) []workbench.IssueRef {
	seen := map[int]bool{}
	issues := []workbench.IssueRef{}
	for _, match := range closingIssueReferencePattern.FindAllStringSubmatch(body, -1) {
		if len(match) < 8 {
			continue
		}
		numberText := match[1]
		referenceRepo := repo
		switch {
		case match[4] != "":
			referenceRepo = match[2] + "/" + match[3]
			numberText = match[4]
		case match[7] != "":
			referenceRepo = match[5] + "/" + match[6]
			numberText = match[7]
		}
		if !strings.EqualFold(referenceRepo, repo) {
			continue
		}
		number, err := strconv.Atoi(numberText)
		if err != nil || seen[number] {
			continue
		}
		issues = append(issues, workbench.IssueRef{
			Number:     number,
			Repository: repo,
			Certain:    true,
		})
		seen[number] = true
	}
	return issues
}

func labelNames(labels []ghLabel) []string {
	names := make([]string, 0, len(labels))
	for _, label := range labels {
		if label.Name != "" {
			names = append(names, label.Name)
		}
	}
	return names
}

func userLogins(users []ghUser) []string {
	logins := make([]string, 0, len(users))
	for _, user := range users {
		if user.Login != "" {
			logins = append(logins, user.Login)
		}
	}
	return logins
}

func milestoneTitle(milestone *struct {
	Title string `json:"title"`
}) string {
	if milestone == nil {
		return ""
	}
	return milestone.Title
}

func reviewRequests(payload []ghReviewRequest) []workbench.ReviewRequestRef {
	requests := make([]workbench.ReviewRequestRef, 0, len(payload))
	for _, request := range payload {
		requests = append(requests, workbench.ReviewRequestRef{
			Kind:  request.TypeName,
			Login: request.Login,
			Name:  reviewRequestName(request),
			Slug:  request.Slug,
		})
	}
	return requests
}

func reviewRequestName(request ghReviewRequest) string {
	switch {
	case request.Name != "":
		return request.Name
	case request.Slug != "":
		return request.Slug
	default:
		return request.Login
	}
}

func latestReviews(payload []ghPullReviewItem) []workbench.PullRequestReviewRef {
	reviews := make([]workbench.PullRequestReviewRef, 0, len(payload))
	for _, review := range payload {
		reviews = append(reviews, workbench.PullRequestReviewRef{
			AuthorLogin: review.Author.Login,
			State:       reviewState(review.State),
		})
	}
	return reviews
}

func nonEmptyLines(value string) []string {
	lines := strings.Split(value, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func isPendingChecksExit(args []string, err error) bool {
	if len(args) < 2 || args[0] != "pr" || args[1] != "checks" {
		return false
	}
	code, ok := exitCode(err)
	return ok && code == 8
}

type exitCoder interface {
	ExitCode() int
}

func exitCode(err error) (int, bool) {
	var exitErr exitCoder
	if !errors.As(err, &exitErr) {
		return 0, false
	}
	return exitErr.ExitCode(), true
}

func reviewState(value string) string {
	value = strings.ToLower(strings.ReplaceAll(value, "_", " "))
	return strings.TrimSpace(value)
}

func summarizeCheckStates(states []string) workbench.CheckSummary {
	summary := workbench.CheckSummary{State: workbench.CheckUnknown}
	for _, state := range states {
		switch normalizedCheckState(state) {
		case workbench.CheckPassing:
			summary.Passing++
		case workbench.CheckFailing:
			summary.Failing++
		case workbench.CheckPending:
			summary.Pending++
		}
	}
	switch {
	case summary.Failing > 0:
		summary.State = workbench.CheckFailing
	case summary.Pending > 0:
		summary.State = workbench.CheckPending
	case summary.Passing > 0:
		summary.State = workbench.CheckPassing
	}
	return summary
}

func normalizedCheckState(value string) workbench.CheckState {
	value = strings.ToLower(value)
	value = strings.ReplaceAll(value, "_", " ")
	value = strings.ReplaceAll(value, "-", " ")
	switch {
	case strings.Contains(value, "fail"), strings.Contains(value, "error"), strings.Contains(value, "cancel"), strings.Contains(value, "timed out"), strings.Contains(value, "timeout"), strings.Contains(value, "action required"):
		return workbench.CheckFailing
	case value == "requested", value == "expected", strings.Contains(value, "pending"), strings.Contains(value, "queued"), strings.Contains(value, "progress"), strings.Contains(value, "waiting"):
		return workbench.CheckPending
	case strings.Contains(value, "pass"), strings.Contains(value, "success"):
		return workbench.CheckPassing
	default:
		return workbench.CheckUnknown
	}
}

func classifyError(op string, output []byte, err error) Error {
	text := strings.ToLower(string(output))
	kind := ErrorCommand
	switch {
	case strings.Contains(text, "auth"), strings.Contains(text, "login"), strings.Contains(text, "credential"):
		kind = ErrorAuth
	case strings.Contains(text, "network"), strings.Contains(text, "resolve"), strings.Contains(text, "connection"), strings.Contains(text, "timeout"), strings.Contains(text, "tls"):
		kind = ErrorNetwork
	}
	return Error{Op: op, Kind: kind, Output: string(output), Err: err}
}
