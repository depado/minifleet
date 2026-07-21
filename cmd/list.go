package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

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
			return outputManifestTable(tasks, mf, sharedFormat, fleetTitle(target))
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
		ui.PrintDim("No fleet in the current directory and no known fleets. Run 'minifleet discover <owner>' first.")
		return nil
	}

	for _, t := range targets {
		tasks, err := manifestToTasks(ctx, t, f)
		if err != nil {
			return err
		}
		mf := loadFleetManifest(t)
		if err := outputManifestTable(tasks, mf, sharedFormat, fleetTitle(t)); err != nil {
			return err
		}
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

	switch sharedFormat {
	case "json":
		return outputJSON(repos)
	case "yaml":
		mf := manifest.Generate(repos, owner)
		data, err := yaml.Marshal(mf)
		if err != nil {
			return err
		}
		fmt.Print(string(data))
		return nil
	default:
		return outputTable(repos)
	}
}

func outputManifestTable(tasks []fleet.RepoTask, mf *manifest.FleetManifest, format, title string) error {
	idx := mf.Index()

	if format == "json" {
		type jr struct {
			Name     string   `json:"name"`
			FullName string   `json:"full_name,omitempty"`
			Language string   `json:"language,omitempty"`
			Topics   []string `json:"topics,omitempty"`
			Archived bool     `json:"archived"`
			Fork     bool     `json:"fork"`
			Updated  string   `json:"updated,omitempty"`
		}
		out := make([]jr, 0, len(tasks))
		for _, t := range tasks {
			mr := idx[t.FullName]
			row := jr{Name: t.RepoName, FullName: t.FullName}
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
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		fmt.Print(string(data))
		return nil
	}

	if format == "yaml" {
		type yr struct {
			Name     string   `yaml:"name"`
			FullName string   `yaml:"full_name,omitempty"`
			Language string   `yaml:"language,omitempty"`
			Topics   []string `yaml:"topics,omitempty"`
			Archived bool     `yaml:"archived"`
			Fork     bool     `yaml:"fork"`
			Updated  string   `yaml:"updated,omitempty"`
		}
		out := make([]yr, 0, len(tasks))
		for _, t := range tasks {
			mr := idx[t.FullName]
			row := yr{Name: t.RepoName, FullName: t.FullName}
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
		data, err := yaml.Marshal(out)
		if err != nil {
			return err
		}
		fmt.Print(string(data))
		return nil
	}

	tbl := ui.NewTitledTable(title, "Name", "Language", "Updated", "Archived", "Fork", "Topics")
	for _, t := range tasks {
		mr := idx[t.FullName]
		archived := "[dim]no[/]"
		fork := "[dim]no[/]"
		language := ""
		updated := ""
		topics := ""
		if mr != nil {
			if mr.Archived {
				archived = "[red]yes[/]"
			}
			if mr.Fork {
				fork = "[yellow]yes[/]"
			}
			language = mr.Language
			if !mr.UpdatedAt.IsZero() {
				updated = mr.UpdatedAt.Format("2006-01-02")
			}
			topics = strings.Join(mr.Topics, ", ")
		}
		tbl.AddRow(
			fmt.Sprintf("[bold]%s[/]", t.RepoName),
			language,
			updated,
			archived,
			fork,
			topics,
		)
	}
	ui.DefaultConsole.Render(tbl)
	return nil
}

func outputTable(repos []*provider.Repo) error {
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
	ui.DefaultConsole.Render(tbl)
	return nil
}

func outputJSON(repos []*provider.Repo) error {
	data, err := json.MarshalIndent(repos, "", "  ")
	if err != nil {
		return err
	}
	fmt.Print(string(data))
	return nil
}
