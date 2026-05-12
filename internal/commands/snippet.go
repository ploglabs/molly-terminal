package commands

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type SnippetCmd struct{}

func NewSnippetCmd() *SnippetCmd {
	return &SnippetCmd{}
}

func (c *SnippetCmd) Name() string { return "snippet" }

func (c *SnippetCmd) Description() string {
	return "Send a code snippet: /snippet [lang] <code>"
}

func (c *SnippetCmd) Execute(args []string) (tea.Cmd, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("usage: /snippet [lang] <code>")
	}

	lang := ""
	code := ""

	// If first arg looks like a language name (no special chars, short), treat as lang
	if len(args) >= 2 && isLangToken(args[0]) {
		lang = args[0]
		code = strings.Join(args[1:], " ")
	} else {
		code = strings.Join(args, " ")
	}

	if strings.TrimSpace(code) == "" {
		return nil, fmt.Errorf("code cannot be empty")
	}

	formatted := fmt.Sprintf("```%s\n%s\n```", lang, code)
	return func() tea.Msg {
		return SendRawMsg{Content: formatted}
	}, nil
}

// isLangToken returns true if s looks like a language identifier (short, alphanumeric/+).
func isLangToken(s string) bool {
	if len(s) > 20 {
		return false
	}
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '+' || r == '#' || r == '-') {
			return false
		}
	}
	return true
}
