package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var emojiMap = map[string]string{
	"rocket":             "🚀",
	"fire":               "🔥",
	"heart":              "❤️",
	"thumbsup":           "👍",
	"thumbsdown":         "👎",
	"check":              "✅",
	"x":                  "❌",
	"warning":            "⚠️",
	"star":               "⭐",
	"wave":               "👋",
	"joy":                "😂",
	"smile":              "😊",
	"eyes":               "👀",
	"100":                "💯",
	"tada":               "🎉",
	"bug":                "🐛",
	"zap":                "⚡",
	"sparkles":           "✨",
	"pray":               "🙏",
	"thinking":           "🤔",
	"clap":               "👏",
	"ok_hand":            "👌",
	"white_check_mark":   "✅",
	"information_source": "ℹ️",
	"pencil":             "✏️",
	"memo":               "📝",
	"computer":           "💻",
	"key":                "🔑",
	"lock":               "🔒",
	"unlock":             "🔓",
	"package":            "📦",
	"link":               "🔗",
	"hammer":             "🔨",
	"wrench":             "🔧",
	"gear":               "⚙️",
	"point_right":        "👉",
	"raised_hand":        "✋",
	"+1":                 "👍",
	"-1":                 "👎",
}

func renderMarkdown(text string, _ int) string {
	var result strings.Builder
	lines := strings.Split(text, "\n")
	inCodeBlock := false

	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}
		if strings.HasPrefix(line, "```") {
			if inCodeBlock {
				inCodeBlock = false
				result.WriteString(lipgloss.NewStyle().Foreground(themeDim).Render("```"))
			} else {
				inCodeBlock = true
				lang := strings.TrimPrefix(line, "```")
				if lang != "" {
					result.WriteString(lipgloss.NewStyle().Foreground(themeDim).Render("```" + lang))
				} else {
					result.WriteString(lipgloss.NewStyle().Foreground(themeDim).Render("```"))
				}
			}
			continue
		}
		if inCodeBlock {
			result.WriteString(lipgloss.NewStyle().Foreground(themeCyan).Render(line))
			continue
		}
		result.WriteString(renderInline(line))
	}
	return result.String()
}

// renderInline applies inline markdown to a single line using a one-pass tokenizer.
func renderInline(text string) string {
	runes := []rune(text)
	n := len(runes)
	var out strings.Builder
	i := 0

	for i < n {
		switch {
		case runes[i] == ':':
			// emoji shortcode :name:
			j := i + 1
			for j < n && runes[j] != ':' && runes[j] != ' ' && runes[j] != '\n' {
				j++
			}
			if j < n && runes[j] == ':' && j > i+1 {
				name := string(runes[i+1 : j])
				if emoji, ok := emojiMap[name]; ok {
					out.WriteString(emoji)
					i = j + 1
					continue
				}
			}
			out.WriteRune(runes[i])
			i++

		case i+1 < n && runes[i] == '*' && runes[i+1] == '*':
			// bold **text**
			j := i + 2
			for j+1 < n && !(runes[j] == '*' && runes[j+1] == '*') {
				j++
			}
			if j+1 < n {
				inner := string(runes[i+2 : j])
				out.WriteString(lipgloss.NewStyle().Bold(true).Foreground(themeFg).Render(inner))
				i = j + 2
			} else {
				out.WriteRune('*')
				out.WriteRune('*')
				i += 2
			}

		case runes[i] == '*' || runes[i] == '_':
			// italic *text* or _text_
			delim := runes[i]
			j := i + 1
			for j < n && runes[j] != delim && runes[j] != '\n' {
				j++
			}
			if j < n && runes[j] == delim && j > i+1 {
				inner := string(runes[i+1 : j])
				out.WriteString(lipgloss.NewStyle().Italic(true).Foreground(themeFg).Render(inner))
				i = j + 1
			} else {
				out.WriteRune(runes[i])
				i++
			}

		case runes[i] == '`':
			// inline code `code`
			j := i + 1
			for j < n && runes[j] != '`' && runes[j] != '\n' {
				j++
			}
			if j < n && runes[j] == '`' {
				inner := string(runes[i+1 : j])
				out.WriteString(lipgloss.NewStyle().Foreground(themeCyan).Render(inner))
				i = j + 1
			} else {
				out.WriteRune(runes[i])
				i++
			}

		case i+7 <= n && string(runes[i:i+7]) == "http://":
			j := i
			for j < n && runes[j] != ' ' && runes[j] != '\n' {
				j++
			}
			url := string(runes[i:j])
			out.WriteString(lipgloss.NewStyle().Foreground(themeCyan).Underline(true).Render(url))
			i = j

		case i+8 <= n && string(runes[i:i+8]) == "https://":
			j := i
			for j < n && runes[j] != ' ' && runes[j] != '\n' {
				j++
			}
			url := string(runes[i:j])
			out.WriteString(lipgloss.NewStyle().Foreground(themeCyan).Underline(true).Render(url))
			i = j

		default:
			out.WriteRune(runes[i])
			i++
		}
	}

	return out.String()
}
