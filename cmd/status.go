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
	)

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show git status overview of all cloned repositories",
		Long:  "Operates on the fleet in CWD (if fleet.yml is present), or all known fleets otherwise.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			conf, err := confFromCtx(cmd)
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			targets := discoverFleets(conf)
			if len(targets) == 0 {
				ui.PrintDim("No fleet in CWD and no known fleets. Run 'minifleet discover <owner>' first.")
				return nil
			}

			var allRows []statusRow
			for _, t := range targets {
				rows, err := runStatusForFleet(ctx, conf, t, filters, format)
				if err != nil {
					return err
				}
				allRows = append(allRows, rows...)
			}

			switch format {
			case "json":
				data, err := json.MarshalIndent(allRows, "", "  ")
				if err != nil {
					return err
				}
				fmt.Print(string(data))
				return nil
			default:
				renderStatusTable(allRows)
				return nil
			}
		},
	}

	addFilterFlags(cmd, &filters)
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

func renderStatusTable(rows []statusRow) {
	sort.Slice(rows, func(i, j int) bool { return rows[i].Repo < rows[j].Repo })

	tbl := ui.NewTable("Repo", "Branch", "Behind", "Ahead", "Dirty", "Stash")
	for _, r := range rows {
		branch := "?"
		behind := "?"
		ahead := "?"
		dirty := "?"
		stash := "?"

		if r.Error != "" {
			dirty = "[red]error[/]"
		} else if r.Status != nil {
			branch = r.Status.Branch
			behind = fmt.Sprintf("%d", r.Status.Behind)
			ahead = fmt.Sprintf("%d", r.Status.Ahead)
			if r.Status.Dirty {
				dirty = "[red]yes[/]"
			} else {
				dirty = "[dim]no[/]"
			}
			stash = fmt.Sprintf("%d", r.Status.StashCount)
		}

		tbl.AddRow(
			fmt.Sprintf("[bold]%s[/]", r.Repo),
			branch,
			behind,
			ahead,
			dirty,
			stash,
		)
	}
	ui.DefaultConsole.Render(tbl)
}
