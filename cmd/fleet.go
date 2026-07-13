package cmd

import (
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/depado/minifleet/internal/fleet"
	"github.com/depado/minifleet/internal/manifest"
)

// fleetTarget is a single fleet directory + its owner. Commands operate on one
// or more fleetTarget values returned by resolveFleet / discoverFleets.
type fleetTarget struct {
	Owner string
	Dir   string
}

// resolveFleet returns the directory to operate on for a command with an owner.
// Resolution order:
//  1. --fleet.path (explicit one-shot override)
//  2. CWD (whether or not fleet.yml exists)
//  3. known_fleets[owner]
//
// Returns an empty target when no directory can be resolved.
func resolveFleet(conf *Conf, host, owner string) (fleetTarget, bool) {
	// 1. --fleet.path
	if conf.Fleet.Path != "" {
		dir := expandPath(conf.Fleet.Path)
		slog.Debug("resolving fleet from --fleet.path", "dir", dir)
		mf, _ := manifest.Load(manifest.Path(dir))
		resolvedOwner := owner
		if mf != nil && mf.Owner != "" {
			resolvedOwner = mf.Owner
		}
		return fleetTarget{Owner: resolvedOwner, Dir: dir}, mf != nil
	}

	// 2. CWD
	if cwd, err := os.Getwd(); err == nil {
		mf, _ := manifest.Load(manifest.Path(cwd))
		if mf == nil || owner == "" || mf.Owner == owner {
			resolvedOwner := owner
			if mf != nil && mf.Owner != "" {
				resolvedOwner = mf.Owner
			}
			slog.Debug("resolving fleet from current directory", "dir", cwd, "owner", resolvedOwner)
			return fleetTarget{Owner: resolvedOwner, Dir: cwd}, mf != nil
		}
	}

	// 3. known_fleets
	if owner != "" && conf.Fleet.KnownFleets != nil {
		if dir, ok := conf.Fleet.KnownFleets[owner]; ok && dir != "" {
			slog.Debug("resolving fleet from known_fleets", "owner", owner, "dir", dir)
			return fleetTarget{Owner: owner, Dir: dir}, fileExists(manifest.Path(dir))
		}
	}

	return fleetTarget{}, false
}

// discoverFleets returns the fleet targets a no-owner command (status, run)
// should operate on. Order:
//  1. --fleet.path
//  2. CWD if it contains fleet.yml
//  3. all known_fleets (sorted by owner)
func discoverFleets(conf *Conf) []fleetTarget {
	// 1. --fleet.path
	if conf.Fleet.Path != "" {
		dir := expandPath(conf.Fleet.Path)
		slog.Debug("discovering fleets from --fleet.path", "dir", dir)
		owner := ""
		if mf, _ := manifest.Load(manifest.Path(dir)); mf != nil {
			owner = mf.Owner
		}
		return []fleetTarget{{Owner: owner, Dir: dir}}
	}

	// 2. CWD has fleet.yml
	if cwd, err := os.Getwd(); err == nil {
		if mf, err := manifest.Load(manifest.Path(cwd)); err == nil && mf != nil {
			slog.Debug("discovering fleets from current directory fleet.yml", "dir", cwd, "owner", mf.Owner)
			return []fleetTarget{{Owner: mf.Owner, Dir: cwd}}
		}
	}

	// 3. known_fleets
	if len(conf.Fleet.KnownFleets) == 0 {
		slog.Debug("no known_fleets configured")
		return nil
	}
	owners := make([]string, 0, len(conf.Fleet.KnownFleets))
	for k := range conf.Fleet.KnownFleets {
		owners = append(owners, k)
	}
	sort.Strings(owners)
	out := make([]fleetTarget, 0, len(owners))
	for _, o := range owners {
		dir := conf.Fleet.KnownFleets[o]
		if dir == "" {
			continue
		}
		if !fileExists(manifest.Path(dir)) {
			slog.Debug("skipping known fleet: fleet.yml not found", "owner", o, "dir", dir)
			continue
		}
		out = append(out, fleetTarget{Owner: o, Dir: dir})
	}
	slog.Debug("discovered fleets", "count", len(out))
	return out
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

// loadFleetManifest loads the manifest for a target. Errors are tolerated — a
// fleet directory without a manifest (e.g. created mid-sync) returns nil.
func loadFleetManifest(t fleetTarget) *manifest.FleetManifest {
	mf, _ := manifest.Load(manifest.Path(t.Dir))
	return mf
}

// reposForTarget returns the repo tasks for a fleet target. Manifest-first:
// uses manifest repos when available; falls back to filesystem scan.
func reposForTarget(t fleetTarget, f Filters) ([]fleet.RepoTask, error) {
	tasks, err := manifestToTasks(t, f)
	if err != nil {
		return nil, err
	}
	if len(tasks) > 0 {
		return tasks, nil
	}

	mf := loadFleetManifest(t)

	scanned, err := fleet.Scan(t.Dir, f.IncludeRegex, mf)
	if err != nil {
		return nil, err
	}

	tasksWithName := make([]taskWithName, len(scanned))
	for i, tk := range scanned {
		tasksWithName[i] = taskWithName{RepoName: tk.RepoName, FullName: tk.FullName, ID: tk.ID, Dir: tk.Dir}
	}
	tasksWithName, err = f.ApplyTasks(tasksWithName, mf)
	if err != nil {
		return nil, err
	}

	out := make([]fleet.RepoTask, len(tasksWithName))
	for i, tn := range tasksWithName {
		out[i] = fleet.RepoTask{RepoName: tn.RepoName, ID: tn.ID, FullName: tn.FullName, Dir: tn.Dir}
	}
	return out, nil
}

// manifestToTasks converts a fleet manifest's repos into RepoTasks with
// optional filtering. Returns nil (not error) when the manifest is absent.
func manifestToTasks(t fleetTarget, f Filters) ([]fleet.RepoTask, error) {
	mf := loadFleetManifest(t)
	if mf == nil {
		return nil, nil
	}

	tasks := make([]taskWithName, 0, len(mf.Repos))
	for _, r := range mf.Repos {
		name := r.FullName
		if i := strings.LastIndexByte(name, '/'); i >= 0 {
			name = name[i+1:]
		}
		tasks = append(tasks, taskWithName{
			RepoName: name,
			FullName: r.FullName,
			ID:       r.FullName,
			Dir:      filepath.Join(t.Dir, name),
		})
	}

	filtered, err := f.ApplyTasks(tasks, mf)
	if err != nil {
		return nil, err
	}

	out := make([]fleet.RepoTask, 0, len(filtered))
	for _, tn := range filtered {
		out = append(out, fleet.RepoTask{
			RepoName: tn.RepoName,
			ID:       tn.ID,
			FullName: tn.FullName,
			Dir:      tn.Dir,
		})
	}
	return out, nil
}
