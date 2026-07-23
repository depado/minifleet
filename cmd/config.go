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
	Concurrent  int               `yaml:"concurrent"`
	Interactive string            `yaml:"interactive,omitempty"`
	Fleets      map[string]string `yaml:"fleets,omitempty"`
	Log         struct {
		Level  string `yaml:"level"`
		Format string `yaml:"format"`
		Source bool   `yaml:"source"`
		Color  string `yaml:"color"`
	} `yaml:"log"`
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
	cfg.Concurrent = conf.Concurrent
	cfg.Fleets = conf.Fleets
	cfg.Log.Level = conf.Log.Level
	cfg.Log.Format = conf.Log.Format
	cfg.Log.Source = conf.Log.Source
	cfg.Log.Color = conf.Log.Color
	cfg.Interactive = conf.Interactive
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

// RegisterFleet records an owner → directory mapping in conf.Fleets
// and persists config to disk. Idempotent: re-registering an existing mapping
// is a no-op.
func RegisterFleet(conf *Conf, owner, dir string) error {
	if conf.Fleets == nil {
		conf.Fleets = make(map[string]string)
	}
	if existing, ok := conf.Fleets[owner]; ok && existing == dir {
		return nil
	}
	conf.Fleets[owner] = dir
	return SaveConf(conf)
}

func printConfig(conf *Conf) {
	fmt.Printf("Config path:   %s\n", ConfigPath())
	fmt.Printf("Concurrency:   %d\n", conf.Concurrent)
	fmt.Printf("GitHub host:   %s\n", conf.GitHub.Host)
	fmt.Printf("Log level:     %s\n", conf.Log.Level)
	fmt.Printf("Log format:    %s\n", conf.Log.Format)

	if len(conf.Fleets) > 0 {
		fmt.Printf("\nKnown fleets:\n")
		owners := make([]string, 0, len(conf.Fleets))
		for k := range conf.Fleets {
			owners = append(owners, k)
		}
		sort.Strings(owners)
		for _, owner := range owners {
			dir := conf.Fleets[owner]
			msg := fmt.Sprintf("  %s → %s", owner, dir)
			if _, err := os.Stat(filepath.Join(dir, "fleet.yml")); err != nil {
				msg += "  [fleet.yml missing]"
			}
			fmt.Println(msg)
		}
	}
}
