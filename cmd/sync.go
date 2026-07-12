package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/depado/minifleet/internal/fleet"
	"github.com/depado/minifleet/internal/git"
	"github.com/depado/minifleet/internal/manifest"
	"github.com/depado/minifleet/internal/provider"
	"github.com/depado/minifleet/internal/provider/github"
	"github.com/depado/minifleet/internal/ui"
)

func newSyncCmd() *cobra.Command {
	var filters Filters

	cmd := &cobra.Command{
		Use:   "sync [owner]",
		Short: "Clone missing repos and pull existing ones",
		Long:  "Sync repositories for a GitHub user or organization.\nIf no owner is given, syncs all owners in the fleet manifest.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conf, err := confFromCtx(cmd)
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			prov := github.New(conf.GitHub.Token, conf.GitHub.Host)

			if len(args) == 0 {
				return syncAll(ctx, conf, prov, filters)
			}
			return syncOne(ctx, conf, prov, args[0], filters)
		},
	}

	addFilterFlags(cmd, &filters)

	return cmd
}

func syncAll(ctx context.Context, conf *Conf, prov provider.Provider, f Filters) error {
	mf, _ := manifest.Load(manifest.ManifestPath())
	if mf == nil || len(mf.Owners) == 0 {
		ui.PrintDim("No owners in fleet manifest. Run 'minifleet sync <owner>' first.")
		return nil
	}

	for _, owner := range mf.OwnerNames() {
		if err := syncOne(ctx, conf, prov, owner, f); err != nil {
			return err
		}
	}
	return nil
}

func syncOne(ctx context.Context, conf *Conf, prov provider.Provider, owner string, f Filters) error {
	mf, _ := manifest.Load(manifest.ManifestPath())

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

	repos, err = f.Apply(repos, mf)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		ui.PrintDim(fmt.Sprintf("No repositories found for %s", owner))
		return nil
	}

	mf = manifest.Merge(mf, owner, repos)
	idx := mf.Index()

	ownerDir := computeOwnerDir(conf, prov.Host(), owner, mf)
	shallow := conf.Fleet.Shallow

	tasks := make([]fleet.RepoTask, len(repos))
	repoDirs := make(map[string]string, len(repos))
	for i, r := range repos {
		tasks[i] = fleet.RepoTask{RepoName: r.Name, ID: r.FullName, FullName: r.FullName}
		repoDirs[r.FullName] = repoDir(r.FullName, ownerDir, idx)
	}

	exec := fleet.NewExecutor(fleet.ExecutorConfig{
		Concurrency: conf.Fleet.Concurrent,
		Progress:    conf.UI.Progress,
		ProgressConfig: fleet.ProgressConfig{
			Description: fmt.Sprintf("Syncing %s", owner),
		},
	})

	result := exec.Run(ctx, tasks, func(ctx context.Context, task fleet.RepoTask) (any, error) {
		dir := repoDirs[task.ID]

		if git.IsRepo(dir) {
			if mr := idx[task.ID]; mr != nil && mr.Ignored {
				return nil, &fleet.SkipError{Reason: "ignored"}
			}
			return nil, git.Pull(ctx, dir)
		}

		if mr := idx[task.ID]; mr != nil && mr.Ignored {
			return nil, &fleet.SkipError{Reason: "ignored"}
		}

		saved := ""
		if mr := idx[task.ID]; mr != nil {
			saved = mr.Protocol
		}
		if saved != "" {
			url := prov.CloneURL(saved, task.ID)
			if err := git.Clone(ctx, url, dir, shallow); err != nil {
				return nil, err
			}
			setRepoPath(idx, task.ID, dir)
			return nil, nil
		}

		sshU := prov.CloneURL("ssh", task.ID)
		httpsU := prov.CloneURL("https", task.ID)

		protocol := "ssh"
		if err := git.Clone(ctx, sshU, dir, shallow); err != nil {
			if err := git.Clone(ctx, httpsU, dir, shallow); err != nil {
				return nil, err
			}
			protocol = "https"
		}
		setProtocol(idx, task.ID, protocol)
		setRepoPath(idx, task.ID, dir)
		return nil, nil
	})

	if result.Succeeded > 0 || result.Skipped > 0 {
		if err := os.MkdirAll(ownerDir, 0o755); err != nil {
			return fmt.Errorf("create dir: %w", err)
		}
		_ = manifest.Save(mf, manifest.ManifestPath())
	}

	printBulkSummary(result, false)
	return nil
}

// computeOwnerDir resolves where this owner's repos are cloned.
// When --path is set, it bypasses host/owner nesting entirely.
func computeOwnerDir(conf *Conf, host, owner string, mf *manifest.FleetManifest) string {
	if conf.Fleet.Path != "" {
		return expandPath(conf.Fleet.Path)
	}
	defaultOwnerDir := filepath.Join(conf.Fleet.Base, host, owner)
	if entry, ok := mf.Owners[owner]; ok && entry.Path != "" {
		return expandPath(entry.Path)
	}
	return defaultOwnerDir
}

func repoDir(fullName, defaultOwnerDir string, idx map[string]*manifest.ManifestRepo) string {
	if r := idx[fullName]; r != nil && r.Path != "" {
		return expandPath(r.Path)
	}
	parts := strings.SplitN(fullName, "/", 2)
	return filepath.Join(defaultOwnerDir, parts[len(parts)-1])
}

func expandPath(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}

func setProtocol(idx map[string]*manifest.ManifestRepo, fullName, protocol string) {
	if r := idx[fullName]; r != nil {
		r.Protocol = protocol
	}
}

func setRepoPath(idx map[string]*manifest.ManifestRepo, fullName, path string) {
	if r := idx[fullName]; r != nil {
		r.Path = path
	}
}