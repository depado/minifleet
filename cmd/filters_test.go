package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
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
		{"target regex", Filters{IncludeRegex: "^svc", IncludeArchived: true, IncludeForks: true}, []string{"svc-api"}},
		{"exclude regex", Filters{ExcludeRegex: "^svc", IncludeArchived: true, IncludeForks: true}, []string{"old-thing", "forked-lib"}},
		{"include exact", Filters{Include: []string{"svc-api", "old-thing"}, IncludeArchived: true, IncludeForks: true}, []string{"svc-api", "old-thing"}},
		{"exclude exact", Filters{Exclude: []string{"svc-api"}, IncludeArchived: true, IncludeForks: true}, []string{"old-thing", "forked-lib"}},
		{"exclude wins over include", Filters{Include: []string{"svc-api"}, Exclude: []string{"svc-api"}, IncludeArchived: true, IncludeForks: true}, nil},
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

func TestApplyTasks(t *testing.T) {
	tasks := []taskWithName{
		{RepoName: "svc-api", FullName: "o/svc-api", ID: "/x/svc-api"},
		{RepoName: "loose", FullName: "", ID: "/x/loose"},
		{RepoName: "old-thing", FullName: "o/old-thing", ID: "/x/old-thing"},
	}
	mf := &manifest.FleetManifest{
		Version: "1",
		Owner:   "o",
		Groups:  map[string][]string{"backend": {"o/svc-api"}},
		Repos: []manifest.ManifestRepo{
			{FullName: "o/svc-api", Language: "Go", Archived: false, Labels: map[string]string{"tier": "1"}},
			{FullName: "o/old-thing", Language: "Python", Archived: true, Labels: map[string]string{"tier": "3"}},
		},
	}
	tests := []struct {
		name string
		f    Filters
		want []string
	}{
		{"defaults include loose+svc", Filters{}, []string{"svc-api", "loose"}},
		{"include-archived keeps loose", Filters{IncludeArchived: true}, []string{"svc-api", "loose", "old-thing"}},
		{"language go drops loose", Filters{Language: "Go"}, []string{"svc-api"}},
		{"label tier drops loose", Filters{Labels: []string{"tier"}}, []string{"svc-api"}},
		{"group backend drops loose", Filters{Group: "backend"}, []string{"svc-api"}},
		{"archived excluded by default", Filters{}, []string{"svc-api", "loose"}},
		{"include-archived includes manifest archived", Filters{IncludeArchived: true}, []string{"svc-api", "loose", "old-thing"}},
		{"label tier=3 on archived needs include-archived", Filters{Labels: []string{"tier=3"}, IncludeArchived: true}, []string{"old-thing"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.f.ApplyTasks(tasks, mf)
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			names := make([]string, 0, len(got))
			for _, g := range got {
				names = append(names, g.RepoName)
			}
			if !equalSets(names, tt.want) {
				t.Errorf("got %v, want %v", names, tt.want)
			}
		})
	}
}

func TestApplyTasksHasFiles(t *testing.T) {
	dir := t.TempDir()
	apiDir := filepath.Join(dir, "svc-api")
	looseDir := filepath.Join(dir, "loose")
	oldDir := filepath.Join(dir, "old-thing")
	for _, d := range []string{apiDir, looseDir, oldDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []struct {
		path    string
		content string
	}{
		{filepath.Join(apiDir, "go.mod"), "module o/svc-api\n"},
		{filepath.Join(apiDir, "main.go"), "package main\n"},
		{filepath.Join(looseDir, "Makefile"), "all:\n"},
	} {
		if err := os.WriteFile(f.path, []byte(f.content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	tasks := []taskWithName{
		{RepoName: "svc-api", FullName: "o/svc-api", ID: "/x/svc-api", Dir: apiDir},
		{RepoName: "loose", FullName: "", ID: "/x/loose", Dir: looseDir},
		{RepoName: "old-thing", FullName: "o/old-thing", ID: "/x/old-thing", Dir: oldDir},
	}
	mf := &manifest.FleetManifest{
		Version: "1",
		Owner:   "o",
		Repos: []manifest.ManifestRepo{
			{FullName: "o/svc-api", Language: "Go", Archived: false},
			{FullName: "o/old-thing", Language: "Python", Archived: true},
		},
	}
	tests := []struct {
		name string
		f    Filters
		want []string
	}{
		{"has-file go.mod matches svc-api only", Filters{HasFiles: []string{"go.mod"}, IncludeArchived: true}, []string{"svc-api"}},
		{"has-file main.go matches svc-api only", Filters{HasFiles: []string{"main.go"}, IncludeArchived: true}, []string{"svc-api"}},
		{"has-file Makefile matches loose only", Filters{HasFiles: []string{"Makefile"}}, []string{"loose"}},
		{"has-file go.mod AND main.go matches svc-api", Filters{HasFiles: []string{"go.mod", "main.go"}, IncludeArchived: true}, []string{"svc-api"}},
		{"has-file go.mod AND Makefile matches none", Filters{HasFiles: []string{"go.mod", "Makefile"}, IncludeArchived: true}, nil},
		{"has-file missing file matches none", Filters{HasFiles: []string{"package.json"}, IncludeArchived: true}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.f.ApplyTasks(tasks, mf)
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			names := make([]string, 0, len(got))
			for _, g := range got {
				names = append(names, g.RepoName)
			}
			if !equalSets(names, tt.want) {
				t.Errorf("got %v, want %v", names, tt.want)
			}
		})
	}
}

func TestApplyTasksIfCmd(t *testing.T) {
	dir := t.TempDir()
	apiDir := filepath.Join(dir, "svc-api")
	looseDir := filepath.Join(dir, "loose")
	oldDir := filepath.Join(dir, "old-thing")
	for _, d := range []string{apiDir, looseDir, oldDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []struct {
		path    string
		content string
	}{
		{filepath.Join(apiDir, "go.mod"), "module o/svc-api\n\ngo 1.21\nrequire (\n\tgithub.com/foo/bar v2.3.0\n)\n"},
		{filepath.Join(looseDir, "Makefile"), "all:\n"},
	} {
		if err := os.WriteFile(f.path, []byte(f.content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	tasks := []taskWithName{
		{RepoName: "svc-api", FullName: "o/svc-api", ID: "/x/svc-api", Dir: apiDir},
		{RepoName: "loose", FullName: "", ID: "/x/loose", Dir: looseDir},
		{RepoName: "old-thing", FullName: "o/old-thing", ID: "/x/old-thing", Dir: oldDir},
	}
	mf := &manifest.FleetManifest{
		Version: "1",
		Owner:   "o",
		Repos: []manifest.ManifestRepo{
			{FullName: "o/svc-api", Language: "Go", Archived: false},
			{FullName: "o/old-thing", Language: "Python", Archived: true},
		},
	}
	tests := []struct {
		name string
		f    Filters
		want []string
	}{
		{"if grep matches svc-api", Filters{IfCmd: "grep -q foo/bar go.mod", IncludeArchived: true}, []string{"svc-api"}},
		{"if grep no match filters all", Filters{IfCmd: "grep -q nonexistent go.mod", IncludeArchived: true}, nil},
		{"if true passes all", Filters{IfCmd: "true", IncludeArchived: true}, []string{"svc-api", "loose", "old-thing"}},
		{"if false filters all", Filters{IfCmd: "false", IncludeArchived: true}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.f.ApplyTasks(tasks, mf)
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			names := make([]string, 0, len(got))
			for _, g := range got {
				names = append(names, g.RepoName)
			}
			if !equalSets(names, tt.want) {
				t.Errorf("got %v, want %v", names, tt.want)
			}
		})
	}
}

func TestApplyTasksDirty(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	cleanDir := filepath.Join(dir, "clean")
	dirtyDir := filepath.Join(dir, "dirty")
	plainDir := filepath.Join(dir, "plain")

	gitRun := func(d string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = d
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	for _, d := range []string{cleanDir, dirtyDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		gitRun(d, "init")
		gitRun(d, "config", "user.email", "test@test.com")
		gitRun(d, "config", "user.name", "test")
		if err := os.WriteFile(filepath.Join(d, "f.txt"), []byte("a"), 0o644); err != nil {
			t.Fatal(err)
		}
		gitRun(d, "add", ".")
		gitRun(d, "commit", "-m", "init")
	}
	if err := os.MkdirAll(plainDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirtyDir, "f.txt"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}

	tasks := []taskWithName{
		{RepoName: "clean", FullName: "o/clean", ID: "o/clean", Dir: cleanDir},
		{RepoName: "dirty", FullName: "o/dirty", ID: "o/dirty", Dir: dirtyDir},
		{RepoName: "plain", FullName: "o/plain", ID: "o/plain", Dir: plainDir},
	}

	got, err := Filters{Dirty: true}.ApplyTasks(tasks, nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	names := make([]string, 0, len(got))
	for _, g := range got {
		names = append(names, g.RepoName)
	}
	if !equalSets(names, []string{"dirty"}) {
		t.Errorf("got %v, want [dirty]", names)
	}
}

func TestApplyTasksAheadBehind(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmp := t.TempDir()
	remoteDir := filepath.Join(tmp, "remote.git")
	setupDir := filepath.Join(tmp, "setup")
	alignedDir := filepath.Join(tmp, "aligned")
	aheadDir := filepath.Join(tmp, "ahead")
	plainDir := filepath.Join(tmp, "plain")

	gitRun := func(d string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = d
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	gitRun(tmp, "init", "--bare", remoteDir)
	gitRun(tmp, "clone", remoteDir, setupDir)
	gitRun(setupDir, "config", "user.email", "test@test.com")
	gitRun(setupDir, "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(setupDir, "seed.txt"), []byte("seed"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(setupDir, "add", ".")
	gitRun(setupDir, "commit", "-m", "seed")
	gitRun(setupDir, "push", "origin", "main")

	gitRun(tmp, "clone", remoteDir, alignedDir)
	gitRun(alignedDir, "config", "user.email", "test@test.com")
	gitRun(alignedDir, "config", "user.name", "test")

	gitRun(tmp, "clone", remoteDir, aheadDir)
	gitRun(aheadDir, "config", "user.email", "test@test.com")
	gitRun(aheadDir, "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(aheadDir, "g.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(aheadDir, "add", ".")
	gitRun(aheadDir, "commit", "-m", "ahead-commit")

	if err := os.MkdirAll(plainDir, 0o755); err != nil {
		t.Fatal(err)
	}

	tasks := []taskWithName{
		{RepoName: "ahead", FullName: "o/ahead", ID: "o/ahead", Dir: aheadDir},
		{RepoName: "aligned", FullName: "o/aligned", ID: "o/aligned", Dir: alignedDir},
		{RepoName: "plain", FullName: "o/plain", ID: "o/plain", Dir: plainDir},
	}

	tests := []struct {
		name string
		f    Filters
		want []string
	}{
		{"ahead >= 1", Filters{Ahead: 1}, []string{"ahead"}},
		{"ahead >= 2", Filters{Ahead: 2}, nil},
		{"behind >= 1 (none behind)", Filters{Behind: 1}, nil},
		{"both ahead>=1 behind>=1", Filters{Ahead: 1, Behind: 1}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.f.ApplyTasks(tasks, nil)
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			names := make([]string, 0, len(got))
			for _, g := range got {
				names = append(names, g.RepoName)
			}
			if !equalSets(names, tt.want) {
				t.Errorf("got %v, want %v", names, tt.want)
			}
		})
	}
}

func TestApplyTasksWip(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmp := t.TempDir()
	remoteDir := filepath.Join(tmp, "remote.git")
	setupDir := filepath.Join(tmp, "setup")
	alignedDir := filepath.Join(tmp, "aligned")
	aheadDir := filepath.Join(tmp, "ahead")
	dirtyDir := filepath.Join(tmp, "dirty")
	plainDir := filepath.Join(tmp, "plain")

	gitRun := func(d string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = d
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	gitRun(tmp, "init", "--bare", remoteDir)
	gitRun(tmp, "clone", remoteDir, setupDir)
	gitRun(setupDir, "config", "user.email", "test@test.com")
	gitRun(setupDir, "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(setupDir, "seed.txt"), []byte("seed"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(setupDir, "add", ".")
	gitRun(setupDir, "commit", "-m", "seed")
	gitRun(setupDir, "push", "origin", "main")

	gitRun(tmp, "clone", remoteDir, alignedDir)
	gitRun(alignedDir, "config", "user.email", "test@test.com")
	gitRun(alignedDir, "config", "user.name", "test")

	gitRun(tmp, "clone", remoteDir, aheadDir)
	gitRun(aheadDir, "config", "user.email", "test@test.com")
	gitRun(aheadDir, "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(aheadDir, "g.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(aheadDir, "add", ".")
	gitRun(aheadDir, "commit", "-m", "ahead-commit")

	gitRun(tmp, "clone", remoteDir, dirtyDir)
	gitRun(dirtyDir, "config", "user.email", "test@test.com")
	gitRun(dirtyDir, "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(dirtyDir, "seed.txt"), []byte("modified"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(plainDir, 0o755); err != nil {
		t.Fatal(err)
	}

	tasks := []taskWithName{
		{RepoName: "ahead", FullName: "o/ahead", ID: "o/ahead", Dir: aheadDir},
		{RepoName: "aligned", FullName: "o/aligned", ID: "o/aligned", Dir: alignedDir},
		{RepoName: "dirty", FullName: "o/dirty", ID: "o/dirty", Dir: dirtyDir},
		{RepoName: "plain", FullName: "o/plain", ID: "o/plain", Dir: plainDir},
	}

	tests := []struct {
		name string
		f    Filters
		want []string
	}{
		{"wip catches ahead and dirty", Filters{Wip: true}, []string{"ahead", "dirty"}},
		{"wip + ahead > 1", Filters{Wip: true, Ahead: 2}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.f.ApplyTasks(tasks, nil)
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			names := make([]string, 0, len(got))
			for _, g := range got {
				names = append(names, g.RepoName)
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
