package cmd

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/depado/minifleet/internal/manifest"
	"github.com/depado/minifleet/internal/provider"
	"github.com/spf13/cobra"
)

// Filters is the unified filter set applied by every command that operates on
// repositories. Flags are bound by addFilterFlags; Apply runs them against a
// list of repos, optionally consulting the manifest for label/group filtering.
type Filters struct {
	Target          string
	Topics          []string
	IncludeArchived bool
	IncludeForks    bool
	Visibility      string
	Language        string
	Labels          []string
	Group           string
}

// addFilterFlags binds the full filter flag set on a command. All commands that
// operate on repos use the same flags so users get a consistent vocabulary.
func addFilterFlags(c *cobra.Command, f *Filters) {
	c.Flags().StringVarP(&f.Target, "target", "t", "", "regex to match repo names")
	c.Flags().StringArrayVar(&f.Topics, "topic", nil, "filter by topic (repeatable)")
	c.Flags().BoolVar(&f.IncludeArchived, "include-archived", false, "include archived repos")
	c.Flags().BoolVar(&f.IncludeForks, "include-forks", false, "include forked repos")
	c.Flags().StringVar(&f.Visibility, "visibility", "all", "filter by visibility: all, public, private")
	c.Flags().StringVar(&f.Language, "language", "", "filter by primary language")
	c.Flags().StringArrayVar(&f.Labels, "label", nil, "filter by manifest label (key=value or key, repeatable)")
	c.Flags().StringVar(&f.Group, "group", "", "filter by manifest group")
}

// Apply filters a slice of repos. mf may be nil; manifest-based filters are
// skipped when it is. A target regex compile error is returned as-is.
func (f Filters) Apply(repos []*provider.Repo, mf *manifest.FleetManifest) ([]*provider.Repo, error) {
	var targetPat *regexp.Regexp
	if f.Target != "" {
		p, err := regexp.Compile(f.Target)
		if err != nil {
			return nil, fmt.Errorf("target regex: %w", err)
		}
		targetPat = p
	}

	labelFilters := parseLabels(f.Labels)
	var groupSet map[string]struct{}
	if f.Group != "" {
		groupSet = groupRepos(mf, f.Group)
		if groupSet == nil {
			return nil, fmt.Errorf("group %q not found in manifest", f.Group)
		}
	}

	idx := mf.Index()

	out := make([]*provider.Repo, 0, len(repos))
	for _, r := range repos {
		mr := idx[r.FullName]

		if !f.IncludeArchived {
			if mr != nil && mr.Archived {
				continue
			}
			if r.Archived {
				continue
			}
		}
		if !f.IncludeForks {
			if mr != nil && mr.Fork {
				continue
			}
			if r.Fork {
				continue
			}
		}
		if len(f.Topics) > 0 && !matchAnyTopic(r.Topics, f.Topics) {
			continue
		}
		if f.Visibility != "" && f.Visibility != "all" {
			if !strings.EqualFold(r.Visibility, f.Visibility) {
				continue
			}
		}
		if f.Language != "" && !strings.EqualFold(r.Language, f.Language) {
			continue
		}
		if targetPat != nil && !targetPat.MatchString(r.Name) {
			continue
		}
		if len(labelFilters) > 0 {
			if mr == nil || !matchLabels(mr.Labels, labelFilters) {
				continue
			}
		}
		if groupSet != nil {
			if _, ok := groupSet[r.FullName]; !ok {
				continue
			}
		}

		out = append(out, r)
	}
	return out, nil
}

// ApplyTasks filters fleet.RepoTask-style entries (from the local scanner).
// Only target and manifest-based filters apply since task entries may not
// carry API metadata.
func (f Filters) ApplyTasks(tasks []taskWithName, mf *manifest.FleetManifest) ([]taskWithName, error) {
	var targetPat *regexp.Regexp
	if f.Target != "" {
		p, err := regexp.Compile(f.Target)
		if err != nil {
			return nil, fmt.Errorf("target regex: %w", err)
		}
		targetPat = p
	}

	labelFilters := parseLabels(f.Labels)
	var groupSet map[string]struct{}
	if f.Group != "" {
		groupSet = groupRepos(mf, f.Group)
		if groupSet == nil {
			return nil, fmt.Errorf("group %q not found in manifest", f.Group)
		}
	}

	idx := mf.Index()

	out := make([]taskWithName, 0, len(tasks))
	for _, t := range tasks {
		mr := idx[t.FullName]
		if mr == nil {
			if len(labelFilters) > 0 || groupSet != nil {
				continue
			}
			if len(f.Topics) > 0 || f.Language != "" {
				continue
			}
		} else {
			if !f.IncludeArchived && mr.Archived {
				continue
			}
			if !f.IncludeForks && mr.Fork {
				continue
			}
			if len(f.Topics) > 0 && !matchAnyTopic(mr.Topics, f.Topics) {
				continue
			}
			if f.Language != "" && !strings.EqualFold(mr.Language, f.Language) {
				continue
			}
			if len(labelFilters) > 0 && !matchLabels(mr.Labels, labelFilters) {
				continue
			}
			if groupSet != nil {
				if _, ok := groupSet[t.FullName]; !ok {
					continue
				}
			}
		}
		if targetPat != nil && !targetPat.MatchString(t.RepoName) {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

// taskWithName pairs a local task with its full_name when known from manifest.
type taskWithName struct {
	RepoName string
	FullName string
	ID       string
	Dir      string
}

type labelFilter struct {
	key   string
	value string
	any   bool
}

func parseLabels(labels []string) []labelFilter {
	out := make([]labelFilter, 0, len(labels))
	for _, l := range labels {
		if k, v, ok := strings.Cut(l, "="); ok {
			out = append(out, labelFilter{key: k, value: v})
		} else {
			out = append(out, labelFilter{key: l, any: true})
		}
	}
	return out
}

func matchLabels(repoLabels map[string]string, filters []labelFilter) bool {
	for _, f := range filters {
		v, ok := repoLabels[f.key]
		if !ok {
			return false
		}
		if !f.any && v != f.value {
			return false
		}
	}
	return true
}

func matchAnyTopic(repoTopics, filterTopics []string) bool {
	s := make(map[string]struct{}, len(repoTopics))
	for _, t := range repoTopics {
		s[t] = struct{}{}
	}
	for _, t := range filterTopics {
		if _, ok := s[t]; ok {
			return true
		}
	}
	return false
}

// groupRepos returns the set of full_names belonging to a group in the
// single-owner manifest. Returns nil if the group does not exist.
func groupRepos(mf *manifest.FleetManifest, group string) map[string]struct{} {
	return mf.GroupRepos(group)
}
