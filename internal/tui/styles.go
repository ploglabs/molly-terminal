package tui

import (
	"hash/fnv"

	"github.com/charmbracelet/lipgloss"
)

var (
	themeBg               = lipgloss.Color("#000000")
	themeFg               = lipgloss.Color("#b3b3b3")
	themeAccent           = lipgloss.Color("#ffffff")
	themeAccentDim        = lipgloss.Color("#666666")
	themeCyan             = lipgloss.Color("#888888")
	themeDim              = lipgloss.Color("#555555")
	themeBorder           = lipgloss.Color("#333333")
	themeStatusBg         = lipgloss.Color("#0a0a0a")
	themeErr              = lipgloss.Color("#ff5454")
	themeWarn             = lipgloss.Color("#ffaa33")
	themeInputBorder      = lipgloss.Color("#444444")
	themeInputBorderFocus = lipgloss.Color("#b0b0b0")
	themeSelectedBg       = lipgloss.Color("#1a1a1a")

	usernameColors = []lipgloss.Color{
		lipgloss.Color("#ff6b6b"),
		lipgloss.Color("#ffd93d"),
		lipgloss.Color("#6bcb77"),
		lipgloss.Color("#4d96ff"),
		lipgloss.Color("#ff922b"),
		lipgloss.Color("#cc5de8"),
		lipgloss.Color("#20c997"),
		lipgloss.Color("#f06595"),
		lipgloss.Color("#74c0fc"),
		lipgloss.Color("#ff8787"),
		lipgloss.Color("#a9e34b"),
		lipgloss.Color("#c0eb75"),
		lipgloss.Color("#ffc9c9"),
		lipgloss.Color("#91a7ff"),
		lipgloss.Color("#e599f7"),
	}
)

func baseStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(themeBg).Foreground(themeFg)
}

func panelStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(themeBorder).
		Background(themeBg).
		Padding(0, 1)
}

func panelTitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(themeAccent).
		Background(themeBg).
		Bold(true).
		Padding(0, 1)
}

func channelStyle(active bool) lipgloss.Style {
	s := lipgloss.NewStyle().
		Foreground(themeAccentDim).
		PaddingLeft(1)
	if active {
		s = s.Background(themeSelectedBg).Foreground(themeAccent).Bold(true)
	}
	return s
}

func userStyle(selected bool) lipgloss.Style {
	s := lipgloss.NewStyle().Foreground(themeAccentDim).PaddingLeft(1)
	if selected {
		s = s.Background(themeSelectedBg).Foreground(themeCyan).Bold(true)
	}
	return s
}

func messageStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(themeFg).PaddingLeft(1)
}

func inputStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(themeInputBorder).
		Background(themeBg).
		Padding(0, 1)
}

func inputFocusedStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(themeInputBorderFocus).
		Background(themeBg).
		Padding(0, 1)
}

func statusBarStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(themeStatusBg).
		Foreground(themeAccent).
		Padding(0, 1)
}

func statusErrorStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(themeStatusBg).
		Foreground(themeErr).
		Padding(0, 1)
}

func loadingStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(themeDim).
		Italic(true)
}

func promptStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(themeAccent).
		Bold(true)
}

func usernameColor(name string) lipgloss.Color {
	h := fnv.New32a()
	h.Write([]byte(name))
	return usernameColors[int(h.Sum32())%len(usernameColors)]
}

func systemMessageStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(themeDim).
		Italic(true).
		PaddingLeft(1)
}

func coloredUsername(name string) string {
	return lipgloss.NewStyle().Foreground(usernameColor(name)).Render(name)
}

func typingStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(themeDim).
		Italic(true).
		PaddingLeft(1)
}

func commandSuggestionStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(themeFg).
		Background(themeStatusBg).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(themeBorder).
		Padding(0, 1)
}

func presenceStatusStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(themeDim).
		Italic(true).
		PaddingLeft(2)
}

func newMsgBannerStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(themeAccent).
		Background(themeSelectedBg).
		Bold(true)
}

func replyPreviewStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(themeDim).
		Italic(true).
		PaddingLeft(3).
		BorderLeft(true).
		BorderStyle(lipgloss.ThickBorder()).
		BorderForeground(themeAccentDim)
}

func replyBarStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(themeWarn).
		Background(themeStatusBg).
		Bold(false).
		Padding(0, 1)
}

func notifItemStyle(selected bool) lipgloss.Style {
	s := lipgloss.NewStyle().Foreground(themeFg).PaddingLeft(1)
	if selected {
		s = s.Background(themeSelectedBg).Foreground(themeAccent).Bold(true)
	}
	return s
}

func mentionBadgeStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(themeStatusBg).
		Background(themeWarn).
		Bold(true).
		Padding(0, 1)
}

func replySelectStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(lipgloss.Color("#1e1e1e")).
		BorderLeft(true).
		BorderStyle(lipgloss.ThickBorder()).
		BorderForeground(themeAccent).
		PaddingLeft(1)
}

func replySelectPromptStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(themeAccent).
		Background(themeStatusBg).
		Bold(true).
		Padding(0, 1)
}

func autoCompleteStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(themeDim).
		Background(themeStatusBg).
		Padding(0, 1)
}

func autoCompleteItemStyle(highlighted bool) lipgloss.Style {
	s := lipgloss.NewStyle().Foreground(themeAccentDim)
	if highlighted {
		s = s.Foreground(themeAccent).Bold(true)
	}
	return s
}
