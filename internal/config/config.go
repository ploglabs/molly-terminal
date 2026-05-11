package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	General GeneralConfig `toml:"general"`
	Server  ServerConfig  `toml:"server"`
	Auth    AuthConfig    `toml:"auth"`
	UI      UIConfig      `toml:"ui"`
}

type GeneralConfig struct {
	Username          string `toml:"username"`
	Channel           string `toml:"channel"`
	DiscordID         string `toml:"discord_id"`
	DiscordUsername   string `toml:"discord_username"`
	DiscordGlobalName string `toml:"discord_global_name"`
	DiscordAvatarURL  string `toml:"discord_avatar_url"`
}

type ServerConfig struct {
	WebsocketURL string `toml:"websocket_url"`
	WebhookURL   string `toml:"webhook_url"`
	RelayURL     string `toml:"relay_url"`
}

type AuthConfig struct {
	Enabled  bool              `toml:"enabled"`
	Provider string            `toml:"provider"`
	Discord  DiscordAuthConfig `toml:"discord"`
}

type DiscordAuthConfig struct {
	ClientID     string `toml:"client_id"`
	ClientSecret string `toml:"client_secret"`
	RedirectURL  string `toml:"redirect_url"`
	AccessToken  string `toml:"access_token"`
	RefreshToken string `toml:"refresh_token"`
	TokenType    string `toml:"token_type"`
	Scope        string `toml:"scope"`
	Expiry       string `toml:"expiry"`
}

type UIConfig struct {
	Theme        string `toml:"theme"`
	HistoryLimit int    `toml:"history_limit"`
}

func Default() *Config {
	return &Config{
		General: GeneralConfig{
			Username: "",
			Channel:  "general",
		},
		Server: ServerConfig{
			WebsocketURL: "",
			WebhookURL:   "",
			RelayURL:     "",
		},
		Auth: AuthConfig{
			Enabled:  false,
			Provider: "",
			Discord: DiscordAuthConfig{
				RedirectURL: "http://127.0.0.1:53682/callback",
			},
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

func ConfigPathFromArgs(args []string) (string, error) {
	configPath := ""

	fs := flag.NewFlagSet("molly", flag.ContinueOnError)
	fs.StringVar(&configPath, "config", "", "path to config file")
	fs.StringVar(&configPath, "c", "", "path to config file (shorthand)")

	if err := fs.Parse(args); err != nil {
		return "", fmt.Errorf("parsing CLI flags: %w", err)
	}

	if configPath != "" {
		return configPath, nil
	}

	defaultPath, err := DefaultConfigPath()
	if err != nil {
		return "", fmt.Errorf("resolving default config path: %w", err)
	}
	return defaultPath, nil
}

func Load() (*Config, error) {
	cfg := Default()

	configPath, err := ConfigPathFromArgs(os.Args[1:])
	if err != nil {
		return nil, err
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

func (c *Config) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	safe := *c
	safe.Auth.Discord.ClientSecret = ""

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating config file: %w", err)
	}
	defer f.Close()

	if err := toml.NewEncoder(f).Encode(&safe); err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	return nil
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
	if v := os.Getenv("MOLLY_RELAY_URL"); v != "" {
		cfg.Server.RelayURL = v
	}
	if v := os.Getenv("MOLLY_AUTH_ENABLED"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.Auth.Enabled = b
		}
	}
	if v := os.Getenv("MOLLY_AUTH_PROVIDER"); v != "" {
		cfg.Auth.Provider = v
	}
	if v := os.Getenv("MOLLY_DISCORD_CLIENT_ID"); v != "" {
		cfg.Auth.Discord.ClientID = v
	}
	if v := os.Getenv("MOLLY_DISCORD_CLIENT_SECRET"); v != "" {
		cfg.Auth.Discord.ClientSecret = v
	}
	if v := os.Getenv("MOLLY_DISCORD_REDIRECT_URL"); v != "" {
		cfg.Auth.Discord.RedirectURL = v
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
	if c.General.Username == "" && !c.UsesDiscordAuth() {
		return fmt.Errorf("missing general.username — set it in config, or use MOLLY_USERNAME env var")
	}
	if c.Server.WebsocketURL == "" {
		return fmt.Errorf("missing server.websocket_url — set it in config, or use MOLLY_WEBSOCKET_URL env var")
	}
	if c.Server.WebhookURL == "" {
		return fmt.Errorf("missing server.webhook_url — set it in config, or use MOLLY_WEBHOOK_URL env var")
	}
	if c.Auth.Enabled {
		if c.Auth.Provider != "discord" {
			return fmt.Errorf("unsupported auth.provider %q — currently only \"discord\" is supported", c.Auth.Provider)
		}
		if c.Auth.Discord.ClientID == "" {
			return fmt.Errorf("missing auth.discord.client_id — set it in config, or use MOLLY_DISCORD_CLIENT_ID env var")
		}
		if c.Auth.Discord.ClientSecret == "" {
			return fmt.Errorf("missing auth.discord.client_secret — set it in config, or use MOLLY_DISCORD_CLIENT_SECRET env var")
		}
		if c.Auth.Discord.RedirectURL == "" {
			return fmt.Errorf("missing auth.discord.redirect_url — set it in config, or use MOLLY_DISCORD_REDIRECT_URL env var")
		}
	}
	return nil
}

func (c *Config) UsesDiscordAuth() bool {
	return c.Auth.Enabled && c.Auth.Provider == "discord"
}

func (c *Config) DiscordTokenExpiry() (time.Time, error) {
	if c.Auth.Discord.Expiry == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, c.Auth.Discord.Expiry)
}
