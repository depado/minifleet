package manifest

import (
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/depado/minifleet/internal/provider"
	"gopkg.in/yaml.v3"
)

// FleetManifest lives alongside the repos it describes. One file per owner.
// The directory containing the file IS the fleet directory; repos are cloned
// directly into it, so no per-repo path is tracked.
type FleetManifest struct {
	Version string              `yaml:"version"`
	Owner   string              `yaml:"owner"`
	Groups  map[string][]string `yaml:"groups,omitempty"`
	Repos   []ManifestRepo      `yaml:"repos"`
}

type ManifestRepo struct {
	FullName string `yaml:"full_name"`

	// API-tracked fields, overwritten by sync from the GitHub API.
	Topics    []string  `yaml:"topics,omitempty"`
	Language  string    `yaml:"language,omitempty"`
	Archived  bool      `yaml:"archived,omitempty"`
	Fork      bool      `yaml:"fork,omitempty"`
	Private   bool      `yaml:"private,omitempty"`
	UpdatedAt time.Time `yaml:"updated_at,omitempty"`

	// User-set fields, never touched by sync.
	Labels   map[string]string `yaml:"labels,omitempty"`
	Protocol string            `yaml:"protocol,omitempty"`
	Ignored  bool              `yaml:"ignored,omitempty"`
}

// Path returns the fleet.yml path inside the given directory.
func Path(fleetDir string) string { return filepath.Join(fleetDir, "fleet.yml") }

func Load(path string) (*FleetManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var mf FleetManifest
	if err := yaml.Unmarshal(data, &mf); err != nil {
		return nil, err
	}
	return &mf, nil
}

func Save(mf *FleetManifest, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(mf)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func Generate(repos []*provider.Repo, owner string) *FleetManifest {
	return &FleetManifest{
		Version: "1",
		Owner:   owner,
		Repos:   convertRepos(repos),
	}
}

// Merge refreshes API-tracked fields; user-set fields (Labels, Protocol,
// Ignored) are preserved from the existing manifest for repos that already
// exist, and default to zero for new repos.
func Merge(existing *FleetManifest, owner string, repos []*provider.Repo) *FleetManifest {
	if existing == nil {
		existing = &FleetManifest{Version: "1", Owner: owner}
	}
	existing.Owner = owner

	prev := make(map[string]ManifestRepo, len(existing.Repos))
	for _, r := range existing.Repos {
		prev[r.FullName] = r
	}

	merged := make([]ManifestRepo, 0, len(repos))
	for _, api := range repos {
		if old, ok := prev[api.FullName]; ok {
			merged = append(merged, ManifestRepo{
				FullName:  api.FullName,
				Topics:    api.Topics,
				Language:  api.Language,
				Archived:  api.Archived,
				Fork:      api.Fork,
				Private:   api.Private,
				UpdatedAt: api.UpdatedAt,
				Labels:    old.Labels,
				Protocol:  old.Protocol,
				Ignored:   old.Ignored,
			})
		} else {
			merged = append(merged, ManifestRepo{
				FullName:  api.FullName,
				Topics:    api.Topics,
				Language:  api.Language,
				Archived:  api.Archived,
				Fork:      api.Fork,
				Private:   api.Private,
				UpdatedAt: api.UpdatedAt,
			})
		}
	}
	existing.Repos = merged
	return existing
}

// Index flattens repos into a FullName → *ManifestRepo lookup. Returns nil
// when mf is nil.
func (mf *FleetManifest) Index() map[string]*ManifestRepo {
	if mf == nil {
		return nil
	}
	idx := make(map[string]*ManifestRepo, len(mf.Repos))
	for i := range mf.Repos {
		idx[mf.Repos[i].FullName] = &mf.Repos[i]
	}
	return idx
}

// GroupRepos returns the set of full_names belonging to the named group, or
// nil when the group does not exist.
func (mf *FleetManifest) GroupRepos(group string) map[string]struct{} {
	if mf == nil || mf.Groups == nil {
		return nil
	}
	names, ok := mf.Groups[group]
	if !ok {
		return nil
	}
	set := make(map[string]struct{}, len(names))
	for _, n := range names {
		set[n] = struct{}{}
	}
	return set
}

// RepoNamesSorted returns repo short names (the last path segment of
// full_name) in sorted order, mainly for deterministic iteration in
// diagnostics.
func (mf *FleetManifest) RepoNamesSorted() []string {
	if mf == nil {
		return nil
	}
	names := make([]string, 0, len(mf.Repos))
	for _, r := range mf.Repos {
		short := r.FullName
		if i := lastIndexByte(r.FullName, '/'); i >= 0 {
			short = r.FullName[i+1:]
		}
		names = append(names, short)
	}
	sort.Strings(names)
	return names
}

func lastIndexByte(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func convertRepos(repos []*provider.Repo) []ManifestRepo {
	result := make([]ManifestRepo, len(repos))
	for i, r := range repos {
		result[i] = ManifestRepo{
			FullName:  r.FullName,
			Topics:    r.Topics,
			Language:  r.Language,
			Archived:  r.Archived,
			Fork:      r.Fork,
			Private:   r.Private,
			UpdatedAt: r.UpdatedAt,
		}
	}
	return result
}
