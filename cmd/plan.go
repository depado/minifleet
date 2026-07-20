package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type Plan struct {
	Fleet      string   `yaml:"fleet,omitempty"`
	All        bool     `yaml:"all,omitempty"`
	Format     string   `yaml:"format,omitempty"`
	Shell      string   `yaml:"shell,omitempty"`
	Command    string   `yaml:"command,omitempty"`
	BlockLines int      `yaml:"block_lines,omitempty"`
	DryRun     bool     `yaml:"dry_run,omitempty"`
	Summary    bool     `yaml:"summary,omitempty"`
	Progress   bool     `yaml:"progress,omitempty"`
	Limit      int      `yaml:"limit,omitempty"`
	Filters    *Filters `yaml:"filters,omitempty"`
}

type planKey struct{}

func LoadPlan(path string) (*Plan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read plan file: %w", err)
	}
	var plan Plan
	if err := yaml.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("parse plan file: %w", err)
	}
	return &plan, nil
}

func ctxWithPlan(ctx context.Context, plan *Plan) context.Context {
	return context.WithValue(ctx, planKey{}, plan)
}

func planFromCtx(ctx context.Context) *Plan {
	p, _ := ctx.Value(planKey{}).(*Plan)
	return p
}

func ApplyPlan(f *Filters, plan *Plan, cmd *cobra.Command) {
	if plan == nil || plan.Filters == nil {
		return
	}
	p := plan.Filters
	c := cmd.Flags()

	if !c.Changed("include-regex") {
		f.IncludeRegex = p.IncludeRegex
	}
	if !c.Changed("exclude-regex") {
		f.ExcludeRegex = p.ExcludeRegex
	}
	if !c.Changed("include") {
		f.Include = p.Include
	}
	if !c.Changed("exclude") {
		f.Exclude = p.Exclude
	}
	if !c.Changed("topic") {
		f.Topics = p.Topics
	}
	if !c.Changed("include-archived") {
		f.IncludeArchived = p.IncludeArchived
	}
	if !c.Changed("include-forks") {
		f.IncludeForks = p.IncludeForks
	}
	if !c.Changed("visibility") {
		f.Visibility = p.Visibility
	}
	if !c.Changed("language") {
		f.Language = p.Language
	}
	if !c.Changed("label") {
		f.Labels = p.Labels
	}
	if !c.Changed("group") {
		f.Group = p.Group
	}
	if !c.Changed("has-file") {
		f.HasFiles = p.HasFiles
	}
	if !c.Changed("if") {
		f.IfCmd = p.IfCmd
	}
	if !c.Changed("dirty") {
		f.Dirty = p.Dirty
	}
	if !c.Changed("ahead") {
		f.Ahead = p.Ahead
	}
	if !c.Changed("behind") {
		f.Behind = p.Behind
	}
	if !c.Changed("wip") {
		f.Wip = p.Wip
	}
}

func planTargets(conf *Conf, plan *Plan, all bool) ([]fleetTarget, error) {
	if plan != nil && plan.Fleet != "" {
		target, _ := resolveFleet(conf, "", plan.Fleet)
		if target.Dir == "" {
			return nil, fmt.Errorf("could not resolve fleet %q in known_fleets; run 'minifleet discover %s' first", plan.Fleet, plan.Fleet)
		}
		return []fleetTarget{target}, nil
	}
	return discoverFleets(conf, all), nil
}
