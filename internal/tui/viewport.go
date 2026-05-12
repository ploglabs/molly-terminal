package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/ploglabs/molly-terminal/internal/model"
)

type ViewportModel struct {
	width     int
	height    int
	offset    int
	messages  []model.Message
	loading   bool
	allLoaded bool
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

	visible := v.visibleLines()
	total := len(v.messages)

	if total == 0 {
		return lipgloss.NewStyle().
			Width(v.width).
			Height(v.height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(themeDim).
			Render("no messages yet...")
	}

	// Window: show `visible` messages ending at total - offset
	end := total - v.offset
	if end > total {
		end = total
	}
	if end < 0 {
		end = 0
	}

	// Reserve a line for the "newer messages" indicator when scrolled up
	effectiveVisible := visible
	showNewerBanner := v.offset > 0
	if showNewerBanner {
		effectiveVisible = visible - 1
		if effectiveVisible < 1 {
			effectiveVisible = 1
		}
	}

	start := end - effectiveVisible
	if start < 0 {
		start = 0
	}

	msgStyle := messageStyle()
	tsStyle := lipgloss.NewStyle().Foreground(themeDim)
	var lines []string

	// Loading older history indicator at top
	if v.loading {
		lines = append(lines, loadingStyle().Render("  loading older messages..."))
	}

	for _, m := range v.messages[start:end] {
		var line string
		if m.Username == "system" {
			line = systemMessageStyle().Render(m.Content)
		} else {
			ts := tsStyle.Render(formatTime(m.Timestamp))
			username := coloredUsername(m.Username)
			content := renderMarkdown(m.Content, v.width-12)
			raw := fmt.Sprintf("%s <%s> %s", ts, username, content)
			wrapped := wrapText(raw, v.width)
			line = msgStyle.Width(v.width).Render(wrapped)
		}
		lines = append(lines, line)
	}

	// Newer messages indicator at bottom when scrolled up
	if showNewerBanner {
		indicator := fmt.Sprintf("  ↑ %d new — PgDn/↓ to scroll down", v.offset)
		lines = append(lines, newMsgBannerStyle().Width(v.width).Render(indicator))
	}

	content := strings.Join(lines, "\n")

	// Pad top with empty lines so content is bottom-anchored
	currentLines := len(strings.Split(content, "\n"))
	if currentLines < visible {
		content = strings.Repeat("\n", visible-currentLines) + content
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
