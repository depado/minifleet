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
	IncludeRegex    string
	ExcludeRegex    string
	Include         []string
	Exclude         []string
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
	flags := c.Flags()
	flags.StringVarP(&f.IncludeRegex, "include-regex", "r", "", "regex to match repo names")
	flags.StringVarP(&f.ExcludeRegex, "exclude-regex", "R", "", "regex to exclude repo names")
	flags.StringArrayVarP(&f.Include, "include", "i", nil, "include repo by exact name (repeatable)")
	flags.StringArrayVarP(&f.Exclude, "exclude", "e", nil, "exclude repo by exact name (repeatable)")
	flags.StringArrayVarP(&f.Topics, "topic", "t", nil, "filter by topic (repeatable)")
	flags.BoolVar(&f.IncludeArchived, "include-archived", false, "include archived repos")
	flags.BoolVar(&f.IncludeForks, "include-forks", false, "include forked repos")
	flags.StringVarP(&f.Visibility, "visibility", "v", "all", "filter by visibility: all, public, private")
	flags.StringVarP(&f.Language, "language", "l", "", "filter by primary language")
	flags.StringArrayVarP(&f.Labels, "label", "L", nil, "filter by manifest label (key=value or key, repeatable)")
	flags.StringVarP(&f.Group, "group", "g", "", "filter by manifest group")
}

// Apply filters a slice of repos. mf may be nil; manifest-based filters are
// skipped when it is. A name-filter regex compile error is returned as-is.
func (f Filters) Apply(repos []*provider.Repo, mf *manifest.FleetManifest) ([]*provider.Repo, error) {
	nm, err := f.nameMatcher()
	if err != nil {
		return nil, err
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
		if !nm.match(r.Name) {
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
// Only name and manifest-based filters apply since task entries may not
// carry API metadata.
func (f Filters) ApplyTasks(tasks []taskWithName, mf *manifest.FleetManifest) ([]taskWithName, error) {
	nm, err := f.nameMatcher()
	if err != nil {
		return nil, err
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
		if !nm.match(t.RepoName) {
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

// nameMatcher decides whether a repo name passes the include/exclude filters.
// Exclude wins over include. Include (regex or exact list) is allow-list: when
// either is set, a name must match to pass.
type nameMatcher struct {
	includeRe *regexp.Regexp
	excludeRe *regexp.Regexp
	include   map[string]struct{}
	exclude   map[string]struct{}
}

func (f Filters) nameMatcher() (nameMatcher, error) {
	var nm nameMatcher
	if f.IncludeRegex != "" {
		p, err := regexp.Compile(f.IncludeRegex)
		if err != nil {
			return nm, fmt.Errorf("include-regex: %w", err)
		}
		nm.includeRe = p
	}
	if f.ExcludeRegex != "" {
		p, err := regexp.Compile(f.ExcludeRegex)
		if err != nil {
			return nm, fmt.Errorf("exclude-regex: %w", err)
		}
		nm.excludeRe = p
	}
	nm.include = toSet(f.Include)
	nm.exclude = toSet(f.Exclude)
	return nm, nil
}

func (nm nameMatcher) match(name string) bool {
	if _, ok := nm.exclude[name]; ok {
		return false
	}
	if nm.excludeRe != nil && nm.excludeRe.MatchString(name) {
		return false
	}
	if nm.include == nil && nm.includeRe == nil {
		return true
	}
	if _, ok := nm.include[name]; ok {
		return true
	}
	return nm.includeRe != nil && nm.includeRe.MatchString(name)
}

func toSet(vals []string) map[string]struct{} {
	if len(vals) == 0 {
		return nil
	}
	s := make(map[string]struct{}, len(vals))
	for _, v := range vals {
		s[v] = struct{}{}
	}
	return s
}
