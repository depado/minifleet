package cmd

import (
	"context"
	"encoding/json"
	"fmt"
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
	var (
		filters Filters
		format  string
	)

	cmd := &cobra.Command{
		Use:   "sync [owner]",
		Short: "Clone missing repos and pull existing ones",
		Long:  "Sync repositories for a GitHub user or organization.\nIf no owner is given, syncs the fleet in CWD (or all known fleets if not in one).",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conf, err := confFromCtx(cmd)
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			prov := github.New(conf.GitHub.Token, conf.GitHub.Host)

			var results []fleet.BulkResult
			collect := func(r *fleet.BulkResult) {
				if format == "json" {
					results = append(results, *r)
				}
			}

			if len(args) == 0 {
				err = syncAll(ctx, conf, prov, filters, format, collect)
			} else {
				err = syncOne(ctx, conf, prov, args[0], filters, format, collect)
			}
			if err != nil {
				return err
			}

			if format == "json" {
				return outputSyncJSON(results)
			}
			return nil
		},
	}

	addFilterFlags(cmd, &filters)
	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format: table, json")

	return cmd
}

func syncAll(ctx context.Context, conf *Conf, prov provider.Provider, f Filters, format string, collect func(*fleet.BulkResult)) error {
	targets := discoverFleets(conf)
	if len(targets) == 0 {
		if format != "json" {
			ui.PrintDim("No fleet in CWD and no known fleets. Run 'minifleet sync <owner>' first.")
		}
		return nil
	}

	for _, t := range targets {
		if err := syncOne(ctx, conf, prov, t.Owner, f, format, collect); err != nil {
			return err
		}
	}
	return nil
}

func syncOne(ctx context.Context, conf *Conf, prov provider.Provider, owner string, f Filters, format string, collect func(*fleet.BulkResult)) error {
	host := prov.Host()
	target, _ := resolveFleet(conf, host, owner)

	if target.Dir == "" {
		return fmt.Errorf("could not resolve fleet directory for %s (no --fleet.path, CWD, or known_fleets entry)", owner)
	}

	mf := loadFleetManifest(target)
	if mf != nil && mf.Owner != "" && mf.Owner != owner {
		dir := filepath.Join(conf.Fleet.Base, host, owner)
		target = fleetTarget{Owner: owner, Dir: dir}
		mf = loadFleetManifest(target)
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

	repos, err = f.Apply(repos, mf)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		if format != "json" {
			ui.PrintDim(fmt.Sprintf("No repositories found for %s", owner))
		}
		return nil
	}

	mf = manifest.Merge(mf, owner, repos)
	idx := mf.Index()

	if err := os.MkdirAll(target.Dir, 0o755); err != nil {
		return fmt.Errorf("create fleet dir: %w", err)
	}

	shallow := conf.Fleet.Shallow

	tasks := make([]fleet.RepoTask, len(repos))
	for i, r := range repos {
		dir := filepath.Join(target.Dir, r.Name)
		tasks[i] = fleet.RepoTask{RepoName: r.Name, ID: r.FullName, FullName: r.FullName}
		tasks[i].Dir = dir
	}

	exec := fleet.NewExecutor(fleet.ExecutorConfig{
		Concurrency: conf.Fleet.Concurrent,
		Progress:    conf.UI.Progress && format != "json",
		ProgressConfig: fleet.ProgressConfig{
			Description: fmt.Sprintf("Syncing %s", owner),
		},
	})

	result := exec.Run(ctx, tasks, func(ctx context.Context, task fleet.RepoTask) (any, error) {
		dir := task.Dir

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
		_ = manifest.Save(mf, manifest.Path(target.Dir))
		if err := RegisterFleet(conf, owner, target.Dir); err != nil {
			if format != "json" {
				ui.PrintDim(fmt.Sprintf("warning: could not register fleet in config: %v", err))
			}
		}
	}

	collect(result)

	if format != "json" {
		printBulkSummary(result, false)
	}
	return nil
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
