package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCountAheadBehind(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	ctx := context.Background()
	tmp := t.TempDir()

	remoteDir := filepath.Join(tmp, "remote")
	localDir := filepath.Join(tmp, "local")
	setupDir := filepath.Join(tmp, "setup")

	gitRunAt := func(d string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = d
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	gitRunAt(tmp, "-c", "init.defaultBranch=main", "init", "--bare", remoteDir)
	gitRunAt(tmp, "-c", "init.defaultBranch=main", "clone", remoteDir, setupDir)
	gitRunAt(setupDir, "config", "user.email", "test@test.com")
	gitRunAt(setupDir, "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(setupDir, "seed.txt"), []byte("seed"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRunAt(setupDir, "add", ".")
	gitRunAt(setupDir, "commit", "-m", "seed")
	gitRunAt(setupDir, "push", "origin", "HEAD:main")

	gitRunAt(tmp, "-c", "init.defaultBranch=main", "clone", remoteDir, localDir)
	gitRunAt(localDir, "config", "user.email", "test@test.com")
	gitRunAt(localDir, "config", "user.name", "test")

	b, a := CountAheadBehind(ctx, localDir)
	if b != 0 || a != 0 {
		t.Errorf("fresh clone: want (0,0), got (%d,%d)", b, a)
	}

	if err := os.WriteFile(filepath.Join(localDir, "f.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRunAt(localDir, "add", ".")
	gitRunAt(localDir, "commit", "-m", "local")

	b, a = CountAheadBehind(ctx, localDir)
	if b != 0 || a != 1 {
		t.Errorf("one commit ahead: want (0,1), got (%d,%d)", b, a)
	}

	gitRunAt(localDir, "push", "origin", "HEAD:main")

	b, a = CountAheadBehind(ctx, localDir)
	if b != 0 || a != 0 {
		t.Errorf("after push: want (0,0), got (%d,%d)", b, a)
	}
}

func TestCountAheadBehindNoUpstream(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	ctx := context.Background()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	b, a := CountAheadBehind(ctx, dir)
	if b != 0 || a != 0 {
		t.Errorf("no upstream: want (0,0), got (%d,%d)", b, a)
	}
}

func TestStatusDirtyVsUntracked(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	ctx := context.Background()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "test"},
	} {
		if _, err := run(ctx, dir, args...); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}

	tracked := filepath.Join(dir, "tracked.txt")
	if err := os.WriteFile(tracked, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := run(ctx, dir, "add", "."); err != nil {
		t.Fatal(err)
	}
	if _, err := run(ctx, dir, "commit", "-m", "init"); err != nil {
		t.Fatal(err)
	}

	s, err := Status(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.Dirty || s.Untracked != 0 {
		t.Errorf("clean repo: dirty=%v untracked=%d", s.Dirty, s.Untracked)
	}

	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err = Status(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.Dirty {
		t.Error("untracked file should not mark repo dirty")
	}
	if s.Untracked != 1 {
		t.Errorf("untracked = %d, want 1", s.Untracked)
	}

	if err := os.WriteFile(tracked, []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err = Status(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if !s.Dirty {
		t.Error("modified tracked file should mark repo dirty")
	}
}
