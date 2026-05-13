# molly-terminal

A terminal-native realtime chat client built in Go. Molly connects to a relay server over WebSockets, sends messages via Discord webhooks, and stores everything locally in SQLite — giving you a keyboard-driven, offline-capable chat experience right in your terminal.

## Features

- **Realtime messaging** — WebSocket-based receiving with automatic reconnection and exponential backoff
- **Discord webhook sending** — Low-latency message delivery through Discord webhooks
- **Local SQLite storage** — Offline message history, fast full-text search, persistent sessions
- **Multi-pane TUI** — Channels sidebar, chat viewport, and users panel with toggleable visibility
- **Markdown rendering** — Bold, italic, inline code, code blocks, links, and emoji shortcodes
- **Typing indicators** — See who's typing in real time
- **Tab completion** — Complete `@usernames` and `#channels` with Tab
- **Slash commands** — `/help`, `/join`, `/history`, `/search`, `/clear`, `/quit`
- **Discord OAuth2** — First-run browser-based Discord login; identity persisted across sessions
- **Cross-platform** — Works on Linux, macOS, and Windows (pure Go, no CGO required)

## Installation

### Pre-built binaries

Download the latest release from [GitHub Releases](https://github.com/ploglabs/molly-terminal/releases).

### macOS — Homebrew

```bash
brew install ploglabs/tap/molly
```

### Linux — apt (Debian/Ubuntu)

```bash
# Download the latest .deb from GitHub Releases
curl -LO https://github.com/ploglabs/molly-terminal/releases/latest/download/molly_$(curl -s https://api.github.com/repos/ploglabs/molly-terminal/releases/latest | grep tag_name | cut -d'"' -f4)_linux_amd64.deb
sudo dpkg -i molly_*_linux_amd64.deb
```

### Linux — pacman (Arch)

```bash
yay -S molly-bin
# or download .pkg.tar.zst from GitHub Releases
```

### Linux — rpm (Fedora/RHEL)

```bash
curl -LO https://github.com/ploglabs/molly-terminal/releases/latest/download/molly_$(curl -s https://api.github.com/repos/ploglabs/molly-terminal/releases/latest | grep tag_name | cut -d'"' -f4)_linux_amd64.rpm
sudo rpm -i molly_*_linux_amd64.rpm
```

### Linux — Snap

```bash
sudo snap install molly
```

### Linux/macOS — Nix

```bash
nix profile install github:ploglabs/molly-terminal
```

### Windows — Scoop

```powershell
scoop bucket add ploglabs https://github.com/ploglabs/scoop-bucket
scoop install molly
```

### go install

```bash
go install github.com/ploglabs/molly-terminal/cmd/molly@latest
```

### Build from source

```bash
git clone https://github.com/ploglabs/molly-terminal.git
cd molly-terminal
make build
```

The binary is output at `./bin/molly`.

## Quick Start

1. **Create a config file** at `~/.config/molly/config.toml` (or use `--config PATH`):

   ```toml
   [general]
   username = "your_name"
   channel = "general"

   [server]
   websocket_url = "wss://relay.example.com/ws"
   webhook_url = "https://discord.com/api/webhooks/YOUR_WEBHOOK_ID/YOUR_WEBHOOK_TOKEN"
   relay_url = "https://relay.example.com"
   ```

2. **Or use environment variables**:

   ```bash
   export MOLLY_USERNAME=your_name
   export MOLLY_CHANNEL=general
   export MOLLY_WEBSOCKET_URL=wss://relay.example.com/ws
   export MOLLY_WEBHOOK_URL=https://discord.com/api/webhooks/ID/TOKEN
   export MOLLY_RELAY_URL=https://relay.example.com
   ```

3. **Run Molly**:

   ```bash
   molly
   ```

   On first launch with Discord auth enabled, Molly will open your browser for OAuth2 consent and save the resulting identity to your config file automatically.

## Configuration

Molly loads configuration from **three sources** in priority order:

1. **Environment variables** (highest priority — override everything)
2. **Config file** (`~/.config/molly/config.toml` by default)
3. **Defaults**

### Config file location

| Platform | Path |
|----------|------|
| Linux / macOS | `~/.config/molly/config.toml` (or `$XDG_CONFIG_HOME/molly/config.toml`) |
| Windows | `%APPDATA%\molly\config.toml` |

Override with `--config PATH` or `-c PATH`.

### Config reference

See [`config.example.toml`](config.example.toml) for a fully-documented example.

#### `[general]`

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `username` | string | `""` | Display name. Required unless using Discord auth. |
| `channel` | string | `"general"` | Default channel to join on startup. |
| `discord_id` | string | `""` | Populated automatically by Discord auth. |
| `discord_username` | string | `""` | Populated automatically by Discord auth. |
| `discord_global_name` | string | `""` | Populated automatically by Discord auth. |
| `discord_avatar_url` | string | `""` | Populated automatically by Discord auth. |

#### `[server]`

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `websocket_url` | string | `""` | **Required.** WebSocket relay URL (e.g. `wss://relay.example.com/ws`). |
| `webhook_url` | string | `""` | **Required.** Discord webhook URL for sending messages. |
| `relay_url` | string | `""` | REST API base URL for fetching message history. |

#### `[auth]`

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `enabled` | bool | `false` | Enable Discord OAuth2 login flow. |
| `provider` | string | `""` | Auth provider. Currently only `"discord"` is supported. |

#### `[auth.discord]`

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `client_id` | string | `""` | Discord application client ID. |
| `client_secret` | string | `""` | Discord application client secret. **Use `MOLLY_DISCORD_CLIENT_SECRET` env var instead of committing this.** |
| `redirect_url` | string | `"http://127.0.0.1:53682/callback"` | OAuth2 redirect URL. Must match the Discord Developer Portal. |
| `access_token` | string | `""` | Populated automatically after login. |
| `refresh_token` | string | `""` | Populated automatically after login. |
| `token_type` | string | `""` | Populated automatically after login. |
| `scope` | string | `""` | Populated automatically after login. |
| `expiry` | string | `""` | Populated automatically after login (RFC 3339). |

#### `[ui]`

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `theme` | string | `"default"` | Color theme: `default`, `dracula`, or `solarized`. |
| `history_limit` | int | `100` | Number of messages to fetch on startup. |

### Environment variables

| Variable | Maps to |
|----------|---------|
| `MOLLY_USERNAME` | `general.username` |
| `MOLLY_CHANNEL` | `general.channel` |
| `MOLLY_WEBSOCKET_URL` | `server.websocket_url` |
| `MOLLY_WEBHOOK_URL` | `server.webhook_url` |
| `MOLLY_RELAY_URL` | `server.relay_url` |
| `MOLLY_AUTH_ENABLED` | `auth.enabled` |
| `MOLLY_AUTH_PROVIDER` | `auth.provider` |
| `MOLLY_DISCORD_CLIENT_ID` | `auth.discord.client_id` |
| `MOLLY_DISCORD_CLIENT_SECRET` | `auth.discord.client_secret` |
| `MOLLY_DISCORD_REDIRECT_URL` | `auth.discord.redirect_url` |
| `MOLLY_THEME` | `ui.theme` |
| `MOLLY_HISTORY_LIMIT` | `ui.history_limit` |

Environment variables take precedence over config file values. Any field can be set via env var, which is especially useful for secrets like `MOLLY_DISCORD_CLIENT_SECRET`.

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Enter` | Send message (or run command if prefixed with `/`) |
| `Alt+Enter` / `Shift+Enter` | Insert newline |
| `Ctrl+C` / `Ctrl+Q` | Quit |
| `Esc` | Clear input |
| `Ctrl+B` | Toggle channels sidebar |
| `Ctrl+Y` | Toggle users panel |
| `Ctrl+L` | Scroll to bottom / clear unread counter |
| `Ctrl+P` | Previous channel |
| `Ctrl+N` | Next channel |
| `Ctrl+W` | Delete word backwards |
| `Ctrl+A` | Move cursor to start of input |
| `Ctrl+E` | Move cursor to end of input |
| `Ctrl+K` | Delete from cursor to end of line |
| `Ctrl+U` | Clear entire input |
| `Tab` | Complete `@user` or `#channel` |
| `Up` | Scroll up / view older messages |
| `Down` | Scroll down / view newer messages |
| `PgUp` | Scroll up by half a page |
| `PgDn` | Scroll down by half a page |

### Slash Commands

| Command | Description |
|---------|-------------|
| `/help` | Show available commands |
| `/join #channel` | Switch to a channel |
| `/history` | Load older messages |
| `/search <query>` | Search local message history |
| `/clear` | Clear the current chat view |
| `/quit` | Exit Molly |

## Discord OAuth2 Setup

Molly supports Discord OAuth2 for seamless identity management. On first run, Molly opens a browser for consent, exchanges the auth code, and stores your Discord identity.

1. Create an application at [Discord Developer Portal](https://discord.com/developers/applications)
2. Under **OAuth2**, add `http://127.0.0.1:53682/callback` as a redirect
3. Copy the **Client ID** and **Client Secret**
4. Export the secret: `export MOLLY_DISCORD_CLIENT_SECRET=your_secret`
5. Configure Molly:

   ```toml
   [auth]
   enabled = true
   provider = "discord"

   [auth.discord]
   client_id = "YOUR_DISCORD_CLIENT_ID"
   redirect_url = "http://127.0.0.1:53682/callback"
   ```

6. Run `molly` — it will open your browser, complete the OAuth flow, and save your identity.

## Architecture

```
cmd/molly/          Entry point
internal/
  auth/discord/      Discord OAuth2 loopback flow, token refresh
  commands/          Slash command registry (/help, /join, etc.)
  config/            TOML config loader with env-var overrides
  db/                SQLite storage (modernc.org/sqlite — pure Go, no CGO)
  history/           Relay REST client for fetching message history
  model/             Shared data types (Message, Channel, TypingEvent)
  tui/               Bubble Tea TUI: multi-pane layout, viewport, input, markdown
  webhook/           Discord webhook HTTP sender
  wsclient/          WebSocket client with auto-reconnect
```

Molly uses a pure-Go SQLite driver (`modernc.org/sqlite`) so it builds with no CGO dependency, making cross-compilation straightforward.

## Data Locations

| Platform | Config | Database |
|----------|--------|----------|
| Linux/macOS | `$XDG_CONFIG_HOME/molly/config.toml` or `~/.config/molly/config.toml` | `$XDG_DATA_HOME/molly/molly.db` or `~/.local/share/molly/molly.db` |
| Windows | `%APPDATA%\molly\config.toml` | `%LOCALAPPDATA%\molly\molly.db` |

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b my-feature`
3. Make your changes and add tests
4. Run `make test` to verify
5. Submit a pull request

Please open an issue first to discuss significant changes.

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE) for details.