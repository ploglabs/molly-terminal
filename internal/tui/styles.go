package tui

import (
	"hash/fnv"

	"github.com/charmbracelet/lipgloss"
)

var (
	themeBg          = lipgloss.Color("#0a0e14")
	themeFg          = lipgloss.Color("#b3b3b3")
	themeAccent      = lipgloss.Color("#39c965")
	themeAccentDim   = lipgloss.Color("#1e8a3e")
	themeCyan        = lipgloss.Color("#59c2ff")
	themeDim         = lipgloss.Color("#505050")
	themeBorder      = lipgloss.Color("#1a3a1a")
	themeStatusBg    = lipgloss.Color("#0f1a0f")
	themeErr         = lipgloss.Color("#ff5454")
	themeWarn        = lipgloss.Color("#ffaa33")
	themeInputBorder = lipgloss.Color("#2a5a2a")
	themeSelectedBg  = lipgloss.Color("#1a2a1a")

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
		BorderStyle(lipgloss.RoundedBorder()).
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
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(themeInputBorder).
		Background(themeBg).
		Padding(0, 1)
}

func inputFocusedStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(themeAccent).
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
