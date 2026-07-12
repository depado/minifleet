package cmd

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/depado/minifleet/internal/manifest"
)

// fleetTarget is a single fleet directory + its owner. Commands operate on one
// or more fleetTarget values returned by resolveFleet / discoverFleets.
type fleetTarget struct {
	Owner string
	Dir   string
}

// resolveFleet returns the directory to operate on for `sync <owner>` (or an
// owner-less command acting on CWD). Resolution order:
//  1. --fleet.path (explicit one-shot override; dir must already exist or be creatable)
//  2. CWD if it contains a fleet.yml with matching owner (or any owner when
//     owner is "")
//  3. known_fleets[owner]
//  4. default: {base}/{host}/{owner} (created on first sync)
//
// Returns the dir, the owner (echoed back when known), and whether the fleet
// file already existed when returned.
func resolveFleet(conf *Conf, host, owner string) (fleetTarget, bool) {
	// 1. --fleet.path
	if conf.Fleet.Path != "" {
		dir := expandPath(conf.Fleet.Path)
		mf, _ := manifest.Load(manifest.Path(dir))
		resolvedOwner := owner
		if mf != nil && mf.Owner != "" {
			resolvedOwner = mf.Owner
		}
		return fleetTarget{Owner: resolvedOwner, Dir: dir}, mf != nil
	}

	// 2. CWD has fleet.yml
	if cwd, err := os.Getwd(); err == nil {
		if mf, err := manifest.Load(manifest.Path(cwd)); err == nil && mf != nil {
			if owner == "" || mf.Owner == owner {
				return fleetTarget{Owner: mf.Owner, Dir: cwd}, true
			}
		}
	}

	// 3. known_fleets
	if owner != "" && conf.Fleet.KnownFleets != nil {
		if dir, ok := conf.Fleet.KnownFleets[owner]; ok && dir != "" {
			return fleetTarget{Owner: owner, Dir: dir}, fileExists(manifest.Path(dir))
		}
	}

	// 4. default layout
	if owner != "" {
		dir := filepath.Join(conf.Fleet.Base, host, owner)
		return fleetTarget{Owner: owner, Dir: dir}, fileExists(manifest.Path(dir))
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
		owner := ""
		if mf, _ := manifest.Load(manifest.Path(dir)); mf != nil {
			owner = mf.Owner
		}
		return []fleetTarget{{Owner: owner, Dir: dir}}
	}

	// 2. CWD has fleet.yml
	if cwd, err := os.Getwd(); err == nil {
		if mf, err := manifest.Load(manifest.Path(cwd)); err == nil && mf != nil {
			return []fleetTarget{{Owner: mf.Owner, Dir: cwd}}
		}
	}

	// 3. known_fleets
	if len(conf.Fleet.KnownFleets) == 0 {
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
			continue
		}
		out = append(out, fleetTarget{Owner: o, Dir: dir})
	}
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

