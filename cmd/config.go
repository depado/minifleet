package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type configFile struct {
	GitHub struct {
		Token string `yaml:"token,omitempty"`
		Host  string `yaml:"host,omitempty"`
	} `yaml:"github"`
	Fleet struct {
		Base        string            `yaml:"base"`
		Shallow     bool              `yaml:"shallow,omitempty"`
		Concurrent  int               `yaml:"concurrent"`
		KnownFleets map[string]string `yaml:"known_fleets,omitempty"`
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

			if err := writeConfig(token, conf.GitHub.Host, base, conf.Fleet.Concurrent, conf.Fleet.KnownFleets); err != nil {
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

func writeConfig(token, host, base string, concurrent int, knownFleets map[string]string) error {
	cfg := configFile{}
	cfg.GitHub.Token = token
	cfg.GitHub.Host = host
	cfg.Fleet.Base = base
	cfg.Fleet.Concurrent = concurrent
	cfg.Fleet.Shallow = false
	cfg.Fleet.KnownFleets = knownFleets
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

// SaveConf writes the current Conf back to disk, preserving known_fleets.
// Used by sync to register newly-created fleet directories.
func SaveConf(conf *Conf) error {
	return writeConfig(conf.GitHub.Token, conf.GitHub.Host, conf.Fleet.Base, conf.Fleet.Concurrent, conf.Fleet.KnownFleets)
}

// RegisterFleet records an owner → directory mapping in conf.Fleet.KnownFleets
// and persists config to disk. Idempotent: re-registering an existing mapping
// is a no-op.
func RegisterFleet(conf *Conf, owner, dir string) error {
	if conf.Fleet.KnownFleets == nil {
		conf.Fleet.KnownFleets = make(map[string]string)
	}
	if existing, ok := conf.Fleet.KnownFleets[owner]; ok && existing == dir {
		return nil
	}
	conf.Fleet.KnownFleets[owner] = dir
	return SaveConf(conf)
}

func printConfig(conf *Conf) {
	fmt.Printf("Config path:   %s\n", ConfigPath())
	fmt.Printf("Base dir:      %s\n", conf.Fleet.Base)
	fmt.Printf("Concurrency:   %d\n", conf.Fleet.Concurrent)
	fmt.Printf("GitHub host:   %s\n", conf.GitHub.Host)
	fmt.Printf("Log level:     %s\n", conf.Log.Level)
	fmt.Printf("Log format:    %s\n", conf.Log.Format)

	if len(conf.Fleet.KnownFleets) > 0 {
		fmt.Printf("\nKnown fleets:\n")
		owners := make([]string, 0, len(conf.Fleet.KnownFleets))
		for k := range conf.Fleet.KnownFleets {
			owners = append(owners, k)
		}
		sort.Strings(owners)
		for _, owner := range owners {
			dir := conf.Fleet.KnownFleets[owner]
			msg := fmt.Sprintf("  %s → %s", owner, dir)
			if _, err := os.Stat(filepath.Join(dir, "fleet.yml")); err != nil {
				msg += "  [fleet.yml missing]"
			}
			fmt.Println(msg)
		}
	}
}