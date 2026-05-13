package commands

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ploglabs/molly-terminal/internal/config"
)

type LogoutCmd struct {
	cfg        *config.Config
	configPath string
}

func NewLogoutCmd(cfg *config.Config, configPath string) *LogoutCmd {
	return &LogoutCmd{cfg: cfg, configPath: configPath}
}

func (c *LogoutCmd) Name() string { return "logout" }

func (c *LogoutCmd) Description() string {
	return "Log out — clears Discord auth and quits (re-auth on next launch)"
}

func (c *LogoutCmd) Execute(args []string) (tea.Cmd, error) {
	c.cfg.Auth.Discord.AccessToken = ""
	c.cfg.Auth.Discord.RefreshToken = ""
	c.cfg.Auth.Discord.TokenType = ""
	c.cfg.Auth.Discord.Scope = ""
	c.cfg.Auth.Discord.Expiry = ""
	c.cfg.General.DiscordID = ""
	c.cfg.General.DiscordUsername = ""
	c.cfg.General.DiscordGlobalName = ""
	c.cfg.General.DiscordAvatarURL = ""
	c.cfg.General.Username = ""

	if err := c.cfg.Save(c.configPath); err != nil {
		return nil, fmt.Errorf("saving config: %w", err)
	}

	return func() tea.Msg {
		return LogoutMsg{}
	}, nil
}

type LogoutMsg struct{}