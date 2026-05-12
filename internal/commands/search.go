package commands

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ploglabs/molly-terminal/internal/db"
	"github.com/ploglabs/molly-terminal/internal/model"
)

type SearchCmd struct {
	store *db.Store
}

func NewSearchCmd(store *db.Store) *SearchCmd {
	return &SearchCmd{store: store}
}

func (c *SearchCmd) Name() string {
	return "search"
}

func (c *SearchCmd) Description() string {
	return "Search messages: /search hello"
}

func (c *SearchCmd) Execute(args []string) (tea.Cmd, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("usage: /search <query>")
	}

	query := strings.Join(args, " ")

	return func() tea.Msg {
		msgs, err := c.store.SearchMessages(query)
		if err != nil {
			return CommandOutputMsg{
				Messages: []model.Message{
					SystemMsg(fmt.Sprintf("search error: %v", err)),
				},
			}
		}

		if len(msgs) == 0 {
			return CommandOutputMsg{
				Messages: []model.Message{
					SystemMsg(fmt.Sprintf("no results found for \"%s\"", query)),
				},
			}
		}

		var lines []string
		lines = append(lines, fmt.Sprintf("Search results for \"%s\":", query))
		for _, m := range msgs {
			ts := m.Timestamp.Format("2006-01-02 15:04")
			lines = append(lines, fmt.Sprintf("  [%s] <%s> %s", ts, m.Username, m.Content))
		}

		return CommandOutputMsg{
			Messages: []model.Message{
				SystemMsg(strings.Join(lines, "\n")),
			},
		}
	}, nil
}
