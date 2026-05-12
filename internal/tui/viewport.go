package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/ploglabs/molly-terminal/internal/model"
)

type ViewportModel struct {
	width      int
	height     int
	offset     int
	messages   []model.Message
	loading    bool
	allLoaded  bool
	myUsername string
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

	end := total - v.offset
	if end > total {
		end = total
	}
	if end < 0 {
		end = 0
	}

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

	if v.loading {
		lines = append(lines, loadingStyle().Render("  loading older messages..."))
	}

	for _, m := range v.messages[start:end] {
		var msgLines []string

		if m.Username == "system" {
			msgLines = append(msgLines, systemMessageStyle().Render(m.Content))
		} else {
			// Reply preview line
			if m.ReplyToID != "" {
				author := m.ReplyToAuthor
				if author == "" {
					author = "unknown"
				}
				snippet := m.ReplyToContent
				maxSnip := v.width - 12
				if maxSnip < 10 {
					maxSnip = 10
				}
				if len([]rune(snippet)) > maxSnip {
					snippet = string([]rune(snippet)[:maxSnip]) + "…"
				}
				// Strip newlines from snippet
				snippet = strings.ReplaceAll(snippet, "\n", " ")
				replyLine := replyPreviewStyle().Render(fmt.Sprintf("↩ %s: %s", author, snippet))
				msgLines = append(msgLines, replyLine)
			}

			ts := tsStyle.Render(formatTime(m.Timestamp))
			username := coloredUsername(m.Username)
			content := renderMarkdown(m.Content, v.myUsername, v.width-12)
			raw := fmt.Sprintf("%s <%s> %s", ts, username, content)
			wrapped := wrapText(raw, v.width)
			msgLines = append(msgLines, msgStyle.Width(v.width).Render(wrapped))
		}

		lines = append(lines, msgLines...)
	}

	if showNewerBanner {
		indicator := fmt.Sprintf("  ↑ %d new — PgDn/↓ to scroll down", v.offset)
		lines = append(lines, newMsgBannerStyle().Width(v.width).Render(indicator))
	}

	content := strings.Join(lines, "\n")

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
		if m.Username == "system" {
			continue
		}
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

func truncateStatus(s string, max int) string {
	if max <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}

func clipLines(s string, maxLines int) string {
	if maxLines <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	return strings.Join(lines[:maxLines], "\n")
}

func formatTime(t time.Time) string {
	local := t.Local()
	now := time.Now()
	if local.Year() == now.Year() && local.YearDay() == now.YearDay() {
		return local.Format("15:04")
	}
	return local.Format("01/02 15:04")
}
