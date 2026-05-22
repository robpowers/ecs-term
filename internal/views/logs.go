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

	awsclient "github.com/robpowers/ecs-term/internal/aws"
	"github.com/robpowers/ecs-term/internal/config"
	"github.com/robpowers/ecs-term/internal/domain"
	"github.com/robpowers/ecs-term/internal/model"
	"github.com/robpowers/ecs-term/internal/ui"
)

type LogsView struct {
	viewport      viewport.Model
	events        []domain.LogEvent
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
	}
}

func (m *LogsView) ViewID() model.ViewID { return model.ViewLogs }

func (m *LogsView) KeyHints() []string {
	tailHint := "f:follow"
	if m.following {
		tailHint = "f:stop-follow"
	}
	return []string{"↑/k:up", "↓/j:down", tailHint, "r:refresh", "esc:back", "q:quit"}
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

func (m *LogsView) fetchLogsCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		events, err := m.clients.GetRecentLogs(ctx, m.logGroup, m.logStream, 500, time.Time{})
		return model.LogEventsMsg{Events: events, Err: err}
	}
}

func (m *LogsView) tailLogsCmd() tea.Cmd {
	var since time.Time
	if len(m.events) > 0 {
		since = m.events[len(m.events)-1].Timestamp
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		events, err := m.clients.GetRecentLogs(ctx, m.logGroup, m.logStream, 100, since)
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
		return m, m.fetchLogsCmd()

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
			// append new events
			m.events = append(m.events, msg.Events...)
		} else {
			m.events = msg.Events
		}
		m.viewport.SetContent(renderLogs(m.events))
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
		switch {
		case key.Matches(msg, model.GlobalKeys.Back):
			return m, func() tea.Msg { return model.NavigatePopMsg{} }
		case key.Matches(msg, model.GlobalKeys.Refresh):
			if m.configLoaded {
				m.loading = true
				return m, tea.Batch(m.spinner.Tick, m.fetchLogsCmd())
			}
		case msg.String() == "f":
			m.following = !m.following
			if m.following && m.configLoaded {
				m.viewport.GotoBottom()
				return m, m.tailLogsCmd()
			}
			return m, nil
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

func (m *LogsView) View() string {
	title := ui.TitleStyle.Render(
		fmt.Sprintf("Logs — %s / %s", m.containerName, truncate(m.logStream, 60)),
	)
	if m.loading && len(m.events) == 0 {
		return title + "\n\n  " + m.spinner.View() + " Loading logs…"
	}
	following := ""
	if m.following {
		following = " " + ui.HealthyStyle.Render("[following]")
	}
	header := title + following + "\n"
	return header + m.viewport.View()
}

func (m *LogsView) SetSize(w, h int) {
	// subtract 2 lines for title header
	m.viewport.Width = w
	m.viewport.Height = h - 2
	if m.viewport.Height < 1 {
		m.viewport.Height = 1
	}
}

func renderLogs(events []domain.LogEvent) string {
	if len(events) == 0 {
		return ui.DimStyle.Render("No log events found")
	}
	var b strings.Builder
	for _, e := range events {
		ts := ui.DimStyle.Render(e.Timestamp.Format("15:04:05"))
		b.WriteString(ts + "  " + e.Message + "\n")
	}
	return b.String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n+1:]
}
