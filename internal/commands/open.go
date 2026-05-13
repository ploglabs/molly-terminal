package commands

import (
	"fmt"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
)

type OpenCmd struct{}

func NewOpenCmd() *OpenCmd {
	return &OpenCmd{}
}

func (c *OpenCmd) Name() string { return "open" }

func (c *OpenCmd) Description() string {
	return "Open an image attachment: /open or /open <n> (1 = latest)"
}

func (c *OpenCmd) Execute(args []string) (tea.Cmd, error) {
	index := 1
	if len(args) > 0 {
		n, err := strconv.Atoi(args[0])
		if err != nil || n < 1 {
			return nil, fmt.Errorf("invalid index — use a positive number, e.g. /open 1")
		}
		index = n
	}
	return func() tea.Msg {
		return OpenImageMsg{Index: index}
	}, nil
}
