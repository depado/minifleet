package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/depado/minifleet/internal/fleet"
	"github.com/depado/minifleet/internal/provider"
	"github.com/depado/minifleet/internal/provider/github"
	"github.com/depado/minifleet/internal/ui"
)

func newPRsCmd() *cobra.Command {
	var (
		filters Filters
		state   string
		author  string
		noDraft bool
		format  string
	)

	cmd := &cobra.Command{
		Use:   "prs [owner]",
		Short: "List open pull requests across repositories with CI and review status",
		Long:  "Without an owner, shows PRs for the fleet in CWD (or all known fleets).\nWith an owner, uses the local manifest for the repo list; falls back to fetching the repo list from the API if no manifest exists.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conf, err := confFromCtx(cmd)
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			if len(args) == 0 {
				return prsAll(ctx, conf, filters, state, author, noDraft, format)
			}

			owner := args[0]
			prov, err := github.New(conf.GitHub.Token, conf.GitHub.Host)
		if err != nil {
			return err
		}

			target, _ := resolveFleet(conf, prov.Host(), owner)
			tasks, err := manifestToTasks(target, filters)
			if err != nil {
				return err
			}
			if len(tasks) == 0 {
				return prsFromAPI(ctx, conf, prov, owner, filters, state, author, noDraft, format)
			}

			return execPRs(ctx, conf, prov, owner, tasks, state, author, noDraft, format)
		},
	}

	addFilterFlags(cmd, &filters)
	cmd.Flags().StringVar(&state, "state", "open", "filter by state: open, closed, all")
	cmd.Flags().StringVarP(&author, "author", "a", "", "filter by PR author")
	cmd.Flags().BoolVar(&noDraft, "no-draft", false, "exclude draft PRs")
	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format: table, json")

	return cmd
}

func prsAll(ctx context.Context, conf *Conf, f Filters, state, author string, noDraft bool, format string) error {
	targets := discoverFleets(conf)
	if len(targets) == 0 {
		ui.PrintDim("No fleet in CWD and no known fleets. Run 'minifleet discover <owner>' first.")
		return nil
	}

	prov, err := github.New(conf.GitHub.Token, conf.GitHub.Host)
	if err != nil {
		return err
	}
	for _, t := range targets {
		tasks, err := manifestToTasks(t, f)
		if err != nil {
			return err
		}
		if len(tasks) == 0 {
			continue
		}
		if err := execPRs(ctx, conf, prov, t.Owner, tasks, state, author, noDraft, format); err != nil {
			return err
		}
	}
	return nil
}

func prsFromAPI(ctx context.Context, conf *Conf, prov provider.Provider, owner string, f Filters, state, author string, noDraft bool, format string) error {
	isOrg, err := prov.DetectOwner(ctx, owner)
	if err != nil {
		return fmt.Errorf("detect owner: %w", err)
	}

	repos, err := prov.ListRepos(ctx, owner, provider.ListOptions{
		Visibility: f.Visibility,
		IsOrg:      isOrg,
	})
	if err != nil {
		return fmt.Errorf("list repos: %w", err)
	}

	target, _ := resolveFleet(conf, prov.Host(), owner)
	mf := loadFleetManifest(target)
	repos, err = f.Apply(repos, mf)
	if err != nil {
		return err
	}

	tasks := make([]fleet.RepoTask, len(repos))
	for i, r := range repos {
		tasks[i] = fleet.RepoTask{
			RepoName: r.Name,
			ID:       r.FullName,
			FullName: r.FullName,
		}
	}

	return execPRs(ctx, conf, prov, owner, tasks, state, author, noDraft, format)
}

func execPRs(ctx context.Context, conf *Conf, prov provider.Provider, owner string, tasks []fleet.RepoTask, state, author string, noDraft bool, format string) error {
	exec := fleet.NewExecutor(fleet.ExecutorConfig{
		Concurrency: conf.Fleet.Concurrent,
		Progress:    conf.UI.Progress && format != "json",
		ProgressConfig: fleet.ProgressConfig{
			Description: "Fetching pull requests",
		},
	})

	result := exec.Run(ctx, tasks, func(ctx context.Context, task fleet.RepoTask) (any, error) {
		prs, err := prov.ListPullRequests(ctx, owner, task.RepoName, provider.ListPROptions{
			State: state,
			Sort:  "updated",
			Limit: 0,
		})
		if err != nil {
			return nil, err
		}

		if author != "" {
			filtered := prs[:0]
			for _, pr := range prs {
				if pr.Author == author {
					filtered = append(filtered, pr)
				}
			}
			prs = filtered
		}

		if noDraft {
			filtered := prs[:0]
			for _, pr := range prs {
				if !pr.Draft {
					filtered = append(filtered, pr)
				}
			}
			prs = filtered
		}

		if len(prs) == 0 {
			return nil, &fleet.SkipError{Reason: "no matching PRs"}
		}

		return prs, nil
	})

	switch format {
	case "json":
		return outputPRsJSON(result)
	default:
		return outputPRsTable(result)
	}
}

type prRow struct {
	Repo string                `json:"repo"`
	PR   *provider.PullRequest `json:"pr"`
}

func outputPRsTable(result *fleet.BulkResult) error {
	tbl := ui.NewTable("Repo", "Pull Request", "Author", "CI", "Review")
	totalPRs := 0
	reposWithPRs := 0

	rows := make([]fleet.RepoResult, len(result.Results))
	copy(rows, result.Results)
	sort.Slice(rows, func(i, j int) bool { return rows[i].Task.RepoName < rows[j].Task.RepoName })

	for i := range rows {
		r := &rows[i]
		if r.Status == fleet.StatusFailed {
			continue
		}
		if prs, ok := r.Payload.([]*provider.PullRequest); ok {
			if len(prs) > 0 {
				reposWithPRs++
			}
			for _, pr := range prs {
				ci := ciDisplay(pr.CIStatus)
				review := reviewDisplay(pr.ReviewStatus)
				title := pr.Title
				if pr.Draft {
					title = "[dim]draft[/] " + title
				}
				tbl.AddRow(
					fmt.Sprintf("[bold]%s[/]", r.Task.RepoName),
					title,
					pr.Author,
					ci,
					review,
				)
				totalPRs++
			}
		}
	}

	ui.DefaultConsole.Render(tbl)
	ui.PrintDim(fmt.Sprintf("%d open PRs across %d repos", totalPRs, reposWithPRs))
	return nil
}

func outputPRsJSON(result *fleet.BulkResult) error {
	out := make([]prRow, 0)
	for i := range result.Results {
		r := &result.Results[i]
		if r.Status == fleet.StatusFailed {
			continue
		}
		if prs, ok := r.Payload.([]*provider.PullRequest); ok {
			for _, pr := range prs {
				out = append(out, prRow{Repo: r.Task.RepoName, PR: pr})
			}
		}
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Print(string(data))
	return nil
}

func ciDisplay(s provider.CIStatus) string {
	switch s {
	case provider.CISuccess:
		return "[green]✓[/]"
	case provider.CIPending:
		return "[yellow]…[/]"
	case provider.CIFailure, provider.CIError:
		return "[red]✗[/]"
	default:
		return "[dim]?[/]"
	}
}

func reviewDisplay(s provider.ReviewStatus) string {
	switch s {
	case provider.ReviewApproved:
		return "[green]approved[/]"
	case provider.ReviewChangesRequested:
		return "[yellow]changes[/]"
	case provider.ReviewCommented:
		return "[dim]commented[/]"
	case provider.ReviewDismissed:
		return "[dim]dismissed[/]"
	default:
		return "[dim]pending[/]"
	}
}
