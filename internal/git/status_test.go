package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

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
