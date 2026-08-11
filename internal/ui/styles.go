package ui

import (
	"io"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

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

// tableRenderer is pinned to the ANSI (4-bit, 16-color) profile so styled
// text has a short, predictable escape-sequence length. bubbles/table
// (v1.0.0) truncates cell content with go-runewidth, which is not
// ANSI-aware — it counts every byte of a color escape sequence as a
// printable character. With the default renderer's auto-detected profile
// (often TrueColor, ~20+ escape bytes per styled cell) that miscount causes
// truncation to cut through the escape codes and corrupt the terminal's
// render state. Styles built from this renderer keep sequences short enough
// that the status column widths (see services.go/tasks.go/cluster_tasks.go)
// comfortably stay under bubbles' truncation threshold, so it never
// triggers. Never embed styles from the default renderer in table cell text.
var tableRenderer = func() *lipgloss.Renderer {
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(termenv.ANSI)
	return r
}()

var (
	TableHealthyStyle = tableRenderer.NewStyle().Foreground(ColorGreen)
	TableWarningStyle = tableRenderer.NewStyle().Foreground(ColorYellow)
	TableErrorStyle   = tableRenderer.NewStyle().Foreground(ColorRed)
	TableDimStyle     = tableRenderer.NewStyle().Foreground(ColorDim)
)
