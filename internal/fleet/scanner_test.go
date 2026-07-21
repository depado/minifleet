package fleet

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/depado/minifleet/internal/manifest"
)

func TestLastSegment(t *testing.T) {
	tests := []struct {
		fullName string
		want     string
	}{
		{"owner/repo", "repo"},
		{"noslash", ""},
		{"a/b/c", "c"},
		{"/leading", "leading"},
		{"trailing/", ""},
	}
	for _, tt := range tests {
		t.Run(tt.fullName, func(t *testing.T) {
			got := lastSegment(tt.fullName)
			if got != tt.want {
				t.Errorf("lastSegment(%q) = %q, want %q", tt.fullName, got, tt.want)
			}
		})
	}
}

func TestShortToFullName(t *testing.T) {
	t.Run("nil manifest", func(t *testing.T) {
		got := shortToFullName(nil)
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})
	t.Run("maps short to full", func(t *testing.T) {
		mf := &manifest.FleetManifest{
			Repos: []manifest.ManifestRepo{
				{FullName: "o/svc-api"},
				{FullName: "o/lib"},
			},
		}
		got := shortToFullName(mf)
		if got["svc-api"] != "o/svc-api" {
			t.Errorf("svc-api = %q, want o/svc-api", got["svc-api"])
		}
		if got["lib"] != "o/lib" {
			t.Errorf("lib = %q, want o/lib", got["lib"])
		}
	})
	t.Run("skips repos without slash", func(t *testing.T) {
		mf := &manifest.FleetManifest{
			Repos: []manifest.ManifestRepo{
				{FullName: "noslash"},
			},
		}
		got := shortToFullName(mf)
		if len(got) != 0 {
			t.Errorf("expected empty map, got %v", got)
		}
	})
}

func TestIgnoredSet(t *testing.T) {
	t.Run("nil manifest", func(t *testing.T) {
		if got := ignoredSet(nil); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})
	t.Run("includes full and short names", func(t *testing.T) {
		mf := &manifest.FleetManifest{
			Repos: []manifest.ManifestRepo{
				{FullName: "o/keep", Ignored: false},
				{FullName: "o/skip", Ignored: true},
			},
		}
		set := ignoredSet(mf)
		if _, ok := set["o/skip"]; !ok {
			t.Error("expected o/skip to be in ignored set")
		}
		if _, ok := set["skip"]; !ok {
			t.Error("expected short name skip to be in ignored set")
		}
		if _, ok := set["o/keep"]; ok {
			t.Error("expected o/keep to NOT be in ignored set")
		}
	})
}

func TestIsIgnored(t *testing.T) {
	set := map[string]struct{}{
		"skip":   {},
		"o/skip": {},
	}
	t.Run("matches short name", func(t *testing.T) {
		if !isIgnored(set, "skip", "o/skip") {
			t.Error("expected isIgnored by short name")
		}
	})
	t.Run("matches full name", func(t *testing.T) {
		if !isIgnored(set, "other", "o/skip") {
			t.Error("expected isIgnored by full name")
		}
	})
	t.Run("no match", func(t *testing.T) {
		if isIgnored(set, "keep", "o/keep") {
			t.Error("expected not ignored")
		}
	})
	t.Run("nil set", func(t *testing.T) {
		if isIgnored(nil, "x", "o/x") {
			t.Error("expected not ignored with nil set")
		}
	})
}

func TestScan(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	repo := filepath.Join(dir, "svc-api")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", repo, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	if err := os.MkdirAll(filepath.Join(dir, "plain"), 0o755); err != nil {
		t.Fatal(err)
	}
	tasks, err := Scan(context.Background(), dir, "", nil)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(tasks) != 1 || tasks[0].RepoName != "svc-api" {
		t.Fatalf("got %+v", tasks)
	}
	if tasks[0].Dir != repo {
		t.Errorf("Dir = %q, want %q", tasks[0].Dir, repo)
	}
}
