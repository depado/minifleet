package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/depado/gorich/progress"
	"github.com/spf13/cobra"

	"github.com/depado/minifleet/internal/manifest"
	"github.com/depado/minifleet/internal/provider"
	"github.com/depado/minifleet/internal/provider/github"
)

func newDiscoverCmd() *cobra.Command {
	var (
		filters    Filters
		noRegister bool
	)

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

			plan := planFromCtx(cmd.Context())
			ApplyPlan(&filters, plan, cmd)

			owner := args[0]
			ctx := cmd.Context()
			prov, err := github.New(conf.GitHub.Token, conf.GitHub.Host)
			if err != nil {
				return err
			}

			return discoverOne(ctx, conf, prov, owner, filters, noRegister)
		},
	}

	addFilterFlags(cmd, &filters)
	cmd.Flags().BoolVar(&noRegister, "no-register", false, "do not register this fleet in config (fleets)")

	return cmd
}

func discoverOne(ctx context.Context, conf *Conf, prov provider.Provider, owner string, f Filters, noRegister bool) error {
	host := prov.Host()
	target, _ := resolveFleet(conf, host, owner)

	if target.Dir == "" {
		return fmt.Errorf("could not resolve fleet directory for %s (no --path, current directory, or known_fleets entry)", owner)
	}

	isOrg, err := prov.DetectOwner(ctx, owner)
	if err != nil {
		return fmt.Errorf("detect owner: %w", err)
	}

	var repos []*provider.Repo
	var p *progress.Progress
	var tid progress.TaskID
	if !conf.Console.IsTerminal() {
		slog.Info("discovering", "owner", owner)
	}
	if conf.Console.IsTerminal() {
		p = progress.New(
			progress.WithColumns(
				progress.NewSpinnerColumn(progress.WithSpinnerName("dots")),
				progress.DescriptionColumn(),
			),
		)
		p.Start(ctx)
		tid = p.AddTask(fmt.Sprintf("Discovering repositories for %s", owner), nil)
		defer p.Stop()
	}
	repos, err = prov.ListRepos(ctx, owner, provider.ListOptions{Visibility: f.Visibility, IsOrg: isOrg})
	if err != nil {
		return err
	}

	mf := loadFleetManifest(target)

	repos, err = f.Apply(repos, mf)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		if p != nil {
			p.Done(tid, fmt.Sprintf("[dim]No repositories found for %s[/]", owner))
		} else {
			conf.PrintDim(fmt.Sprintf("No repositories found for %s", owner))
		}
		return nil
	}

	mf = manifest.Merge(mf, owner, repos)
	mf.NoRegister = noRegister

	if err := os.MkdirAll(target.Dir, 0o755); err != nil {
		return fmt.Errorf("create fleet dir: %w", err)
	}
	if err := manifest.Save(mf, manifest.Path(target.Dir)); err != nil {
		return fmt.Errorf("save manifest: %w", err)
	}
	if !noRegister {
		if err := RegisterFleet(conf, owner, target.Dir); err != nil {
			conf.PrintDim(fmt.Sprintf("warning: could not register fleet in config: %v", err))
		}
	}

	if p != nil {
		p.Done(tid, fmt.Sprintf("Discovered [bold]%d[/] repositories for [bold]%s[/] [dim]%s[/]", len(repos), owner, target.Dir))
	} else {
		conf.PrintInfo(fmt.Sprintf("Discovered %d repositories for %s %s", len(repos), owner, target.Dir))
	}
	return nil
}
