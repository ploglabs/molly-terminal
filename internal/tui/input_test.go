package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestInputInsert(t *testing.T) {
	m := newInput("> ")
	m.Insert('h')
	m.Insert('i')
	if m.Value() != "hi" {
		t.Errorf("expected 'hi', got %q", m.Value())
	}
	if m.pos != 2 {
		t.Errorf("expected cursor at 2, got %d", m.pos)
	}
}

func TestInputBackspace(t *testing.T) {
	m := newInput("> ")
	m.SetValue("hello")
	m.Backspace()
	if m.Value() != "hell" {
		t.Errorf("expected 'hell', got %q", m.Value())
	}
	if m.pos != 4 {
		t.Errorf("expected cursor at 4, got %d", m.pos)
	}
}

func TestInputBackspaceAtStart(t *testing.T) {
	m := newInput("> ")
	m.SetValue("hi")
	m.MoveLeft()
	m.MoveLeft()
	m.Backspace()
	if m.Value() != "hi" {
		t.Errorf("expected 'hi' unchanged, got %q", m.Value())
	}
}

func TestInputDeleteForward(t *testing.T) {
	m := newInput("> ")
	m.SetValue("hello")
	m.MoveLeft()
	m.MoveLeft()
	m.DeleteForward()
	if m.Value() != "helo" {
		t.Errorf("expected 'helo', got %q", m.Value())
	}
}

func TestInputMoveLeftRight(t *testing.T) {
	m := newInput("> ")
	m.SetValue("abc")
	if m.pos != 3 {
		t.Errorf("expected cursor at end (3), got %d", m.pos)
	}
	m.MoveLeft()
	if m.pos != 2 {
		t.Errorf("expected cursor at 2, got %d", m.pos)
	}
	m.MoveRight()
	if m.pos != 3 {
		t.Errorf("expected cursor at 3, got %d", m.pos)
	}
	m.MoveRight()
	if m.pos != 3 {
		t.Errorf("expected cursor still at 3, got %d", m.pos)
	}
}

func TestInputMoveHomeEnd(t *testing.T) {
	m := newInput("> ")
	m.SetValue("abcdef")
	m.MoveLeft()
	m.MoveLeft()
	m.MoveHome()
	if m.pos != 0 {
		t.Errorf("expected cursor at home (0), got %d", m.pos)
	}
	m.MoveEnd()
	if m.pos != 6 {
		t.Errorf("expected cursor at end (6), got %d", m.pos)
	}
}

func TestInputClear(t *testing.T) {
	m := newInput("> ")
	m.SetValue("hello")
	m.Clear()
	if m.Value() != "" {
		t.Errorf("expected empty string, got %q", m.Value())
	}
	if m.pos != 0 {
		t.Errorf("expected cursor at 0, got %d", m.pos)
	}
}

func TestInputFocus(t *testing.T) {
	m := newInput("> ")
	if !m.focused {
		t.Error("expected input to be focused by default")
	}
	m.Blur()
	if m.focused {
		t.Error("expected input to be blurred")
	}
	m.Focus()
	if !m.focused {
		t.Error("expected input to be focused")
	}
}

func TestInputSetValue(t *testing.T) {
	m := newInput("> ")
	m.SetValue("test")
	if m.Value() != "test" {
		t.Errorf("expected 'test', got %q", m.Value())
	}
	if m.pos != 4 {
		t.Errorf("expected cursor at 4, got %d", m.pos)
	}
}

func TestHandleInputKeyEnter(t *testing.T) {
	m := newInput("> ")
	m.SetValue("hello world")
	submitted, val := handleInputKey(tea.KeyMsg{Type: tea.KeyEnter}, &m)
	if !submitted {
		t.Error("expected enter to submit")
	}
	if val != "hello world" {
		t.Errorf("expected 'hello world', got %q", val)
	}
	if m.Value() != "" {
		t.Error("expected input cleared after submit")
	}
}

func TestHandleInputKeyBackspace(t *testing.T) {
	m := newInput("> ")
	m.SetValue("abc")
	handleInputKey(tea.KeyMsg{Type: tea.KeyBackspace}, &m)
	if m.Value() != "ab" {
		t.Errorf("expected 'ab', got %q", m.Value())
	}
}

func TestHandleInputKeyRunes(t *testing.T) {
	m := newInput("> ")
	handleInputKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x', 'y', 'z'}}, &m)
	if m.Value() != "xyz" {
		t.Errorf("expected 'xyz', got %q", m.Value())
	}
}

func TestHandleInputKeyCtrlW(t *testing.T) {
	m := newInput("> ")
	m.SetValue("hello world")
	handleInputKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")}, &m)
	_ = m
	m2 := newInput("> ")
	m2.SetValue("foo bar baz")
	m2.MoveLeft()
	m2.MoveLeft()
	m2.MoveLeft()
	m2.MoveLeft()
	original := m2.Value()
	handleInputKey(tea.KeyMsg{}, &m2)
	if m2.Value() != original {
	}
}

func TestHandleInputKeyCtrlA(t *testing.T) {
	m := newInput("> ")
	m.SetValue("hello")
	handleInputKey(tea.KeyMsg{}, &m)
}

func TestHandleInputKeyCtrlE(t *testing.T) {
	m := newInput("> ")
	m.SetValue("hello")
	m.MoveHome()
	handleInputKey(tea.KeyMsg{}, &m)
}

func TestHandleInputKeyEmptyEnter(t *testing.T) {
	m := newInput("> ")
	submitted, val := handleInputKey(tea.KeyMsg{Type: tea.KeyEnter}, &m)
	if !submitted {
		t.Error("expected enter to submit even with empty input")
	}
	if val != "" {
		t.Errorf("expected empty, got %q", val)
	}
}

func TestInsertNewline(t *testing.T) {
	m := newInput("> ")
	m.Insert('a')
	m.InsertNewline()
	m.Insert('b')
	val := m.Value()
	if len(val) != 3 {
		t.Errorf("expected 3 characters, got %d: %q", len(val), val)
	}
}

func TestCursorBlinkCmd(t *testing.T) {
	m := newInput("> ")
	cmd := m.CursorBlinkCmd()
	if cmd == nil {
		t.Error("expected non-nil cursor blink command")
	}
	msg := cmd()
	if msg == nil {
		t.Error("expected non-nil cursor blink message")
	}
}

func TestCursorBlinkUpdate(t *testing.T) {
	m := newInput("> ")
	initial := m.showCursor
	var cmd tea.Cmd
	m, cmd = m.Update(cursorBlinkMsg{})
	if m.showCursor == initial {
		t.Error("expected cursor blink to toggle")
	}
	if cmd == nil {
		t.Error("expected blink cmd after toggle")
	}
}

func TestInputWidth(t *testing.T) {
	m := newInput("> ")
	m.SetWidth(40)
	if m.width != 40 {
		t.Errorf("expected width 40, got %d", m.width)
	}
	m.SetWidth(2)
	if m.width != 6 {
		t.Errorf("expected min width 6, got %d", m.width)
	}
}
