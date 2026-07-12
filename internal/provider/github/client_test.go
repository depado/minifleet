package github

import "testing"

func TestCloneURL(t *testing.T) {
	tests := []struct {
		name, host, protocol, full, want string
	}{
		{"ssh github.com", "github.com", "ssh", "depado/minifleet", "git@github.com:depado/minifleet.git"},
		{"https github.com", "github.com", "https", "depado/minifleet", "https://github.com/depado/minifleet.git"},
		{"ssh ghe", "github.enterprise.com", "ssh", "org/svc", "git@github.enterprise.com:org/svc.git"},
		{"https ghe", "ghe.internal.io", "https", "team/pkg", "https://ghe.internal.io/team/pkg.git"},
		{"unknown protocol defaults to ssh", "github.com", "weird", "a/b", "git@github.com:a/b.git"},
		{"empty protocol defaults to ssh", "github.com", "", "a/b", "git@github.com:a/b.git"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{host: tt.host}
			if got := c.CloneURL(tt.protocol, tt.full); got != tt.want {
				t.Errorf("CloneURL(%q,%q) = %q, want %q", tt.protocol, tt.full, got, tt.want)
			}
		})
	}
}