package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/depado/minifleet/internal/fleet"
	"github.com/depado/minifleet/internal/git"
	"github.com/depado/minifleet/internal/manifest"
	"github.com/depado/minifleet/internal/ui"
)

type statusRow struct {
	Repo      string           `json:"repo"`
	Status    *git.RepoStatus  `json:"status,omitempty"`
	Error     string           `json:"error,omitempty"`
}

func newStatusCmd() *cobra.Command {
	var (
		filters Filters
		format  string
	)

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show git status overview of all cloned repositories",
		RunE: func(cmd *cobra.Command, _ []string) error {
			conf, err := confFromCtx(cmd)
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			mf, _ := manifest.Load(manifest.ManifestPath())

			scanDir := conf.Fleet.Base
			flat := false
			if conf.Fleet.Path != "" {
				scanDir = expandPath(conf.Fleet.Path)
				flat = true
			}

			tasks, err := fleet.Scan(scanDir, filters.Target, mf, flat)
			if err != nil {
				return fmt.Errorf("scan repos: %w", err)
			}

			tasksWithName := make([]taskWithName, len(tasks))
			for i, t := range tasks {
				tasksWithName[i] = taskWithName{RepoName: t.RepoName, FullName: t.FullName, ID: t.ID}
			}
			tasksWithName, err = filters.ApplyTasks(tasksWithName, mf)
			if err != nil {
				return err
			}

			filteredTasks := make([]fleet.RepoTask, len(tasksWithName))
			for i, t := range tasksWithName {
				filteredTasks[i] = fleet.RepoTask{RepoName: t.RepoName, ID: t.ID, FullName: t.FullName}
			}

			if len(filteredTasks) == 0 {
				ui.PrintDim("No repositories found in " + scanDir)
				return nil
			}

			exec := fleet.NewExecutor(fleet.ExecutorConfig{
				Concurrency: conf.Fleet.Concurrent,
				Progress:    false,
			})

			result := exec.Run(ctx, filteredTasks, func(ctx context.Context, task fleet.RepoTask) (any, error) {
				status, err := git.Status(ctx, task.ID)
				if err != nil {
					return nil, err
				}
				return status, nil
			})

			switch format {
			case "json":
				return outputStatusJSON(result)
			default:
				return outputStatusTable(result)
			}
		},
	}

	addFilterFlags(cmd, &filters)
	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format: table, json")

	return cmd
}

func outputStatusTable(result *fleet.BulkResult) error {
	tbl := ui.NewTable("Repo", "Branch", "Behind", "Ahead", "Dirty", "Stash")
	rows := make([]fleet.RepoResult, len(result.Results))
	copy(rows, result.Results)
	sort.Slice(rows, func(i, j int) bool { return rows[i].Task.RepoName < rows[j].Task.RepoName })

	for i := range rows {
		r := &rows[i]
		branch := "?"
		behind := "?"
		ahead := "?"
		dirty := "?"
		stash := "?"

		if r.Status != fleet.StatusFailed {
			if s, ok := r.Payload.(*git.RepoStatus); ok {
				branch = s.Branch
				behind = fmt.Sprintf("%d", s.Behind)
				ahead = fmt.Sprintf("%d", s.Ahead)
				if s.Dirty {
					dirty = "[red]yes[/]"
				} else {
					dirty = "[dim]no[/]"
				}
				stash = fmt.Sprintf("%d", s.StashCount)
			}
		}

		tbl.AddRow(
			fmt.Sprintf("[bold]%s[/]", r.Task.RepoName),
			branch,
			behind,
			ahead,
			dirty,
			stash,
		)
	}
	ui.DefaultConsole.Render(tbl)
	return nil
}

func outputStatusJSON(result *fleet.BulkResult) error {
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
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Print(string(data))
	return nil
}