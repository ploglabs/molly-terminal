package commands

import (
	tea "github.com/charmbracelet/bubbletea"
)

type QuitCmd struct{}

func NewQuitCmd() *QuitCmd {
	return &QuitCmd{}
}

func (c *QuitCmd) Name() string {
	return "quit"
}

func (c *QuitCmd) Description() string {
	return "Gracefully exit the application"
}

func (c *QuitCmd) Execute(args []string) (tea.Cmd, error) {
	return tea.Quit, nil
}
