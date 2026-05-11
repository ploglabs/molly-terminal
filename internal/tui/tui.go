package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ploglabs/molly-terminal/internal/model"
	"github.com/ploglabs/molly-terminal/internal/wsclient"
)

type wsMsg model.Message

type wsStatusMsg struct {
	Status wsclient.Status
	Err    error
}

type Model struct {
	width   int
	height  int
	client  *wsclient.Client
	msgs    []model.Message
	status  wsclient.Status
	channel string
}

func New(client *wsclient.Client, channel string) Model {
	return Model{
		client:  client,
		channel: channel,
		status:  wsclient.StatusDisconnected,
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
		return m, m.listenMessages()
	case wsStatusMsg:
		m.status = msg.Status
		if msg.Status == wsclient.StatusConnected && m.channel != "" {
			return m, tea.Batch(m.listenStatus(), m.subscribeCmd(m.channel))
		}
		return m, m.listenStatus()
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
	if len(m.msgs) > 0 {
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