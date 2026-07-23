package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/depado/minifleet/internal/fleet"
	"github.com/depado/minifleet/internal/manifest"
	"github.com/depado/minifleet/internal/provider"
	"github.com/depado/minifleet/internal/provider/github"
	"github.com/depado/minifleet/internal/ui"
)

func newListCmd() *cobra.Command {
	var (
		filters Filters
		limit   int
	)

	cmd := &cobra.Command{
		Use:   "list [owner]",
		Short: "List repositories from the manifest, or from GitHub if no manifest exists",
		Long:  "Without an owner, lists repositories from the fleet in the current directory (or all known fleets).\nWith an owner, uses the local manifest; falls back to fetching from the API if no manifest exists.\nUse --all to always list all known fleets, ignoring the current directory.",
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
				return listAll(ctx, conf, filters, plan)
			}

			owner := args[0]
			prov, err := github.New(conf.GitHub.Token, conf.GitHub.Host)
			if err != nil {
				return err
			}
			target, _ := resolveFleet(conf, prov.Host(), owner)
			tasks, err := manifestToTasks(ctx, target, filters)
			if err != nil {
				return err
			}
			if len(tasks) == 0 {
				return listFromAPI(ctx, conf, prov, owner, filters, limit)
			}

			mf := loadFleetManifest(target)
			repos := manifestRepos(tasks, mf)
			if sharedJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string][]listRepo{target.Owner: repos})
			}
			renderManifestTable(repos, fleetTitle(target), conf)
			return nil
		},
	}

	addFilterFlags(cmd, &filters)
	cmd.Flags().IntVar(&limit, "limit", 0, "max repos to list (0 = unlimited)")

	return cmd
}

func listAll(ctx context.Context, conf *Conf, f Filters, plan *Plan) error {
	targets, err := planTargets(conf, plan, sharedAll)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		conf.PrintDim("No fleet in the current directory and no known fleets. Run 'minifleet discover <owner>' first.")
		return nil
	}

	if sharedJSON {
		out := make(map[string][]listRepo)
		for _, t := range targets {
			tasks, err := manifestToTasks(ctx, t, f)
			if err != nil {
				return err
			}
			mf := loadFleetManifest(t)
			out[t.Owner] = manifestRepos(tasks, mf)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	for _, t := range targets {
		tasks, err := manifestToTasks(ctx, t, f)
		if err != nil {
			return err
		}
		mf := loadFleetManifest(t)
		repos := manifestRepos(tasks, mf)
		renderManifestTable(repos, fleetTitle(t), conf)
	}
	return nil
}

func listFromAPI(ctx context.Context, conf *Conf, prov provider.Provider, owner string, f Filters, limit int) error {
	repos, _, err := fetchReposFromAPI(ctx, conf, prov, owner, f)
	if err != nil {
		return err
	}

	if limit > 0 && len(repos) > limit {
		repos = repos[:limit]
	}

	if sharedJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(repos)
	}
	renderAPITable(repos, conf)
	return nil
}

type listRepo struct {
	Name     string   `json:"name"`
	FullName string   `json:"full_name,omitempty"`
	Language string   `json:"language,omitempty"`
	Topics   []string `json:"topics,omitempty"`
	Archived bool     `json:"archived"`
	Fork     bool     `json:"fork"`
	Updated  string   `json:"updated,omitempty"`
}

func manifestRepos(tasks []fleet.RepoTask, mf *manifest.FleetManifest) []listRepo {
	idx := mf.Index()
	out := make([]listRepo, 0, len(tasks))
	for _, t := range tasks {
		mr := idx[t.FullName]
		row := listRepo{Name: t.RepoName, FullName: t.FullName}
		if mr != nil {
			row.Language = mr.Language
			row.Topics = mr.Topics
			row.Archived = mr.Archived
			row.Fork = mr.Fork
			if !mr.UpdatedAt.IsZero() {
				row.Updated = mr.UpdatedAt.Format("2006-01-02")
			}
		}
		out = append(out, row)
	}
	return out
}

func renderManifestTable(repos []listRepo, title string, conf *Conf) {
	tbl := ui.NewTitledTable(title, "Name", "Language", "Updated", "Archived", "Fork", "Topics")
	for _, r := range repos {
		archived := "[dim]no[/]"
		fork := "[dim]no[/]"
		if r.Archived {
			archived = "[red]yes[/]"
		}
		if r.Fork {
			fork = "[yellow]yes[/]"
		}
		tbl.AddRow(
			fmt.Sprintf("[bold]%s[/]", r.Name),
			r.Language,
			r.Updated,
			archived,
			fork,
			strings.Join(r.Topics, ", "),
		)
	}
	conf.Console.Render(tbl)
}

func renderAPITable(repos []*provider.Repo, conf *Conf) {
	tbl := ui.NewTable("Name", "Vis.", "Language", "Updated", "Archived", "Fork", "Topics")
	for _, r := range repos {
		archived := "[dim]no[/]"
		if r.Archived {
			archived = "[red]yes[/]"
		}
		fork := "[dim]no[/]"
		if r.Fork {
			fork = "[yellow]yes[/]"
		}
		tbl.AddRow(
			fmt.Sprintf("[bold]%s[/]", r.Name),
			r.Visibility,
			r.Language,
			r.UpdatedAt.Format("2006-01-02"),
			archived,
			fork,
			strings.Join(r.Topics, ", "),
		)
	}
	conf.Console.Render(tbl)
}
