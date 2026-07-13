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
		format  string
		limit   int
	)

	cmd := &cobra.Command{
		Use:   "list [owner]",
		Short: "List repositories from the manifest, or from GitHub if no manifest exists",
		Long:  "Without an owner, lists repositories from the fleet in CWD (or all known fleets).\nWith an owner, uses the local manifest; falls back to fetching from the API if no manifest exists.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conf, err := confFromCtx(cmd)
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			if len(args) == 0 {
				return listAll(ctx, conf, filters, format)
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
				return listFromAPI(ctx, conf, owner, filters, format, limit)
			}

			mf := loadFleetManifest(target)
			return outputManifestTable(tasks, mf, format)
		},
	}

	addFilterFlags(cmd, &filters)
	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format: table, json, yaml")
	cmd.Flags().IntVar(&limit, "limit", 1000, "max repos to list")

	return cmd
}

func listAll(ctx context.Context, conf *Conf, f Filters, format string) error {
	targets := discoverFleets(conf)
	if len(targets) == 0 {
		ui.PrintDim("No fleet in CWD and no known fleets. Run 'minifleet discover <owner>' first.")
		return nil
	}

	for _, t := range targets {
		tasks, err := manifestToTasks(t, f)
		if err != nil {
			return err
		}
		mf := loadFleetManifest(t)
		if err := outputManifestTable(tasks, mf, format); err != nil {
			return err
		}
	}
	return nil
}

func listFromAPI(ctx context.Context, conf *Conf, owner string, f Filters, format string, limit int) error {
	prov, err := github.New(conf.GitHub.Token, conf.GitHub.Host)
	if err != nil {
		return err
	}

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

	if limit > 0 && len(repos) > limit {
		repos = repos[:limit]
	}

	switch format {
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

func outputManifestTable(tasks []fleet.RepoTask, mf *manifest.FleetManifest, format string) error {
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

	tbl := ui.NewTable("Name", "Language", "Updated", "Archived", "Fork", "Topics")
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
