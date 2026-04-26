package workbench

import "fmt"

type RepoRef struct {
	Owner string
	Name  string
}

func (r RepoRef) FullName() string {
	switch {
	case r.Owner != "" && r.Name != "":
		return r.Owner + "/" + r.Name
	case r.Name != "":
		return r.Name
	default:
		return "unknown repo"
	}
}

type BranchRef struct {
	Name       string
	Base       string
	RemoteOnly bool
}

type WorktreeRef struct {
	Path string
}

type IssueRef struct {
	Number  int
	Title   string
	State   string
	URL     string
	Certain bool
}

func (i IssueRef) Label() string {
	if i.Number == 0 {
		return "issue"
	}
	if i.Title == "" {
		return fmt.Sprintf("#%d", i.Number)
	}
	return fmt.Sprintf("#%d %s", i.Number, i.Title)
}

type PullRequestRef struct {
	Number      int
	Title       string
	State       string
	URL         string
	ReviewState string
}

func (p PullRequestRef) Label() string {
	label := p.NumberLabel()
	if p.Title == "" {
		return label
	}
	return fmt.Sprintf("%s %s", label, p.Title)
}

func (p PullRequestRef) NumberLabel() string {
	if p.Number == 0 {
		return "PR"
	}
	return fmt.Sprintf("PR #%d", p.Number)
}

type CheckState string

const (
	CheckUnknown CheckState = "unknown"
	CheckPassing CheckState = "passing"
	CheckFailing CheckState = "failing"
	CheckPending CheckState = "pending"
)

type CheckSummary struct {
	State   CheckState
	Passing int
	Failing int
	Pending int
}

func (c CheckSummary) Label() string {
	switch c.State {
	case CheckPassing:
		return "checks passing"
	case CheckFailing:
		if c.Failing > 0 {
			return fmt.Sprintf("checks failing (%d)", c.Failing)
		}
		return "checks failing"
	case CheckPending:
		if c.Pending > 0 {
			return fmt.Sprintf("checks pending (%d)", c.Pending)
		}
		return "checks pending"
	default:
		return "checks unknown"
	}
}

type LocalState string

const (
	LocalUnknown  LocalState = "unknown"
	LocalClean    LocalState = "clean"
	LocalDirty    LocalState = "dirty"
	LocalDetached LocalState = "detached"
	LocalMissing  LocalState = "missing"
)

type LocalStatus struct {
	State   LocalState
	Summary string
}

func (s LocalStatus) Label() string {
	if s.Summary != "" {
		return fmt.Sprintf("%s, %s", s.StateLabel(), s.Summary)
	}
	return s.StateLabel()
}

func (s LocalStatus) StateLabel() string {
	if s.State == "" {
		return string(LocalUnknown)
	}
	return string(s.State)
}

type CommitRef struct {
	ShortSHA string
	Subject  string
}

type WorkItem struct {
	ID          string
	Repo        RepoRef
	Branch      *BranchRef
	Worktree    *WorktreeRef
	Issue       *IssueRef
	PullRequest *PullRequestRef
	Checks      CheckSummary
	Local       *LocalStatus
	Commits     []CommitRef
}

func (w WorkItem) Title() string {
	switch {
	case w.Branch != nil && w.Branch.Name != "":
		return w.Branch.Name
	case w.PullRequest != nil:
		return w.PullRequest.Label()
	case w.Issue != nil:
		return w.Issue.Label()
	default:
		return "untracked work"
	}
}

func (w WorkItem) Location() string {
	switch {
	case w.Worktree != nil && w.Worktree.Path != "":
		return w.Worktree.Path
	case w.Branch != nil && w.Branch.RemoteOnly:
		return "remote only"
	case w.Issue != nil && w.Branch == nil && w.PullRequest == nil:
		return "issue only"
	default:
		return w.Repo.FullName()
	}
}

func (w WorkItem) LocalLabel() string {
	if w.Local == nil {
		return string(LocalUnknown)
	}
	return w.Local.StateLabel()
}

func (w WorkItem) PullRequestLabel() string {
	if w.PullRequest == nil {
		return "no PR"
	}
	label := w.PullRequest.NumberLabel()
	if w.PullRequest.State == "" {
		return label
	}
	return fmt.Sprintf("%s %s", label, w.PullRequest.State)
}

func (w WorkItem) IssueLabel() string {
	if w.Issue == nil {
		return "no issue"
	}
	label := w.Issue.Label()
	if !w.Issue.Certain {
		return fmt.Sprintf("%s (uncertain)", label)
	}
	return label
}
