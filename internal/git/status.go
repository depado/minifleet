package git

import (
	"context"
	"strconv"
	"strings"
)

type RepoStatus struct {
	Branch     string
	Behind     int
	Ahead      int
	Dirty      bool
	StashCount int
}

func Status(ctx context.Context, dir string) (*RepoStatus, error) {
	branch, err := run(ctx, dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return nil, err
	}

	dirty := checkDirty(ctx, dir)

	behind, ahead := 0, 0
	if upstream, err := run(ctx, dir, "rev-parse", "--abbrev-ref", "@{upstream}"); err == nil && upstream != "" {
		behind, ahead, _ = countAheadBehind(ctx, dir)
	}

	stashCount := countStashes(ctx, dir)

	return &RepoStatus{
		Branch:     branch,
		Behind:     behind,
		Ahead:      ahead,
		Dirty:      dirty,
		StashCount: stashCount,
	}, nil
}

func checkDirty(ctx context.Context, dir string) bool {
	out, err := run(ctx, dir, "status", "--porcelain")
	if err != nil {
		return false
	}
	return len(out) > 0
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
