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
	"github.com/ploglabs/molly-terminal/internal/config"
	"github.com/ploglabs/molly-terminal/internal/db"
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
	sender := webhook.New(cfg.Server.WebhookURL, cfg.General.Username)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go client.ConnectWithRetry(ctx)

	model := tui.New(client, sender, store, cfg.General.Channel)
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
