package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/depado/minifleet/internal/manifest"
)

type configFile struct {
	GitHub struct {
		Token string `yaml:"token,omitempty"`
	} `yaml:"github"`
	Fleet struct {
		Base       string `yaml:"base"`
		Shallow    bool   `yaml:"shallow,omitempty"`
		Concurrent int    `yaml:"concurrent"`
	} `yaml:"fleet"`
	Log struct {
		Level  string `yaml:"level"`
		Format string `yaml:"format"`
		Source bool   `yaml:"source"`
		Color  string `yaml:"color"`
	} `yaml:"log"`
	UI struct {
		Progress bool `yaml:"progress"`
		Color    bool `yaml:"color"`
	} `yaml:"ui"`
}

func newInitCmd() *cobra.Command {
	var (
		token string
		base  string
		show  bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize or show minifleet configuration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			conf, err := confFromCtx(cmd)
			if err != nil {
				return err
			}

			if show {
				printConfig(conf)
				return nil
			}

			if token == "" {
				token = os.Getenv("GITHUB_TOKEN")
			}
			if base == "" {
				base = conf.Fleet.Base
			}

			if err := writeConfig(token, base, conf.Fleet.Concurrent); err != nil {
				return err
			}
			fmt.Printf("Config written to %s\n", ConfigPath())
			return nil
		},
	}

	cmd.Flags().StringVarP(&token, "token", "t", "", "GitHub personal access token")
	cmd.Flags().StringVarP(&base, "base", "b", "", "base directory for clones")
	cmd.Flags().BoolVarP(&show, "show", "s", false, "show current configuration")

	return cmd
}

func writeConfig(token, base string, concurrent int) error {
	cfg := configFile{}
	cfg.GitHub.Token = token
	cfg.Fleet.Base = base
	cfg.Fleet.Concurrent = concurrent
	cfg.Fleet.Shallow = false
	cfg.Log.Level = "info"
	cfg.Log.Format = "text"
	cfg.Log.Source = false
	cfg.Log.Color = "auto"
	cfg.UI.Progress = true
	cfg.UI.Color = true

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	dir := filepath.Dir(ConfigPath())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	return os.WriteFile(ConfigPath(), append([]byte("# minifleet configuration\n"), data...), 0o644)
}

func printConfig(conf *Conf) {
	fmt.Printf("Config path:  %s\n", ConfigPath())
	fmt.Printf("Fleet path:   %s\n", manifest.ManifestPath())
	fmt.Printf("Base dir:     %s\n", conf.Fleet.Base)
	fmt.Printf("Concurrency:  %d\n", conf.Fleet.Concurrent)
	fmt.Printf("Log level:    %s\n", conf.Log.Level)
	fmt.Printf("Log format:   %s\n", conf.Log.Format)

	mf, _ := manifest.Load(manifest.ManifestPath())
	if mf != nil && len(mf.Owners) > 0 {
		fmt.Printf("\nTracked owners:\n")
		for owner := range mf.Owners {
			total := len(mf.Owners[owner].Repos)
			ignored := 0
			for _, r := range mf.Owners[owner].Repos {
				if r.Ignored {
					ignored++
				}
			}
			fmt.Printf("  %s: %d repos", owner, total)
			if ignored > 0 {
				fmt.Printf(" (%d ignored)", ignored)
			}
			fmt.Println()
		}
	}
}
