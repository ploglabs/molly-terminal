# molly-terminal

A terminal-native realtime chat client built in **Go** using **Bubble Tea** and **Lip Gloss**. It provides a modern, keyboard-driven experience similar to IRC or Slack.

---

### Core Features

* **Realtime Messaging:** WebSocket-based receiving with live UI updates.
* **Hybrid Sending:** Uses Discord webhooks for low-latency message delivery.
* **Local Storage:** SQLite integration for offline history, fast search, and persistent sessions.
* **Rich TUI:** Multi-pane layout (channels, chat, users) with markdown rendering, smooth scrolling, and typing indicators.

### Commands

`/help` | `/join` | `/history` | `/search` | `/quit` | `/clear`

---

### Tech Stack

* **Language:** Go
* **UI:** Bubble Tea, Lip Gloss
* **Database:** SQLite
* **Protocol:** WebSockets & HTTP (Webhooks)

### Discord Authentication

For a terminal app like Molly, use Discord OAuth2 Authorization Code Grant with the `identify` scope. The user flow should be:

1. User runs `molly`
2. Molly checks whether a Discord access token or stored Discord identity exists
3. If not, Molly opens the Discord consent URL in the browser
4. Discord redirects to a loopback callback like `http://127.0.0.1:53682/callback`
5. Molly exchanges the code for a token, calls `GET /users/@me`, and writes the returned identity into config
6. `general.username` is set from `global_name` first, then `username`

Developer Portal setup:

1. Create an application at `https://discord.com/developers/applications`
2. Open `OAuth2`
3. Add `http://127.0.0.1:53682/callback` as a redirect
4. Copy the `Client ID` and `Client Secret`
5. Export `MOLLY_DISCORD_CLIENT_SECRET` in your shell
6. Configure Molly under `[auth]` and `[auth.discord]`

Relevant Discord docs:

* OAuth2: https://docs.discord.com/developers/topics/oauth2
* User object / `identify` scope: https://docs.discord.com/developers/resources/user

Repo integration points:

* `internal/auth/discord`: OAuth2 loopback flow, token refresh, and `/users/@me`
* `internal/config/config.go`: stores Discord app credentials, tokens, and mapped terminal identity
* `cmd/molly/main.go`: runs `discord.EnsureUserConfig(...)` before starting the TUI
