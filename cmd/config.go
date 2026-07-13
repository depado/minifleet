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
			conf.GitHub.Token = token

			if err := writeConfigFile(buildConfigFile(conf)); err != nil {
				return err
			}
			fmt.Printf("Config written to %s\n", ConfigPath())
			return nil
		},
	}

	cmd.Flags().StringVarP(&token, "token", "t", "", "GitHub personal access token")
	cmd.Flags().BoolVarP(&show, "show", "s", false, "show current configuration")

	return cmd
}

// buildConfigFile projects the runtime Conf onto the on-disk file shape,
// preserving every field (no hardcoded defaults).
func buildConfigFile(conf *Conf) configFile {
	cfg := configFile{}
	cfg.GitHub.Token = conf.GitHub.Token
	cfg.GitHub.Host = conf.GitHub.Host
	cfg.Fleet.Shallow = conf.Fleet.Shallow
	cfg.Fleet.Concurrent = conf.Fleet.Concurrent
	cfg.Fleet.KnownFleets = conf.Fleet.KnownFleets
	cfg.Log.Level = conf.Log.Level
	cfg.Log.Format = conf.Log.Format
	cfg.Log.Source = conf.Log.Source
	cfg.Log.Color = conf.Log.Color
	cfg.UI.Progress = conf.UI.Progress
	cfg.UI.Color = conf.UI.Color
	return cfg
}

// writeConfigFile marshals cfg to the config path with 0600 perms (the file
// holds a GitHub token).
func writeConfigFile(cfg configFile) error {
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	dir := filepath.Dir(ConfigPath())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	return os.WriteFile(ConfigPath(), append([]byte("# minifleet configuration\n"), data...), 0o600)
}

// SaveConf persists conf to disk with 0600 perms, preserving all fields.
// The token is taken from the existing on-disk config (when present) rather
// than from conf.GitHub.Token, so a token sourced only from the GITHUB_TOKEN
// environment variable is never written to disk by a background save such as
// sync registering a fleet.
func SaveConf(conf *Conf) error {
	cfg := buildConfigFile(conf)
	cfg.GitHub.Token = ""
	if existing, err := os.ReadFile(ConfigPath()); err == nil {
		var prev configFile
		if yaml.Unmarshal(existing, &prev) == nil {
			cfg.GitHub.Token = prev.GitHub.Token
		}
	}
	return writeConfigFile(cfg)
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
