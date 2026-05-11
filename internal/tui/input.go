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
}

func (m *InputModel) Backspace() {
	if m.pos > 0 {
		m.text = append(m.text[:m.pos-1], m.text[m.pos:]...)
		m.pos--
	}
}

func (m *InputModel) DeleteForward() {
	if m.pos < len(m.text) {
		m.text = append(m.text[:m.pos], m.text[m.pos+1:]...)
	}
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
	text := string(m.text)

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

	displayText := text
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
		if cursorPos > len(displayText) {
			displayText += c
		} else if cursorPos == len(displayText) {
			displayText += c
		} else {
			displayText = displayText[:cursorPos] + c + displayText[cursorPos:]
		}
	}
	if len(displayText) > maxText {
		displayText = displayText[:maxText]
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

	if msg.String() == "ctrl+w" {
		wordStart := m.pos
		for wordStart > 0 && m.text[wordStart-1] == ' ' {
			wordStart--
		}
		for wordStart > 0 && m.text[wordStart-1] != ' ' {
			wordStart--
		}
		m.text = append(m.text[:wordStart], m.text[m.pos:]...)
		m.pos = wordStart
	}
	if msg.String() == "ctrl+a" {
		m.MoveHome()
	}
	if msg.String() == "ctrl+e" {
		m.MoveEnd()
	}
	if msg.String() == "ctrl+k" {
		m.text = m.text[:m.pos]
	}
	if msg.String() == "ctrl+u" {
		m.text = m.text[m.pos:]
		m.pos = 0
	}

	return false, ""
}

func (m *InputModel) modifierHeld(msg tea.KeyMsg, mod string) bool {
	return strings.Contains(msg.String(), mod+"+")
}
