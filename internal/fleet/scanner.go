package fleet

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/depado/minifleet/internal/git"
	"github.com/depado/minifleet/internal/manifest"
)

// Scan walks fleetDir one level deep and returns a RepoTask for each
// subdirectory that is a git repository. The manifest (when non-nil) is
// consulted to skip ignored repos and to resolve each repo's full_name for
// filter context.
func Scan(fleetDir string, targetRegex string, mf *manifest.FleetManifest) ([]RepoTask, error) {
	entries, err := os.ReadDir(fleetDir)
	if err != nil {
		return nil, err
	}

	var pattern *regexp.Regexp
	if targetRegex != "" {
		pattern, err = regexp.Compile(targetRegex)
		if err != nil {
			return nil, err
		}
	}

	ignored := ignoredSet(mf)
	byShort := shortToFullName(mf)

	slog.Debug("scanning fleet directory", "dir", fleetDir, "entries", len(entries))

	var tasks []RepoTask
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		repoDir := filepath.Join(fleetDir, name)

		if !git.IsRepo(context.Background(), repoDir) {
			continue
		}
		if pattern != nil && !pattern.MatchString(name) {
			slog.Debug("skipping repo: does not match regex", "repo", name)
			continue
		}
		if isIgnored(ignored, name, byShort[name]) {
			slog.Debug("skipping repo: ignored", "repo", name)
			continue
		}

		id := byShort[name]
		if id == "" {
			id = name
		}
		tasks = append(tasks, RepoTask{
			RepoName: name,
			ID:       id,
			FullName: byShort[name],
			Dir:      repoDir,
		})
	}

	slog.Debug("scan complete", "dir", fleetDir, "tasks", len(tasks))
	return tasks, nil
}

func ignoredSet(mf *manifest.FleetManifest) map[string]struct{} {
	if mf == nil {
		return nil
	}
	set := make(map[string]struct{})
	for _, r := range mf.Repos {
		if r.Ignored {
			set[r.FullName] = struct{}{}
			if short := lastSegment(r.FullName); short != "" {
				set[short] = struct{}{}
			}
		}
	}
	return set
}

func shortToFullName(mf *manifest.FleetManifest) map[string]string {
	if mf == nil {
		return nil
	}
	idx := make(map[string]string)
	for _, r := range mf.Repos {
		short := lastSegment(r.FullName)
		if short != "" {
			idx[short] = r.FullName
		}
	}
	return idx
}

func isIgnored(set map[string]struct{}, shortName, fullName string) bool {
	if _, ok := set[shortName]; ok {
		return true
	}
	if fullName != "" {
		if _, ok := set[fullName]; ok {
			return true
		}
	}
	return false
}

func lastSegment(fullName string) string {
	if i := strings.LastIndexByte(fullName, '/'); i >= 0 {
		return fullName[i+1:]
	}
	return ""
}
