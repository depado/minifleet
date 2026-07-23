package cmd

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSaveConf(t *testing.T) {
	t.Run("preserves settings and uses 0600", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		conf := &Conf{}
		conf.Log.Level = "debug"
		conf.Log.Format = "json"
		conf.Interactive = "never"
		conf.Concurrent = 9
		if err := SaveConf(conf); err != nil {
			t.Fatalf("SaveConf: %v", err)
		}

		data, err := os.ReadFile(ConfigPath())
		if err != nil {
			t.Fatalf("read config: %v", err)
		}
		var cfg configFile
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			t.Fatalf("unmarshal config: %v", err)
		}

		if cfg.Log.Level != "debug" {
			t.Errorf("Log.Level = %q, want %q", cfg.Log.Level, "debug")
		}
		if cfg.Log.Format != "json" {
			t.Errorf("Log.Format = %q, want %q", cfg.Log.Format, "json")
		}
		if cfg.Interactive != "never" {
			t.Errorf("Interactive = %q, want %q", cfg.Interactive, "never")
		}
		if cfg.Concurrent != 9 {
			t.Errorf("Concurrent = %d, want 9", cfg.Concurrent)
		}

		info, err := os.Stat(ConfigPath())
		if err != nil {
			t.Fatalf("stat config: %v", err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("perms = %o, want 0o600", info.Mode().Perm())
		}
	})

	t.Run("env token not leaked", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		conf := &Conf{}
		conf.GitHub.Token = "from-env"
		if err := SaveConf(conf); err != nil {
			t.Fatalf("SaveConf: %v", err)
		}

		data, err := os.ReadFile(ConfigPath())
		if err != nil {
			t.Fatalf("read config: %v", err)
		}
		var cfg configFile
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			t.Fatalf("unmarshal config: %v", err)
		}

		if cfg.GitHub.Token != "" {
			t.Errorf("GitHub.Token = %q, want empty", cfg.GitHub.Token)
		}
	})

	t.Run("on-disk token preserved", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		existing := configFile{}
		existing.GitHub.Token = "on-disk"
		if err := writeConfigFile(existing); err != nil {
			t.Fatalf("writeConfigFile: %v", err)
		}

		conf := &Conf{}
		conf.GitHub.Token = "from-env"
		if err := SaveConf(conf); err != nil {
			t.Fatalf("SaveConf: %v", err)
		}

		data, err := os.ReadFile(ConfigPath())
		if err != nil {
			t.Fatalf("read config: %v", err)
		}
		var cfg configFile
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			t.Fatalf("unmarshal config: %v", err)
		}

		if cfg.GitHub.Token != "on-disk" {
			t.Errorf("GitHub.Token = %q, want %q", cfg.GitHub.Token, "on-disk")
		}
	})
}
