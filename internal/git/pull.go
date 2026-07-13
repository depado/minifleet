package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func Pull(ctx context.Context, dir string) error {
	if out, err := run(ctx, dir, "fetch", "origin"); err != nil {
		return fmt.Errorf("fetch: %w\n%s", err, strings.TrimSpace(out))
	}

	branch, err := run(ctx, dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return fmt.Errorf("get branch: %w", err)
	}
	if branch == "HEAD" {
		return &SkipError{Reason: "detached HEAD, cannot pull"}
	}

	cmd := exec.CommandContext(ctx, "git", "rebase", fmt.Sprintf("origin/%s", branch), "--autostash")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rebase: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
