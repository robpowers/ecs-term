package ui

import "github.com/charmbracelet/lipgloss"

type ErrorOverlay struct {
	Visible bool
	Message string
}

func (e *ErrorOverlay) Show(msg string) {
	e.Visible = true
	e.Message = msg
}

func (e *ErrorOverlay) Hide() {
	e.Visible = false
	e.Message = ""
}

func (e *ErrorOverlay) Render(bgView string, width, height int) string {
	if !e.Visible {
		return bgView
	}
	boxWidth := min(width-8, 60)
	box := ErrorStyle.Width(boxWidth).Render(
		"Error\n\n" + e.Message + "\n\n[Enter or Esc to dismiss]",
	)
	return lipgloss.Place(width, height,
		lipgloss.Center, lipgloss.Center,
		box,
		lipgloss.WithWhitespaceChars(" "),
	)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
