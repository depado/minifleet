package fleet

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/depado/minifleet/internal/git"
	"github.com/depado/minifleet/internal/manifest"
)

// Scan walks baseDir for git repositories. When flat is true, expects repos
// directly under baseDir (one level deep). Otherwise scans the nested layout
// baseDir/{host}/{owner}/{repo}. The manifest is consulted to skip ignored
// repos and to resolve full names.
func Scan(baseDir string, targetRegex string, mf *manifest.FleetManifest, flat bool) ([]RepoTask, error) {
	entries, err := os.ReadDir(baseDir)
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
	byName := nameToFullName(mf)

	var tasks []RepoTask
	if flat {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			repoDir := filepath.Join(baseDir, e.Name())
			if !git.IsRepo(repoDir) {
				continue
			}
			if pattern != nil && !pattern.MatchString(e.Name()) {
				continue
			}
			if _, ok := ignored[e.Name()]; ok {
				continue
			}
			tasks = append(tasks, RepoTask{
				RepoName: e.Name(),
				ID:       repoDir,
			})
		}
		return tasks, nil
	}

	for _, hostEntry := range entries {
		if !hostEntry.IsDir() {
			continue
		}
		hostDir := filepath.Join(baseDir, hostEntry.Name())

		owners, err := os.ReadDir(hostDir)
		if err != nil {
			continue
		}
		for _, ownerEntry := range owners {
			if !ownerEntry.IsDir() {
				continue
			}
			ownerDir := filepath.Join(hostDir, ownerEntry.Name())

			repos, err := os.ReadDir(ownerDir)
			if err != nil {
				continue
			}
			for _, repoEntry := range repos {
				if !repoEntry.IsDir() {
					continue
				}
				repoDir := filepath.Join(ownerDir, repoEntry.Name())

				if !git.IsRepo(repoDir) {
					continue
				}
				if pattern != nil && !pattern.MatchString(repoEntry.Name()) {
					continue
				}
				if _, ok := ignored[repoEntry.Name()]; ok {
					continue
				}
				full := byName[repoEntry.Name()]
				tasks = append(tasks, RepoTask{
					RepoName: repoEntry.Name(),
					ID:       repoDir,
					FullName: full,
				})
			}
		}
	}

	return tasks, nil
}

func ignoredSet(mf *manifest.FleetManifest) map[string]struct{} {
	if mf == nil {
		return nil
	}
	set := make(map[string]struct{})
	for _, o := range mf.Owners {
		for _, r := range o.Repos {
			if r.Ignored {
				set[r.FullName] = struct{}{}
				if parts := strings.SplitN(r.FullName, "/", 2); len(parts) == 2 {
					set[parts[1]] = struct{}{}
				}
			}
		}
	}
	return set
}

func nameToFullName(mf *manifest.FleetManifest) map[string]string {
	if mf == nil {
		return nil
	}
	idx := make(map[string]string)
	for _, o := range mf.Owners {
		for _, r := range o.Repos {
			parts := strings.SplitN(r.FullName, "/", 2)
			if len(parts) == 2 {
				idx[parts[1]] = r.FullName
			}
		}
	}
	return idx
}