package views

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/robpowers/ecs-term/internal/ui"
)

// FilterBox is a small, reusable "/"-to-search text input with k9s-style
// two-stage Esc: the first Esc clears an active or committed filter, the
// second (once the filter is already empty) is left unhandled so the caller
// falls through to its normal Back/pop behavior.
type FilterBox struct {
	active bool
	input  textinput.Model
	value  string
}

func NewFilterBox() FilterBox {
	ti := textinput.New()
	ti.Placeholder = "filter…"
	ti.Prompt = "/ "
	ti.CharLimit = 200
	return FilterBox{input: ti}
}

func (f *FilterBox) Active() bool  { return f.active }
func (f *FilterBox) Value() string { return f.value }
func (f *FilterBox) View() string  { return f.input.View() }

func (f *FilterBox) SetWidth(w int) {
	if w < 4 {
		w = 4
	}
	f.input.Width = w - 4
}

// Start enters edit mode, seeding the input with the currently committed value.
func (f *FilterBox) Start() tea.Cmd {
	f.active = true
	f.input.SetValue(f.value)
	f.input.Focus()
	return textinput.Blink
}

// HandleKey processes a key while the box is active (being typed into).
// Returns handled=false if the box isn't active, so callers can fall through
// to their normal key switch.
func (f *FilterBox) HandleKey(msg tea.KeyMsg) (handled bool, cmd tea.Cmd) {
	if !f.active {
		return false, nil
	}
	switch msg.String() {
	case "enter":
		f.value = f.input.Value()
		f.active = false
		f.input.Blur()
		return true, nil
	case "esc":
		f.value = ""
		f.input.SetValue("")
		f.active = false
		f.input.Blur()
		return true, nil
	}
	var c tea.Cmd
	f.input, c = f.input.Update(msg)
	return true, c
}

// HandleBack implements the two-stage Esc behavior for when the box is not
// currently being edited. If a filter is committed, it clears it and returns
// true (caller should not pop). If there's nothing to clear, it returns false
// so the caller proceeds with its normal Back handling.
func (f *FilterBox) HandleBack() bool {
	if f.value == "" {
		return false
	}
	f.value = ""
	return true
}

// highlightMatches wraps all case-insensitive occurrences of `pat` in `s` with
// the HighlightStyle. Returns the string with ANSI codes embedded.
func highlightMatches(s, pat string) string {
	if pat == "" {
		return s
	}
	lowS := strings.ToLower(s)
	lowP := strings.ToLower(pat)
	var b strings.Builder
	i := 0
	for {
		j := strings.Index(lowS[i:], lowP)
		if j < 0 {
			b.WriteString(s[i:])
			return b.String()
		}
		b.WriteString(s[i : i+j])
		b.WriteString(ui.HighlightStyle.Render(s[i+j : i+j+len(pat)]))
		i += j + len(pat)
	}
}
