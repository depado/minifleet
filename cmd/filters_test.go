package cmd

import (
	"testing"

	"github.com/depado/minifleet/internal/manifest"
	"github.com/depado/minifleet/internal/provider"
)

func TestFiltersApply(t *testing.T) {
	repos := []*provider.Repo{
		{Name: "svc-api", FullName: "o/svc-api", Language: "Go", Archived: false, Fork: false, Visibility: "private", Topics: []string{"go", "service"}},
		{Name: "old-thing", FullName: "o/old-thing", Language: "Python", Archived: true, Fork: false, Visibility: "public", Topics: []string{}},
		{Name: "forked-lib", FullName: "o/forked-lib", Language: "Go", Archived: false, Fork: true, Visibility: "public", Topics: []string{"go"}},
	}
	mf := &manifest.FleetManifest{
		Version: "1",
		Owner:   "o",
		Groups: map[string][]string{
			"backend": {"o/svc-api"},
		},
		Repos: []manifest.ManifestRepo{
			{FullName: "o/svc-api", Labels: map[string]string{"tier": "1"}, Archived: false, Fork: false, Topics: []string{"go", "service"}, Language: "Go"},
			{FullName: "o/old-thing", Labels: map[string]string{"tier": "3"}, Archived: true, Fork: false, Language: "Python"},
			{FullName: "o/forked-lib", Labels: nil, Archived: false, Fork: true, Language: "Go", Topics: []string{"go"}},
		},
	}

	tests := []struct {
		name string
		f    Filters
		want []string // repo names expected
	}{
		{"defaults exclude archived+forks", Filters{}, []string{"svc-api"}},
		{"include-archived", Filters{IncludeArchived: true}, []string{"svc-api", "old-thing"}},
		{"include-forks", Filters{IncludeForks: true}, []string{"svc-api", "forked-lib"}},
		{"topic go", Filters{Topics: []string{"go"}, IncludeForks: true, IncludeArchived: true}, []string{"svc-api", "forked-lib"}},
		{"language python archived", Filters{Language: "Python", IncludeArchived: true}, []string{"old-thing"}},
		{"label tier=1", Filters{Labels: []string{"tier=1"}}, []string{"svc-api"}},
		{"label tier (any)", Filters{Labels: []string{"tier"}, IncludeArchived: true}, []string{"svc-api", "old-thing"}},
		{"group backend", Filters{Group: "backend"}, []string{"svc-api"}},
		{"group missing errors", Filters{Group: "nope"}, nil},
		{"target regex", Filters{Target: "^svc", IncludeArchived: true, IncludeForks: true}, []string{"svc-api"}},
		{"visibility private", Filters{Visibility: "private", IncludeArchived: true, IncludeForks: true}, []string{"svc-api"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.f.Apply(repos, mf)
			if err != nil {
				if tt.want != nil {
					t.Fatalf("unexpected err: %v", err)
				}
				return
			}
			names := make([]string, 0, len(got))
			for _, r := range got {
				names = append(names, r.Name)
			}
			if !equalSets(names, tt.want) {
				t.Errorf("got %v, want %v", names, tt.want)
			}
		})
	}
}

func equalSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	m := make(map[string]int, len(a))
	for _, x := range a {
		m[x]++
	}
	for _, x := range b {
		m[x]--
		if m[x] < 0 {
			return false
		}
	}
	return true
}
