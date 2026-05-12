package commands

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ploglabs/molly-terminal/internal/db"
	"github.com/ploglabs/molly-terminal/internal/history"
	"github.com/ploglabs/molly-terminal/internal/wsclient"
)

type JoinCmd struct {
	client  *wsclient.Client
	fetcher *history.Fetcher
	store   *db.Store
}

func NewJoinCmd(client *wsclient.Client, fetcher *history.Fetcher, store *db.Store) *JoinCmd {
	return &JoinCmd{client: client, fetcher: fetcher, store: store}
}

func (c *JoinCmd) Name() string {
	return "join"
}

func (c *JoinCmd) Description() string {
	return "Switch to a channel: /join #general"
}

func (c *JoinCmd) Execute(args []string) (tea.Cmd, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("usage: /join #channel")
	}

	channel := strings.TrimPrefix(args[0], "#")
	if channel == "" {
		return nil, fmt.Errorf("invalid channel name")
	}

	return func() tea.Msg {
		return SwitchChannelMsg{Channel: channel}
	}, nil
}
