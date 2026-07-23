package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/depado/minifleet/internal/fleet"
	"github.com/depado/minifleet/internal/git"
	"github.com/depado/minifleet/internal/provider"
	"github.com/depado/minifleet/internal/provider/github"
)

func newFetchCmd() *cobra.Command {
	var (
		filters Filters
		force   bool
	)

	cmd := &cobra.Command{
		Use:   "fetch [owner]",
		Short: "Fetch remotes for all cloned repositories",
		Long:  "Run git fetch origin in every cloned repository. Without an owner, operates on the fleet in the current directory (or all known fleets).\nWith an owner, fetches repositories for that specific fleet.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conf, err := confFromCtx(cmd)
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			plan := planFromCtx(ctx)
			ApplyPlan(&filters, plan, cmd)

			if sharedAll || len(args) == 0 {
				return fetchAll(ctx, conf, filters, plan, force)
			}

			prov, err := github.New(conf.GitHub.Token, conf.GitHub.Host)
			if err != nil {
				return err
			}
			return fetchOne(ctx, conf, prov, args[0], filters, force)
		},
	}

	addLocalFilterFlags(cmd, &filters)
	cmd.Flags().BoolVar(&force, "force", false, "force fetch, overwriting diverged local tags")

	return cmd
}

func fetchAll(ctx context.Context, conf *Conf, f Filters, plan *Plan, force bool) error {
	targets, err := planTargets(conf, plan, sharedAll)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		conf.PrintDim("No fleet in the current directory and no known fleets. Run 'minifleet discover <owner>' first.")
		return nil
	}

	if sharedJSON {
		return fetchTargetsJSON(ctx, conf, targets, f, force)
	}

	for _, t := range targets {
		if err := fetchTarget(ctx, conf, t, f, force); err != nil {
			return err
		}
	}
	return nil
}

func fetchOne(ctx context.Context, conf *Conf, prov provider.Provider, owner string, f Filters, force bool) error {
	target, _ := resolveFleet(conf, prov.Host(), owner)
	if target.Dir == "" {
		return fmt.Errorf("could not resolve fleet directory for %s (no --path, current directory, or known_fleets entry)", owner)
	}

	if sharedJSON {
		return fetchTargetsJSON(ctx, conf, []fleetTarget{target}, f, force)
	}
	return fetchTarget(ctx, conf, target, f, force)
}

func fetchTarget(ctx context.Context, conf *Conf, t fleetTarget, f Filters, force bool) error {
	tasks, err := reposForTarget(ctx, t, f)
	if err != nil {
		return fmt.Errorf("scan %s: %w", t.Dir, err)
	}
	if len(tasks) == 0 {
		return nil
	}

	if !conf.Console.IsTerminal() {
		slog.Info("fetching", "owner", t.Owner, "repos", len(tasks), "dir", t.Dir)
	}

	exec := fleet.NewExecutor(fleet.ExecutorConfig{
		Concurrency: conf.Concurrent,
		Interactive: conf.Console.IsTerminal(),
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

	if !conf.Console.IsTerminal() {
		slog.Info("fetched",
			"owner", t.Owner,
			"repos", len(tasks),
			"succeeded", result.Succeeded,
			"skipped", result.Skipped,
			"failed", result.Failed,
			"elapsed", result.Elapsed.Round(time.Millisecond),
		)
	}

	printBulkSummary(result, false, conf)
	return nil
}

func fetchTargetsJSON(ctx context.Context, conf *Conf, targets []fleetTarget, f Filters, force bool) error {
	out := make(map[string][]fetchRepoResult)
	for _, t := range targets {
		tasks, err := reposForTarget(ctx, t, f)
		if err != nil {
			return err
		}
		if len(tasks) == 0 {
			continue
		}

		slog.Info("fetching", "owner", t.Owner, "repos", len(tasks), "dir", t.Dir)

		exec := fleet.NewExecutor(fleet.ExecutorConfig{
			Concurrency: conf.Concurrent,
			Interactive: false,
		})

		result := exec.Run(ctx, tasks, func(ctx context.Context, task fleet.RepoTask) (any, error) {
			if !git.IsRepo(ctx, task.Dir) {
				return nil, &fleet.SkipError{Reason: "not cloned"}
			}
			return nil, git.Fetch(ctx, task.Dir, force)
		})

		slog.Info("fetched",
			"owner", t.Owner,
			"repos", len(tasks),
			"succeeded", result.Succeeded,
			"skipped", result.Skipped,
			"failed", result.Failed,
			"elapsed", result.Elapsed.Round(time.Millisecond),
		)

		for _, r := range result.Results {
			status := "ok"
			errMsg := ""
			switch r.Status {
			case fleet.StatusSkipped:
				status = "skipped"
				if r.Err != nil {
					errMsg = r.Err.Error()
				}
			case fleet.StatusFailed:
				status = "failed"
				if r.Err != nil {
					errMsg = r.Err.Error()
				}
			}
			out[t.Owner] = append(out[t.Owner], fetchRepoResult{
				Repo:   r.Task.RepoName,
				Status: status,
				Error:  errMsg,
			})
		}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if len(out) == 1 {
		for _, v := range out {
			return enc.Encode(v)
		}
	}
	return enc.Encode(out)
}

type fetchRepoResult struct {
	Repo   string `json:"repo"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}
