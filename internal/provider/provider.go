package provider

import (
	"context"
	"time"
)

type Repo struct {
	Name          string
	FullName      string
	Description   string
	CloneURL      string
	SSHURL        string
	DefaultBranch string
	Language      string
	Archived      bool
	Fork          bool
	Private       bool
	Visibility    string
	Topics        []string
	PushedAt      time.Time
	UpdatedAt     time.Time
	CreatedAt     time.Time
}

type PullRequest struct {
	Number       int
	Title        string
	State        string
	Author       string
	Draft        bool
	URL          string
	BaseBranch   string
	HeadBranch   string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	CIStatus     CIStatus
	ReviewStatus ReviewStatus
}

type CIStatus string

const (
	CIUnknown  CIStatus = "unknown"
	CIPending  CIStatus = "pending"
	CISuccess  CIStatus = "success"
	CIFailure  CIStatus = "failure"
	CIError    CIStatus = "error"
)

type ReviewStatus string

const (
	ReviewUnknown          ReviewStatus = "unknown"
	ReviewApproved         ReviewStatus = "approved"
	ReviewChangesRequested ReviewStatus = "changes_requested"
	ReviewPending          ReviewStatus = "pending"
	ReviewDismissed        ReviewStatus = "dismissed"
	ReviewCommented        ReviewStatus = "commented"
)

type ListOptions struct {
	IncludeArchived bool
	IncludeForks    bool
	Topics          []string
	Visibility      string
	IsOrg           bool // true = use org API, false = use user API
}

type ListPROptions struct {
	State  string
	Author string
	Sort   string
	Limit  int
}

type Provider interface {
	// Host returns the canonical hostname for this provider's API and clone URLs
	// (e.g. "github.com" or "github.enterprise.example.com"). Used by callers to
	// build clone URLs and lay out the local directory tree.
	Host() string

	// CloneURL returns the git clone URL for a repo identified by its
	// "owner/repo" full name, using the given protocol ("ssh" or "https").
	// Each provider formats its URLs per-platform convention.
	CloneURL(protocol, fullName string) string

	ListRepos(ctx context.Context, owner string, opts ListOptions) ([]*Repo, error)
	ListPullRequests(ctx context.Context, owner, repo string, opts ListPROptions) ([]*PullRequest, error)
	DetectOwner(ctx context.Context, owner string) (IsOrg bool, err error)
}
