package commands

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ploglabs/molly-terminal/internal/model"
)

type HelpCmd struct {
	registry *Registry
}

func NewHelpCmd(r *Registry) *HelpCmd {
	return &HelpCmd{registry: r}
}

func (c *HelpCmd) Name() string {
	return "help"
}

func (c *HelpCmd) Description() string {
	return "Show available commands"
}

func (c *HelpCmd) Execute(args []string) (tea.Cmd, error) {
	cmds := c.registry.List()

	maxLen := 0
	for _, c := range cmds {
		if len(c.Name()) > maxLen {
			maxLen = len(c.Name())
		}
	}

	var lines []string
	lines = append(lines, "Available commands:")
	for _, c := range cmds {
		pad := strings.Repeat(" ", maxLen-len(c.Name())+2)
		lines = append(lines, fmt.Sprintf("  /%s%s- %s", c.Name(), pad, c.Description()))
	}

	return func() tea.Msg {
		return CommandOutputMsg{
			Messages: []model.Message{
				SystemMsg(strings.Join(lines, "\n")),
			},
		}
	}, nil
}
