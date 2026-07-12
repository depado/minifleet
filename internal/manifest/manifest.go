package manifest

import (
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/depado/minifleet/internal/provider"
	"gopkg.in/yaml.v3"
)

type FleetManifest struct {
	Version string              `yaml:"version"`
	Owners  map[string]Owner    `yaml:"owners"`
}

type Owner struct {
	Path   string              `yaml:"path,omitempty"`
	Groups map[string][]string `yaml:"groups,omitempty"`
	Repos  []ManifestRepo      `yaml:"repos"`
}

type ManifestRepo struct {
	FullName string `yaml:"full_name"`
	Path     string `yaml:"path,omitempty"`

	Topics    []string  `yaml:"topics,omitempty"`
	Language  string    `yaml:"language,omitempty"`
	Archived  bool      `yaml:"archived,omitempty"`
	Fork      bool      `yaml:"fork,omitempty"`
	Private   bool      `yaml:"private,omitempty"`
	UpdatedAt time.Time `yaml:"updated_at,omitempty"`

	Labels   map[string]string `yaml:"labels,omitempty"`
	Protocol string            `yaml:"protocol,omitempty"`
	Ignored  bool              `yaml:"ignored,omitempty"`
}

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
		Owners: map[string]Owner{
			owner: {
				Repos: convertRepos(repos),
			},
		},
	}
}

func Merge(mf *FleetManifest, owner string, repos []*provider.Repo) *FleetManifest {
	if mf == nil {
		mf = &FleetManifest{Version: "1", Owners: make(map[string]Owner)}
	}
	if mf.Owners == nil {
		mf.Owners = make(map[string]Owner)
	}

	prev := mf.Owners[owner]
	prevByFullName := make(map[string]ManifestRepo, len(prev.Repos))
	for _, r := range prev.Repos {
		prevByFullName[r.FullName] = r
	}

	merged := Owner{
		Groups: prev.Groups,
		Repos:  make([]ManifestRepo, 0, len(repos)),
	}
	for _, api := range repos {
		if existing, ok := prevByFullName[api.FullName]; ok {
			merged.Repos = append(merged.Repos, ManifestRepo{
				FullName:  api.FullName,
				Topics:    api.Topics,
				Language:  api.Language,
				Archived:  api.Archived,
				Fork:      api.Fork,
				Private:   api.Private,
				UpdatedAt: api.UpdatedAt,
				Labels:    existing.Labels,
				Protocol:  existing.Protocol,
				Ignored:   existing.Ignored,
			})
		} else {
			merged.Repos = append(merged.Repos, ManifestRepo{
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

	mf.Owners[owner] = merged
	return mf
}

// Index flattens all owners' repos into a FullName → *ManifestRepo lookup.
// Callers may mutate entries through the pointer; iteration order is not
// guaranteed (Owners is a map).
func (mf *FleetManifest) Index() map[string]*ManifestRepo {
	if mf == nil {
		return nil
	}
	idx := make(map[string]*ManifestRepo)
	for owner := range mf.Owners {
		for i := range mf.Owners[owner].Repos {
			r := &mf.Owners[owner].Repos[i]
			idx[r.FullName] = r
		}
	}
	return idx
}

// OwnerNames returns the list of owner names in sorted order for deterministic
// iteration over the Owners map.
func (mf *FleetManifest) OwnerNames() []string {
	if mf == nil {
		return nil
	}
	names := make([]string, 0, len(mf.Owners))
	for k := range mf.Owners {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

func ManifestPath() string {
	return filepath.Join(configDir(), "fleet.yml")
}

func configDir() string {
	return filepath.Join(os.Getenv("HOME"), ".config", "minifleet")
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
