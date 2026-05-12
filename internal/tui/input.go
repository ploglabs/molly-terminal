package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type InputModel struct {
	width      int
	prompt     string
	text       []rune
	pos        int
	focused    bool
	showCursor bool

	completions []string
	compIdx     int
}

func newInput(prompt string) InputModel {
	return InputModel{
		prompt:     prompt,
		text:       nil,
		pos:        0,
		focused:    true,
		showCursor: true,
	}
}

func (m *InputModel) SetWidth(w int) {
	if w < 6 {
		w = 6
	}
	m.width = w
}

func (m *InputModel) Value() string {
	return string(m.text)
}

func (m *InputModel) SetValue(v string) {
	m.text = []rune(v)
	m.pos = len(m.text)
}

func (m *InputModel) Focus() {
	m.focused = true
}

func (m *InputModel) Blur() {
	m.focused = false
}

func (m *InputModel) Insert(ch rune) {
	m.text = append(m.text[:m.pos], append([]rune{ch}, m.text[m.pos:]...)...)
	m.pos++
	m.clearCompletions()
}

func (m *InputModel) Backspace() {
	if m.pos > 0 {
		m.text = append(m.text[:m.pos-1], m.text[m.pos:]...)
		m.pos--
	}
	m.clearCompletions()
}

func (m *InputModel) DeleteForward() {
	if m.pos < len(m.text) {
		m.text = append(m.text[:m.pos], m.text[m.pos+1:]...)
	}
	m.clearCompletions()
}

func (m *InputModel) MoveLeft() {
	if m.pos > 0 {
		m.pos--
	}
}

func (m *InputModel) MoveRight() {
	if m.pos < len(m.text) {
		m.pos++
	}
}

func (m *InputModel) MoveHome() {
	// Move to start of current line
	p := m.pos
	for p > 0 && m.text[p-1] != '\n' {
		p--
	}
	m.pos = p
}

func (m *InputModel) MoveEnd() {
	// Move to end of current line
	p := m.pos
	for p < len(m.text) && m.text[p] != '\n' {
		p++
	}
	m.pos = p
}

func (m *InputModel) InsertNewline() {
	m.Insert('\n')
}

func (m *InputModel) Clear() {
	m.text = nil
	m.pos = 0
	m.clearCompletions()
}

func (m *InputModel) clearCompletions() {
	m.completions = nil
	m.compIdx = 0
}

func (m *InputModel) SetCompletions(completions []string) {
	m.completions = completions
	m.compIdx = 0
}

func (m *InputModel) ApplyNextCompletion() {
	if len(m.completions) == 0 {
		return
	}
	completion := m.completions[m.compIdx]
	m.compIdx = (m.compIdx + 1) % len(m.completions)

	wordStart := m.pos
	for wordStart > 0 && m.text[wordStart-1] != ' ' && m.text[wordStart-1] != '\n' {
		wordStart--
	}

	newText := make([]rune, 0, wordStart+len([]rune(completion))+1)
	newText = append(newText, m.text[:wordStart]...)
	newText = append(newText, []rune(completion)...)
	newText = append(newText, ' ')
	newText = append(newText, m.text[m.pos:]...)
	m.text = newText
	m.pos = wordStart + len([]rune(completion)) + 1
}

func (m *InputModel) WordAtCursor() (word, prefix string) {
	wordStart := m.pos
	for wordStart > 0 && m.text[wordStart-1] != ' ' && m.text[wordStart-1] != '\n' {
		wordStart--
	}
	w := string(m.text[wordStart:m.pos])
	if len(w) > 0 && (w[0] == '@' || w[0] == '#' || w[0] == '/') {
		return w[1:], string(w[0:1])
	}
	return w, ""
}

// LineCount returns the number of lines currently in the input.
func (m *InputModel) LineCount() int {
	return strings.Count(string(m.text), "\n") + 1
}

func (m InputModel) CursorBlinkCmd() tea.Cmd {
	return tea.Tick(530*time.Millisecond, func(t time.Time) tea.Msg {
		return cursorBlinkMsg{}
	})
}

type cursorBlinkMsg struct{}

func (m InputModel) Update(msg tea.Msg) (InputModel, tea.Cmd) {
	switch msg.(type) {
	case cursorBlinkMsg:
		m.showCursor = !m.showCursor
		return m, m.CursorBlinkCmd()
	}
	return m, nil
}

func (m InputModel) View() string {
	return m.view(0)
}

func (m InputModel) ViewHeight(maxHeight int) string {
	return m.view(maxHeight)
}

func (m InputModel) view(maxHeight int) string {
	style := inputStyle()
	if m.focused {
		style = inputFocusedStyle()
	}

	styleW := m.width - 2
	if styleW < 1 {
		styleW = 1
	}

	promptStr := promptStyle().Render(m.prompt)
	promptW := lipgloss.Width(promptStr)
	contentW := styleW - promptW - 2
	if contentW < 4 {
		contentW = 4
	}

	rawText := string(m.text)
	lines := strings.Split(rawText, "\n")

	// Compute cursor row/col within the text
	cursorRow, cursorCol := 0, 0
	pos := 0
	for row, line := range lines {
		lineLen := len([]rune(line))
		if pos+lineLen >= m.pos {
			cursorRow = row
			cursorCol = m.pos - pos
			break
		}
		pos += lineLen + 1 // +1 for \n
	}

	// Build display lines, inserting cursor
	var displayLines []string
	for row, line := range lines {
		runes := []rune(line)

		// Scroll horizontally if line is too long
		start := 0
		if row == cursorRow && cursorCol > contentW-1 {
			start = cursorCol - contentW + 1
			if start < 0 {
				start = 0
			}
		}
		if start > len(runes) {
			start = len(runes)
		}
		visible := runes[start:]
		if len(visible) > contentW {
			visible = visible[:contentW]
		}

		var lineStr string
		if m.focused && m.showCursor && row == cursorRow {
			col := cursorCol - start
			if col < 0 {
				col = 0
			}
			if col > len(visible) {
				col = len(visible)
			}
			lineStr = string(visible[:col]) + cursorChar + string(visible[col:])
		} else {
			lineStr = string(visible)
		}

		if row == 0 {
			displayLines = append(displayLines, promptStr+lineStr)
		} else {
			displayLines = append(displayLines, strings.Repeat(" ", promptW)+lineStr)
		}
	}

	if maxHeight > 0 {
		maxContentLines := maxHeight - 2
		if maxContentLines < 1 {
			maxContentLines = 1
		}
		if len(displayLines) > maxContentLines {
			start := cursorRow - maxContentLines + 1
			if start < 0 {
				start = 0
			}
			if start+maxContentLines > len(displayLines) {
				start = len(displayLines) - maxContentLines
			}
			displayLines = displayLines[start : start+maxContentLines]
		}
		style = style.Height(maxContentLines).MaxHeight(maxHeight)
	}

	content := strings.Join(displayLines, "\n")
	return style.Width(styleW).MaxWidth(m.width).Render(content)
}

const cursorChar = "\033[7m \033[0m"

func handleInputKey(msg tea.KeyMsg, m *InputModel) (bool, string) {
	switch msg.Type {
	case tea.KeyEnter:
		// Shift+Enter, Alt+Enter, or Ctrl+J inserts newline; plain Enter submits.
		if msg.Alt || msg.String() == "shift+enter" || msg.String() == "shift+ctrl+j" {
			m.InsertNewline()
			return false, ""
		}
		val := m.Value()
		m.Clear()
		return true, val

	case tea.KeyBackspace:
		m.Backspace()
	case tea.KeyDelete:
		m.DeleteForward()
	case tea.KeyLeft:
		m.MoveLeft()
	case tea.KeyRight:
		m.MoveRight()
	case tea.KeyHome:
		m.MoveHome()
	case tea.KeyEnd:
		m.MoveEnd()
	case tea.KeyRunes:
		for _, r := range msg.Runes {
			m.Insert(r)
		}
	case tea.KeySpace:
		m.Insert(' ')
	}

	switch msg.String() {
	case "shift+enter", "shift+ctrl+j":
		m.InsertNewline()
	case "ctrl+j":
		// Ctrl+J = newline (classic terminal shortcut)
		m.InsertNewline()
	case "ctrl+w":
		wordStart := m.pos
		for wordStart > 0 && (m.text[wordStart-1] == ' ' || m.text[wordStart-1] == '\n') {
			wordStart--
		}
		for wordStart > 0 && m.text[wordStart-1] != ' ' && m.text[wordStart-1] != '\n' {
			wordStart--
		}
		m.text = append(m.text[:wordStart], m.text[m.pos:]...)
		m.pos = wordStart
	case "ctrl+a":
		m.MoveHome()
	case "ctrl+e":
		m.MoveEnd()
	case "ctrl+k":
		// Delete to end of current line
		end := m.pos
		for end < len(m.text) && m.text[end] != '\n' {
			end++
		}
		m.text = append(m.text[:m.pos], m.text[end:]...)
	case "ctrl+u":
		m.Clear()
	}

	return false, ""
}
