package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/depado/minifleet/internal/manifest"
	"github.com/depado/minifleet/internal/provider"
	"github.com/depado/minifleet/internal/provider/github"
	"github.com/depado/minifleet/internal/ui"
)

func newDiscoverCmd() *cobra.Command {
	var filters Filters

	cmd := &cobra.Command{
		Use:   "discover <owner>",
		Short: "Fetch repositories from GitHub and create or update fleet.yml",
		Long:  "Discover repositories from the GitHub API for the given owner, merge with any existing fleet.yml, and save. Does not clone or pull repositories — use 'sync' for that.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conf, err := confFromCtx(cmd)
			if err != nil {
				return err
			}

			owner := args[0]
			ctx := cmd.Context()
			prov, err := github.New(conf.GitHub.Token, conf.GitHub.Host)
			if err != nil {
				return err
			}

			return discoverOne(ctx, conf, prov, owner, filters)
		},
	}

	addFilterFlags(cmd, &filters)

	return cmd
}

func discoverOne(ctx context.Context, conf *Conf, prov provider.Provider, owner string, f Filters) error {
	host := prov.Host()
	target, _ := resolveFleet(conf, host, owner)

	if target.Dir == "" {
		return fmt.Errorf("could not resolve fleet directory for %s (no --fleet.path, CWD, or known_fleets entry)", owner)
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

	mf := loadFleetManifest(target)

	repos, err = f.Apply(repos, mf)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		ui.PrintDim(fmt.Sprintf("No repositories found for %s", owner))
		return nil
	}

	mf = manifest.Merge(mf, owner, repos)

	if err := os.MkdirAll(target.Dir, 0o755); err != nil {
		return fmt.Errorf("create fleet dir: %w", err)
	}
	if err := manifest.Save(mf, manifest.Path(target.Dir)); err != nil {
		return fmt.Errorf("save manifest: %w", err)
	}
	if err := RegisterFleet(conf, owner, target.Dir); err != nil {
		ui.PrintDim(fmt.Sprintf("warning: could not register fleet in config: %v", err))
	}

	ui.PrintInfo(fmt.Sprintf("Discovered %d repositories for %s in %s", len(repos), owner, target.Dir))
	return nil
}
