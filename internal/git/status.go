package git

import (
	"context"
	"strconv"
	"strings"
)

type RepoStatus struct {
	Branch     string
	Remote     string
	Behind     int
	Ahead      int
	Dirty      bool
	Untracked  int
	StashCount int
}

func Status(ctx context.Context, dir string) (*RepoStatus, error) {
	branch, err := run(ctx, dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return nil, err
	}

	remote, _ := run(ctx, dir, "remote", "get-url", "origin")

	dirty, untracked := checkDirty(ctx, dir)
	behind, ahead := CountAheadBehind(ctx, dir)

	stashCount := countStashes(ctx, dir)

	return &RepoStatus{
		Branch:     branch,
		Remote:     remote,
		Behind:     behind,
		Ahead:      ahead,
		Dirty:      dirty,
		Untracked:  untracked,
		StashCount: stashCount,
	}, nil
}

// IsDirty reports whether the repo has uncommitted changes to tracked files.
// Untracked files do not count. Returns false when dir is not a git repo.
func IsDirty(ctx context.Context, dir string) bool {
	dirty, _ := checkDirty(ctx, dir)
	return dirty
}

// CountAheadBehind returns the number of commits the repo is behind/ahead of its
// upstream tracking branch. Returns 0,0 when no upstream is configured.
func CountAheadBehind(ctx context.Context, dir string) (behind, ahead int) {
	if upstream, err := run(ctx, dir, "rev-parse", "--abbrev-ref", "@{upstream}"); err != nil || upstream == "" {
		return 0, 0
	}
	b, a, _ := countAheadBehind(ctx, dir)
	return b, a
}

func checkDirty(ctx context.Context, dir string) (dirty bool, untracked int) {
	out, err := run(ctx, dir, "status", "--porcelain")
	if err != nil || out == "" {
		return false, 0
	}
	for line := range strings.SplitSeq(out, "\n") {
		if strings.HasPrefix(line, "??") {
			untracked++
		} else if line != "" {
			dirty = true
		}
	}
	return dirty, untracked
}

func countAheadBehind(ctx context.Context, dir string) (behind, ahead int, err error) {
	out, err := run(ctx, dir, "rev-list", "--left-right", "--count", "@{upstream}...HEAD")
	if err != nil {
		return 0, 0, err
	}
	parts := strings.Split(out, "\t")
	if len(parts) == 2 {
		behind, _ = strconv.Atoi(parts[0])
		ahead, _ = strconv.Atoi(parts[1])
	}
	return
}

func countStashes(ctx context.Context, dir string) int {
	out, err := run(ctx, dir, "stash", "list")
	if err != nil || out == "" {
		return 0
	}
	return len(strings.Split(out, "\n"))
}
