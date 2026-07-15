package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/depado/minifleet/internal/fleet"
	"github.com/depado/minifleet/internal/git"
	"github.com/depado/minifleet/internal/ui"
)

type statusRow struct {
	Repo   string          `json:"repo"`
	Status *git.RepoStatus `json:"status,omitempty"`
	Error  string          `json:"error,omitempty"`
}

func newStatusCmd() *cobra.Command {
	var (
		filters Filters
		format  string
		all     bool
	)

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show git status overview of all cloned repositories",
		Long:  "Operates on the fleet in the current directory (if fleet.yml is present), or all known fleets otherwise. Use --all to always operate on every known fleet.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			conf, err := confFromCtx(cmd)
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			targets := discoverFleets(conf, all)
			if len(targets) == 0 {
				ui.PrintDim("No fleet in the current directory and no known fleets. Run 'minifleet discover <owner>' first.")
				return nil
			}

			var allRows []statusRow
			for _, t := range targets {
				rows, err := runStatusForFleet(ctx, conf, t, filters, format)
				if err != nil {
					return err
				}
				if format == "json" {
					allRows = append(allRows, rows...)
					continue
				}
				if len(rows) > 0 {
					renderStatusTable(fleetTitle(t), rows)
				}
			}

			if format == "json" {
				data, err := json.MarshalIndent(allRows, "", "  ")
				if err != nil {
					return err
				}
				fmt.Print(string(data))
			}
			return nil
		},
	}

	addLocalFilterFlags(cmd, &filters)
	addAllFlag(cmd, &all)
	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format: table, json")

	return cmd
}

func runStatusForFleet(ctx context.Context, conf *Conf, t fleetTarget, f Filters, format string) ([]statusRow, error) {
	tasks, err := reposForTarget(t, f)
	if err != nil {
		return nil, fmt.Errorf("scan %s: %w", t.Dir, err)
	}

	if len(tasks) == 0 {
		return nil, nil
	}

	exec := fleet.NewExecutor(fleet.ExecutorConfig{
		Concurrency: conf.Fleet.Concurrent,
		Progress:    false,
	})

	result := exec.Run(ctx, tasks, func(ctx context.Context, task fleet.RepoTask) (any, error) {
		if !git.IsRepo(ctx, task.Dir) {
			return nil, &fleet.SkipError{Reason: "not cloned"}
		}
		return git.Status(ctx, task.Dir)
	})

	out := make([]statusRow, 0, len(result.Results))
	for i := range result.Results {
		r := &result.Results[i]
		row := statusRow{Repo: r.Task.RepoName}
		if r.Status == fleet.StatusFailed {
			row.Error = r.Err.Error()
		} else if s, ok := r.Payload.(*git.RepoStatus); ok {
			row.Status = s
		}
		out = append(out, row)
	}
	return out, nil
}

func renderStatusTable(title string, rows []statusRow) {
	sort.Slice(rows, func(i, j int) bool { return rows[i].Repo < rows[j].Repo })

	tbl := ui.NewTitledTable(title, "Repo", "Remote", "Branch", "Behind", "Ahead", "Dirty", "Untracked", "Stash")
	for _, r := range rows {
		remote := "?"
		branch := "?"
		behind := "?"
		ahead := "?"
		dirty := "?"
		untracked := "?"
		stash := "?"

		if r.Error != "" {
			dirty = "[red]error[/]"
		} else if r.Status != nil {
			remote = r.Status.Remote
			branch = r.Status.Branch
			behind = fmt.Sprintf("%d", r.Status.Behind)
			ahead = fmt.Sprintf("%d", r.Status.Ahead)
			if r.Status.Dirty {
				dirty = "[red]yes[/]"
			} else {
				dirty = "[dim]no[/]"
			}
			untracked = fmt.Sprintf("%d", r.Status.Untracked)
			stash = fmt.Sprintf("%d", r.Status.StashCount)
		}

		tbl.AddRow(
			fmt.Sprintf("[bold]%s[/]", r.Repo),
			remote,
			branch,
			behind,
			ahead,
			dirty,
			untracked,
			stash,
		)
	}
	ui.DefaultConsole.Render(tbl)
}
