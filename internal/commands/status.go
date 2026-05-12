package commands

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type SetStatusMsg struct {
	Status string
}

type StatusCmd struct{}

func NewStatusCmd() *StatusCmd {
	return &StatusCmd{}
}

func (c *StatusCmd) Name() string { return "status" }

func (c *StatusCmd) Description() string {
	return "Set your activity status: /status <text> | /status clear"
}

func (c *StatusCmd) Execute(args []string) (tea.Cmd, error) {
	status := strings.Join(args, " ")
	if strings.EqualFold(strings.TrimSpace(status), "clear") {
		status = ""
	}
	return func() tea.Msg {
		return SetStatusMsg{Status: status}
	}, nil
}
