package ui

import "github.com/charmbracelet/lipgloss"

var (
	ColorPrimary   = lipgloss.Color("#00ADD8") // Go cyan / k9s-like
	ColorSecondary = lipgloss.Color("#5C5C5C")
	ColorGreen     = lipgloss.Color("#27AE60")
	ColorYellow    = lipgloss.Color("#F39C12")
	ColorRed       = lipgloss.Color("#E74C3C")
	ColorDim       = lipgloss.Color("#666666")
	ColorWhite     = lipgloss.Color("#EEEEEE")
	ColorBg        = lipgloss.Color("#1A1A2E")

	HeaderStyle = lipgloss.NewStyle().
			Background(ColorPrimary).
			Foreground(lipgloss.Color("#000000")).
			Bold(true).
			Padding(0, 1)

	FooterStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#2A2A2A")).
			Foreground(ColorDim).
			Padding(0, 1)

	KeyHintStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary)

	KeyDescStyle = lipgloss.NewStyle().
			Foreground(ColorDim)

	ErrorStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorRed).
			Foreground(ColorWhite).
			Padding(1, 2).
			Bold(false)

	HelpStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Foreground(ColorWhite).
			Padding(1, 2)

	TitleStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	HealthyStyle = lipgloss.NewStyle().Foreground(ColorGreen)
	WarningStyle = lipgloss.NewStyle().Foreground(ColorYellow)
	ErrorFgStyle = lipgloss.NewStyle().Foreground(ColorRed)
	DimStyle     = lipgloss.NewStyle().Foreground(ColorDim)

	SelectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")).
			Background(ColorPrimary).
			Bold(true)

	HighlightStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")).
			Background(ColorYellow)
)
