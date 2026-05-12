package commands

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ploglabs/molly-terminal/internal/model"
)

type Command interface {
	Name() string
	Description() string
	Execute(args []string) (tea.Cmd, error)
}

type CommandOutputMsg struct {
	Messages []model.Message
}

type SwitchChannelMsg struct {
	Channel string
}

type ClearMessagesMsg struct{}

type TriggerHistoryLoadMsg struct{}

type DeleteChannelMsg struct {
	Channel string
}

func SystemMsg(content string) model.Message {
	return model.Message{
		ID:        "system-" + time.Now().Format("20060102150405.999999999"),
		Username:  "system",
		Content:   content,
		Timestamp: time.Now(),
	}
}
