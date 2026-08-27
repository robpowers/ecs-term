package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type HelpOverlay struct {
	Visible bool
}

func (h *HelpOverlay) Toggle() { h.Visible = !h.Visible }
func (h *HelpOverlay) Hide()   { h.Visible = false }

// Render draws a centered popup listing the current view's KeyHints, one per
// line, or passes bgView through unchanged when not visible. hints use the
// same "key:description" format as StatusBar.Footer's buildHints.
func (h *HelpOverlay) Render(bgView string, hints []string, width, height int) string {
	if !h.Visible {
		return bgView
	}

	type keyDesc struct{ key, desc string }
	var rows []keyDesc
	maxKeyWidth := 0
	for _, hint := range hints {
		parts := strings.SplitN(hint, ":", 2)
		if len(parts) != 2 {
			continue
		}
		rows = append(rows, keyDesc{parts[0], parts[1]})
		if w := lipgloss.Width(parts[0]); w > maxKeyWidth {
			maxKeyWidth = w
		}
	}

	lines := []string{TitleStyle.Render("Keyboard Shortcuts"), ""}
	for _, r := range rows {
		pad := strings.Repeat(" ", maxKeyWidth-lipgloss.Width(r.key))
		lines = append(lines, KeyHintStyle.Render(r.key+pad)+"  "+KeyDescStyle.Render(r.desc))
	}
	lines = append(lines, "", DimStyle.Render("press ? or esc to close"))

	boxWidth := min(width-8, 50)
	box := HelpStyle.Width(boxWidth).Render(strings.Join(lines, "\n"))
	return lipgloss.Place(width, height,
		lipgloss.Center, lipgloss.Center,
		box,
		lipgloss.WithWhitespaceChars(" "),
	)
}
