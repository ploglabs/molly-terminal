package config

import (
	"fmt"
	"os"
	"path/filepath"
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

func (c *Config) Validate() error {
	if c.Server.WebsocketURL == "" {
		return fmt.Errorf("missing server.websocket_url in config")
	}
	if c.Server.WebhookURL == "" {
		return fmt.Errorf("missing server.webhook_url in config")
	}
	if c.General.Username == "" {
		return fmt.Errorf("missing general.username in config")
	}
	return nil
}
