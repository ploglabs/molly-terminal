package tui

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ploglabs/molly-terminal/internal/commands"
	"github.com/ploglabs/molly-terminal/internal/db"
	"github.com/ploglabs/molly-terminal/internal/history"
	"github.com/ploglabs/molly-terminal/internal/model"
	"github.com/ploglabs/molly-terminal/internal/webhook"
	"github.com/ploglabs/molly-terminal/internal/wsclient"
)

type wsMsg model.Message

type wsStatusMsg struct {
	Status wsclient.Status
	Err    error
}

type typingEventMsg model.TypingEvent

type typingTickMsg struct{}

type presenceMsg model.UserPresence

type terminalUsersMsg []string

type dbWriteResultMsg struct {
	Err error
}

type localHistoryMsg struct {
	Messages []model.Message
	Channels []string
	Channel  string
	Err      error
}

type Model struct {
	width  int
	height int

	client   *wsclient.Client
	sender   *webhook.Sender
	store    *db.Store
	fetcher  *history.Fetcher
	registry *commands.Registry

	msgs       []model.Message
	status     wsclient.Status
	channel    string
	username   string
	channels   []string
	available  map[string]struct{}
	channelsOK bool
	users      []string
	lastSendOk bool
	sendErr    string

	loadingHistory   bool
	historyLoaded    bool
	allHistoryLoaded bool

	scrollOffset int
	unreadCount  int
	input        InputModel
	log          *log.Logger

	channelsVisible bool
	usersVisible    bool
	notifVisible    bool

	typingUsers map[string]time.Time
	sentHashes  map[string]time.Time
	presences   map[string]model.UserPresence
	myStatus    string

	terminalOnline []string

	replyTo       *model.Message
	notifications []model.Notification
	notifIdx      int
	notifFocused  bool
	jumpToID      string

	replySelectMode bool
	replySelectIdx  int
}

func New(client *wsclient.Client, sender *webhook.Sender, store *db.Store, fetcher *history.Fetcher, registry *commands.Registry, channel, username string) Model {
	channels := []string{channel}
	var notifications []model.Notification
	if store != nil {
		if storedChannels, err := store.GetChannels(); err == nil {
			channels = mergeChannels(channels, storedChannels)
		}
		_ = store.InsertChannel(channel)
		if storedNotifs, err := store.GetNotifications(); err == nil {
			notifications = storedNotifs
		}
	}

	return Model{
		client:           client,
		sender:           sender,
		store:            store,
		fetcher:          fetcher,
		registry:         registry,
		channel:          channel,
		username:         username,
		channels:         channels,
		available:        make(map[string]struct{}),
		status:           wsclient.StatusDisconnected,
		lastSendOk:       true,
		loadingHistory:   false,
		historyLoaded:    false,
		allHistoryLoaded: false,
		input:            newInput("> "),
		channelsVisible:  true,
		usersVisible:     true,
		notifVisible:     true,
		notifications:    notifications,
		typingUsers:      make(map[string]time.Time),
		sentHashes:       make(map[string]time.Time),
		presences:        make(map[string]model.UserPresence),
		log:              log.Default(),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.listenMessages(),
		m.listenStatus(),
		m.listenTyping(),
		m.listenPresence(),
		m.listenTerminalUsers(),
		history.FetchChannels(m.fetcher),
		m.loadLocalHistory(m.channel, 100),
		history.InitialFetch(m.fetcher, m.channel, 100),
		m.input.CursorBlinkCmd(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.SetWidth(m.chatWidth())
		return m, nil

	case wsMsg:
		m.addChannel(msg.Channel)

		// Check for @mention
		var notifCmd tea.Cmd
		if msg.Username != m.username && strings.Contains(
			strings.ToLower(msg.Content), "@"+strings.ToLower(m.username)) {
			n := model.Notification{
				Channel:   msg.Channel,
				Username:  msg.Username,
				Content:   msg.Content,
				Timestamp: msg.Timestamp,
				MsgID:     msg.ID,
			}
			m.notifications = append(m.notifications, n)
			notifCmd = m.persistNotification(n)
		}

		if msg.Channel != m.channel {
			batch := []tea.Cmd{m.listenMessages(), m.persistMessage(model.Message(msg))}
			if notifCmd != nil {
				batch = append(batch, notifCmd)
			}
			return m, tea.Batch(batch...)
		}
		key := contentHash(msg.Username, msg.Channel, msg.Content)
		if _, exists := m.sentHashes[key]; exists {
			delete(m.sentHashes, key)
			return m, m.listenMessages()
		}
		m.msgs = insertSorted(m.msgs, model.Message(msg))
		if len(m.msgs) > 1000 {
			m.msgs = m.msgs[len(m.msgs)-1000:]
		}
		m.users = msgsToUsers(m.msgs)
		if m.scrollOffset > 0 {
			m.unreadCount++
		} else {
			m.scrollOffset = 0
		}
		batch := []tea.Cmd{m.listenMessages(), m.persistMessage(model.Message(msg))}
		if notifCmd != nil {
			batch = append(batch, notifCmd)
		}
		return m, tea.Batch(batch...)

	case wsStatusMsg:
		m.status = msg.Status
		if msg.Status == wsclient.StatusConnected && m.channel != "" {
			return m, tea.Batch(m.listenStatus(), m.subscribeCmd(m.channel))
		}
		return m, m.listenStatus()

	case typingEventMsg:
		m.addChannel(msg.Channel)
		if msg.Channel == m.channel || msg.Channel == "" {
			if m.typingUsers == nil {
				m.typingUsers = make(map[string]time.Time)
			}
			m.typingUsers[msg.Username] = time.Now()
		}
		return m, tea.Batch(m.listenTyping(), typingTickCmd())

	case typingTickMsg:
		now := time.Now()
		for user, lastSeen := range m.typingUsers {
			if now.Sub(lastSeen) > 3*time.Second {
				delete(m.typingUsers, user)
			}
		}
		for h, ts := range m.sentHashes {
			if now.Sub(ts) > 10*time.Second {
				delete(m.sentHashes, h)
			}
		}
		var cmds []tea.Cmd
		if len(m.typingUsers) > 0 {
			cmds = append(cmds, typingTickCmd())
		}
		return m, tea.Batch(cmds...)

	case terminalUsersMsg:
		m.terminalOnline = []string(msg)
		return m, m.listenTerminalUsers()

	case history.ChannelsResultMsg:
		if msg.Err != nil {
			m.log.Printf("channels: %v", msg.Err)
			m.channelsOK = true // allow /join even if channels couldn't load
			return m, nil
		}
		m.channelsOK = true
		if len(msg.Channels) == 0 {
			return m, nil
		}
		m.channels = mergeChannels([]string{m.channel}, msg.Channels)
		m.available = channelsToSet(m.channels)
		for _, ch := range m.channels {
			if m.store != nil {
				_ = m.store.InsertChannel(ch)
			}
		}
		return m, nil

	case history.FetchResultMsg:
		if msg.Channel != "" && msg.Channel != m.channel {
			return m, nil
		}
		m.loadingHistory = false
		if msg.Err != nil {
			m.log.Printf("history: %v", msg.Err)
			return m, nil
		}
		if len(msg.Messages) == 0 {
			m.allHistoryLoaded = true
			return m, nil
		}
		if len(msg.Messages) < 100 {
			m.allHistoryLoaded = true
		}
		m.msgs = mergeMessages(m.msgs, msg.Messages)
		m.historyLoaded = true
		if m.jumpToID != "" && m.jumpToMessage(m.jumpToID) {
			m.jumpToID = ""
		}
		for _, msg := range msg.Messages {
			cmds = append(cmds, m.persistMessage(msg))
		}
		m.users = msgsToUsers(m.msgs)
		return m, tea.Batch(cmds...)

	case localHistoryMsg:
		if msg.Channel != "" && msg.Channel != m.channel {
			return m, nil
		}
		if msg.Err != nil {
			m.log.Printf("local history: %v", msg.Err)
			return m, nil
		}
		if len(m.channels) == 0 {
			m.channels = mergeChannels([]string{m.channel}, msg.Channels)
		}
		if len(msg.Messages) > 0 {
			m.msgs = mergeMessages(m.msgs, msg.Messages)
			m.users = msgsToUsers(m.msgs)
			m.historyLoaded = true
			if m.jumpToID != "" && m.jumpToMessage(m.jumpToID) {
				m.jumpToID = ""
			}
		}
		return m, nil

	case webhook.SendResultMsg:
		if msg.Err != nil {
			m.lastSendOk = false
			m.sendErr = msg.Err.Error()
		} else {
			m.lastSendOk = true
			m.sendErr = ""
		}
		return m, nil

	case webhook.SendFileResultMsg:
		if msg.Err != nil {
			m.lastSendOk = false
			m.sendErr = msg.Err.Error()
		} else {
			m.lastSendOk = true
			m.sendErr = ""
		}
		return m, nil

	case dbWriteResultMsg:
		if msg.Err != nil {
			m.log.Printf("db: %v", msg.Err)
		}
		return m, nil

	case commands.CommandOutputMsg:
		for _, sysMsg := range msg.Messages {
			m.msgs = append(m.msgs, sysMsg)
		}
		if len(m.msgs) > 1000 {
			m.msgs = m.msgs[len(m.msgs)-1000:]
		}
		m.scrollOffset = 0
		return m, nil

	case commands.SwitchChannelMsg:
		oldChannel := m.channel
		m.channel = msg.Channel

		m.addChannel(msg.Channel)

		m.msgs = nil
		m.scrollOffset = 0
		m.allHistoryLoaded = false
		m.historyLoaded = false
		m.replyTo = nil

		sysMsg := commands.SystemMsg(fmt.Sprintf("switched to #%s", msg.Channel))
		m.msgs = append(m.msgs, sysMsg)

		cmds = append(cmds, m.subscribeSwitchCmd(oldChannel, msg.Channel))
		cmds = append(cmds, m.loadLocalHistory(msg.Channel, 100))
		cmds = append(cmds, history.InitialFetch(m.fetcher, msg.Channel, 100))

		return m, tea.Batch(cmds...)

	case commands.ClearMessagesMsg:
		m.clearScreen()
		return m, nil

	case commands.DeleteChannelMsg:
		chName := msg.Channel
		if len(m.channels) <= 1 {
			sysMsg := commands.SystemMsg("cannot leave — you must be in at least one channel")
			m.msgs = append(m.msgs, sysMsg)
			m.scrollOffset = 0
			return m, nil
		}
		found := false
		for _, ch := range m.channels {
			if ch == chName {
				found = true
				break
			}
		}
		if !found {
			sysMsg := commands.SystemMsg(fmt.Sprintf("channel #%s not found in your list", chName))
			m.msgs = append(m.msgs, sysMsg)
			m.scrollOffset = 0
			return m, nil
		}
		m.removeChannel(chName)
		if m.store != nil {
			_ = m.store.DeleteChannel(chName)
		}
		if m.channel == chName {
			newChannel := m.channels[0]
			m.channel = newChannel
			m.msgs = nil
			m.scrollOffset = 0
			m.allHistoryLoaded = false
			m.historyLoaded = false
			sysMsg := commands.SystemMsg(fmt.Sprintf("left #%s, switched to #%s", chName, newChannel))
			m.msgs = append(m.msgs, sysMsg)
			cmds = append(cmds, m.subscribeSwitchCmd(chName, newChannel))
			cmds = append(cmds, m.loadLocalHistory(newChannel, 100))
			cmds = append(cmds, history.InitialFetch(m.fetcher, newChannel, 100))
		} else {
			sysMsg := commands.SystemMsg(fmt.Sprintf("removed #%s from channels", chName))
			m.msgs = append(m.msgs, sysMsg)
			m.scrollOffset = 0
		}
		return m, tea.Batch(cmds...)

	case commands.TriggerHistoryLoadMsg:
		if m.loadingHistory || m.allHistoryLoaded || len(m.msgs) == 0 {
			return m, nil
		}
		m.loadingHistory = true
		oldest := m.msgs[0].Timestamp
		return m, history.LoadOlder(m.fetcher, m.channel, oldest)

	case presenceMsg:
		p := model.UserPresence(msg)
		m.presences[p.Username] = p
		if m.store != nil {
			store := m.store
			go func() { _ = store.UpsertPresence(p) }()
		}
		return m, m.listenPresence()

	case commands.SendRawMsg:
		return m, m.SendMessage(msg.Content, m.channel, "")

	case commands.SendFileMsg:
		return m.sendFileWithEcho(msg.Path, msg.Content)

	case commands.OpenImageMsg:
		return m.openImage(msg.Index)

	case commands.LogoutMsg:
		sysMsg := commands.SystemMsg("logged out — restart molly to re-authenticate")
		m.msgs = append(m.msgs, sysMsg)
		m.scrollOffset = 0
		if m.client != nil {
			_ = m.client.Close()
		}
		return m, tea.Quit

	case commands.SetStatusMsg:
		m.myStatus = msg.Status
		now := time.Now()
		p := model.UserPresence{
			Username:  m.username,
			Status:    msg.Status,
			Online:    true,
			LastSeen:  now,
			UpdatedAt: now,
		}
		m.presences[m.username] = p
		var sysContent string
		if msg.Status == "" {
			sysContent = "status cleared"
		} else {
			sysContent = fmt.Sprintf("status set: %s", msg.Status)
		}
		sysMsg := commands.SystemMsg(sysContent)
		m.msgs = append(m.msgs, sysMsg)
		m.scrollOffset = 0
		client := m.client
		store := m.store
		return m, func() tea.Msg {
			if client != nil {
				_ = client.SendStatus(msg.Status)
			}
			if store != nil {
				_ = store.UpsertPresence(p)
			}
			return nil
		}

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	var inputCmd tea.Cmd
	m.input, inputCmd = m.input.Update(msg)
	if inputCmd != nil {
		cmds = append(cmds, inputCmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle Alt+R or Ctrl+G: enter reply select mode
	altR := msg.Alt && len(msg.Runes) == 1 && (msg.Runes[0] == 'r' || msg.Runes[0] == 'R')
	isCtrlG := msg.String() == "ctrl+g"
	if altR || isCtrlG {
		if m.replySelectMode {
			return m, nil
		}
		m.replySelectMode = true
		m.replySelectIdx = -1
		// Default to most recent non-system message
		for i := len(m.msgs) - 1; i >= 0; i-- {
			if m.msgs[i].Username != "system" {
				m.replySelectIdx = i
				break
			}
		}
		if m.replySelectIdx >= 0 {
			m.ensureReplySelectVisible()
		}
		return m, nil
	}

	switch msg.String() {
	case "ctrl+c":
		if m.client != nil {
			_ = m.client.Close()
		}
		return m, tea.Quit

	case "ctrl+q":
		if m.client != nil {
			_ = m.client.Close()
		}
		return m, tea.Quit

	case "esc":
		if m.replySelectMode {
			m.replySelectMode = false
			m.replySelectIdx = -1
			return m, nil
		}
		if m.replyTo != nil {
			m.replyTo = nil
			return m, nil
		}
		if m.input.Value() != "" {
			m.input.Clear()
		}
		return m, nil

	case "ctrl+b":
		m.channelsVisible = !m.channelsVisible
		return m, nil

	case "ctrl+y":
		m.usersVisible = !m.usersVisible
		return m, nil

	case "ctrl+l":
		m.scrollOffset = 0
		m.unreadCount = 0
		return m, nil

	case "ctrl+p":
		m.prevChannel()
		m.replyTo = nil
		return m, m.subscribeCmd(m.channel)

	case "ctrl+n":
		m.nextChannel()
		m.replyTo = nil
		return m, m.subscribeCmd(m.channel)

	case "ctrl+r":
		// Reply to most recent non-system message
		for i := len(m.msgs) - 1; i >= 0; i-- {
			if m.msgs[i].Username != "system" {
				m.replyTo = &m.msgs[i]
				break
			}
		}
		return m, nil

	case "ctrl+]":
		m.notifVisible = true
		if !m.notifFocused {
			m.notifFocused = true
		} else if len(m.notifications) > 0 {
			m.notifIdx = (m.notifIdx + 1) % len(m.notifications)
		}
		return m, nil

	case "up":
		if m.replySelectMode {
			m.moveReplySelectUp()
			return m, nil
		}
		if m.notifFocused {
			if m.notifIdx > 0 {
				m.notifIdx--
			}
			return m, nil
		}
		m.scrollOffset++
		if m.scrollOffset > len(m.msgs)-1 {
			m.scrollOffset = len(m.msgs) - 1
		}
		return m, m.loadOlderIfNeeded()

	case "down":
		if m.replySelectMode {
			m.moveReplySelectDown()
			return m, nil
		}
		if m.notifFocused {
			if m.notifIdx < len(m.notifications)-1 {
				m.notifIdx++
			}
			return m, nil
		}
		m.scrollOffset--
		if m.scrollOffset < 0 {
			m.scrollOffset = 0
			m.unreadCount = 0
		}
		return m, nil

	case "pgup":
		m.scrollOffset += m.chatHeight() / 2
		if m.scrollOffset > len(m.msgs)-1 {
			m.scrollOffset = len(m.msgs) - 1
		}
		if m.scrollOffset < 0 {
			m.scrollOffset = 0
		}
		return m, m.loadOlderIfNeeded()

	case "pgdown":
		m.scrollOffset -= m.chatHeight() / 2
		if m.scrollOffset < 0 {
			m.scrollOffset = 0
			m.unreadCount = 0
		}
		return m, nil

	case "enter":
		// Confirm reply selection
		if m.replySelectMode {
			if m.replySelectIdx >= 0 && m.replySelectIdx < len(m.msgs) && m.msgs[m.replySelectIdx].Username != "system" {
				msg := m.msgs[m.replySelectIdx]
				m.replyTo = &msg
			}
			m.replySelectMode = false
			m.replySelectIdx = -1
			return m, nil
		}
		// Jump to notification's channel
		if m.notifFocused && len(m.notifications) > 0 && m.notifIdx < len(m.notifications) {
			n := m.notifications[m.notifIdx]
			if n.Channel != m.channel {
				m.jumpToID = n.MsgID
				return m, func() tea.Msg {
					return commands.SwitchChannelMsg{Channel: n.Channel}
				}
			}
			m.jumpToMessage(n.MsgID)
			return m, nil
		}

	case "tab":
		word, prefix := m.input.WordAtCursor()
		var candidates []string
		if m.isJoinInput() {
			needle := strings.TrimPrefix(word, "#")
			for _, ch := range m.channels {
				if strings.HasPrefix(strings.ToLower(ch), strings.ToLower(needle)) {
					candidates = append(candidates, "#"+ch)
				}
			}
		} else if m.isOpenInput() {
			// Autocomplete image indices for /open
			inputStr := strings.TrimSpace(string(m.input.text))
			parts := strings.Fields(inputStr)
			needle := ""
			if len(parts) > 1 {
				needle = parts[1]
			}
			var images []model.Attachment
			for i := len(m.msgs) - 1; i >= 0; i-- {
				for _, att := range m.msgs[i].Attachments {
					if isImageAttachment(att) {
						images = append(images, att)
					}
				}
			}
			for idx, img := range images {
				n := idx + 1
				candidate := fmt.Sprintf("%d", n)
				if needle == "" || strings.HasPrefix(candidate, needle) {
					candidates = append(candidates, fmt.Sprintf("%d (%s)", n, img.Filename))
				}
			}
		} else {
			switch prefix {
			case "/":
				for _, c := range m.commandCandidates(word) {
					candidates = append(candidates, "/"+c.Name())
				}
			case "@":
				allUsers := m.allKnownUsers()
				for _, u := range allUsers {
					if strings.HasPrefix(strings.ToLower(u), strings.ToLower(word)) {
						candidates = append(candidates, "@"+u)
					}
				}
			case "#":
				for _, ch := range m.channels {
					if strings.HasPrefix(strings.ToLower(ch), strings.ToLower(word)) {
						candidates = append(candidates, "#"+ch)
					}
				}
			default:
				if word != "" {
					allUsers := m.allKnownUsers()
					for _, u := range allUsers {
						if strings.HasPrefix(strings.ToLower(u), strings.ToLower(word)) {
							candidates = append(candidates, "@"+u)
						}
					}
				}
			}
		}
		if len(candidates) > 0 {
			if len(m.input.completions) == 0 {
				m.input.SetCompletions(candidates)
			}
			m.input.ApplyNextCompletion()
		}
		return m, nil
	}

	if m.replySelectMode {
		return m, nil
	}

	submitted, value := handleInputKey(msg, &m.input)
	if submitted {
		if strings.TrimSpace(value) == "" {
			return m, nil
		}
		val := strings.TrimRight(value, "\n")

		if strings.HasPrefix(strings.TrimSpace(val), "/") {
			cmdName, args := commands.ParseInput(strings.TrimSpace(val))
			if cmdName == "" {
				return m, nil
			}
			if cmdName == "join" {
				if err := m.validateJoin(args); err != nil {
					sysMsg := commands.SystemMsg(err.Error())
					m.msgs = append(m.msgs, sysMsg)
					m.scrollOffset = 0
					return m, nil
				}
			}
			cmd, err := m.registry.Execute(cmdName, args)
			if err != nil {
				sysMsg := commands.SystemMsg(fmt.Sprintf("error: /%s — %v", cmdName, err))
				m.msgs = append(m.msgs, sysMsg)
				m.scrollOffset = 0
				return m, nil
			}
			return m, cmd
		}

		return m.sendWithEcho(val)
	}

	m.maybeAutoComplete()

	return m, nil
}

func (m *Model) clearScreen() {
	m.msgs = nil
	m.scrollOffset = 0
	m.unreadCount = 0
}

func (m *Model) loadOlderIfNeeded() tea.Cmd {
	if !m.loadingHistory && !m.allHistoryLoaded && m.historyLoaded && len(m.msgs) > 0 {
		if m.scrollOffset >= len(m.msgs)-m.chatHeight() {
			oldest := m.msgs[0].Timestamp
			m.loadingHistory = true
			return history.LoadOlder(m.fetcher, m.channel, oldest)
		}
	}
	return nil
}

// onlineUsers returns only terminal-connected users.
func (m Model) onlineUsers() []string {
	if m.terminalOnline == nil {
		return []string{}
	}
	return m.terminalOnline
}

func (m *Model) moveReplySelectUp() {
	if len(m.msgs) == 0 {
		return
	}
	// Move to previous non-system message
	for i := m.replySelectIdx - 1; i >= 0; i-- {
		if m.msgs[i].Username != "system" {
			m.replySelectIdx = i
			m.ensureReplySelectVisible()
			return
		}
	}
}

func (m *Model) moveReplySelectDown() {
	if len(m.msgs) == 0 {
		return
	}
	// Move to next non-system message
	for i := m.replySelectIdx + 1; i < len(m.msgs); i++ {
		if m.msgs[i].Username != "system" {
			m.replySelectIdx = i
			m.ensureReplySelectVisible()
			return
		}
	}
}

func (m *Model) ensureReplySelectVisible() {
	if len(m.msgs) == 0 {
		return
	}
	total := len(m.msgs)
	fromBottom := total - 1 - m.replySelectIdx
	if fromBottom < 0 {
		fromBottom = 0
	}
	chatH := m.chatHeight()
	if chatH < 1 {
		chatH = 1
	}
	m.scrollOffset = clampInt(fromBottom-chatH/2, 0, total-1)
}

func (m *Model) maybeAutoComplete() {
	word, prefix := m.input.WordAtCursor()
	if prefix != "@" && prefix != "#" {
		return
	}
	var candidates []string
	switch prefix {
	case "@":
		allUsers := m.allKnownUsers()
		for _, u := range allUsers {
			if strings.HasPrefix(strings.ToLower(u), strings.ToLower(word)) {
				candidates = append(candidates, "@"+u)
			}
		}
	case "#":
		for _, ch := range m.channels {
			if strings.HasPrefix(strings.ToLower(ch), strings.ToLower(word)) {
				candidates = append(candidates, "#"+ch)
			}
		}
	}
	if len(candidates) > 0 {
		m.input.SetCompletions(candidates)
	}
}

func (m Model) allKnownUsers() []string {
	seen := make(map[string]struct{})
	var users []string
	for _, u := range m.onlineUsers() {
		if _, ok := seen[u]; !ok {
			seen[u] = struct{}{}
			users = append(users, u)
		}
	}
	for _, u := range m.users {
		if _, ok := seen[u]; !ok {
			seen[u] = struct{}{}
			users = append(users, u)
		}
	}
	return users
}

func fitToSize(content string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	clipped := lipgloss.NewStyle().
		MaxWidth(width).
		MaxHeight(height).
		Render(content)
	return lipgloss.Place(width, height, lipgloss.Left, lipgloss.Top, clipped)
}

func borderedStyleWidth(totalWidth int) int {
	if totalWidth <= 2 {
		return 1
	}
	return totalWidth - 2
}

func borderedStyleHeight(totalHeight int) int {
	if totalHeight <= 2 {
		return 0
	}
	return totalHeight - 2
}

func borderedContentHeight(totalHeight int) int {
	if totalHeight <= 2 {
		return 0
	}
	return totalHeight - 2
}

func panelContentWidth(totalWidth int) int {
	if totalWidth <= 4 {
		return 1
	}
	return totalWidth - 4
}

func renderBorderedBox(style lipgloss.Style, width, height int, content string) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	style = style.Width(borderedStyleWidth(width)).MaxWidth(width).MaxHeight(height)
	if h := borderedStyleHeight(height); h > 0 {
		style = style.Height(h)
	}
	return fitToSize(style.Render(content), width, height)
}

func (m Model) View() string {
	if m.width < 40 || m.height < 12 {
		return lipgloss.NewStyle().
			Width(m.width).Height(m.height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(themeAccent).
			Render(fmt.Sprintf("terminal too small (%dx%d)\nmin 80x24 recommended", m.width, m.height))
	}

	statusBar := fitToSize(m.renderStatusBar(), m.width, 1)
	contentHeight := m.height - 1

	chWidth := m.channelSidebarWidth()
	rightWidth := m.rightSidebarWidth()
	showChannels := m.channelsVisible && chWidth > 0
	showRight := rightWidth > 0

	chatW := m.width
	if showChannels {
		chatW -= chWidth
	}
	if showRight {
		chatW -= rightWidth
	}
	if chatW < 20 {
		chatW = 20
	}

	m.input.SetWidth(chatW)

	channelsPanel := m.renderChannels(chWidth, contentHeight)
	chatPanel := m.renderChatArea(chatW, contentHeight)
	rightPanel := m.renderRightSidebar(rightWidth, contentHeight)

	var content string
	if !showChannels && !showRight {
		content = chatPanel
	} else if !showChannels {
		content = lipgloss.JoinHorizontal(lipgloss.Top, chatPanel, rightPanel)
	} else if !showRight {
		content = lipgloss.JoinHorizontal(lipgloss.Top, channelsPanel, chatPanel)
	} else {
		content = lipgloss.JoinHorizontal(lipgloss.Top, channelsPanel, chatPanel, rightPanel)
	}

	content = fitToSize(content, m.width, contentHeight)
	return lipgloss.JoinVertical(lipgloss.Left, statusBar, content)
}

func (m Model) renderStatusBar() string {
	connStr := string(m.status)
	switch m.status {
	case wsclient.StatusConnected:
		connStr = "connected"
	case wsclient.StatusDisconnected:
		connStr = "disconnected"
	case wsclient.StatusReconnecting:
		connStr = "reconnecting"
	}

	onlineCount := len(m.onlineUsers())
	left := fmt.Sprintf(" %s  #%s  %d online", connStr, m.channel, onlineCount)

	var right string
	if m.width >= 130 {
		right = shortcutScrollStyle().Render(" ↑↓/PgUp scroll") + "  " +
			shortcutFileStyle().Render("/file attach") + "  " +
			shortcutSearchStyle().Render("/search") + "  " +
			shortcutReplyStyle().Render("ctrl+r reply") + "  " +
			shortcutSelectStyle().Render("ctrl+g select") + "  " +
			shortcutMentionStyle().Render("ctrl+] mentions") + "  " +
			shortcutChStyle().Render("ctrl+b ch") + "  " +
			shortcutLatestStyle().Render("ctrl+l latest") + "  " +
			shortcutQuitStyle().Render("ctrl+c quit") + " "
	} else if m.width >= 110 {
		right = shortcutScrollStyle().Render(" ↑↓ scroll") + "  " +
			shortcutFileStyle().Render("/file") + "  " +
			shortcutReplyStyle().Render("ctrl+r reply") + "  " +
			shortcutSelectStyle().Render("ctrl+g select") + "  " +
			shortcutMentionStyle().Render("ctrl+] mentions") + "  " +
			shortcutChStyle().Render("ctrl+b ch") + "  " +
			shortcutLatestStyle().Render("ctrl+l latest") + "  " +
			shortcutQuitStyle().Render("ctrl+c quit") + " "
	} else if m.width >= 80 {
		right = shortcutReplyStyle().Render("ctrl+r reply") + "  " +
			shortcutSelectStyle().Render("ctrl+g select") + "  " +
			shortcutMentionStyle().Render("ctrl+] mentions") + "  " +
			shortcutChStyle().Render("ctrl+b ch") + "  " +
			shortcutQuitStyle().Render("ctrl+c quit") + " "
	} else {
		right = shortcutQuitStyle().Render("ctrl+c quit") + " "
	}

	if !m.lastSendOk && m.sendErr != "" {
		left = statusErrorStyle().Render(fmt.Sprintf(" ⚠ %s", m.sendErr))
	}

	unreadPart := ""
	if m.unreadCount > 0 {
		unreadPart = fmt.Sprintf(" | ↑ %d new", m.unreadCount)
	}

	leftStr := left + unreadPart
	leftW := lipgloss.Width(leftStr)
	rightW := lipgloss.Width(right)
	gap := m.width - 2 - leftW - rightW
	if gap < 0 {
		gap = 0
	}

	bar := leftStr + strings.Repeat(" ", gap) + right
	return statusBarStyle().Width(m.width).MaxWidth(m.width).MaxHeight(1).Render(bar)
}

func (m Model) renderChannels(width, height int) string {
	if width < 8 {
		return ""
	}

	title := panelTitleStyle().Render(fmt.Sprintf(" Channels %d ", len(m.channels)))
	var items []string
	for _, ch := range m.channels {
		active := ch == m.channel
		prefix := "  "
		if active {
			prefix = "* "
		}
		items = append(items, channelStyle(active).Render(prefix+"#"+ch))
	}

	content := strings.Join(items, "\n")
	boxHeight := height - 1
	if boxHeight < 1 {
		boxHeight = 1
	}
	content = clipLines(content, borderedContentHeight(boxHeight))
	box := renderBorderedBox(panelStyle(), width, boxHeight, content)
	return fitToSize(title+"\n"+box, width, height)
}

func (m Model) renderUsers(width, height int) string {
	if width < 8 {
		return ""
	}

	online := m.onlineUsers()
	label := "Online"
	title := panelTitleStyle().Render(fmt.Sprintf(" %s %d ", label, len(online)))
	innerW := panelContentWidth(width)
	var items []string
	for _, u := range online {
		colored := lipgloss.NewStyle().Foreground(usernameColor(u)).Render("@" + u)
		items = append(items, userStyle(false).Render(colored))
		if p, ok := m.presences[u]; ok && p.Status != "" {
			truncated := truncateStatus(p.Status, maxInt(1, innerW-2))
			items = append(items, presenceStatusStyle().Render("↳ "+truncated))
		}
	}

	content := strings.Join(items, "\n")
	boxHeight := height - 1
	if boxHeight < 1 {
		boxHeight = 1
	}
	content = clipLines(content, borderedContentHeight(boxHeight))
	box := renderBorderedBox(panelStyle(), width, boxHeight, content)
	return fitToSize(title+"\n"+box, width, height)
}

func (m Model) renderRightSidebar(width, height int) string {
	if width < 10 {
		return ""
	}
	usersHeight := 0
	if m.usersVisible {
		usersHeight = clampInt(len(m.onlineUsers())+4, 7, height/3)
		if usersHeight < 7 {
			usersHeight = 7
		}
	}
	notificationsHeight := height - usersHeight
	if notificationsHeight < 8 {
		notificationsHeight = 8
		if usersHeight > 0 {
			usersHeight = height - notificationsHeight
		}
	}

	if usersHeight <= 0 {
		return m.renderNotifications(width, height)
	}
	usersPanel := m.renderUsers(width, usersHeight)
	notificationsPanel := m.renderNotifications(width, notificationsHeight)
	return fitToSize(lipgloss.JoinVertical(lipgloss.Left, usersPanel, notificationsPanel), width, height)
}

func (m Model) renderNotifications(width, height int) string {
	if width < 10 {
		return ""
	}

	title := panelTitleStyle().Render(fmt.Sprintf(" Mentions %d ", len(m.notifications)))
	var items []string

	if len(m.notifications) == 0 {
		items = append(items, lipgloss.NewStyle().Foreground(themeDim).Italic(true).PaddingLeft(2).Render("no mentions yet"))
	} else {
		maxPreview := width - 6
		if maxPreview < 6 {
			maxPreview = 6
		}
		for i, n := range m.notifications {
			selected := i == m.notifIdx
			ts := n.Timestamp.Local().Format("15:04")
			preview := strings.ReplaceAll(n.Content, "\n", " ")
			if len([]rune(preview)) > maxPreview {
				preview = string([]rune(preview)[:maxPreview]) + "…"
			}
			line := fmt.Sprintf("%s #%s\n  @%s: %s", ts, n.Channel, n.Username, preview)
			items = append(items, notifItemStyle(selected).Width(panelContentWidth(width)).Render(line))
		}
	}

	content := strings.Join(items, "\n")
	boxHeight := height - 1
	if boxHeight < 1 {
		boxHeight = 1
	}
	content = clipLines(content, borderedContentHeight(boxHeight))
	box := renderBorderedBox(panelStyle(), width, boxHeight, content)
	return fitToSize(title+"\n"+box, width, height)
}

func (m Model) renderChatArea(width, height int) string {
	titleText := fmt.Sprintf(" #%s ", m.channel)
	if m.loadingHistory {
		titleText = fmt.Sprintf(" #%s %s", m.channel, loadingStyle().Render("(loading...)"))
	}
	title := panelTitleStyle().Render(titleText)

	typingLine := m.renderTypingIndicator()
	typingHeight := 0
	if typingLine != "" {
		typingHeight = 1
	}

	commandSuggestions := m.renderCommandSuggestions(width)
	suggestionsHeight := 0
	if commandSuggestions != "" {
		suggestionsHeight = lipgloss.Height(commandSuggestions)
	}

	// Reply bar above input
	replyBar := ""
	replyBarHeight := 0
	if m.replySelectMode {
		if m.replySelectIdx >= 0 && m.replySelectIdx < len(m.msgs) {
			sm := m.msgs[m.replySelectIdx]
			snippet := strings.ReplaceAll(sm.Content, "\n", " ")
			if len([]rune(snippet)) > width-30 {
				snippet = string([]rune(snippet)[:width-30]) + "…"
			}
			replyBar = fitToSize(replySelectPromptStyle().Width(width).MaxWidth(width).MaxHeight(1).Render(
				fmt.Sprintf("↩ reply to @%s: %s  [↑↓ move  enter confirm  esc cancel]", sm.Username, snippet),
			), width, 1)
		} else {
			replyBar = fitToSize(replySelectPromptStyle().Width(width).MaxWidth(width).MaxHeight(1).Render(
				"↩ select a message to reply to  [↑↓ move  enter confirm  esc cancel]",
			), width, 1)
		}
		replyBarHeight = 1
	} else if m.replyTo != nil {
		snippet := strings.ReplaceAll(m.replyTo.Content, "\n", " ")
		if len([]rune(snippet)) > width-20 {
			snippet = string([]rune(snippet)[:width-20]) + "…"
		}
		replyBar = fitToSize(replyBarStyle().Width(width).MaxWidth(width).MaxHeight(1).Render(
			fmt.Sprintf("↩ replying to @%s: %s  [Esc to cancel]", m.replyTo.Username, snippet),
		), width, 1)
		replyBarHeight = 1
	}

	// Mention autocomplete suggestions
	mentionSuggestions := ""
	mentionsHeight := 0
	if !m.replySelectMode {
		_, prefix := m.input.WordAtCursor()
		if (prefix == "@" || prefix == "#") && len(m.input.completions) > 0 {
			mentionSuggestions = m.renderMentionSuggestions(width)
			if mentionSuggestions != "" {
				mentionsHeight = lipgloss.Height(mentionSuggestions)
			}
		}
	}

	inputLines := m.input.LineCount()
	inputHeight := inputLines + 2
	if inputHeight > 8 {
		inputHeight = 8
	}

	chatBoxHeight := height - inputHeight - typingHeight - suggestionsHeight - replyBarHeight - mentionsHeight - 1
	if chatBoxHeight < 1 {
		chatBoxHeight = 1
	}
	chatH := borderedContentHeight(chatBoxHeight)
	if chatH < 1 {
		chatH = 1
	}
	chatW := panelContentWidth(width)

	vp := ViewportModel{
		width:       chatW,
		height:      chatH,
		offset:      m.scrollOffset,
		messages:    m.msgs,
		loading:     m.loadingHistory,
		allLoaded:   m.allHistoryLoaded,
		myUsername:  m.username,
		selectMode:  m.replySelectMode,
		selectedIdx: m.replySelectIdx,
	}
	chatContent := vp.View()

	chatBox := renderBorderedBox(panelStyle(), width, chatBoxHeight, chatContent)

	inputBox := m.input.ViewHeight(inputHeight)

	parts := []string{title, chatBox}
	if typingLine != "" {
		parts = append(parts, typingLine)
	}
	if commandSuggestions != "" {
		parts = append(parts, commandSuggestions)
	}
	if replyBar != "" {
		parts = append(parts, replyBar)
	}
	if mentionSuggestions != "" {
		parts = append(parts, mentionSuggestions)
	}
	parts = append(parts, inputBox)

	return fitToSize(strings.Join(parts, "\n"), width, height)
}

func (m Model) renderCommandSuggestions(width int) string {
	value := m.input.Value()
	if m.isJoinInput() {
		_, word := splitJoinInput(value)
		needle := strings.TrimPrefix(word, "#")
		var lines []string
		for _, ch := range m.channels {
			if strings.HasPrefix(strings.ToLower(ch), strings.ToLower(needle)) {
				lines = append(lines, fmt.Sprintf("#%s", ch))
			}
		}
		if len(lines) == 0 {
			lines = append(lines, "no matching channels")
		}
		return renderBorderedBox(commandSuggestionStyle(), width, lipgloss.Height(strings.Join(lines, "\n"))+2, "channels: "+strings.Join(lines, "  "))
	}

	if !strings.HasPrefix(value, "/") || strings.Contains(value, " ") {
		return ""
	}
	prefix := strings.TrimPrefix(value, "/")
	candidates := m.commandCandidates(prefix)
	if len(candidates) == 0 {
		return ""
	}

	lines := make([]string, 0, len(candidates))
	for _, c := range candidates {
		lines = append(lines, fmt.Sprintf("/%s - %s", c.Name(), c.Description()))
	}
	content := strings.Join(lines, "\n")
	return renderBorderedBox(commandSuggestionStyle(), width, lipgloss.Height(content)+2, content)
}

func (m Model) renderMentionSuggestions(width int) string {
	completions := m.input.completions
	if len(completions) == 0 {
		return ""
	}
	_, compIdx := m.input.CurrentCompletion()
	displayW := width - 4
	if displayW < 20 {
		displayW = 20
	}
	var items []string
	for i, c := range completions {
		highlighted := i == compIdx
		items = append(items, autoCompleteItemStyle(highlighted).Render(c))
	}
	line := strings.Join(items, " ")
	if lipgloss.Width(line) > displayW {
		line = line[:displayW] + "…"
	}
	return autoCompleteStyle().Width(width).MaxWidth(width).Render(line)
}

func (m Model) renderTypingIndicator() string {
	if len(m.typingUsers) == 0 {
		return ""
	}
	var names []string
	for u := range m.typingUsers {
		names = append(names, u)
	}
	sort.Strings(names)
	var text string
	switch len(names) {
	case 1:
		text = fmt.Sprintf(" %s is typing...", names[0])
	case 2:
		text = fmt.Sprintf(" %s and %s are typing...", names[0], names[1])
	default:
		text = fmt.Sprintf(" %s and others are typing...", names[0])
	}
	return typingStyle().Render(text)
}

func typingTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return typingTickMsg{}
	})
}

func (m Model) listenMessages() tea.Cmd {
	if m.client == nil {
		return nil
	}
	ch := m.client.Messages()
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return wsMsg(msg)
	}
}

func (m Model) listenStatus() tea.Cmd {
	if m.client == nil {
		return nil
	}
	ch := m.client.StatusChanges()
	return func() tea.Msg {
		sc, ok := <-ch
		if !ok {
			return nil
		}
		return wsStatusMsg{Status: sc.Status, Err: sc.Err}
	}
}

func (m Model) listenTyping() tea.Cmd {
	if m.client == nil {
		return nil
	}
	ch := m.client.TypingEvents()
	return func() tea.Msg {
		te, ok := <-ch
		if !ok {
			return nil
		}
		return typingEventMsg(te)
	}
}

func (m Model) listenPresence() tea.Cmd {
	if m.client == nil {
		return nil
	}
	ch := m.client.Presences()
	return func() tea.Msg {
		p, ok := <-ch
		if !ok {
			return nil
		}
		return presenceMsg(p)
	}
}

func (m Model) listenTerminalUsers() tea.Cmd {
	if m.client == nil {
		return nil
	}
	ch := m.client.TerminalUsers()
	return func() tea.Msg {
		users, ok := <-ch
		if !ok {
			return nil
		}
		return terminalUsersMsg(users)
	}
}

func (m Model) loadLocalHistory(channel string, limit int) tea.Cmd {
	if m.store == nil {
		return nil
	}
	return func() tea.Msg {
		messages, err := m.store.GetMessages(channel, limit, nil)
		if err != nil {
			return localHistoryMsg{Err: err}
		}
		channels, err := m.store.GetChannels()
		if err != nil {
			return localHistoryMsg{Err: err}
		}
		return localHistoryMsg{
			Messages: reverseMessages(messages),
			Channels: channels,
			Channel:  channel,
		}
	}
}

func (m Model) subscribeCmd(channel string) tea.Cmd {
	return func() tea.Msg {
		if m.client != nil {
			_ = m.client.Subscribe(channel)
		}
		return nil
	}
}

func (m Model) subscribeSwitchCmd(oldChannel, newChannel string) tea.Cmd {
	return func() tea.Msg {
		if m.client != nil {
			if oldChannel != "" {
				_ = m.client.Unsubscribe(oldChannel)
			}
			_ = m.client.Subscribe(newChannel)
		}
		return nil
	}
}

func (m Model) sendWithEcho(content string) (tea.Model, tea.Cmd) {
	content = normalizeSingleLineCodeFence(content)
	key := contentHash(m.username, m.channel, content)
	m.sentHashes[key] = time.Now()

	replyToID := ""
	replyToContent := ""
	replyToAuthor := ""
	if m.replyTo != nil {
		replyToID = m.replyTo.ID
		replyToContent = m.replyTo.Content
		replyToAuthor = m.replyTo.Username
		m.replyTo = nil
	}

	echo := model.Message{
		ID:             fmt.Sprintf("echo-%d", time.Now().UnixNano()),
		Username:       m.username,
		Content:        content,
		Channel:        m.channel,
		Timestamp:      time.Now(),
		ReplyToID:      replyToID,
		ReplyToContent: replyToContent,
		ReplyToAuthor:  replyToAuthor,
	}
	m.msgs = insertSorted(m.msgs, echo)
	m.users = msgsToUsers(m.msgs)
	m.scrollOffset = 0
	m.unreadCount = 0

	return m, tea.Batch(m.SendMessage(content, m.channel, replyToID), m.persistMessage(echo))
}

func normalizeSingleLineCodeFence(content string) string {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "```") || !strings.HasSuffix(trimmed, "```") || strings.Contains(trimmed, "\n") || len(trimmed) <= 6 {
		return content
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(trimmed, "```"), "```")
	parts := strings.SplitN(strings.TrimSpace(inner), " ", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return content
	}
	return fmt.Sprintf("```%s\n%s\n```", parts[0], parts[1])
}

func (m Model) sendFileWithEcho(path, content string) (tea.Model, tea.Cmd) {
	echo := model.Message{
		ID:        fmt.Sprintf("file-echo-%d", time.Now().UnixNano()),
		Username:  m.username,
		Content:   content,
		Channel:   m.channel,
		Timestamp: time.Now(),
	}

	// Add local attachment for inline rendering
	if path != "" {
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() {
			base := filepath.Base(path)
			ext := strings.ToLower(filepath.Ext(path))
			contentType := MimeTypeFromExt(ext)
			echo.Attachments = []model.Attachment{{
				URL:         path,
				Filename:    base,
				ContentType: contentType,
				Size:        int(info.Size()),
			}}
		}
	}

	m.msgs = insertSorted(m.msgs, echo)
	m.users = msgsToUsers(m.msgs)
	m.scrollOffset = 0
	m.unreadCount = 0

	cmds := []tea.Cmd{m.SendFile(path, m.channel, content), m.persistMessage(echo)}
	return m, tea.Batch(cmds...)
}

func (m Model) openImage(index int) (tea.Model, tea.Cmd) {
	var images []model.Attachment
	for i := len(m.msgs) - 1; i >= 0; i-- {
		for _, att := range m.msgs[i].Attachments {
			if isImageAttachment(att) {
				images = append(images, att)
			}
		}
	}
	if len(images) == 0 {
		sysMsg := commands.SystemMsg("no images found in this channel")
		m.msgs = append(m.msgs, sysMsg)
		m.scrollOffset = 0
		return m, nil
	}
	if index > len(images) {
		sysMsg := commands.SystemMsg(fmt.Sprintf("only %d image(s) available", len(images)))
		m.msgs = append(m.msgs, sysMsg)
		m.scrollOffset = 0
		return m, nil
	}
	img := images[index-1]
	url := img.ProxyURL
	if url == "" {
		url = img.URL
	}
	if url == "" {
		sysMsg := commands.SystemMsg("image has no URL")
		m.msgs = append(m.msgs, sysMsg)
		m.scrollOffset = 0
		return m, nil
	}
	go func() {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", url)
		case "windows":
			cmd = exec.Command("cmd", "/c", "start", url)
		default:
			cmd = exec.Command("xdg-open", url)
		}
		_ = cmd.Run()
	}()
	sysMsg := commands.SystemMsg(fmt.Sprintf("opening %s", img.Filename))
	m.msgs = append(m.msgs, sysMsg)
	m.scrollOffset = 0
	return m, nil
}

func (m Model) SendMessage(content, channel, replyToID string) tea.Cmd {
	if m.sender == nil {
		return nil
	}
	return m.sender.SendAsync(content, channel, replyToID)
}

func (m Model) SendFile(path, channel, content string) tea.Cmd {
	if m.sender == nil {
		return nil
	}
	return m.sender.SendFileAsync(path, channel, content)
}

func (m Model) persistMessage(msg model.Message) tea.Cmd {
	if m.store == nil {
		return nil
	}
	return func() tea.Msg {
		err := m.store.InsertMessage(msg)
		return dbWriteResultMsg{Err: err}
	}
}

func (m Model) persistNotification(n model.Notification) tea.Cmd {
	if m.store == nil {
		return nil
	}
	return func() tea.Msg {
		err := m.store.InsertNotification(n)
		return dbWriteResultMsg{Err: err}
	}
}

func (m *Model) jumpToMessage(id string) bool {
	if id == "" {
		return false
	}
	for i, msg := range m.msgs {
		if msg.ID == id {
			fromBottom := len(m.msgs) - 1 - i
			if fromBottom < 0 {
				fromBottom = 0
			}
			m.scrollOffset = fromBottom
			m.unreadCount = 0
			return true
		}
	}
	return false
}

func (m *Model) addChannel(channel string) {
	channel = strings.TrimPrefix(strings.TrimSpace(channel), "#")
	if channel == "" {
		return
	}
	for _, ch := range m.channels {
		if ch == channel {
			return
		}
	}
	m.channels = append(m.channels, channel)
	sort.Strings(m.channels)
	if m.store != nil {
		_ = m.store.InsertChannel(channel)
	}
	if m.channelsOK {
		if m.available == nil {
			m.available = make(map[string]struct{})
		}
		m.available[channel] = struct{}{}
	}
}

func (m *Model) removeChannel(channel string) {
	var newChannels []string
	for _, ch := range m.channels {
		if ch != channel {
			newChannels = append(newChannels, ch)
		}
	}
	m.channels = newChannels
	delete(m.available, channel)
}

func (m Model) validateJoin(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: /join #channel")
	}
	channel := strings.TrimPrefix(strings.TrimSpace(args[0]), "#")
	if channel == "" {
		return fmt.Errorf("invalid channel name")
	}
	// Allow joining any channel; if relay doesn't know it, send will fail gracefully
	return nil
}

func (m Model) isJoinInput() bool {
	isJoin, _ := splitJoinInput(m.input.Value())
	return isJoin
}

func (m Model) isOpenInput() bool {
	val := strings.TrimSpace(m.input.Value())
	return strings.HasPrefix(val, "/open")
}

func (m Model) commandCandidates(prefix string) []commands.Command {
	if m.registry == nil {
		return nil
	}
	prefix = strings.ToLower(prefix)
	var candidates []commands.Command
	for _, c := range m.registry.List() {
		if strings.HasPrefix(strings.ToLower(c.Name()), prefix) {
			candidates = append(candidates, c)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Name() < candidates[j].Name()
	})
	return candidates
}

func (m Model) chatWidth() int {
	w := m.width
	if m.channelsVisible {
		w -= m.channelSidebarWidth()
	}
	if rightWidth := m.rightSidebarWidth(); rightWidth > 0 {
		w -= rightWidth
	}
	if w < 20 {
		w = 20
	}
	return w
}

func (m Model) chatHeight() int {
	h := m.height - 1
	inputH := m.input.LineCount() + 2
	if inputH > 8 {
		inputH = 8
	}
	chatBoxH := h - inputH - 1
	chatH := borderedContentHeight(chatBoxH)
	if chatH < 1 {
		chatH = 1
	}
	return chatH
}

func (m Model) channelSidebarWidth() int {
	if !m.channelsVisible {
		return 0
	}
	if m.width < 72 {
		return 0
	}
	w := m.width * 18 / 100
	w = clampInt(w, 16, 28)
	return w
}

func (m Model) userSidebarWidth() int {
	if !m.usersVisible {
		return 0
	}
	if m.width < 96 {
		return 0
	}
	w := m.width * 20 / 100
	w = clampInt(w, 22, 34)
	return w
}

func (m Model) notifSidebarWidth() int {
	if !m.notifVisible {
		return 0
	}
	if m.width < 96 {
		return 0
	}
	w := m.width * 28 / 100
	w = clampInt(w, 28, 44)
	return w
}

func (m Model) rightSidebarWidth() int {
	if !m.notifVisible || m.width < 96 {
		return 0
	}
	w := m.width * 20 / 100
	return clampInt(w, 28, 38)
}

func (m *Model) nextChannel() {
	if len(m.channels) == 0 {
		return
	}
	idx := 0
	for i, ch := range m.channels {
		if ch == m.channel {
			idx = i
			break
		}
	}
	m.channel = m.channels[(idx+1)%len(m.channels)]
}

func (m *Model) prevChannel() {
	if len(m.channels) == 0 {
		return
	}
	idx := 0
	for i, ch := range m.channels {
		if ch == m.channel {
			idx = i
			break
		}
	}
	m.channel = m.channels[(idx-1+len(m.channels))%len(m.channels)]
}

func insertSorted(msgs []model.Message, m model.Message) []model.Message {
	for i, existing := range msgs {
		if existing.ID == m.ID {
			return msgs
		}
		if m.Timestamp.Before(existing.Timestamp) {
			msgs = append(msgs[:i], append([]model.Message{m}, msgs[i:]...)...)
			return msgs
		}
	}
	return append(msgs, m)
}

func mergeMessages(existing, incoming []model.Message) []model.Message {
	seen := make(map[string]struct{}, len(existing))
	for _, m := range existing {
		seen[m.ID] = struct{}{}
	}

	var newMsgs []model.Message
	for _, m := range incoming {
		if _, ok := seen[m.ID]; !ok {
			newMsgs = append(newMsgs, m)
			seen[m.ID] = struct{}{}
		}
	}

	all := append(existing, newMsgs...)
	sort.Slice(all, func(i, j int) bool {
		return all[i].Timestamp.Before(all[j].Timestamp)
	})
	return all
}

func mergeChannels(existing, incoming []string) []string {
	seen := make(map[string]struct{}, len(existing)+len(incoming))
	var merged []string
	for _, ch := range append(existing, incoming...) {
		ch = strings.TrimPrefix(strings.TrimSpace(ch), "#")
		if ch == "" {
			continue
		}
		if _, ok := seen[ch]; ok {
			continue
		}
		seen[ch] = struct{}{}
		merged = append(merged, ch)
	}
	sort.Strings(merged)
	return merged
}

func channelsToSet(channels []string) map[string]struct{} {
	set := make(map[string]struct{}, len(channels))
	for _, ch := range channels {
		ch = strings.TrimPrefix(strings.TrimSpace(ch), "#")
		if ch != "" {
			set[ch] = struct{}{}
		}
	}
	return set
}

func splitJoinInput(input string) (bool, string) {
	fields := strings.Fields(input)
	if len(fields) == 0 || fields[0] != "/join" {
		return false, ""
	}
	if len(fields) == 1 {
		if strings.HasSuffix(input, " ") {
			return true, ""
		}
		return false, ""
	}
	return true, fields[1]
}

func reverseMessages(msgs []model.Message) []model.Message {
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs
}

func contentHash(username, channel, content string) string {
	h := sha256.New()
	h.Write([]byte(username + "\x00" + channel + "\x00" + content))
	return hex.EncodeToString(h.Sum(nil))[:16]
}
