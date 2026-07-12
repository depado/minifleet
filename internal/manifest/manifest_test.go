package manifest

import (
	"reflect"
	"testing"
	"time"

	"github.com/depado/minifleet/internal/provider"
)

func TestMerge(t *testing.T) {
	t.Run("preserves user fields overwrites API fields", func(t *testing.T) {
		existing := &FleetManifest{
			Version: "1",
			Owner:   "o",
			Repos: []ManifestRepo{
				{
					FullName: "o/svc",
					Labels:   map[string]string{"tier": "1"},
					Protocol: "ssh",
					Ignored:  true,
					Language: "Python",
					Archived: true,
				},
			},
		}
		api := []*provider.Repo{
			{Name: "svc", FullName: "o/svc", Language: "Go", Archived: false, Topics: []string{"go"}},
		}
		merged := Merge(existing, "o", api)
		if len(merged.Repos) != 1 {
			t.Fatalf("want 1 repo, got %d", len(merged.Repos))
		}
		r := merged.Repos[0]
		if !reflect.DeepEqual(r.Labels, map[string]string{"tier": "1"}) {
			t.Errorf("Labels: got %v, want {tier:1}", r.Labels)
		}
		if r.Protocol != "ssh" {
			t.Errorf("Protocol: got %q, want ssh", r.Protocol)
		}
		if !r.Ignored {
			t.Errorf("Ignored: got false, want true")
		}
		if r.Language != "Go" {
			t.Errorf("Language: got %q, want Go", r.Language)
		}
		if r.Archived {
			t.Errorf("Archived: got true, want false")
		}
		if !reflect.DeepEqual(r.Topics, []string{"go"}) {
			t.Errorf("Topics: got %v, want [go]", r.Topics)
		}
	})

	t.Run("nil existing", func(t *testing.T) {
		api := []*provider.Repo{
			{Name: "svc", FullName: "o/svc", Language: "Go"},
		}
		merged := Merge(nil, "o", api)
		if merged.Version != "1" {
			t.Errorf("Version: got %q, want 1", merged.Version)
		}
		if merged.Owner != "o" {
			t.Errorf("Owner: got %q, want o", merged.Owner)
		}
		if len(merged.Repos) != 1 {
			t.Fatalf("want 1 repo, got %d", len(merged.Repos))
		}
		r := merged.Repos[0]
		if r.Labels != nil {
			t.Errorf("Labels: got %v, want nil", r.Labels)
		}
		if r.Protocol != "" {
			t.Errorf("Protocol: got %q, want empty", r.Protocol)
		}
		if r.Ignored {
			t.Errorf("Ignored: got true, want false")
		}
	})

	t.Run("drops repos absent from API", func(t *testing.T) {
		existing := &FleetManifest{
			Version: "1",
			Owner:   "o",
			Repos: []ManifestRepo{
				{FullName: "o/a"},
				{FullName: "o/b"},
			},
		}
		api := []*provider.Repo{
			{Name: "a", FullName: "o/a"},
		}
		merged := Merge(existing, "o", api)
		if len(merged.Repos) != 1 {
			t.Fatalf("want 1 repo, got %d", len(merged.Repos))
		}
		if merged.Repos[0].FullName != "o/a" {
			t.Errorf("got %q, want o/a", merged.Repos[0].FullName)
		}
	})

	t.Run("brand-new repo", func(t *testing.T) {
		existing := &FleetManifest{
			Version: "1",
			Owner:   "o",
			Repos: []ManifestRepo{
				{FullName: "o/a", Language: "Go"},
			},
		}
		api := []*provider.Repo{
			{Name: "a", FullName: "o/a", Language: "Go"},
			{Name: "b", FullName: "o/b", Language: "Python", Topics: []string{"ml"}},
		}
		merged := Merge(existing, "o", api)
		if len(merged.Repos) != 2 {
			t.Fatalf("want 2 repos, got %d", len(merged.Repos))
		}
		var newRepo ManifestRepo
		for _, r := range merged.Repos {
			if r.FullName == "o/b" {
				newRepo = r
				break
			}
		}
		if newRepo.FullName != "o/b" {
			t.Fatal("o/b not found in merged repos")
		}
		if newRepo.Language != "Python" {
			t.Errorf("Language: got %q, want Python", newRepo.Language)
		}
		if !reflect.DeepEqual(newRepo.Topics, []string{"ml"}) {
			t.Errorf("Topics: got %v, want [ml]", newRepo.Topics)
		}
		if newRepo.Labels != nil {
			t.Errorf("Labels: got %v, want nil", newRepo.Labels)
		}
		if newRepo.Protocol != "" {
			t.Errorf("Protocol: got %q, want empty", newRepo.Protocol)
		}
		if newRepo.Ignored {
			t.Errorf("Ignored: got true, want false")
		}
	})
}

func TestIndex(t *testing.T) {
	t.Run("maps and mutates", func(t *testing.T) {
		mf := &FleetManifest{
			Version: "1",
			Owner:   "o",
			Repos: []ManifestRepo{
				{FullName: "o/a", Language: "Go"},
				{FullName: "o/b", Language: "Python"},
			},
		}
		idx := mf.Index()
		if len(idx) != 2 {
			t.Fatalf("want 2, got %d", len(idx))
		}
		if idx["o/a"] == nil || idx["o/b"] == nil {
			t.Fatal("missing key in index")
		}
		idx["o/a"].Language = "Rust"
		if mf.Repos[0].Language != "Rust" {
			t.Error("index pointer does not refer into manifest slice")
		}
	})

	t.Run("nil receiver", func(t *testing.T) {
		var mf *FleetManifest
		idx := mf.Index()
		if idx != nil {
			t.Errorf("got %v, want nil", idx)
		}
	})
}

func TestGroupRepos(t *testing.T) {
	mf := &FleetManifest{
		Version: "1",
		Owner:   "o",
		Groups: map[string][]string{
			"backend": {"o/a", "o/b"},
		},
		Repos: []ManifestRepo{
			{FullName: "o/a"},
			{FullName: "o/b"},
		},
	}

	t.Run("existing group", func(t *testing.T) {
		got := mf.GroupRepos("backend")
		want := map[string]struct{}{"o/a": {}, "o/b": {}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("missing group", func(t *testing.T) {
		got := mf.GroupRepos("nope")
		if got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})

	t.Run("nil Groups", func(t *testing.T) {
		mf2 := &FleetManifest{Version: "1", Owner: "o"}
		got := mf2.GroupRepos("any")
		if got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})

	t.Run("nil receiver", func(t *testing.T) {
		var mf3 *FleetManifest
		got := mf3.GroupRepos("any")
		if got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})
}

func TestGenerate(t *testing.T) {
	t.Run("sets version owner and API fields", func(t *testing.T) {
		now := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
		api := []*provider.Repo{
			{Name: "svc", FullName: "o/svc", Language: "Go", Archived: false, Fork: false, Private: true, Topics: []string{"go"}, UpdatedAt: now},
		}
		got := Generate(api, "o")
		if got.Version != "1" {
			t.Errorf("Version: got %q, want 1", got.Version)
		}
		if got.Owner != "o" {
			t.Errorf("Owner: got %q, want o", got.Owner)
		}
		if len(got.Repos) != 1 {
			t.Fatalf("want 1 repo, got %d", len(got.Repos))
		}
		r := got.Repos[0]
		if r.FullName != "o/svc" {
			t.Errorf("FullName: got %q, want o/svc", r.FullName)
		}
		if r.Language != "Go" {
			t.Errorf("Language: got %q, want Go", r.Language)
		}
		if r.Archived {
			t.Errorf("Archived: got true, want false")
		}
		if r.Fork {
			t.Errorf("Fork: got true, want false")
		}
		if !r.Private {
			t.Errorf("Private: got false, want true")
		}
		if !reflect.DeepEqual(r.Topics, []string{"go"}) {
			t.Errorf("Topics: got %v, want [go]", r.Topics)
		}
		if !r.UpdatedAt.Equal(now) {
			t.Errorf("UpdatedAt: got %v, want %v", r.UpdatedAt, now)
		}
	})
}
