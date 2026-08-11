package views

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	awsclient "github.com/robpowers/ecs-term/internal/aws"
	"github.com/robpowers/ecs-term/internal/config"
	"github.com/robpowers/ecs-term/internal/model"
	"github.com/robpowers/ecs-term/internal/ui"
)

// TaskDefRawView shows the raw task definition as YAML or JSON text. Both
// formats are fetched together, so switching format (y/J) is instant and
// doesn't refetch.
type TaskDefRawView struct {
	viewport   viewport.Model
	jsonText   string
	yamlText   string
	format     string // "yaml" or "json"
	loaded     bool
	loading    bool
	err        error
	ctx        config.Context
	clients    *awsclient.ClientSet
	taskDefARN string
	spinner    spinner.Model
	filter     FilterBox
}

func NewTaskDefRawView(ctx config.Context, clients *awsclient.ClientSet, taskDefARN, format string) TaskDefRawView {
	vp := viewport.New(0, 0)
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = ui.HealthyStyle

	if format != "yaml" && format != "json" {
		format = "yaml"
	}

	return TaskDefRawView{
		viewport:   vp,
		loading:    true,
		ctx:        ctx,
		clients:    clients,
		taskDefARN: taskDefARN,
		format:     format,
		spinner:    sp,
		filter:     NewFilterBox(),
	}
}

func (m *TaskDefRawView) ViewID() model.ViewID { return model.ViewTaskDefRaw }

func (m *TaskDefRawView) KeyHints() []string {
	return []string{"↑/k:up", "↓/j:down", "y:yaml", "J:json", "/:search", "r:refresh", "esc:back", "q:quit"}
}

func (m *TaskDefRawView) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.fetchCmd())
}

func (m *TaskDefRawView) fetchCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		jsonText, yamlText, err := m.clients.GetTaskDefinitionRaw(ctx, m.taskDefARN)
		return model.TaskDefRawMsg{JSON: jsonText, YAML: yamlText, Err: err}
	}
}

func (m *TaskDefRawView) renderContent() string {
	text := m.yamlText
	if m.format == "json" {
		text = m.jsonText
	}
	if f := m.filter.Value(); f != "" {
		text = highlightMatches(text, f)
	}
	return text
}

func (m *TaskDefRawView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case model.TaskDefRawMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
			m.viewport.SetContent(ui.ErrorFgStyle.Render("Error: " + msg.Err.Error()))
			return m, nil
		}
		m.err = nil
		m.jsonText = msg.JSON
		m.yamlText = msg.YAML
		m.loaded = true
		m.viewport.SetContent(m.renderContent())
		return m, nil

	case tea.KeyMsg:
		if handled, cmd := m.filter.HandleKey(msg); handled {
			m.viewport.SetContent(m.renderContent())
			return m, cmd
		}

		switch {
		case key.Matches(msg, model.GlobalKeys.Back):
			if m.filter.HandleBack() {
				m.viewport.SetContent(m.renderContent())
				return m, nil
			}
			return m, func() tea.Msg { return model.NavigatePopMsg{} }
		case key.Matches(msg, model.GlobalKeys.Refresh):
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, m.fetchCmd())
		case key.Matches(msg, model.GlobalKeys.Search):
			return m, m.filter.Start()
		case msg.String() == "y":
			m.format = "yaml"
			m.viewport.SetContent(m.renderContent())
			return m, nil
		case msg.String() == "J":
			m.format = "json"
			m.viewport.SetContent(m.renderContent())
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

func (m *TaskDefRawView) View() string {
	title := ui.TitleStyle.Render("Task Definition (" + m.format + ") — " + shortARN(m.taskDefARN))
	if m.loading && !m.loaded {
		return title + "\n\n  " + m.spinner.View() + " Loading task definition…"
	}
	if m.filter.Active() {
		return title + "\n" + m.viewport.View() + "\n" + m.filter.View()
	}
	return title + "\n" + m.viewport.View()
}

func (m *TaskDefRawView) SetSize(w, h int) {
	m.filter.SetWidth(w)
	extra := 2
	if m.filter.Active() {
		extra++
	}
	m.viewport.Width = w
	m.viewport.Height = h - extra
	if m.viewport.Height < 1 {
		m.viewport.Height = 1
	}
}
