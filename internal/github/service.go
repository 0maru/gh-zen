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

// GHRunner executes the gh binary.
type GHRunner struct{}

// ErrorKind classifies gh failures into user-actionable categories.
type ErrorKind string

const (
	ErrorAuth    ErrorKind = "auth"
	ErrorNetwork ErrorKind = "network"
	ErrorCommand ErrorKind = "command"

	issueListFields = "number,title,state,url,body,labels,assignees,milestone,updatedAt"
	listLimit       = "1000"
	prListFields    = "number,title,state,url,headRefName,headRepositoryOwner,baseRefName,isDraft,updatedAt,author,reviewRequests,latestReviews,reviewDecision,body"

	repositoryPullRequestsQuery = `
query($owner:String!, $name:String!, $after:String) {
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
              statusCheckRollup {
                contexts(first:100) {
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
            statusCheckRollup {
              contexts(first:100) {
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

	pullRequestClosingIssuesQuery = `
query($owner:String!, $name:String!, $after:String) {
  repository(owner:$owner, name:$name) {
    pullRequests(first:100, after:$after, states:[OPEN, CLOSED, MERGED], orderBy:{field:UPDATED_AT, direction:DESC}) {
      nodes {
        number
        closingIssuesReferences(first:20) {
          nodes {
            number
            title
            state
            url
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
)

var (
	closingIssueTextPattern = regexp.MustCompile(`(?i)\b(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?)\b[^\n\r.]*`)
	issueNumberPattern      = regexp.MustCompile(`#(\d+)`)
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
	closingIssuesByPR, _ := s.pullRequestClosingIssues(ctx, repo)

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
			LinkedIssues:   linkedIssues(closingIssuesByPR[pr.Number], pr.Body),
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
		for _, node := range payload.Data.Repository.PullRequests.Nodes {
			prs = append(prs, pullRequestFromGraphQL(node))
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

// Issues loads issue summaries through gh.
func (s CLIService) Issues(ctx context.Context, repo string) ([]workbench.IssueRef, error) {
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
		UpdatedAt string `json:"updatedAt"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		return nil, fmt.Errorf("parse gh issue list output: %w", err)
	}
	issues := make([]workbench.IssueRef, 0, len(payload))
	for _, issue := range payload {
		issues = append(issues, workbench.IssueRef{
			Number:    issue.Number,
			Title:     issue.Title,
			State:     strings.ToLower(issue.State),
			URL:       issue.URL,
			Body:      issue.Body,
			Labels:    labelNames(issue.Labels),
			Assignees: userLogins(issue.Assignees),
			Milestone: milestoneTitle(issue.Milestone),
			UpdatedAt: issue.UpdatedAt,
			Certain:   true,
		})
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

func (s CLIService) pullRequestClosingIssues(ctx context.Context, repo string) (map[int][]workbench.IssueRef, error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok || owner == "" || name == "" {
		return nil, nil
	}

	issuesByPR := map[int][]workbench.IssueRef{}
	after := ""
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
								Nodes []ghIssue `json:"nodes"`
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
			if pr.Number > 0 {
				issuesByPR[pr.Number] = issuesFromGraphQL(pr.ClosingIssues.Nodes)
			}
		}
		if !payload.Data.Repository.PullRequests.PageInfo.HasNextPage {
			return issuesByPR, nil
		}
		after = payload.Data.Repository.PullRequests.PageInfo.EndCursor
		if after == "" {
			return issuesByPR, nil
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
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
	URL    string `json:"url"`
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
				StatusCheckRollup *struct {
					Contexts struct {
						Nodes []ghCheckContext `json:"nodes"`
					} `json:"contexts"`
				} `json:"statusCheckRollup"`
			} `json:"commit"`
		} `json:"nodes"`
	} `json:"commits"`
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
	for _, issue := range linkedIssuesFromBody(body) {
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
		StatusCheckRollup *struct {
			Contexts struct {
				Nodes []ghCheckContext `json:"nodes"`
			} `json:"contexts"`
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

func linkedIssues(closingIssues []workbench.IssueRef, body string) []workbench.IssueRef {
	issues := append([]workbench.IssueRef(nil), closingIssues...)
	seen := map[int]bool{}
	for _, issue := range issues {
		if issue.Number > 0 {
			seen[issue.Number] = true
		}
	}
	for _, issue := range linkedIssuesFromBody(body) {
		if issue.Number == 0 || seen[issue.Number] {
			continue
		}
		issues = append(issues, issue)
		seen[issue.Number] = true
	}
	return issues
}

func issuesFromGraphQL(payload []ghIssue) []workbench.IssueRef {
	issues := make([]workbench.IssueRef, 0, len(payload))
	for _, issue := range payload {
		issues = append(issues, workbench.IssueRef{
			Number:  issue.Number,
			Title:   issue.Title,
			State:   strings.ToLower(issue.State),
			URL:     issue.URL,
			Certain: true,
		})
	}
	return issues
}

func linkedIssuesFromBody(body string) []workbench.IssueRef {
	seen := map[int]bool{}
	issues := []workbench.IssueRef{}
	for _, text := range closingIssueTextPattern.FindAllString(body, -1) {
		for _, match := range issueNumberPattern.FindAllStringSubmatch(text, -1) {
			if len(match) < 2 {
				continue
			}
			number, err := strconv.Atoi(match[1])
			if err != nil || seen[number] {
				continue
			}
			issues = append(issues, workbench.IssueRef{
				Number:  number,
				Certain: true,
			})
			seen[number] = true
		}
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
	case strings.Contains(value, "pending"), strings.Contains(value, "queued"), strings.Contains(value, "progress"), strings.Contains(value, "waiting"):
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
