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
