package tui

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
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

	typingUsers map[string]time.Time
	sentHashes  map[string]time.Time
}

func New(client *wsclient.Client, sender *webhook.Sender, store *db.Store, fetcher *history.Fetcher, registry *commands.Registry, channel, username string) Model {
	channels := []string{channel}
	if store != nil {
		if storedChannels, err := store.GetChannels(); err == nil {
			channels = mergeChannels(channels, storedChannels)
		}
		_ = store.InsertChannel(channel)
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
		typingUsers:      make(map[string]time.Time),
		sentHashes:       make(map[string]time.Time),
		log:              log.Default(),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.listenMessages(),
		m.listenStatus(),
		m.listenTyping(),
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
		m.input.SetWidth(m.chatWidth() - 4)
		return m, nil

	case wsMsg:
		m.addChannel(msg.Channel)
		if msg.Channel != m.channel {
			return m, tea.Batch(m.listenMessages(), m.persistMessage(model.Message(msg)))
		}
		key := contentHash(msg.Username, msg.Channel, msg.Content)
		if _, exists := m.sentHashes[key]; exists {
			delete(m.sentHashes, key)
			return m, nil
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
		return m, tea.Batch(m.listenMessages(), m.persistMessage(model.Message(msg)))

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
		if len(m.typingUsers) > 0 {
			return m, typingTickCmd()
		}
		return m, nil

	case history.ChannelsResultMsg:
		if msg.Err != nil {
			m.log.Printf("channels: %v", msg.Err)
			return m, nil
		}
		if len(msg.Channels) == 0 {
			return m, nil
		}
		m.channels = mergeChannels([]string{m.channel}, msg.Channels)
		m.available = channelsToSet(m.channels)
		m.channelsOK = true
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
		// Cancel / unfocus: clear input or clear completions
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
		return m, m.subscribeCmd(m.channel)

	case "ctrl+n":
		m.nextChannel()
		return m, m.subscribeCmd(m.channel)

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
		} else {
			switch prefix {
			case "/":
				for _, c := range m.commandCandidates(word) {
					candidates = append(candidates, "/"+c.Name())
				}
			case "@":
				for _, u := range m.users {
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
				// Complete from users if word is non-empty
				if word != "" {
					for _, u := range m.users {
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

	case "up":
		m.scrollOffset++
		if m.scrollOffset > len(m.msgs)-1 {
			m.scrollOffset = len(m.msgs) - 1
		}
		return m, m.loadOlderIfNeeded()

	case "down":
		m.scrollOffset--
		if m.scrollOffset < 0 {
			m.scrollOffset = 0
			m.unreadCount = 0
		}
		return m, nil
	}

	submitted, value := handleInputKey(msg, &m.input)
	if submitted {
		val := strings.TrimSpace(value)
		if val == "" {
			return m, nil
		}

		if strings.HasPrefix(val, "/") {
			cmdName, args := commands.ParseInput(val)
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

func (m Model) View() string {
	if m.width < 40 || m.height < 12 {
		return lipgloss.NewStyle().
			Width(m.width).Height(m.height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(themeAccent).
			Render(fmt.Sprintf("terminal too small (%dx%d)\nmin 80x24 recommended", m.width, m.height))
	}

	statusBar := m.renderStatusBar()
	contentHeight := m.height - 1

	chWidth := m.channelSidebarWidth()
	usersWidth := m.userSidebarWidth()
	showChannels := m.channelsVisible && chWidth > 0
	showUsers := m.usersVisible && usersWidth > 0
	chatW := m.width
	if showChannels {
		chatW -= chWidth
	}
	if showUsers {
		chatW -= usersWidth
	}
	if chatW < 20 {
		chatW = 20
	}

	m.input.SetWidth(chatW - 6)

	channelsPanel := m.renderChannels(chWidth, contentHeight)
	usersPanel := m.renderUsers(usersWidth, contentHeight)
	chatPanel := m.renderChatArea(chatW, contentHeight)

	var content string
	if !showChannels && !showUsers {
		content = chatPanel
	} else if !showChannels {
		content = lipgloss.JoinHorizontal(lipgloss.Top, chatPanel, usersPanel)
	} else if !showUsers {
		content = lipgloss.JoinHorizontal(lipgloss.Top, channelsPanel, chatPanel)
	} else {
		content = lipgloss.JoinHorizontal(lipgloss.Top, channelsPanel, chatPanel, usersPanel)
	}

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

	left := fmt.Sprintf(" %s  #%s  %d users", connStr, m.channel, len(m.users))
	right := " ctrl+b channels  ctrl+y users  ctrl+l latest  ctrl+c quit "
	if m.width < 96 {
		right = " ctrl+b ch  ctrl+y users  ctrl+c quit "
	}
	if m.width < 72 {
		right = " ctrl+c quit "
	}

	if !m.lastSendOk && m.sendErr != "" {
		left = statusErrorStyle().Render(fmt.Sprintf(" ⚠ %s", m.sendErr))
	}

	unreadPart := ""
	if m.unreadCount > 0 {
		unreadPart = fmt.Sprintf(" | ↑ %d new", m.unreadCount)
	}

	leftW := lipgloss.Width(left) + lipgloss.Width(unreadPart)
	rightW := lipgloss.Width(right)
	gap := m.width - leftW - rightW
	if gap < 0 {
		gap = 0
	}

	bar := left + unreadPart + strings.Repeat(" ", gap) + right
	return statusBarStyle().Width(m.width).Render(bar)
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
	innerH := height - 2
	if innerH < 1 {
		innerH = 1
	}
	box := panelStyle().Width(width - 1).Height(innerH).Render(content)
	return title + "\n" + box
}

func (m Model) renderUsers(width, height int) string {
	if width < 8 {
		return ""
	}

	title := panelTitleStyle().Render(fmt.Sprintf(" Users %d ", len(m.users)))
	var items []string
	for _, u := range m.users {
		colored := lipgloss.NewStyle().Foreground(usernameColor(u)).Render("@" + u)
		items = append(items, userStyle(false).Render(colored))
	}

	content := strings.Join(items, "\n")
	innerH := height - 2
	if innerH < 1 {
		innerH = 1
	}
	box := panelStyle().Width(width).Height(innerH).Render(content)
	return title + "\n" + box
}

func (m Model) renderChatArea(width, height int) string {
	titleText := fmt.Sprintf(" #%s ", m.channel)
	if m.loadingHistory {
		titleText = fmt.Sprintf(" #%s %s", m.channel, loadingStyle().Render("(loading...)"))
	}
	title := panelTitleStyle().Render(titleText)

	// typing indicator is 1 line if active, 0 otherwise
	typingLine := m.renderTypingIndicator()
	typingHeight := 0
	if typingLine != "" {
		typingHeight = 1
	}

	commandSuggestions := m.renderCommandSuggestions(width - 4)
	suggestionsHeight := 0
	if commandSuggestions != "" {
		suggestionsHeight = lipgloss.Height(commandSuggestions)
	}

	inputHeight := 3 // border + 1 line
	chatH := height - inputHeight - typingHeight - suggestionsHeight - 2
	if chatH < 1 {
		chatH = 1
	}

	vp := ViewportModel{
		width:     width - 4,
		height:    chatH,
		offset:    m.scrollOffset,
		messages:  m.msgs,
		loading:   m.loadingHistory,
		allLoaded: m.allHistoryLoaded,
	}
	chatContent := vp.View()

	chatBox := panelStyle().
		Width(width - 1).
		Height(chatH).
		Render(chatContent)

	inputBox := m.input.View()

	parts := []string{title, chatBox}
	if typingLine != "" {
		parts = append(parts, typingLine)
	}
	if commandSuggestions != "" {
		parts = append(parts, commandSuggestions)
	}
	parts = append(parts, inputBox)

	return strings.Join(parts, "\n")
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
		return commandSuggestionStyle().Width(width).Render("channels: " + strings.Join(lines, "  "))
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
	return commandSuggestionStyle().Width(width).Render(strings.Join(lines, "\n"))
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
	key := contentHash(m.username, m.channel, content)
	m.sentHashes[key] = time.Now()

	echo := model.Message{
		ID:        fmt.Sprintf("echo-%d", time.Now().UnixNano()),
		Username:  m.username,
		Content:   content,
		Channel:   m.channel,
		Timestamp: time.Now(),
	}
	m.msgs = insertSorted(m.msgs, echo)
	m.users = msgsToUsers(m.msgs)
	m.scrollOffset = 0
	m.unreadCount = 0

	return m, tea.Batch(m.SendMessage(content), m.persistMessage(echo))
}

func (m Model) SendMessage(content string) tea.Cmd {
	if m.sender == nil {
		return nil
	}
	return m.sender.SendAsync(content)
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
	if !m.channelsOK {
		return fmt.Errorf("channels are still loading; try /join again in a moment")
	}
	if _, ok := m.available[channel]; !ok {
		return fmt.Errorf("unknown channel: #%s", channel)
	}
	return nil
}

func (m Model) isJoinInput() bool {
	isJoin, _ := splitJoinInput(m.input.Value())
	return isJoin
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
	if m.usersVisible {
		w -= m.userSidebarWidth()
	}
	if w < 20 {
		w = 20
	}
	return w
}

func (m Model) chatHeight() int {
	h := m.height - 1
	inputH := 3
	chatH := h - inputH - 1
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
	w := m.width * 14 / 100
	w = clampInt(w, 14, 22)
	return w
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
