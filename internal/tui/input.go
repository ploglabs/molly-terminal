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
	m.pos = 0
}

func (m *InputModel) MoveEnd() {
	m.pos = len(m.text)
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

// SetCompletions sets the completion candidates.
func (m *InputModel) SetCompletions(completions []string) {
	m.completions = completions
	m.compIdx = 0
}

// ApplyNextCompletion replaces the current word at the cursor with the next completion.
func (m *InputModel) ApplyNextCompletion() {
	if len(m.completions) == 0 {
		return
	}
	completion := m.completions[m.compIdx]
	m.compIdx = (m.compIdx + 1) % len(m.completions)

	// Find start of current word
	wordStart := m.pos
	for wordStart > 0 && m.text[wordStart-1] != ' ' {
		wordStart--
	}

	// Replace word with completion + space
	newText := make([]rune, 0, wordStart+len([]rune(completion))+1)
	newText = append(newText, m.text[:wordStart]...)
	newText = append(newText, []rune(completion)...)
	newText = append(newText, ' ')
	newText = append(newText, m.text[m.pos:]...)
	m.text = newText
	m.pos = wordStart + len([]rune(completion)) + 1
}

// WordAtCursor returns (word, prefix) where prefix is "@" or "#" if present.
func (m *InputModel) WordAtCursor() (word, prefix string) {
	wordStart := m.pos
	for wordStart > 0 && m.text[wordStart-1] != ' ' {
		wordStart--
	}
	w := string(m.text[wordStart:m.pos])
	if len(w) > 0 && (w[0] == '@' || w[0] == '#') {
		return w[1:], string(w[0:1])
	}
	return w, ""
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
	prefix := promptStyle().Render(m.prompt)

	style := inputStyle()
	if m.focused {
		style = inputFocusedStyle()
	}

	maxText := m.width - lipgloss.Width(prefix) - 3
	if maxText < 1 {
		maxText = 1
	}
	if m.pos > maxText {
		maxText = m.pos
	}

	displayText := string(m.text)
	cursorPos := m.pos
	if len(displayText) > maxText {
		start := m.pos - maxText + 1
		if start < 0 {
			start = 0
		}
		displayText = displayText[start:]
		cursorPos = m.pos - start
	}

	if m.focused && m.showCursor {
		c := cursorChar
		if cursorPos >= len([]rune(displayText)) {
			displayText += c
		} else {
			r := []rune(displayText)
			displayText = string(r[:cursorPos]) + c + string(r[cursorPos:])
		}
	}
	if len([]rune(displayText)) > maxText {
		displayText = string([]rune(displayText)[:maxText])
	}

	content := prefix + displayText
	return style.Width(m.width).Render(content)
}

const cursorChar = "\033[7m \033[0m"

func handleInputKey(msg tea.KeyMsg, m *InputModel) (bool, string) {
	switch msg.Type {
	case tea.KeyEnter:
		if msg.Alt || m.modifierHeld(msg, "shift") {
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
	case "ctrl+w":
		wordStart := m.pos
		for wordStart > 0 && m.text[wordStart-1] == ' ' {
			wordStart--
		}
		for wordStart > 0 && m.text[wordStart-1] != ' ' {
			wordStart--
		}
		m.text = append(m.text[:wordStart], m.text[m.pos:]...)
		m.pos = wordStart
	case "ctrl+a":
		m.MoveHome()
	case "ctrl+e":
		m.MoveEnd()
	case "ctrl+k":
		m.text = m.text[:m.pos]
	case "ctrl+u":
		m.Clear()
	}

	return false, ""
}

func (m *InputModel) modifierHeld(msg tea.KeyMsg, mod string) bool {
	return strings.Contains(msg.String(), mod+"+")
}
