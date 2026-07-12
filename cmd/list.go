package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

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
		Short: "List repositories from GitHub",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conf, err := confFromCtx(cmd)
			if err != nil {
				return err
			}

			owner := args[0]
			ctx := cmd.Context()
			prov := github.New(conf.GitHub.Token, conf.GitHub.Host)

			isOrg, err := prov.DetectOwner(ctx, owner)
			if err != nil {
				return fmt.Errorf("detect owner: %w", err)
			}

			repos, err := prov.ListRepos(ctx, owner, provider.ListOptions{
				Visibility: filters.Visibility,
				IsOrg:      isOrg,
			})
			if err != nil {
				return fmt.Errorf("list repos: %w", err)
			}

			// Load the per-owner manifest for filter context (labels/groups).
			target, _ := resolveFleet(conf, prov.Host(), owner)
			mf := loadFleetManifest(target)
			repos, err = filters.Apply(repos, mf)
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
		},
	}

	addFilterFlags(cmd, &filters)
	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format: table, json, yaml")
	cmd.Flags().IntVar(&limit, "limit", 1000, "max repos to list")

	return cmd
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