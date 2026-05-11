package tui

import (
	"fmt"
	"log"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ploglabs/molly-terminal/internal/db"
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
	width      int
	height     int
	client     *wsclient.Client
	sender     *webhook.Sender
	store      *db.Store
	msgs       []model.Message
	status     wsclient.Status
	channel    string
	lastSendOk bool
	sendErr    string
	log        *log.Logger
}

func New(client *wsclient.Client, sender *webhook.Sender, store *db.Store, channel string) Model {
	return Model{
		client:     client,
		sender:     sender,
		store:      store,
		channel:    channel,
		status:     wsclient.StatusDisconnected,
		lastSendOk: true,
		log:        log.Default(),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.listenMessages(), m.listenStatus())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case wsMsg:
		m.msgs = append(m.msgs, model.Message(msg))
		if len(m.msgs) > 200 {
			m.msgs = m.msgs[len(m.msgs)-200:]
		}
		return m, tea.Batch(m.listenMessages(), m.persistMessage(model.Message(msg)))
	case wsStatusMsg:
		m.status = msg.Status
		if msg.Status == wsclient.StatusConnected && m.channel != "" {
			return m, tea.Batch(m.listenStatus(), m.subscribeCmd(m.channel))
		}
		return m, m.listenStatus()
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
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			if m.client != nil {
				_ = m.client.Close()
			}
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) View() string {
	statusLine := fmt.Sprintf("[%s] channel: %s", m.status, m.channel)
	if !m.lastSendOk && m.sendErr != "" {
		statusLine += fmt.Sprintf("\n⚠ send error: %s", m.sendErr)
	}
	if m.lastSendOk && len(m.msgs) > 0 {
		last := m.msgs[len(m.msgs)-1]
		statusLine += fmt.Sprintf("\n%s: %s", last.Username, last.Content)
	}
	return statusLine + "\n\nmolly-terminal — press q or ctrl+c to exit"
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
