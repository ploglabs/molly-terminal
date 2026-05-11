package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/ploglabs/molly-terminal/internal/model"
)

type ViewportModel struct {
	width           int
	height          int
	offset          int
	messages        []model.Message
	showTimestamps  bool
	loading         bool
	allLoaded       bool
}

func newViewport() ViewportModel {
	return ViewportModel{}
}

func (v *ViewportModel) SetSize(w, h int) {
	v.width = w
	v.height = h
}

func (v *ViewportModel) SetMessages(msgs []model.Message) {
	v.messages = msgs
}

func (v *ViewportModel) ScrollUp(n int) {
	v.offset += n
	maxOff := len(v.messages) - 1
	if v.offset > maxOff {
		v.offset = maxOff
	}
	if v.offset < 0 {
		v.offset = 0
	}
}

func (v *ViewportModel) ScrollDown(n int) {
	v.offset -= n
	if v.offset < 0 {
		v.offset = 0
	}
}

func (v *ViewportModel) ScrollToBottom() {
	v.offset = 0
}

func (v *ViewportModel) AtTop() bool {
	return v.offset >= len(v.messages)-v.visibleLines() && len(v.messages) > 0
}

func (v *ViewportModel) visibleLines() int {
	if v.height <= 0 {
		return 1
	}
	return v.height
}

func (v *ViewportModel) View() string {
	if v.width <= 0 || v.height <= 0 {
		return ""
	}

	style := messageStyle().Width(v.width)

	visible := v.visibleLines()
	total := len(v.messages)

	if total == 0 {
		empty := lipgloss.NewStyle().
			Width(v.width).
			Height(v.height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(themeDim).
			Render("no messages yet...")
		return empty
	}

	start := total - visible - v.offset
	if start < 0 {
		start = 0
	}
	end := total - v.offset
	if end > total {
		end = total
	}

	if v.offset > 0 {
		scrollUpCount := total - end
		if scrollUpCount > 0 {
			more := fmt.Sprintf("  %d older messages  ", scrollUpCount)
			return loadingStyle().Width(v.width).Align(lipgloss.Center, lipgloss.Center).Height(v.height).Render(more)
		}
	}

	var lines []string
	for _, m := range v.messages[start:end] {
		username := coloredUsername(m.Username)
		line := fmt.Sprintf("<%s> %s", username, m.Content)

		if v.showTimestamps {
			ts := m.Timestamp.Format("15:04")
			line = fmt.Sprintf("%s %s", lipgloss.NewStyle().Foreground(themeDim).Render(ts), line)
		}

		line = style.Render(line)
		line = wrapText(line, v.width)
		lines = append(lines, line)
	}

	content := strings.Join(lines, "\n")

	if v.loading && v.offset >= len(v.messages)-visible {
		content += "\n" + loadingStyle().Render("  loading older messages...")
	}

	height := v.height
	currentLines := len(strings.Split(content, "\n"))
	if currentLines < height {
		content = strings.Repeat("\n", height-currentLines) + content
	}

	return content
}

func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}
	var lines []string
	parts := strings.Split(text, "\n")
	for _, part := range parts {
		if lipgloss.Width(part) <= width {
			lines = append(lines, part)
			continue
		}
		runes := []rune(part)
		for len(runes) > 0 {
			if len(runes) <= width {
				lines = append(lines, string(runes))
				break
			}
			lines = append(lines, string(runes[:width]))
			runes = runes[width:]
		}
	}
	return strings.Join(lines, "\n")
}

func msgsToUsers(msgs []model.Message) []string {
	seen := make(map[string]struct{})
	var users []string
	for _, m := range msgs {
		if _, ok := seen[m.Username]; !ok {
			seen[m.Username] = struct{}{}
			users = append(users, m.Username)
		}
	}
	return users
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func formatTime(t time.Time) string {
	now := time.Now()
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return t.Format("15:04")
	}
	return t.Format("01/02 15:04")
}
