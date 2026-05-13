package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joho/godotenv"
	"github.com/ploglabs/molly-terminal/internal/auth/discord"
	"github.com/ploglabs/molly-terminal/internal/commands"
	"github.com/ploglabs/molly-terminal/internal/config"
	"github.com/ploglabs/molly-terminal/internal/db"
	"github.com/ploglabs/molly-terminal/internal/history"
	"github.com/ploglabs/molly-terminal/internal/tui"
	"github.com/ploglabs/molly-terminal/internal/webhook"
	"github.com/ploglabs/molly-terminal/internal/wsclient"
)

func main() {
	_ = godotenv.Load()

	configPath, err := config.ConfigPathFromArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	if err := discord.EnsureUserConfig(context.Background(), cfg, configPath); err != nil {
		fmt.Fprintf(os.Stderr, "discord auth error: %v\n", err)
		os.Exit(1)
	}

	store, err := db.New("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "database error: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	client := wsclient.New(cfg.Server.WebsocketURL, cfg.General.Username, cfg.General.Channel)
	sender := webhook.New(cfg.Server.WebhookURL, cfg.Server.RelayURL, cfg.Server.APIKey, cfg.General.Username, cfg.General.DiscordAvatarURL)
	fetcher := history.New(cfg.Server.RelayURL)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go client.ConnectWithRetry(ctx)

	registry := commands.NewRegistry()
	registry.Register(commands.NewHelpCmd(registry))
	registry.Register(commands.NewJoinCmd(client, fetcher, store))
	registry.Register(commands.NewHistoryCmd())
	registry.Register(commands.NewSearchCmd(store))
	registry.Register(commands.NewQuitCmd())
	registry.Register(commands.NewClearCmd())
	registry.Register(commands.NewLeaveCmd(store))
	registry.Register(commands.NewStatusCmd())
	registry.Register(commands.NewFileCmd())
	registry.Register(commands.NewSnippetCmd())
	registry.Register(commands.NewLogoutCmd(cfg, configPath))

	tui.InitImageProtocol(cfg.UI.ImageProtocol)

	model := tui.New(client, sender, store, fetcher, registry, cfg.General.Channel, cfg.General.Username)
	p := tea.NewProgram(model, tea.WithAltScreen())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		_ = client.Close()
		cancel()
		p.Quit()
	}()

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
