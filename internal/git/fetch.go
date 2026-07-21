package git

import "context"

func Fetch(ctx context.Context, dir string, force bool) error {
	args := []string{"fetch", "--prune", "--tags"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, "origin")
	if _, err := run(ctx, dir, args...); err != nil {
		return err
	}
	if _, err := run(ctx, dir, "remote", "set-head", "origin", "--auto"); err != nil {
		return err
	}
	return nil
}
