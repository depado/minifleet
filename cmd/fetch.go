package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/depado/minifleet/internal/fleet"
	"github.com/depado/minifleet/internal/git"
	"github.com/depado/minifleet/internal/ui"
)

func newFetchCmd() *cobra.Command {
	var (
		filters Filters
		force   bool
	)

	cmd := &cobra.Command{
		Use:   "fetch",
		Short: "Fetch remotes for all cloned repositories",
		Long:  "Run git fetch origin in every cloned repository. Operates on the fleet in the current directory, or all known fleets otherwise.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			conf, err := confFromCtx(cmd)
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			plan := planFromCtx(ctx)
			ApplyPlan(&filters, plan, cmd)

			targets, err := planTargets(conf, plan, sharedAll)
			if err != nil {
				return err
			}
			if len(targets) == 0 {
				ui.PrintDim("No fleet in the current directory and no known fleets. Run 'minifleet discover <owner>' first.")
				return nil
			}

			for _, t := range targets {
				tasks, err := reposForTarget(ctx, t, filters)
				if err != nil {
					return fmt.Errorf("scan %s: %w", t.Dir, err)
				}
				if len(tasks) == 0 {
					continue
				}

				exec := fleet.NewExecutor(fleet.ExecutorConfig{
					Concurrency: conf.Concurrent,
					Progress:    conf.UI.Progress,
					ProgressConfig: fleet.ProgressConfig{
						Description: fmt.Sprintf("Fetching %s", t.Owner),
					},
				})

				result := exec.Run(ctx, tasks, func(ctx context.Context, task fleet.RepoTask) (any, error) {
					if !git.IsRepo(ctx, task.Dir) {
						return nil, &fleet.SkipError{Reason: "not cloned"}
					}
					return nil, git.Fetch(ctx, task.Dir, force)
				})

				printBulkSummary(result, false)
			}

			return nil
		},
	}

	addLocalFilterFlags(cmd, &filters)
	cmd.Flags().BoolVar(&force, "force", false, "force fetch, overwriting diverged local tags")

	return cmd
}
