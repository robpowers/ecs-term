package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	awsclient "github.com/robpowers/ecs-term/internal/aws"
	"github.com/robpowers/ecs-term/internal/config"
	"github.com/robpowers/ecs-term/internal/domain"
	"github.com/robpowers/ecs-term/internal/model"
	"github.com/robpowers/ecs-term/internal/ui"
)

type LogsView struct {
	viewport      viewport.Model
	rawEvents     []domain.LogEvent
	loading       bool
	configLoaded  bool
	following     bool
	err           error
	ctx           config.Context
	clients       *awsclient.ClientSet
	taskARN       string
	taskDefARN    string
	containerName string
	logGroup      string
	streamPrefix  string
	logStream     string
	spinner       spinner.Model

	windowLabel string
	windowSince time.Time

	searchMode  bool
	searchInput textinput.Model
	filter      string

	width  int
	height int
}

func NewLogsView(ctx config.Context, clients *awsclient.ClientSet, taskARN, taskDefARN, containerName string) LogsView {
	vp := viewport.New(0, 0)
	vp.SetContent("Loading log configuration…")

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = ui.HealthyStyle

	ti := textinput.New()
	ti.Placeholder = "filter…"
	ti.Prompt = "/ "
	ti.CharLimit = 200

	return LogsView{
		viewport:      vp,
		loading:       true,
		ctx:           ctx,
		clients:       clients,
		taskARN:       taskARN,
		taskDefARN:    taskDefARN,
		containerName: containerName,
		spinner:       sp,
		following:     true,
		windowLabel:   "now",
		windowSince:   time.Now(),
		searchInput:   ti,
	}
}

func (m *LogsView) ViewID() model.ViewID { return model.ViewLogs }

func (m *LogsView) KeyHints() []string {
	tailHint := "f:follow"
	if m.following {
		tailHint = "f:stop-follow"
	}
	return []string{tailHint, "/:search", "r:refresh", "esc:back", "q:quit"}
}

// windowHints returns the k9s-style "Since" row rendered under the log body.
// The currently active window is highlighted.
var windowHintOrder = []struct {
	key, label string
}{
	{"1", "1m"}, {"2", "5m"}, {"3", "15m"}, {"4", "30m"},
	{"5", "1h"}, {"6", "6h"}, {"7", "12h"}, {"8", "24h"}, {"0", "all"},
}

func (m *LogsView) windowHints() string {
	var parts []string
	for _, w := range windowHintOrder {
		text := w.key + ":" + w.label
		if m.windowLabel == w.label {
			parts = append(parts, ui.SelectedStyle.Render(" "+text+" "))
		} else {
			parts = append(parts, ui.KeyHintStyle.Render(w.key)+ui.KeyDescStyle.Render(":"+w.label))
		}
	}
	return ui.DimStyle.Render("Since ") + strings.Join(parts, "  ")
}

func (m *LogsView) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.fetchLogConfigCmd(),
	)
}

func (m *LogsView) fetchLogConfigCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		logGroup, prefix, err := m.clients.GetTaskLogConfig(ctx, m.taskDefARN, m.containerName)
		return model.LogConfigMsg{
			LogGroup:      logGroup,
			StreamPrefix:  prefix,
			TaskARN:       m.taskARN,
			ContainerName: m.containerName,
			Err:           err,
		}
	}
}

// fetchWindowCmd fetches log events starting at `since`. If `since` is zero,
// the entire stream is returned (limit-bounded).
func (m *LogsView) fetchWindowCmd(since time.Time) tea.Cmd {
	logGroup := m.logGroup
	logStream := m.logStream
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		events, err := m.clients.GetRecentLogs(ctx, logGroup, logStream, 1000, since)
		return model.LogEventsMsg{Events: events, Err: err}
	}
}

func (m *LogsView) tailLogsCmd() tea.Cmd {
	var since time.Time
	if len(m.rawEvents) > 0 {
		since = m.rawEvents[len(m.rawEvents)-1].Timestamp
	} else {
		since = m.windowSince
	}
	logGroup := m.logGroup
	logStream := m.logStream
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		events, err := m.clients.GetRecentLogs(ctx, logGroup, logStream, 200, since)
		return model.LogEventsMsg{Events: events, Err: err}
	}
}

func (m *LogsView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case model.LogConfigMsg:
		if msg.Err != nil {
			m.loading = false
			m.err = msg.Err
			m.viewport.SetContent(ui.ErrorFgStyle.Render("Error: " + msg.Err.Error()))
			return m, nil
		}
		m.logGroup = msg.LogGroup
		m.streamPrefix = msg.StreamPrefix
		m.logStream = awsclient.BuildLogStreamName(msg.StreamPrefix, msg.ContainerName, msg.TaskARN)
		m.configLoaded = true
		return m, m.fetchWindowCmd(m.windowSince)

	case model.LogEventsMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
			if m.viewport.Height > 0 {
				m.viewport.SetContent(ui.ErrorFgStyle.Render("Error: " + msg.Err.Error() + "\n\nPress r to retry"))
			}
			return m, nil
		}
		m.err = nil
		if m.following {
			m.rawEvents = append(m.rawEvents, msg.Events...)
		} else {
			m.rawEvents = msg.Events
		}
		m.viewport.SetContent(renderLogs(m.rawEvents, m.filter))
		if m.following {
			m.viewport.GotoBottom()
		}

		var cmd tea.Cmd
		if m.following {
			cmd = tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
				return model.RefreshTickMsg{T: t}
			})
		}
		return m, cmd

	case model.RefreshTickMsg:
		if m.following && m.configLoaded {
			return m, m.tailLogsCmd()
		}
		return m, nil

	case tea.KeyMsg:
		if m.searchMode {
			switch msg.String() {
			case "enter":
				m.filter = m.searchInput.Value()
				m.searchMode = false
				m.searchInput.Blur()
				m.viewport.SetContent(renderLogs(m.rawEvents, m.filter))
				return m, nil
			case "esc":
				m.filter = ""
				m.searchInput.SetValue("")
				m.searchMode = false
				m.searchInput.Blur()
				m.viewport.SetContent(renderLogs(m.rawEvents, m.filter))
				return m, nil
			}
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			return m, cmd
		}

		switch {
		case key.Matches(msg, model.GlobalKeys.Back):
			return m, func() tea.Msg { return model.NavigatePopMsg{} }
		case key.Matches(msg, model.GlobalKeys.Refresh):
			if m.configLoaded {
				m.loading = true
				m.rawEvents = nil
				return m, tea.Batch(m.spinner.Tick, m.fetchWindowCmd(m.windowSince))
			}
		case key.Matches(msg, model.GlobalKeys.Search):
			m.searchMode = true
			m.searchInput.SetValue(m.filter)
			m.searchInput.Focus()
			return m, textinput.Blink
		case msg.String() == "f":
			m.following = !m.following
			if m.following && m.configLoaded {
				m.viewport.GotoBottom()
				return m, m.tailLogsCmd()
			}
			return m, nil
		}

		if d, ok := windowForKey(msg.String()); ok {
			return m, m.applyWindow(d.label, d.since)
		}

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

type windowChoice struct {
	label string
	since time.Time
}

// windowForKey maps a digit key to a lookback window. "0" means "all".
func windowForKey(k string) (windowChoice, bool) {
	now := time.Now()
	switch k {
	case "1":
		return windowChoice{"1m", now.Add(-1 * time.Minute)}, true
	case "2":
		return windowChoice{"5m", now.Add(-5 * time.Minute)}, true
	case "3":
		return windowChoice{"15m", now.Add(-15 * time.Minute)}, true
	case "4":
		return windowChoice{"30m", now.Add(-30 * time.Minute)}, true
	case "5":
		return windowChoice{"1h", now.Add(-1 * time.Hour)}, true
	case "6":
		return windowChoice{"6h", now.Add(-6 * time.Hour)}, true
	case "7":
		return windowChoice{"12h", now.Add(-12 * time.Hour)}, true
	case "8":
		return windowChoice{"24h", now.Add(-24 * time.Hour)}, true
	case "0":
		return windowChoice{"all", time.Time{}}, true
	}
	return windowChoice{}, false
}

func (m *LogsView) applyWindow(label string, since time.Time) tea.Cmd {
	m.windowLabel = label
	m.windowSince = since
	m.rawEvents = nil
	if !m.configLoaded {
		return nil
	}
	m.loading = true
	return tea.Batch(m.spinner.Tick, m.fetchWindowCmd(since))
}

func (m *LogsView) View() string {
	header := m.headerLine()
	if m.loading && len(m.rawEvents) == 0 {
		return header + "\n\n  " + m.spinner.View() + " Loading logs…"
	}
	body := m.viewport.View()
	windows := m.windowHints()
	if m.searchMode {
		return header + "\n" + body + "\n" + m.searchInput.View() + "\n" + windows
	}
	return header + "\n" + body + "\n" + windows
}

func (m *LogsView) headerLine() string {
	title := ui.TitleStyle.Render(
		fmt.Sprintf("Logs — %s / %s", m.containerName, truncate(m.logStream, 60)),
	)
	parts := []string{title}
	if m.windowLabel != "" {
		parts = append(parts, ui.DimStyle.Render("window=["+m.windowLabel+"]"))
	}
	if m.filter != "" {
		parts = append(parts, ui.DimStyle.Render(fmt.Sprintf("filter=%q", m.filter)))
	}
	if m.following {
		parts = append(parts, ui.HealthyStyle.Render("[following]"))
	}
	return strings.Join(parts, "  ")
}

func (m *LogsView) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.searchInput.Width = w - 4
	// header (1) + body + windows-banner (1) + optional search (1) == h
	body := h - 2
	if m.searchMode {
		body -= 1
	}
	if body < 1 {
		body = 1
	}
	m.viewport.Width = w
	m.viewport.Height = body
}

func renderLogs(events []domain.LogEvent, filter string) string {
	if len(events) == 0 {
		return ui.DimStyle.Render("No log events found")
	}
	lowerFilter := strings.ToLower(filter)
	var b strings.Builder
	matched := 0
	for _, e := range events {
		if filter != "" && !strings.Contains(strings.ToLower(e.Message), lowerFilter) {
			continue
		}
		matched++
		ts := ui.DimStyle.Render(e.Timestamp.Format("15:04:05"))
		msg := e.Message
		if filter != "" {
			msg = highlightMatches(msg, filter)
		}
		b.WriteString(ts + "  " + msg + "\n")
	}
	if filter != "" && matched == 0 {
		return ui.DimStyle.Render(fmt.Sprintf("No matches for %q (%d events fetched)", filter, len(events)))
	}
	return b.String()
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

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n+1:]
}
