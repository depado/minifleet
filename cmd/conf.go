package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lmittmann/tint"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type LogConf struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Source bool   `mapstructure:"source"`
	Color  string `mapstructure:"color"`
}

type GitHubConf struct {
	Token string `mapstructure:"token"`
	Host  string `mapstructure:"host"` // "github.com" (default) or a GitHub Enterprise host
}

type FleetConf struct {
	Base        string            `mapstructure:"base"`
	Path        string            `mapstructure:"path"` // one-shot override: clone into this directory
	Shallow     bool              `mapstructure:"shallow"`
	Concurrent  int               `mapstructure:"concurrent"`
	KnownFleets map[string]string `mapstructure:"known_fleets,omitempty"` // owner → fleet directory
}

type UIConf struct {
	Progress bool `mapstructure:"progress"`
	Color    bool `mapstructure:"color"`
}

type Conf struct {
	Log    LogConf    `mapstructure:"log"`
	GitHub GitHubConf `mapstructure:"github"`
	Fleet  FleetConf  `mapstructure:"fleet"`
	UI     UIConf     `mapstructure:"ui"`
}

func configDir() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "minifleet")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "minifleet")
}

func ConfigPath() string {
	return filepath.Join(configDir(), "config.yml")
}

func NewLogger(c *Conf) *slog.Logger {
	var level slog.Level
	switch strings.ToLower(c.Log.Level) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level, AddSource: c.Log.Source}

	var handler slog.Handler
	switch strings.ToLower(c.Log.Format) {
	case "json":
		handler = slog.NewJSONHandler(os.Stderr, opts)
	case "text", "console":
		noColor := strings.EqualFold(c.Log.Color, "never")
		if strings.EqualFold(c.Log.Color, "auto") || c.Log.Color == "" {
			noColor = !isatty.IsTerminal(os.Stderr.Fd())
		}
		handler = tint.NewHandler(os.Stderr, &tint.Options{
			Level: level, AddSource: c.Log.Source, TimeFormat: time.DateTime, NoColor: noColor,
		})
	default:
		handler = slog.NewJSONHandler(os.Stderr, opts)
	}
	return slog.New(handler)
}

func NewConf(cmd *cobra.Command) (*Conf, error) {
	v := viper.New()

	if err := v.BindPFlags(cmd.Flags()); err != nil {
		return nil, fmt.Errorf("bind flags: %w", err)
	}

	v.AutomaticEnv()
	v.SetEnvPrefix("MINIFLEET")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	if err := v.BindEnv("github.token", "GITHUB_TOKEN"); err != nil {
		return nil, fmt.Errorf("bind token env: %w", err)
	}

	if path := v.GetString("conf"); path != "" {
		v.SetConfigFile(path)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yml")
		v.AddConfigPath(configDir())
		v.AddConfigPath(".")
	}

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	conf := &Conf{}
	if err := v.Unmarshal(conf); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	if conf.Fleet.Base == "" {
		home, _ := os.UserHomeDir()
		conf.Fleet.Base = filepath.Join(home, "dev")
	}
	if conf.Fleet.Concurrent <= 0 {
		conf.Fleet.Concurrent = 5
	}

	return conf, nil
}
