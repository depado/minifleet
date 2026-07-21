package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/depado/minifleet/internal/fleet"
	"github.com/depado/minifleet/internal/git"
	"github.com/depado/minifleet/internal/manifest"
	"github.com/depado/minifleet/internal/provider"
	"github.com/depado/minifleet/internal/provider/github"
	"github.com/depado/minifleet/internal/ui"
)

func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync [owner]",
		Short: "Clone missing repos and pull existing ones",
		Long:  "Sync repositories listed in fleet.yml. Does not fetch from the API — run 'discover' first to create or refresh the manifest.\nUse --all to sync every known fleet, ignoring the current directory.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conf, err := confFromCtx(cmd)
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			prov, err := github.New(conf.GitHub.Token, conf.GitHub.Host)
			if err != nil {
				return err
			}

			var results []fleet.BulkResult
			collect := func(r *fleet.BulkResult) {
				if sharedFormat == "json" {
					results = append(results, *r)
				}
			}

			if sharedAll || len(args) == 0 {
				plan := planFromCtx(ctx)
				err = syncAll(ctx, conf, prov, plan, collect)
			} else {
				err = syncOne(ctx, conf, prov, args[0], collect)
			}
			if err != nil {
				return err
			}

			if sharedFormat == "json" {
				return outputSyncJSON(results)
			}
			return nil
		},
	}

	return cmd
}

func syncAll(ctx context.Context, conf *Conf, prov provider.Provider, plan *Plan, collect func(*fleet.BulkResult)) error {
	targets, err := planTargets(conf, plan, sharedAll)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		if sharedFormat != "json" {
			ui.PrintDim("No fleet in the current directory and no known fleets. Run 'minifleet discover <owner>' first.")
		}
		return nil
	}

	for _, t := range targets {
		if err := syncTarget(ctx, conf, prov, t, collect); err != nil {
			return err
		}
	}
	return nil
}

func syncOne(ctx context.Context, conf *Conf, prov provider.Provider, owner string, collect func(*fleet.BulkResult)) error {
	target, _ := resolveFleet(conf, prov.Host(), owner)

	if target.Dir == "" {
		return fmt.Errorf("could not resolve fleet directory for %s (no --path, current directory, or known_fleets entry)", owner)
	}

	return syncTarget(ctx, conf, prov, target, collect)
}

func syncTarget(ctx context.Context, conf *Conf, prov provider.Provider, target fleetTarget, collect func(*fleet.BulkResult)) error {
	mf := loadFleetManifest(target)

	if mf == nil {
		return fmt.Errorf("no fleet.yml in %s — run 'minifleet discover %s' first", target.Dir, target.Owner)
	}

	return syncFromManifest(ctx, conf, prov, target, mf, collect)
}

type syncJSONResult struct {
	Owner     string           `json:"owner"`
	Total     int              `json:"total"`
	Succeeded int              `json:"succeeded"`
	Skipped   int              `json:"skipped"`
	Failed    int              `json:"failed"`
	Elapsed   string           `json:"elapsed"`
	Results   []syncRepoResult `json:"results,omitempty"`
}

type syncRepoResult struct {
	Repo   string `json:"repo"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

func outputSyncJSON(results []fleet.BulkResult) error {
	out := make([]syncJSONResult, 0, len(results))
	for _, r := range results {
		jr := syncJSONResult{
			Total:     r.Total,
			Succeeded: r.Succeeded,
			Skipped:   r.Skipped,
			Failed:    r.Failed,
			Elapsed:   r.Elapsed.Round(0).String(),
		}
		for _, res := range r.Results {
			status := "succeeded"
			switch res.Status {
			case fleet.StatusSkipped:
				status = "skipped"
			case fleet.StatusFailed:
				status = "failed"
			}
			sr := syncRepoResult{
				Repo:   res.Task.RepoName,
				Status: status,
			}
			if res.Err != nil {
				sr.Error = res.Err.Error()
			}
			jr.Results = append(jr.Results, sr)
		}
		out = append(out, jr)
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Print(string(data))
	return nil
}

func setProtocol(idx map[string]*manifest.ManifestRepo, fullName, protocol string) {
	if r := idx[fullName]; r != nil {
		r.Protocol = protocol
	}
}

func syncFromManifest(ctx context.Context, conf *Conf, prov provider.Provider, t fleetTarget, mf *manifest.FleetManifest, collect func(*fleet.BulkResult)) error {
	idx := mf.Index()
	shallow := conf.Shallow

	if err := os.MkdirAll(t.Dir, 0o755); err != nil {
		return fmt.Errorf("create fleet dir: %w", err)
	}

	tasks := make([]fleet.RepoTask, 0, len(mf.Repos))
	for _, r := range mf.Repos {
		name := fleet.ShortName(r.FullName)
		tasks = append(tasks, fleet.RepoTask{
			RepoName: name,
			ID:       r.FullName,
			FullName: r.FullName,
			Dir:      filepath.Join(t.Dir, name),
		})
	}

	exec := fleet.NewExecutor(fleet.ExecutorConfig{
		Concurrency: conf.Concurrent,
		Progress:    conf.UI.Progress && sharedFormat != "json",
		ProgressConfig: fleet.ProgressConfig{
			Description: fmt.Sprintf("Syncing %s", t.Owner),
		},
	})

	result := exec.Run(ctx, tasks, func(ctx context.Context, task fleet.RepoTask) (any, error) {
		dir := task.Dir

		if git.IsRepo(ctx, dir) {
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
		return nil, nil
	})

	if result.Succeeded > 0 || result.Skipped > 0 {
		if err := manifest.Save(mf, manifest.Path(t.Dir)); err != nil {
			slog.Warn("failed to save manifest after sync", "path", manifest.Path(t.Dir), "error", err)
		}
		if !mf.NoRegister {
			if err := RegisterFleet(conf, t.Owner, t.Dir); err != nil {
				if sharedFormat != "json" {
					ui.PrintDim(fmt.Sprintf("warning: could not register fleet in config: %v", err))
				}
			}
		}
	}

	collect(result)

	if sharedFormat != "json" {
		printBulkSummary(result, false)
	}
	return nil
}
