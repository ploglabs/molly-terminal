package tui

import (
	"fmt"
	"log"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
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
	width          int
	height         int
	scrollOffset   int
	client         *wsclient.Client
	sender         *webhook.Sender
	store          *db.Store
	fetcher        *history.Fetcher
	msgs           []model.Message
	status         wsclient.Status
	channel        string
	lastSendOk     bool
	sendErr        string
	loadingHistory bool
	historyLoaded  bool
	allHistoryLoaded bool
	log            *log.Logger
}

func New(client *wsclient.Client, sender *webhook.Sender, store *db.Store, fetcher *history.Fetcher, channel string) Model {
	return Model{
		client:           client,
		sender:           sender,
		store:            store,
		fetcher:          fetcher,
		channel:          channel,
		status:           wsclient.StatusDisconnected,
		lastSendOk:       true,
		loadingHistory:   false,
		historyLoaded:    false,
		allHistoryLoaded: false,
		log:              log.Default(),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.listenMessages(), m.listenStatus(), history.InitialFetch(m.fetcher, m.channel, 100))
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case wsMsg:
		m.msgs = insertSorted(m.msgs, model.Message(msg))
		if len(m.msgs) > 1000 {
			m.msgs = m.msgs[len(m.msgs)-1000:]
		}
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
		var cmds []tea.Cmd
		for _, msg := range msg.Messages {
			cmds = append(cmds, m.persistMessage(msg))
		}
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
		switch msg.String() {
		case "ctrl+c", "q":
			if m.client != nil {
				_ = m.client.Close()
			}
			return m, tea.Quit
		case "up", "k":
			if m.scrollOffset < len(m.msgs)-1 {
				m.scrollOffset++
				if m.scrollOffset >= len(m.msgs)-5 && !m.loadingHistory && !m.allHistoryLoaded && m.historyLoaded {
					oldest := m.msgs[0].Timestamp
					m.loadingHistory = true
					return m, history.LoadOlder(m.fetcher, m.channel, oldest)
				}
			}
		case "down", "j":
			if m.scrollOffset > 0 {
				m.scrollOffset--
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	statusLine := fmt.Sprintf("[%s] channel: %s", m.status, m.channel)
	if m.loadingHistory {
		statusLine += " (loading history...)"
	}
	if !m.lastSendOk && m.sendErr != "" {
		statusLine += fmt.Sprintf("\n⚠ send error: %s", m.sendErr)
	}

	if len(m.msgs) > 0 {
		visibleCount := min(m.height-4, len(m.msgs))
		start := len(m.msgs) - visibleCount - m.scrollOffset
		if start < 0 {
			start = 0
		}
		end := start + visibleCount
		if end > len(m.msgs) {
			end = len(m.msgs)
		}
		if m.loadingHistory && start == 0 {
			statusLine += "\n  ▲ loading older messages..."
		}
		for _, msg := range m.msgs[start:end] {
			statusLine += fmt.Sprintf("\n%s: %s", msg.Username, msg.Content)
		}
	}

	return statusLine + fmt.Sprintf("\n\nmolly-terminal — ↑/k scroll up, ↓/j scroll down, q exit")
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
