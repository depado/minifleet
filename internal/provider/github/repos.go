package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/depado/minifleet/internal/provider"
	gogithub "github.com/google/go-github/v89/github"
)

func (c *Client) ListRepos(ctx context.Context, owner string, opts provider.ListOptions) ([]*provider.Repo, error) {
	ctx = withRateLimitRetry(ctx)

	var allRepos []*gogithub.Repository
	var err error

	if opts.IsOrg {
		allRepos, err = listByOrg(ctx, c, owner, opts.Visibility)
	} else if isCurrentUser(ctx, c, owner) {
		allRepos, err = listAuthenticated(ctx, c)
	} else {
		allRepos, err = listByUser(ctx, c, owner)
	}
	if err != nil {
		return nil, err
	}

	repos := make([]*provider.Repo, 0, len(allRepos))
	for _, r := range allRepos {
		repos = append(repos, convertRepo(r))
	}
	return repos, nil
}

func isCurrentUser(ctx context.Context, c *Client, owner string) bool {
	user, _, err := c.client.Users.Get(ctx, "")
	if err != nil {
		return false
	}
	return strings.EqualFold(user.GetLogin(), owner)
}

func listAuthenticated(ctx context.Context, c *Client) ([]*gogithub.Repository, error) {
	opts := &gogithub.RepositoryListByAuthenticatedUserOptions{
		Visibility:  "all",
		Affiliation: "owner",
		ListOptions: gogithub.ListOptions{PerPage: 100},
	}
	var all []*gogithub.Repository
	for {
		page, resp, err := c.client.Repositories.ListByAuthenticatedUser(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("list repos: %w", err)
		}
		all = append(all, page...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return all, nil
}

func listByOrg(ctx context.Context, c *Client, org, visibility string) ([]*gogithub.Repository, error) {
	opts := &gogithub.RepositoryListByOrgOptions{
		Type:        visibility,
		ListOptions: gogithub.ListOptions{PerPage: 100},
	}
	var all []*gogithub.Repository
	for {
		page, resp, err := c.client.Repositories.ListByOrg(ctx, org, opts)
		if err != nil {
			return nil, fmt.Errorf("list org repos: %w", err)
		}
		all = append(all, page...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return all, nil
}

func listByUser(ctx context.Context, c *Client, user string) ([]*gogithub.Repository, error) {
	opts := &gogithub.RepositoryListByUserOptions{
		Type:        "all",
		ListOptions: gogithub.ListOptions{PerPage: 100},
	}
	var all []*gogithub.Repository
	for {
		page, resp, err := c.client.Repositories.ListByUser(ctx, user, opts)
		if err != nil {
			return nil, fmt.Errorf("list user repos: %w", err)
		}
		all = append(all, page...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return all, nil
}

// DetectOwner reports whether the given owner name is an organization (true)
// or a user (false). Uses a single lightweight GET to /orgs/{owner}; any
// non-404 error is returned as-is.
func (c *Client) DetectOwner(ctx context.Context, owner string) (bool, error) {
	ctx = withRateLimitRetry(ctx)

	_, resp, err := c.client.Organizations.Get(ctx, owner)
	if err == nil {
		return true, nil
	}
	if resp != nil && resp.StatusCode == 404 {
		return false, nil
	}
	return false, fmt.Errorf("detect owner %q: %w", owner, err)
}

func convertRepo(r *gogithub.Repository) *provider.Repo {
	return &provider.Repo{
		Name:          r.GetName(),
		FullName:      r.GetFullName(),
		Description:   r.GetDescription(),
		CloneURL:      r.GetCloneURL(),
		SSHURL:        r.GetSSHURL(),
		DefaultBranch: r.GetDefaultBranch(),
		Language:      r.GetLanguage(),
		Archived:      r.GetArchived(),
		Fork:          r.GetFork(),
		Private:       r.GetPrivate(),
		Visibility:    r.GetVisibility(),
		Topics:        r.Topics,
		PushedAt:      r.GetPushedAt().Time,
		UpdatedAt:     r.GetUpdatedAt().Time,
		CreatedAt:     r.GetCreatedAt().Time,
	}
}
