package commands

import (
	tea "github.com/charmbracelet/bubbletea"
)

type ClearCmd struct{}

func NewClearCmd() *ClearCmd {
	return &ClearCmd{}
}

func (c *ClearCmd) Name() string {
	return "clear"
}

func (c *ClearCmd) Description() string {
	return "Clear the current chat view"
}

func (c *ClearCmd) Execute(args []string) (tea.Cmd, error) {
	return func() tea.Msg {
		return ClearMessagesMsg{}
	}, nil
}
