package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"

	"github.com/depado/minifleet/internal/fleet"
	"github.com/depado/minifleet/internal/manifest"
	"github.com/depado/minifleet/internal/provider"
)

// fleetTarget is a single fleet directory + its owner. Commands operate on one
// or more fleetTarget values returned by resolveFleet / discoverFleets.
type fleetTarget struct {
	Owner string
	Dir   string
}

// resolveFleet returns the directory to operate on for a command with an owner.
// Resolution order:
//  1. --path (explicit directory override)
//  2. current directory if its fleet.yml matches the owner
//  3. known_fleets[owner]
//  4. current directory without fleet.yml (e.g. discover into a fresh directory)
//
// Returns an empty target when no directory can be resolved.
func resolveFleet(conf *Conf, host, owner string) (fleetTarget, bool) {
	// 1. --path
	if conf.Path != "" {
		dir := expandPath(conf.Path)
		slog.Debug("resolving fleet from --path", "dir", dir)
		mf, _ := manifest.Load(manifest.Path(dir))
		resolvedOwner := owner
		if mf != nil && mf.Owner != "" {
			resolvedOwner = mf.Owner
		}
		return fleetTarget{Owner: resolvedOwner, Dir: dir}, mf != nil
	}

	// 2. current directory with a matching fleet.yml
	var cwdFallback fleetTarget
	if cwd, err := os.Getwd(); err == nil {
		mf, _ := manifest.Load(manifest.Path(cwd))
		if mf != nil && (owner == "" || mf.Owner == owner) {
			resolvedOwner := owner
			if mf.Owner != "" {
				resolvedOwner = mf.Owner
			}
			slog.Debug("resolving fleet from current directory", "dir", cwd, "owner", resolvedOwner)
			return fleetTarget{Owner: resolvedOwner, Dir: cwd}, true
		}
		if mf == nil {
			cwdFallback = fleetTarget{Owner: owner, Dir: cwd}
		}
	}

	// 3. known_fleets
	if owner != "" && conf.Fleets != nil {
		if dir, ok := conf.Fleets[owner]; ok && dir != "" {
			slog.Debug("resolving fleet from known_fleets", "owner", owner, "dir", dir)
			return fleetTarget{Owner: owner, Dir: dir}, fileExists(manifest.Path(dir))
		}
	}

	// 4. current directory without a manifest
	if cwdFallback.Dir != "" {
		slog.Debug("resolving fleet from current directory (no manifest)", "dir", cwdFallback.Dir, "owner", owner)
		return cwdFallback, false
	}

	return fleetTarget{}, false
}

// discoverFleets returns the fleet targets a no-owner command (status, run)
// should operate on. Order:
//  1. --path (overrides everything, even --all)
//  2. current directory if it contains fleet.yml (--all bypasses this)
//  3. all known_fleets (sorted by owner)
func discoverFleets(conf *Conf, all bool) []fleetTarget {
	// 1. --path (overrides everything, even --all)
	if conf.Path != "" {
		dir := expandPath(conf.Path)
		slog.Debug("discovering fleets from --path", "dir", dir)
		owner := ""
		if mf, _ := manifest.Load(manifest.Path(dir)); mf != nil {
			owner = mf.Owner
		}
		return []fleetTarget{{Owner: owner, Dir: dir}}
	}

	if !all {
		// 2. current directory has fleet.yml
		if cwd, err := os.Getwd(); err == nil {
			if mf, err := manifest.Load(manifest.Path(cwd)); err == nil && mf != nil {
				slog.Debug("discovering fleets from current directory fleet.yml", "dir", cwd, "owner", mf.Owner)
				return []fleetTarget{{Owner: mf.Owner, Dir: cwd}}
			}
		}
	}

	// 3. known_fleets
	if len(conf.Fleets) == 0 {
		slog.Debug("no known_fleets configured")
		return nil
	}
	owners := make([]string, 0, len(conf.Fleets))
	for k := range conf.Fleets {
		owners = append(owners, k)
	}
	sort.Strings(owners)
	out := make([]fleetTarget, 0, len(owners))
	for _, o := range owners {
		dir := conf.Fleets[o]
		if dir == "" {
			continue
		}
		if !fileExists(manifest.Path(dir)) {
			slog.Debug("skipping fleet: fleet.yml not found", "owner", o, "dir", dir)
			continue
		}
		out = append(out, fleetTarget{Owner: o, Dir: dir})
	}
	slog.Debug("discovered fleets", "count", len(out))
	return out
}

// fleetTitle renders a human-readable table title for a fleet target.
func fleetTitle(t fleetTarget) string {
	if t.Owner != "" {
		return fmt.Sprintf("[bold]%s[/] [dim]%s[/]", t.Owner, t.Dir)
	}
	return fmt.Sprintf("[dim]%s[/]", t.Dir)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// expandPath expands a leading ~/ to the user's home directory. Other paths
// are returned as-is.
func expandPath(p string) string {
	if len(p) >= 2 && p[:2] == "~/" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}

// loadFleetManifest loads the manifest for a target. Returns nil when the
// file is absent; logs a warning on parse errors (corrupt YAML).
func loadFleetManifest(t fleetTarget) *manifest.FleetManifest {
	path := manifest.Path(t.Dir)
	mf, err := manifest.Load(path)
	if err != nil && !os.IsNotExist(err) {
		slog.Warn("loading fleet manifest", "path", path, "error", err)
		return nil
	}
	return mf
}

// reposForTarget returns the repo tasks for a fleet target. Manifest-first:
// uses manifest repos when available; falls back to filesystem scan.
func reposForTarget(ctx context.Context, t fleetTarget, f Filters) ([]fleet.RepoTask, error) {
	tasks, err := manifestToTasks(ctx, t, f)
	if err != nil {
		return nil, err
	}
	if len(tasks) > 0 {
		return tasks, nil
	}

	mf := loadFleetManifest(t)

	scanned, err := fleet.Scan(ctx, t.Dir, f.IncludeRegex, mf)
	if err != nil {
		return nil, err
	}

	tasksWithName := make([]taskWithName, len(scanned))
	for i, tk := range scanned {
		tasksWithName[i] = taskWithName{RepoName: tk.RepoName, FullName: tk.FullName, ID: tk.ID, Dir: tk.Dir}
	}
	tasksWithName, err = f.ApplyTasks(ctx, tasksWithName, mf)
	if err != nil {
		return nil, err
	}

	return toRepoTasks(tasksWithName), nil
}

// manifestToTasks converts a fleet manifest's repos into RepoTasks with
// optional filtering. Returns nil (not error) when the manifest is absent.
func manifestToTasks(ctx context.Context, t fleetTarget, f Filters) ([]fleet.RepoTask, error) {
	mf := loadFleetManifest(t)
	if mf == nil {
		return nil, nil
	}

	tasks := make([]taskWithName, 0, len(mf.Repos))
	for _, r := range mf.Repos {
		name := fleet.ShortName(r.FullName)
		tasks = append(tasks, taskWithName{
			RepoName: name,
			FullName: r.FullName,
			ID:       r.FullName,
			Dir:      filepath.Join(t.Dir, name),
		})
	}

	filtered, err := f.ApplyTasks(ctx, tasks, mf)
	if err != nil {
		return nil, err
	}

	return toRepoTasks(filtered), nil
}

func toRepoTasks(tasks []taskWithName) []fleet.RepoTask {
	out := make([]fleet.RepoTask, len(tasks))
	for i, t := range tasks {
		out[i] = fleet.RepoTask{RepoName: t.RepoName, ID: t.ID, FullName: t.FullName, Dir: t.Dir}
	}
	return out
}

func fetchReposFromAPI(ctx context.Context, conf *Conf, prov provider.Provider, owner string, f Filters) ([]*provider.Repo, *manifest.FleetManifest, error) {
	isOrg, err := prov.DetectOwner(ctx, owner)
	if err != nil {
		return nil, nil, fmt.Errorf("detect owner: %w", err)
	}

	repos, err := prov.ListRepos(ctx, owner, provider.ListOptions{
		Visibility: f.Visibility,
		IsOrg:      isOrg,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("list repos: %w", err)
	}

	target, _ := resolveFleet(conf, prov.Host(), owner)
	mf := loadFleetManifest(target)
	repos, err = f.Apply(repos, mf)
	if err != nil {
		return nil, nil, err
	}

	return repos, mf, nil
}
