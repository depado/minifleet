package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// Clone runs git clone. When shallow is true, uses --depth 1 --filter=blob:none
// for a faster, smaller clone (no blob history).
func Clone(ctx context.Context, url, dir string, shallow bool) error {
	args := []string{"clone"}
	if shallow {
		args = append(args, "--depth", "1", "--filter=blob:none")
	}
	args = append(args, url, dir)

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("clone %s: %w\n%s", url, err, string(out))
	}
	return nil
}
