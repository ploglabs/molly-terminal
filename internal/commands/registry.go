package commands

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ploglabs/molly-terminal/internal/model"
)

const SystemUsername = "system"

type Registry struct {
	commands map[string]Command
}

func NewRegistry() *Registry {
	return &Registry{
		commands: make(map[string]Command),
	}
}

func (r *Registry) Register(cmd Command) {
	r.commands[cmd.Name()] = cmd
}

func (r *Registry) Execute(name string, args []string) (tea.Cmd, error) {
	cmd, ok := r.commands[name]
	if !ok {
		msg := SystemMsg(fmt.Sprintf("unknown command: /%s — type /help to see available commands", name))
		return func() tea.Msg {
			return CommandOutputMsg{Messages: []model.Message{msg}}
		}, nil
	}
	return cmd.Execute(args)
}

func (r *Registry) List() []Command {
	var cmds []Command
	for _, c := range r.commands {
		cmds = append(cmds, c)
	}
	return cmds
}

func ParseInput(input string) (string, []string) {
	trimmed := strings.TrimSpace(input)
	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return "", nil
	}
	if !strings.HasPrefix(parts[0], "/") {
		return "", nil
	}
	name := strings.TrimPrefix(parts[0], "/")
	var args []string
	if len(parts) > 1 {
		args = parts[1:]
	}
	return name, args
}
