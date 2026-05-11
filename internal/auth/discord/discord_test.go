package discord

import (
	"net/url"
	"testing"

	"github.com/ploglabs/molly-terminal/internal/config"
)

func TestAuthorizationURLIncludesDiscordOAuthFields(t *testing.T) {
	auth := &Authenticator{
		ClientID:    "12345",
		RedirectURL: "http://127.0.0.1:53682/callback",
	}

	raw := auth.AuthorizationURL("state-1")
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	q := parsed.Query()
	if parsed.Scheme != "https" || parsed.Host != "discord.com" {
		t.Fatalf("expected discord authorization host, got %s", parsed.Host)
	}
	if q.Get("client_id") != "12345" {
		t.Fatalf("expected client_id to be set, got %q", q.Get("client_id"))
	}
	if q.Get("redirect_uri") != "http://127.0.0.1:53682/callback" {
		t.Fatalf("expected redirect_uri to be set, got %q", q.Get("redirect_uri"))
	}
	if q.Get("scope") != "identify" {
		t.Fatalf("expected identify scope, got %q", q.Get("scope"))
	}
	if q.Get("response_type") != "code" {
		t.Fatalf("expected response_type=code, got %q", q.Get("response_type"))
	}
}

func TestApplyDiscordIdentityUsesGlobalNameFirst(t *testing.T) {
	cfg := config.Default()

	applyDiscordIdentity(cfg, User{
		ID:         "1",
		Username:   "discorduser",
		GlobalName: "Display Name",
		Avatar:     "hash",
	})

	if cfg.General.Username != "Display Name" {
		t.Fatalf("expected global_name to become username, got %q", cfg.General.Username)
	}
	if cfg.General.DiscordAvatarURL == "" {
		t.Fatal("expected avatar URL to be populated")
	}
}
