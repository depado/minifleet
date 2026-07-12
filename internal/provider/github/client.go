package github

import (
	"fmt"
	"log/slog"

	"github.com/depado/minifleet/internal/provider"
	gogithub "github.com/google/go-github/v89/github"
)

type Client struct {
	client *gogithub.Client
	host   string
}

// New builds a GitHub provider. host defaults to "github.com" when empty.
// A non-default host is treated as GitHub Enterprise: the REST base URL is
// set to https://<host>/api/v3/ and clone URLs use <host> directly.
func New(token, host string) provider.Provider {
	if host == "" {
		host = "github.com"
	}

	var opts []gogithub.ClientOptionsFunc
	if token != "" {
		opts = append(opts, gogithub.WithAuthToken(token))
	}
	if host != "github.com" {
		baseURL := "https://" + host + "/api/v3/"
		uploadURL := "https://" + host + "/uploads/"
		opts = append(opts, gogithub.WithEnterpriseURLs(baseURL, uploadURL))
	}

	c, err := gogithub.NewClient(opts...)
	if err != nil {
		slog.Error("unable to create GitHub client", "error", err, "host", host)
		return nil
	}
	return &Client{client: c, host: host}
}

// Host returns the clone/API host for this provider.
func (c *Client) Host() string { return c.host }

// CloneURL returns the git clone URL for a repo identified by its full name
// ("owner/repo"), using "ssh" or "https". GitHub's format:
//
//	ssh:   git@<host>:owner/repo.git
//	https: https://<host>/owner/repo.git
func (c *Client) CloneURL(protocol, fullName string) string {
	if protocol == "https" {
		return fmt.Sprintf("https://%s/%s.git", c.host, fullName)
	}
	return fmt.Sprintf("git@%s:%s.git", c.host, fullName)
}
