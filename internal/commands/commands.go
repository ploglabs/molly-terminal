package commands

import tea "github.com/charmbracelet/bubbletea"

type Command interface {
	Name() string
	Description() string
	Execute(args []string) (tea.Cmd, error)
}
