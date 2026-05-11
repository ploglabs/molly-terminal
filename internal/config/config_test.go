package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := Default()

	if cfg.General.Username != "" {
		t.Errorf("expected empty username, got '%s'", cfg.General.Username)
	}
	if cfg.General.Channel != "general" {
		t.Errorf("expected channel 'general', got '%s'", cfg.General.Channel)
	}
	if cfg.Auth.Discord.RedirectURL != "http://127.0.0.1:53682/callback" {
		t.Errorf("expected default redirect url, got '%s'", cfg.Auth.Discord.RedirectURL)
	}
	if cfg.UI.HistoryLimit != 100 {
		t.Errorf("expected history_limit 100, got %d", cfg.UI.HistoryLimit)
	}
	if cfg.UI.Theme != "default" {
		t.Errorf("expected theme 'default', got '%s'", cfg.UI.Theme)
	}
}

func TestValidateMissingFields(t *testing.T) {
	cfg := Default()
	cfg.General.Username = "tester"

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for empty required fields")
	}

	cfg.Server.WebsocketURL = "wss://example.com"
	err = cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for missing webhook_url")
	}

	cfg.Server.WebhookURL = "https://discord.com/api/webhooks/test"
	err = cfg.Validate()
	if err != nil {
		t.Fatalf("expected valid config, got: %v", err)
	}
}

func TestValidateMissingUsername(t *testing.T) {
	cfg := Default()
	cfg.General.Username = ""
	cfg.Server.WebsocketURL = "wss://example.com"
	cfg.Server.WebhookURL = "https://discord.com/api/webhooks/test"

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing username")
	}
}

func TestValidateAllowsMissingUsernameWithDiscordAuth(t *testing.T) {
	cfg := Default()
	cfg.Server.WebsocketURL = "wss://example.com"
	cfg.Server.WebhookURL = "https://discord.com/api/webhooks/test"
	cfg.Auth.Enabled = true
	cfg.Auth.Provider = "discord"
	cfg.Auth.Discord.ClientID = "client-id"
	cfg.Auth.Discord.ClientSecret = "client-secret"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected discord auth config to be valid without username, got: %v", err)
	}
}

func TestLoadFromTOMLFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")

	content := `
[general]
username = "testuser"
channel = "dev"

[server]
websocket_url = "wss://relay.test.com/ws"
webhook_url = "https://discord.com/api/webhooks/123/token"

[ui]
theme = "dracula"
history_limit = 50
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	cfg := Default()
	var loadErr error
	func() {
		defer func() {
			os.Args = []string{"molly", "--config", cfgPath}
			origArgs := os.Args
			os.Args = []string{"molly", "--config", cfgPath}
			cfg, loadErr = Load()
			os.Args = origArgs
		}()
	}()

	_ = cfg
	_ = loadErr

	cfg2 := Default()
	if _, err := tomlDecodeFile(cfgPath, cfg2); err != nil {
		t.Fatalf("failed to decode test config: %v", err)
	}

	if cfg2.General.Username != "testuser" {
		t.Errorf("expected username 'testuser', got '%s'", cfg2.General.Username)
	}
	if cfg2.General.Channel != "dev" {
		t.Errorf("expected channel 'dev', got '%s'", cfg2.General.Channel)
	}
	if cfg2.Server.WebsocketURL != "wss://relay.test.com/ws" {
		t.Errorf("expected websocket_url, got '%s'", cfg2.Server.WebsocketURL)
	}
	if cfg2.UI.Theme != "dracula" {
		t.Errorf("expected theme 'dracula', got '%s'", cfg2.UI.Theme)
	}
	if cfg2.UI.HistoryLimit != 50 {
		t.Errorf("expected history_limit 50, got %d", cfg2.UI.HistoryLimit)
	}
}

func TestEnvOverrides(t *testing.T) {
	cfg := Default()
	cfg.Server.WebsocketURL = "wss://original.com"
	cfg.Server.WebhookURL = "https://original.com/webhook"

	os.Setenv("MOLLY_USERNAME", "envuser")
	os.Setenv("MOLLY_CHANNEL", "envchannel")
	os.Setenv("MOLLY_WEBSOCKET_URL", "wss://env.com/ws")
	os.Setenv("MOLLY_WEBHOOK_URL", "https://env.com/webhook")
	os.Setenv("MOLLY_RELAY_URL", "https://relay.env.com")
	os.Setenv("MOLLY_AUTH_ENABLED", "true")
	os.Setenv("MOLLY_AUTH_PROVIDER", "discord")
	os.Setenv("MOLLY_DISCORD_CLIENT_ID", "discord-client-id")
	os.Setenv("MOLLY_DISCORD_CLIENT_SECRET", "discord-client-secret")
	os.Setenv("MOLLY_DISCORD_REDIRECT_URL", "http://127.0.0.1:9000/callback")
	os.Setenv("MOLLY_THEME", "solarized")
	os.Setenv("MOLLY_HISTORY_LIMIT", "200")
	defer func() {
		os.Unsetenv("MOLLY_USERNAME")
		os.Unsetenv("MOLLY_CHANNEL")
		os.Unsetenv("MOLLY_WEBSOCKET_URL")
		os.Unsetenv("MOLLY_WEBHOOK_URL")
		os.Unsetenv("MOLLY_RELAY_URL")
		os.Unsetenv("MOLLY_AUTH_ENABLED")
		os.Unsetenv("MOLLY_AUTH_PROVIDER")
		os.Unsetenv("MOLLY_DISCORD_CLIENT_ID")
		os.Unsetenv("MOLLY_DISCORD_CLIENT_SECRET")
		os.Unsetenv("MOLLY_DISCORD_REDIRECT_URL")
		os.Unsetenv("MOLLY_THEME")
		os.Unsetenv("MOLLY_HISTORY_LIMIT")
	}()

	applyEnvOverrides(cfg)

	if cfg.General.Username != "envuser" {
		t.Errorf("expected username 'envuser', got '%s'", cfg.General.Username)
	}
	if cfg.General.Channel != "envchannel" {
		t.Errorf("expected channel 'envchannel', got '%s'", cfg.General.Channel)
	}
	if cfg.Server.WebsocketURL != "wss://env.com/ws" {
		t.Errorf("expected overridden websocket_url, got '%s'", cfg.Server.WebsocketURL)
	}
	if cfg.Server.WebhookURL != "https://env.com/webhook" {
		t.Errorf("expected overridden webhook_url, got '%s'", cfg.Server.WebhookURL)
	}
	if cfg.Server.RelayURL != "https://relay.env.com" {
		t.Errorf("expected overridden relay_url, got '%s'", cfg.Server.RelayURL)
	}
	if !cfg.Auth.Enabled || cfg.Auth.Provider != "discord" {
		t.Errorf("expected discord auth env overrides to be applied, got enabled=%v provider=%q", cfg.Auth.Enabled, cfg.Auth.Provider)
	}
	if cfg.Auth.Discord.ClientID != "discord-client-id" {
		t.Errorf("expected discord client id override, got '%s'", cfg.Auth.Discord.ClientID)
	}
	if cfg.Auth.Discord.RedirectURL != "http://127.0.0.1:9000/callback" {
		t.Errorf("expected discord redirect override, got '%s'", cfg.Auth.Discord.RedirectURL)
	}
	if cfg.UI.Theme != "solarized" {
		t.Errorf("expected theme 'solarized', got '%s'", cfg.UI.Theme)
	}
	if cfg.UI.HistoryLimit != 200 {
		t.Errorf("expected history_limit 200, got %d", cfg.UI.HistoryLimit)
	}
}

func TestEnvOverridesInvalidHistoryLimit(t *testing.T) {
	cfg := Default()

	os.Setenv("MOLLY_HISTORY_LIMIT", "notanumber")
	defer os.Unsetenv("MOLLY_HISTORY_LIMIT")

	applyEnvOverrides(cfg)

	if cfg.UI.HistoryLimit != 100 {
		t.Errorf("expected default history_limit 100 to remain, got %d", cfg.UI.HistoryLimit)
	}
}

func TestEnvOverridesNegativeHistoryLimit(t *testing.T) {
	cfg := Default()

	os.Setenv("MOLLY_HISTORY_LIMIT", "-5")
	defer os.Unsetenv("MOLLY_HISTORY_LIMIT")

	applyEnvOverrides(cfg)

	if cfg.UI.HistoryLimit != 100 {
		t.Errorf("expected default history_limit 100 to remain for negative value, got %d", cfg.UI.HistoryLimit)
	}
}

func TestLoadWithCLIConfigFlag(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")

	content := `
[general]
username = "cliuser"
channel = "cli-channel"

[server]
websocket_url = "wss://cli.example.com/ws"
webhook_url = "https://discord.com/api/webhooks/cli/token"

[ui]
theme = "dracula"
history_limit = 75
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	origArgs := os.Args
	os.Args = []string{"molly", "--config", cfgPath}
	defer func() { os.Args = origArgs }()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.General.Username != "cliuser" {
		t.Errorf("expected username 'cliuser', got '%s'", cfg.General.Username)
	}
	if cfg.General.Channel != "cli-channel" {
		t.Errorf("expected channel 'cli-channel', got '%s'", cfg.General.Channel)
	}
	if cfg.Server.WebsocketURL != "wss://cli.example.com/ws" {
		t.Errorf("expected websocket_url from file, got '%s'", cfg.Server.WebsocketURL)
	}
	if cfg.UI.HistoryLimit != 75 {
		t.Errorf("expected history_limit 75, got %d", cfg.UI.HistoryLimit)
	}
}

func TestLoadWithShorthandConfigFlag(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")

	content := `
[general]
username = "shortuser"
channel = "short-channel"

[server]
websocket_url = "wss://short.example.com/ws"
webhook_url = "https://discord.com/api/webhooks/short/token"
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	origArgs := os.Args
	os.Args = []string{"molly", "-c", cfgPath}
	defer func() { os.Args = origArgs }()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.General.Username != "shortuser" {
		t.Errorf("expected username 'shortuser', got '%s'", cfg.General.Username)
	}
}

func TestEnvOverridesTakePrecedenceOverFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")

	content := `
[general]
username = "fileuser"
channel = "file-channel"

[server]
websocket_url = "wss://file.example.com/ws"
webhook_url = "https://discord.com/api/webhooks/file/token"
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	os.Setenv("MOLLY_USERNAME", "envoverride")
	defer os.Unsetenv("MOLLY_USERNAME")

	origArgs := os.Args
	os.Args = []string{"molly", "--config", cfgPath}
	defer func() { os.Args = origArgs }()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.General.Username != "envoverride" {
		t.Errorf("env var should override file value, got '%s'", cfg.General.Username)
	}
	if cfg.General.Channel != "file-channel" {
		t.Errorf("channel should come from file since no env override, got '%s'", cfg.General.Channel)
	}
}

func TestLoadMissingRequiredField(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")

	content := `
[general]
username = "testuser"

[server]
websocket_url = "wss://relay.example.com/ws"
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	origArgs := os.Args
	os.Args = []string{"molly", "--config", cfgPath}
	defer func() { os.Args = origArgs }()

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing webhook_url")
	}
}

func TestLoadNonexistentConfigFile(t *testing.T) {
	origArgs := os.Args
	os.Args = []string{"molly", "--config", "/nonexistent/path/config.toml"}
	defer func() { os.Args = origArgs }()

	clearConfigEnvVars()

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when config file doesn't exist and required fields are empty")
	}
}

func TestDefaultConfigPath(t *testing.T) {
	path, err := DefaultConfigPath()
	if err != nil {
		t.Fatalf("DefaultConfigPath() error: %v", err)
	}
	if path == "" {
		t.Error("DefaultConfigPath() returned empty path")
	}
}

func clearConfigEnvVars() {
	os.Unsetenv("MOLLY_USERNAME")
	os.Unsetenv("MOLLY_CHANNEL")
	os.Unsetenv("MOLLY_WEBSOCKET_URL")
	os.Unsetenv("MOLLY_WEBHOOK_URL")
	os.Unsetenv("MOLLY_RELAY_URL")
	os.Unsetenv("MOLLY_THEME")
	os.Unsetenv("MOLLY_HISTORY_LIMIT")
}
