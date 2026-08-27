package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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

	filter FilterBox
	wrap   bool

	width  int
	height int
}

func NewLogsView(ctx config.Context, clients *awsclient.ClientSet, taskARN, taskDefARN, containerName string) LogsView {
	vp := viewport.New(0, 0)
	vp.SetContent("Loading log configuration…")

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = ui.HealthyStyle

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
		windowLabel:   "all",
		windowSince:   time.Time{},
		filter:        NewFilterBox(),
	}
}

// rawEventsCap bounds the in-memory event buffer so a long-running follow
// session doesn't grow without bound.
const rawEventsCap = 5000

func (m *LogsView) ViewID() model.ViewID { return model.ViewLogs }

func (m *LogsView) KeyHints() []string {
	tailHint := "f:follow"
	if m.following {
		tailHint = "f:stop-follow"
	}
	wrapHint := "w:wrap"
	if m.wrap {
		wrapHint = "w:no-wrap"
	}
	return []string{tailHint, wrapHint, "/:search", "r:refresh", "esc:back", "q:quit", "?:help"}
}

func (m *LogsView) IsCapturingInput() bool { return m.filter.Active() }

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

// fetchRecentCmd loads the most recent events for the stream, replacing the
// in-memory buffer. Time filtering happens client-side in renderLogs.
func (m *LogsView) fetchRecentCmd() tea.Cmd {
	logGroup := m.logGroup
	logStream := m.logStream
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		events, err := m.clients.GetRecentLogs(ctx, logGroup, logStream, 1000, time.Time{})
		return model.LogEventsMsg{Events: events, Replace: true, Err: err}
	}
}

// tailLogsCmd polls for events strictly newer than the most recent one we've
// seen and appends them to the buffer.
func (m *LogsView) tailLogsCmd() tea.Cmd {
	var since time.Time
	if len(m.rawEvents) > 0 {
		since = m.rawEvents[len(m.rawEvents)-1].Timestamp.Add(1 * time.Millisecond)
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
		return m, m.fetchRecentCmd()

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
		if msg.Replace {
			m.rawEvents = msg.Events
		} else {
			m.rawEvents = append(m.rawEvents, msg.Events...)
		}
		if len(m.rawEvents) > rawEventsCap {
			m.rawEvents = m.rawEvents[len(m.rawEvents)-rawEventsCap:]
		}
		m.viewport.SetContent(renderLogs(m.rawEvents, m.filter.Value(), m.windowSince, m.wrap, m.viewport.Width))
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
		if handled, cmd := m.filter.HandleKey(msg); handled {
			m.viewport.SetContent(renderLogs(m.rawEvents, m.filter.Value(), m.windowSince, m.wrap, m.viewport.Width))
			return m, cmd
		}

		switch {
		case key.Matches(msg, model.GlobalKeys.Back):
			if m.filter.HandleBack() {
				m.viewport.SetContent(renderLogs(m.rawEvents, m.filter.Value(), m.windowSince, m.wrap, m.viewport.Width))
				return m, nil
			}
			return m, func() tea.Msg { return model.NavigatePopMsg{} }
		case key.Matches(msg, model.GlobalKeys.Refresh):
			if m.configLoaded {
				m.loading = true
				m.rawEvents = nil
				return m, tea.Batch(m.spinner.Tick, m.fetchRecentCmd())
			}
		case key.Matches(msg, model.GlobalKeys.Search):
			return m, m.filter.Start()
		case msg.String() == "f":
			m.following = !m.following
			if m.following && m.configLoaded {
				m.viewport.GotoBottom()
				return m, m.tailLogsCmd()
			}
			return m, nil
		case msg.String() == "w":
			m.wrap = !m.wrap
			m.viewport.SetContent(renderLogs(m.rawEvents, m.filter.Value(), m.windowSince, m.wrap, m.viewport.Width))
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

// applyWindow updates the visible time window and re-renders. No refetch — we
// filter the already-loaded events client-side, so switching windows is
// instant and predictable regardless of whether new events exist in the
// requested range.
func (m *LogsView) applyWindow(label string, since time.Time) tea.Cmd {
	m.windowLabel = label
	m.windowSince = since
	m.viewport.SetContent(renderLogs(m.rawEvents, m.filter.Value(), m.windowSince, m.wrap, m.viewport.Width))
	if m.following {
		m.viewport.GotoBottom()
	}
	return nil
}

func (m *LogsView) View() string {
	header := m.headerLine()
	if m.loading && len(m.rawEvents) == 0 {
		return header + "\n\n  " + m.spinner.View() + " Loading logs…"
	}
	body := m.viewport.View()
	windows := m.windowHints()
	if m.filter.Active() {
		return header + "\n" + body + "\n" + m.filter.View() + "\n" + windows
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
	if f := m.filter.Value(); f != "" {
		parts = append(parts, ui.DimStyle.Render(fmt.Sprintf("filter=%q", f)))
	}
	if m.following {
		parts = append(parts, ui.HealthyStyle.Render("[following]"))
	}
	if m.wrap {
		parts = append(parts, ui.DimStyle.Render("[wrap]"))
	}
	return strings.Join(parts, "  ")
}

func (m *LogsView) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.filter.SetWidth(w)
	// header (1) + body + windows-banner (1) + optional search (1) == h
	body := h - 2
	if m.filter.Active() {
		body -= 1
	}
	if body < 1 {
		body = 1
	}
	m.viewport.Width = w
	m.viewport.Height = body
}

func renderLogs(events []domain.LogEvent, filter string, since time.Time, wrap bool, width int) string {
	if len(events) == 0 {
		return ui.DimStyle.Render("No log events found")
	}
	lowerFilter := strings.ToLower(filter)
	var b strings.Builder
	inWindow := 0
	matched := 0
	for _, e := range events {
		if !since.IsZero() && e.Timestamp.Before(since) {
			continue
		}
		inWindow++
		if filter != "" && !strings.Contains(strings.ToLower(e.Message), lowerFilter) {
			continue
		}
		matched++
		ts := ui.DimStyle.Render(e.Timestamp.Format("15:04:05"))
		msg := e.Message
		if filter != "" {
			msg = highlightMatches(msg, filter)
		}
		line := ts + "  " + msg
		if wrap && width > 0 {
			line = lipgloss.NewStyle().Width(width).Render(line)
		}
		b.WriteString(line + "\n")
	}
	if inWindow == 0 {
		return ui.DimStyle.Render(fmt.Sprintf("No events in this window (%d cached — press 0 to see all)", len(events)))
	}
	if filter != "" && matched == 0 {
		return ui.DimStyle.Render(fmt.Sprintf("No matches for %q in this window (%d events in window)", filter, inWindow))
	}
	return b.String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n+1:]
}
