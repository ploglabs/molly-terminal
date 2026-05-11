package tui

import (
	"fmt"
	"log"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

type dbWriteResultMsg struct {
	Err error
}

type Model struct {
	width  int
	height int

	client  *wsclient.Client
	sender  *webhook.Sender
	store   *db.Store
	fetcher *history.Fetcher

	msgs       []model.Message
	status     wsclient.Status
	channel    string
	channels   []string
	users      []string
	lastSendOk bool
	sendErr    string

	loadingHistory  bool
	historyLoaded   bool
	allHistoryLoaded bool

	scrollOffset int
	input        InputModel
	log          *log.Logger

	channelsVisible bool
	usersVisible    bool
}

func New(client *wsclient.Client, sender *webhook.Sender, store *db.Store, fetcher *history.Fetcher, channel string) Model {
	return Model{
		client:          client,
		sender:          sender,
		store:           store,
		fetcher:         fetcher,
		channel:         channel,
		channels:        []string{channel},
		status:          wsclient.StatusDisconnected,
		lastSendOk:      true,
		loadingHistory:  false,
		historyLoaded:   false,
		allHistoryLoaded: false,
		input:           newInput("> "),
		channelsVisible: true,
		usersVisible:    true,
		log:             log.Default(),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.listenMessages(),
		m.listenStatus(),
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
		m.msgs = insertSorted(m.msgs, model.Message(msg))
		if len(m.msgs) > 1000 {
			m.msgs = m.msgs[len(m.msgs)-1000:]
		}
		m.users = msgsToUsers(m.msgs)
		m.scrollOffset = 0
		return m, tea.Batch(m.listenMessages(), m.persistMessage(model.Message(msg)))

	case wsStatusMsg:
		m.status = msg.Status
		if msg.Status == wsclient.StatusConnected && m.channel != "" {
			return m, tea.Batch(m.listenStatus(), m.subscribeCmd(m.channel))
		}
		return m, m.listenStatus()

	case history.FetchResultMsg:
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
	case "esc", "ctrl+q":
		return m, tea.Quit
	case "ctrl+b":
		m.channelsVisible = !m.channelsVisible
		return m, nil
	case "ctrl+u":
		m.usersVisible = !m.usersVisible
		return m, nil
	case "ctrl+l":
		m.clearScreen()
		return m, nil
	case "pgup":
		m.scrollOffset += m.chatHeight() / 2
		if m.scrollOffset > len(m.msgs)-1 {
			m.scrollOffset = len(m.msgs) - 1
		}
		if m.scrollOffset < 0 {
			m.scrollOffset = 0
		}
		if m.scrollOffset >= len(m.msgs)-m.chatHeight() && !m.loadingHistory && !m.allHistoryLoaded && m.historyLoaded && len(m.msgs) > 0 {
			oldest := m.msgs[0].Timestamp
			m.loadingHistory = true
			return m, history.LoadOlder(m.fetcher, m.channel, oldest)
		}
		return m, nil
	case "pgdown":
		m.scrollOffset -= m.chatHeight() / 2
		if m.scrollOffset < 0 {
			m.scrollOffset = 0
		}
		return m, nil
	case "up":
		m.scrollOffset++
		if m.scrollOffset > len(m.msgs)-1 {
			m.scrollOffset = len(m.msgs) - 1
		}
		if m.scrollOffset >= len(m.msgs)-m.chatHeight() && !m.loadingHistory && !m.allHistoryLoaded && m.historyLoaded && len(m.msgs) > 0 {
			oldest := m.msgs[0].Timestamp
			m.loadingHistory = true
			return m, history.LoadOlder(m.fetcher, m.channel, oldest)
		}
		return m, nil
	case "down":
		m.scrollOffset--
		if m.scrollOffset < 0 {
			m.scrollOffset = 0
		}
		return m, nil
	}

	submitted, value := handleInputKey(msg, &m.input)
	if submitted && value != "" {
		val := strings.TrimSpace(value)
		if val != "" {
			return m, m.SendMessage(val)
		}
	}

	return m, nil
}

func (m Model) View() string {
	if m.width < 40 || m.height < 12 {
		return lipgloss.NewStyle().
			Width(m.width).Height(m.height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(themeAccent).
			Render(fmt.Sprintf("terminal too small (%dx%d)\nmin 80x24 recommended", m.width, m.height))
	}

	var finalCmd []string

	statusBar := m.renderStatusBar()
	finalCmd = append(finalCmd, statusBar)

	contentHeight := m.height - 1

	chWidth := m.channelSidebarWidth()
	usersWidth := m.userSidebarWidth()
	chatW := m.width

	if m.channelsVisible {
		chatW -= chWidth
	}
	if m.usersVisible {
		chatW -= usersWidth
	}
	if chatW < 20 {
		chatW = 20
	}

	m.input.SetWidth(chatW - 6)

	channelsPanel := m.renderChannels(chWidth, contentHeight)
	usersPanel := m.renderUsers(usersWidth, contentHeight)
	chatPanel := m.renderChatArea(chatW, contentHeight)

	if !m.channelsVisible && !m.usersVisible {
		finalCmd = append(finalCmd, chatPanel)
	} else if !m.channelsVisible {
		finalCmd = append(finalCmd, lipgloss.JoinHorizontal(lipgloss.Top, chatPanel, usersPanel))
	} else if !m.usersVisible {
		finalCmd = append(finalCmd, lipgloss.JoinHorizontal(lipgloss.Top, channelsPanel, chatPanel))
	} else {
		finalCmd = append(finalCmd, lipgloss.JoinHorizontal(lipgloss.Top, channelsPanel, chatPanel, usersPanel))
	}

	return lipgloss.JoinVertical(lipgloss.Left, finalCmd...)
}

func (m Model) renderStatusBar() string {
	left := fmt.Sprintf(" [%s] %s", m.status, m.channel)
	center := fmt.Sprintf("%d users", len(m.users))
	right := "ctrl+b:ch ctrl+u:users pgup/pgdn ↑/↓:scroll ctrl+q:quit"

	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	centerW := lipgloss.Width(center)

	leftPad := 0
	rightPad := 0

	if m.width-leftW-rightW-centerW > 4 {
		leftPad = (m.width - leftW - centerW - rightW - 4) / 2
		rightPad = m.width - leftW - leftPad - centerW - rightW - 4
	}

	style := statusBarStyle()
	if !m.lastSendOk {
		left = statusErrorStyle().Render(fmt.Sprintf(" ⚠ %s", m.sendErr))
		leftW = lipgloss.Width(left)
	}

	return style.Width(m.width).Render(
		left +
			strings.Repeat(" ", leftPad) +
			center +
			strings.Repeat(" ", rightPad) +
			right,
	)
}

func (m Model) renderChannels(width, height int) string {
	if width < 8 {
		return ""
	}

	title := panelTitleStyle().Render(" Channels ")
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

	title := panelTitleStyle().Render(" Users ")
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
	box := panelStyle().Width(width - 1).Height(innerH).Render(content)
	return title + "\n" + box
}

func (m Model) renderChatArea(width, height int) string {
	title := panelTitleStyle().Render(fmt.Sprintf(" #%s ", m.channel))
	if m.loadingHistory {
		title = panelTitleStyle().Render(fmt.Sprintf(" #%s %s", m.channel, loadingStyle().Render("(loading...)")))
	}

	inputHeight := 1
	chatH := height - inputHeight - 2
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

	return title + "\n" + chatBox + "\n" + inputBox
}

func (m *Model) clearScreen() {
	m.msgs = nil
	m.scrollOffset = 0
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

func (m Model) subscribeCmd(channel string) tea.Cmd {
	return func() tea.Msg {
		if m.client != nil {
			_ = m.client.Subscribe(channel)
		}
		return nil
	}
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
	inputH := 1
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
	w := m.width * 15 / 100
	w = clampInt(w, 12, 24)
	return w
}

func (m Model) userSidebarWidth() int {
	if !m.usersVisible {
		return 0
	}
	w := m.width * 12 / 100
	w = clampInt(w, 10, 18)
	return w
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
