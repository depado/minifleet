package github

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/depado/minifleet/internal/provider"
	gogithub "github.com/google/go-github/v89/github"
)

func (c *Client) ListPullRequests(ctx context.Context, owner, repoName string, opts provider.ListPROptions) ([]*provider.PullRequest, error) {
	ctx = withRateLimitRetry(ctx)

	listOpts := &gogithub.PullRequestListOptions{
		State:       opts.State,
		Sort:        opts.Sort,
		ListOptions: gogithub.ListOptions{PerPage: 100},
	}

	var ghPRs []*gogithub.PullRequest
	for {
		page, resp, err := c.client.PullRequests.List(ctx, owner, repoName, listOpts)
		if err != nil {
			return nil, fmt.Errorf("list PRs for %s/%s: %w", owner, repoName, err)
		}
		ghPRs = append(ghPRs, page...)
		if resp.NextPage == 0 {
			break
		}
		if opts.Limit > 0 && len(ghPRs) >= opts.Limit {
			break
		}
		listOpts.Page = resp.NextPage
	}
	if opts.Limit > 0 && len(ghPRs) > opts.Limit {
		ghPRs = ghPRs[:opts.Limit]
	}

	prs := make([]*provider.PullRequest, 0, len(ghPRs))
	for _, pr := range ghPRs {
		if opts.Author != "" && pr.GetUser().GetLogin() != opts.Author {
			continue
		}

		ciStatus := c.getCIStatus(ctx, owner, repoName, pr.GetHead().GetSHA())
		reviewStatus := c.getReviewStatus(ctx, owner, repoName, pr.GetNumber())

		prs = append(prs, &provider.PullRequest{
			Number:       pr.GetNumber(),
			Title:        pr.GetTitle(),
			State:        pr.GetState(),
			Author:       pr.GetUser().GetLogin(),
			Draft:        pr.GetDraft(),
			UpdatedAt:    pr.GetUpdatedAt().Time,
			CIStatus:     ciStatus,
			ReviewStatus: reviewStatus,
		})
	}

	return prs, nil
}

func (c *Client) getCIStatus(ctx context.Context, owner, repo, sha string) provider.CIStatus {
	if sha == "" {
		return provider.CIUnknown
	}

	combined, _, err := c.client.Repositories.GetCombinedStatus(ctx, owner, repo, sha, nil)
	if err != nil {
		slog.Debug("ci status fetch failed", "repo", owner+"/"+repo, "sha", sha, "error", err)
		return provider.CIUnknown
	}

	switch combined.GetState() {
	case "success":
		return provider.CISuccess
	case "pending":
		return provider.CIPending
	case "failure":
		return provider.CIFailure
	case "error":
		return provider.CIError
	default:
		return provider.CIUnknown
	}
}

func (c *Client) getReviewStatus(ctx context.Context, owner, repo string, number int) provider.ReviewStatus {
	reviews, _, err := c.client.PullRequests.ListReviews(ctx, owner, repo, number, nil)
	if err != nil {
		slog.Debug("review fetch failed", "repo", owner+"/"+repo, "pr", number, "error", err)
		return provider.ReviewPending
	}
	if len(reviews) == 0 {
		return provider.ReviewPending
	}

	latestByUser := make(map[string]*gogithub.PullRequestReview)
	for _, r := range reviews {
		login := r.GetUser().GetLogin()
		existing, ok := latestByUser[login]
		if !ok || r.GetSubmittedAt().After(existing.GetSubmittedAt().Time) {
			latestByUser[login] = r
		}
	}

	hasApproval, hasChanges, hasComment := false, false, false
	for _, r := range latestByUser {
		switch r.GetState() {
		case "APPROVED":
			hasApproval = true
		case "CHANGES_REQUESTED":
			hasChanges = true
		case "COMMENTED":
			hasComment = true
		}
	}

	if hasChanges {
		return provider.ReviewChangesRequested
	}
	if hasApproval {
		return provider.ReviewApproved
	}
	if hasComment {
		return provider.ReviewCommented
	}
	return provider.ReviewPending
}
