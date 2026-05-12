package commands

import (
	tea "github.com/charmbracelet/bubbletea"
)

type HistoryCmd struct{}

func NewHistoryCmd() *HistoryCmd {
	return &HistoryCmd{}
}

func (c *HistoryCmd) Name() string {
	return "history"
}

func (c *HistoryCmd) Description() string {
	return "Fetch and display older messages"
}

func (c *HistoryCmd) Execute(args []string) (tea.Cmd, error) {
	return func() tea.Msg {
		return TriggerHistoryLoadMsg{}
	}, nil
}
