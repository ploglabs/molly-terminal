package commands

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ploglabs/molly-terminal/internal/db"
)

type LeaveCmd struct {
	store *db.Store
}

func NewLeaveCmd(store *db.Store) *LeaveCmd {
	return &LeaveCmd{store: store}
}

func (c *LeaveCmd) Name() string {
	return "leave"
}

func (c *LeaveCmd) Description() string {
	return "Remove a channel from the sidebar: /leave #channel"
}

func (c *LeaveCmd) Execute(args []string) (tea.Cmd, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("usage: /leave #channel")
	}

	channel := strings.TrimPrefix(args[0], "#")
	if channel == "" {
		return nil, fmt.Errorf("invalid channel name")
	}

	return func() tea.Msg {
		return DeleteChannelMsg{Channel: channel}
	}, nil
}
