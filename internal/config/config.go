package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/BurntSushi/toml"
)

type Config struct {
	General GeneralConfig `toml:"general"`
	Server  ServerConfig  `toml:"server"`
	UI      UIConfig      `toml:"ui"`
}

type GeneralConfig struct {
	Username string `toml:"username"`
	Channel  string `toml:"channel"`
}

type ServerConfig struct {
	WebsocketURL string `toml:"websocket_url"`
	WebhookURL   string `toml:"webhook_url"`
}

type UIConfig struct {
	Theme        string `toml:"theme"`
	HistoryLimit int    `toml:"history_limit"`
}

func Default() *Config {
	return &Config{
		General: GeneralConfig{
			Username: "anonymous",
			Channel:  "general",
		},
		Server: ServerConfig{
			WebsocketURL: "",
			WebhookURL:   "",
		},
		UI: UIConfig{
			Theme:        "default",
			HistoryLimit: 100,
		},
	}
}

func configDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "molly"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "molly"), nil
}

func DefaultConfigPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

func Load() (*Config, error) {
	configPath := ""

	fs := flag.NewFlagSet("molly", flag.ContinueOnError)
	fs.StringVar(&configPath, "config", "", "path to config file")
	fs.StringVar(&configPath, "c", "", "path to config file (shorthand)")

	if err := fs.Parse(os.Args[1:]); err != nil {
		return nil, fmt.Errorf("parsing CLI flags: %w", err)
	}

	cfg := Default()

	if configPath == "" {
		defaultPath, err := DefaultConfigPath()
		if err != nil {
			return nil, fmt.Errorf("resolving default config path: %w", err)
		}
		configPath = defaultPath
	}

	if _, err := os.Stat(configPath); err == nil {
		if _, err := toml.DecodeFile(configPath, cfg); err != nil {
			return nil, fmt.Errorf("parsing config file %s: %w", configPath, err)
		}
	}

	applyEnvOverrides(cfg)

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func tomlDecodeFile(path string, cfg *Config) (interface{}, error) {
	return toml.DecodeFile(path, cfg)
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("MOLLY_USERNAME"); v != "" {
		cfg.General.Username = v
	}
	if v := os.Getenv("MOLLY_CHANNEL"); v != "" {
		cfg.General.Channel = v
	}
	if v := os.Getenv("MOLLY_WEBSOCKET_URL"); v != "" {
		cfg.Server.WebsocketURL = v
	}
	if v := os.Getenv("MOLLY_WEBHOOK_URL"); v != "" {
		cfg.Server.WebhookURL = v
	}
	if v := os.Getenv("MOLLY_THEME"); v != "" {
		cfg.UI.Theme = v
	}
	if v := os.Getenv("MOLLY_HISTORY_LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.UI.HistoryLimit = n
		}
	}
}

func (c *Config) Validate() error {
	if c.General.Username == "" {
		return fmt.Errorf("missing general.username — set it in config, or use MOLLY_USERNAME env var")
	}
	if c.Server.WebsocketURL == "" {
		return fmt.Errorf("missing server.websocket_url — set it in config, or use MOLLY_WEBSOCKET_URL env var")
	}
	if c.Server.WebhookURL == "" {
		return fmt.Errorf("missing server.webhook_url — set it in config, or use MOLLY_WEBHOOK_URL env var")
	}
	return nil
}
