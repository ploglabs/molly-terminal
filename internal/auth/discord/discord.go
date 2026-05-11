package discord

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/ploglabs/molly-terminal/internal/config"
)

const (
	authorizeURL  = "https://discord.com/oauth2/authorize"
	tokenURL      = "https://discord.com/api/oauth2/token"
	userURL       = "https://discord.com/api/v10/users/@me"
	scopeIdentify = "identify"
)

type Authenticator struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	HTTPClient   *http.Client
	OpenURL      func(string) error
	Notify       func(string)
}

type Session struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	Scope        string
	ExpiresAt    time.Time
	User         User
}

type User struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	GlobalName    string `json:"global_name"`
	Avatar        string `json:"avatar"`
	Discriminator string `json:"discriminator"`
	Locale        string `json:"locale"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
}

type callbackResult struct {
	Code  string
	State string
	Err   string
}

func EnsureUserConfig(ctx context.Context, cfg *config.Config, configPath string) error {
	if !cfg.UsesDiscordAuth() {
		return nil
	}

	auth := New(cfg)

	user, err := auth.FetchCurrentUser(ctx, cfg.Auth.Discord.AccessToken)
	switch {
	case err == nil:
		applyDiscordIdentity(cfg, user)
		return cfg.Save(configPath)
	case cfg.Auth.Discord.RefreshToken != "":
		session, refreshErr := auth.Refresh(ctx, cfg.Auth.Discord.RefreshToken)
		if refreshErr != nil {
			return fmt.Errorf("refreshing discord token: %w (original fetch error: %v)", refreshErr, err)
		}
		applySession(cfg, session)
		return cfg.Save(configPath)
	default:
		session, authErr := auth.Authenticate(ctx)
		if authErr != nil {
			return fmt.Errorf("authenticating with discord: %w", authErr)
		}
		applySession(cfg, session)
		return cfg.Save(configPath)
	}
}

func New(cfg *config.Config) *Authenticator {
	return &Authenticator{
		ClientID:     cfg.Auth.Discord.ClientID,
		ClientSecret: cfg.Auth.Discord.ClientSecret,
		RedirectURL:  cfg.Auth.Discord.RedirectURL,
		HTTPClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		OpenURL: openBrowser,
		Notify:  func(msg string) { fmt.Println(msg) },
	}
}

func (a *Authenticator) Authenticate(ctx context.Context) (*Session, error) {
	state, err := randomState()
	if err != nil {
		return nil, fmt.Errorf("generating oauth state: %w", err)
	}

	redirect, err := url.Parse(a.RedirectURL)
	if err != nil {
		return nil, fmt.Errorf("parsing redirect url: %w", err)
	}
	if redirect.Scheme != "http" {
		return nil, fmt.Errorf("redirect url must use http loopback for terminal auth, got %q", redirect.Scheme)
	}

	addr := redirect.Host
	if !strings.Contains(addr, ":") {
		addr += ":80"
	}

	codeCh := make(chan callbackResult, 1)
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != redirect.Path {
				http.NotFound(w, r)
				return
			}
			result := callbackResult{
				Code:  r.URL.Query().Get("code"),
				State: r.URL.Query().Get("state"),
				Err:   r.URL.Query().Get("error"),
			}
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			if result.Err != "" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, "Discord authentication failed. You can close this tab.")
			} else {
				_, _ = io.WriteString(w, "Discord authentication complete. You can return to Molly.")
			}
			select {
			case codeCh <- result:
			default:
			}
		}),
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listening on redirect address %s: %w", addr, err)
	}
	defer ln.Close()

	go func() {
		_ = server.Serve(ln)
	}()
	defer server.Shutdown(context.Background())

	authURL := a.AuthorizationURL(state)
	if a.Notify != nil {
		a.Notify("Authenticate Molly with Discord:")
		a.Notify(authURL)
	}
	if a.OpenURL != nil {
		if err := a.OpenURL(authURL); err != nil {
			if a.Notify == nil {
				return nil, fmt.Errorf("opening browser for %s: %w", authURL, err)
			}
			a.Notify(fmt.Sprintf("Browser launch failed: %v", err))
			a.Notify("Open the URL above manually to continue.")
		}
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-codeCh:
		if result.Err != "" {
			return nil, fmt.Errorf("discord returned oauth error %q", result.Err)
		}
		if result.State != state {
			return nil, errors.New("oauth state mismatch")
		}
		token, err := a.exchangeCode(ctx, result.Code)
		if err != nil {
			return nil, err
		}
		user, err := a.FetchCurrentUser(ctx, token.AccessToken)
		if err != nil {
			return nil, err
		}
		if a.Notify != nil {
			a.Notify(fmt.Sprintf("Authenticated as %s", preferredDisplayName(user)))
		}
		return &Session{
			AccessToken:  token.AccessToken,
			RefreshToken: token.RefreshToken,
			TokenType:    token.TokenType,
			Scope:        token.Scope,
			ExpiresAt:    time.Now().Add(time.Duration(token.ExpiresIn) * time.Second).UTC(),
			User:         user,
		}, nil
	}
}

func (a *Authenticator) AuthorizationURL(state string) string {
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", a.ClientID)
	q.Set("scope", scopeIdentify)
	q.Set("state", state)
	q.Set("redirect_uri", a.RedirectURL)
	return authorizeURL + "?" + q.Encode()
}

func (a *Authenticator) Refresh(ctx context.Context, refreshToken string) (*Session, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", a.ClientID)
	form.Set("client_secret", a.ClientSecret)

	token, err := a.doTokenRequest(ctx, form)
	if err != nil {
		return nil, err
	}

	user, err := a.FetchCurrentUser(ctx, token.AccessToken)
	if err != nil {
		return nil, err
	}

	return &Session{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenType:    token.TokenType,
		Scope:        token.Scope,
		ExpiresAt:    time.Now().Add(time.Duration(token.ExpiresIn) * time.Second).UTC(),
		User:         user,
	}, nil
}

func (a *Authenticator) FetchCurrentUser(ctx context.Context, accessToken string) (User, error) {
	if accessToken == "" {
		return User{}, errors.New("missing access token")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userURL, nil)
	if err != nil {
		return User{}, fmt.Errorf("building user request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.httpClient().Do(req)
	if err != nil {
		return User{}, fmt.Errorf("requesting discord user profile: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return User{}, fmt.Errorf("discord user profile request failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var user User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return User{}, fmt.Errorf("decoding discord user profile: %w", err)
	}

	return user, nil
}

func (a *Authenticator) exchangeCode(ctx context.Context, code string) (*tokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", a.RedirectURL)
	form.Set("client_id", a.ClientID)
	form.Set("client_secret", a.ClientSecret)
	return a.doTokenRequest(ctx, form)
}

func (a *Authenticator) doTokenRequest(ctx context.Context, form url.Values) (*tokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("building token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting discord token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("discord token request failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var token tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("decoding discord token response: %w", err)
	}
	return &token, nil
}

func (a *Authenticator) httpClient() *http.Client {
	if a.HTTPClient != nil {
		return a.HTTPClient
	}
	return http.DefaultClient
}

func applySession(cfg *config.Config, session *Session) {
	cfg.Auth.Discord.AccessToken = session.AccessToken
	cfg.Auth.Discord.RefreshToken = session.RefreshToken
	cfg.Auth.Discord.TokenType = session.TokenType
	cfg.Auth.Discord.Scope = session.Scope
	cfg.Auth.Discord.Expiry = session.ExpiresAt.Format(time.RFC3339)
	applyDiscordIdentity(cfg, session.User)
}

func applyDiscordIdentity(cfg *config.Config, user User) {
	cfg.General.DiscordID = user.ID
	cfg.General.DiscordUsername = user.Username
	cfg.General.DiscordGlobalName = user.GlobalName
	cfg.General.DiscordAvatarURL = avatarURL(user.ID, user.Avatar)
	cfg.General.Username = preferredDisplayName(user)
}

func preferredDisplayName(user User) string {
	if user.GlobalName != "" {
		return user.GlobalName
	}
	return user.Username
}

func avatarURL(userID, avatarHash string) string {
	if userID == "" || avatarHash == "" {
		return ""
	}
	return fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.png", userID, avatarHash)
}

func randomState() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func openBrowser(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	return cmd.Start()
}
