package testing

import (
	"context"
	"fmt"

	"github.com/depado/minifleet/internal/provider"
)

type FakeProvider struct {
	HostVal    string
	CloneURLFn func(protocol, fullName string) string
	Repos      []*provider.Repo
	ReposErr   error
	PRs        []*provider.PullRequest
	PRsErr     error
	IsOrg      bool
	DetectErr  error
}

func (f *FakeProvider) Host() string {
	if f.HostVal == "" {
		return "github.com"
	}
	return f.HostVal
}

func (f *FakeProvider) CloneURL(protocol, fullName string) string {
	if f.CloneURLFn != nil {
		return f.CloneURLFn(protocol, fullName)
	}
	if protocol == "https" {
		return fmt.Sprintf("https://%s/%s.git", f.Host(), fullName)
	}
	return fmt.Sprintf("git@%s:%s.git", f.Host(), fullName)
}

func (f *FakeProvider) ListRepos(_ context.Context, _ string, _ provider.ListOptions) ([]*provider.Repo, error) {
	if f.ReposErr != nil {
		return nil, f.ReposErr
	}
	return f.Repos, nil
}

func (f *FakeProvider) ListPullRequests(_ context.Context, _, _ string, _ provider.ListPROptions) ([]*provider.PullRequest, error) {
	if f.PRsErr != nil {
		return nil, f.PRsErr
	}
	return f.PRs, nil
}

func (f *FakeProvider) DetectOwner(_ context.Context, _ string) (bool, error) {
	if f.DetectErr != nil {
		return false, f.DetectErr
	}
	return f.IsOrg, nil
}
