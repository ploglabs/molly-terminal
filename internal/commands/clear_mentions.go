package commands

import tea "github.com/charmbracelet/bubbletea"

type ClearNotificationsMsg struct{}

type ClearMentionsCmd struct{}

func NewClearMentionsCmd() *ClearMentionsCmd {
	return &ClearMentionsCmd{}
}

func (c *ClearMentionsCmd) Name() string        { return "clear-mentions" }
func (c *ClearMentionsCmd) Description() string { return "clear all mention notifications" }
func (c *ClearMentionsCmd) Execute(args []string) (tea.Cmd, error) {
	return func() tea.Msg {
		return ClearNotificationsMsg{}
	}, nil
}
