package views

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/robpowers/ecs-term/internal/model"
	"github.com/robpowers/ecs-term/internal/ui"
)

func tickEvery(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return model.RefreshTickMsg{T: t}
	})
}

// sortOption describes one column a list view can be sorted by, keyed off a
// single Shift+letter keypress (reported by bubbletea as the bare uppercase
// rune, so it can't collide with the lowercase action keys already bound on
// the same page).
type sortOption struct {
	key, label string
}

// renderSortBanner renders a k9s-style "Sort: X:field  Y:field" hint line,
// highlighting the active field and its direction. Meant to be appended as
// the last line of a list view's content, mirroring logs.go's windowHints().
func renderSortBanner(options []sortOption, activeKey string, asc bool) string {
	var parts []string
	for _, o := range options {
		text := o.key + ":" + o.label
		if o.key == activeKey {
			dir := "▲"
			if !asc {
				dir = "▼"
			}
			parts = append(parts, ui.SelectedStyle.Render(" "+text+dir+" "))
		} else {
			parts = append(parts, ui.KeyHintStyle.Render(o.key)+ui.KeyDescStyle.Render(":"+o.label))
		}
	}
	return ui.DimStyle.Render("Sort ") + strings.Join(parts, "  ")
}

// sortKeyMatch reports whether s is one of the given sort option keys.
func sortKeyMatch(options []sortOption, s string) bool {
	for _, o := range options {
		if o.key == s {
			return true
		}
	}
	return false
}
