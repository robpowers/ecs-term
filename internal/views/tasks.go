package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	awsclient "github.com/robpowers/ecs-term/internal/aws"
	"github.com/robpowers/ecs-term/internal/config"
	"github.com/robpowers/ecs-term/internal/domain"
	"github.com/robpowers/ecs-term/internal/model"
	"github.com/robpowers/ecs-term/internal/ui"
)

const (
	taskColIDW      = 12
	taskColStartedW = 19
	taskColStatusW  = 10
	taskColPadding  = 2
	taskNumCols     = 4
	taskFixedWidths = taskColIDW + taskColStartedW + taskColStatusW + taskColPadding*taskNumCols
)

type TasksView struct {
	table       table.Model
	items       []domain.ECSTask
	loading     bool
	err         error
	ctx         config.Context
	clients     *awsclient.ClientSet
	spinner     spinner.Model
	serviceName string
	taskDefARN  string
	lastFetch   time.Time
	width       int
	height      int
}

func NewTasksView(ctx config.Context, clients *awsclient.ClientSet, serviceName, taskDefARN string) TasksView {
	t := table.New(
		table.WithFocused(true),
		table.WithStyles(tasksTableStyles()),
	)
	km := table.DefaultKeyMap()
	km.HalfPageDown.SetKeys()
	km.HalfPageUp.SetKeys()
	t.KeyMap = km

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = ui.HealthyStyle

	return TasksView{
		table:       t,
		loading:     true,
		ctx:         ctx,
		clients:     clients,
		spinner:     sp,
		serviceName: serviceName,
		taskDefARN:  taskDefARN,
	}
}

func tasksTableStyles() table.Styles {
	return table.Styles{
		Header:   lipgloss.NewStyle().Bold(true).Foreground(ui.ColorPrimary).Padding(0, 1),
		Cell:     lipgloss.NewStyle().Padding(0, 1),
		Selected: ui.SelectedStyle.Padding(0, 1),
	}
}

func (m *TasksView) ViewID() model.ViewID { return model.ViewTasks }

func (m *TasksView) KeyHints() []string {
	return []string{"↑/k:up", "↓/j:down", "enter:logs", "d:describe", "c:container", "s:shell", "esc:back", "r:refresh", "q:quit"}
}

func (m *TasksView) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.fetchCmd(),
		tickEvery(time.Duration(m.ctx.EffectiveRefreshInterval())*time.Second),
	)
}

func (m *TasksView) fetchCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		tasks, err := m.clients.ListTasks(ctx, m.ctx.Cluster, awsclient.ListTasksOpts{ServiceName: m.serviceName})
		return model.TasksLoadedMsg{Tasks: tasks, Err: err}
	}
}

func (m *TasksView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case model.ContainerPickedMsg:
		return m, execShellCmd(m.ctx, msg.TaskARN, msg.Name)

	case model.RefreshTickMsg:
		return m, tea.Batch(
			tickEvery(time.Duration(m.ctx.EffectiveRefreshInterval())*time.Second),
			m.fetchCmd(),
		)

	case model.TasksLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.err = nil
		m.items = msg.Tasks
		m.lastFetch = time.Now()
		m.table.SetRows(toTaskTableRows(msg.Tasks))
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, model.GlobalKeys.Back):
			return m, func() tea.Msg { return model.NavigatePopMsg{} }
		case key.Matches(msg, model.GlobalKeys.Refresh):
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, m.fetchCmd())
		case key.Matches(msg, model.GlobalKeys.Enter):
			cursor := m.table.Cursor()
			if cursor < 0 || cursor >= len(m.items) {
				return m, nil
			}
			task := m.items[cursor]
			if len(task.Containers) == 0 {
				return m, nil
			}
			lv := NewLogsView(m.ctx, m.clients, task.TaskARN, m.taskDefARN, task.Containers[0].Name)
			return m, func() tea.Msg { return model.NavigatePushMsg{View: &lv} }
		case key.Matches(msg, model.GlobalKeys.Describe):
			cursor := m.table.Cursor()
			if cursor < 0 || cursor >= len(m.items) {
				return m, nil
			}
			task := m.items[cursor]
			dv := NewTaskDescribeView(m.ctx, m.clients, task.TaskARN)
			return m, func() tea.Msg { return model.NavigatePushMsg{View: &dv} }
		case key.Matches(msg, model.GlobalKeys.Container):
			cursor := m.table.Cursor()
			if cursor < 0 || cursor >= len(m.items) {
				return m, nil
			}
			dv := NewContainerDetailView(m.ctx, m.clients, m.taskDefARN)
			return m, func() tea.Msg { return model.NavigatePushMsg{View: &dv} }
		case key.Matches(msg, model.GlobalKeys.Shell):
			cursor := m.table.Cursor()
			if cursor < 0 || cursor >= len(m.items) {
				return m, nil
			}
			task := m.items[cursor]
			return m, shellForTask(m.ctx, task)
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
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *TasksView) View() string {
	if m.loading && len(m.items) == 0 {
		return fmt.Sprintf("\n  %s Loading tasks…", m.spinner.View())
	}
	if m.err != nil && len(m.items) == 0 {
		return ui.ErrorFgStyle.Render("\n  Error: "+m.err.Error()) +
			ui.DimStyle.Render("\n  Press r to retry")
	}
	if len(m.items) == 0 {
		return ui.DimStyle.Render("\n  No running tasks")
	}
	title := ui.TitleStyle.Width(m.width).Align(lipgloss.Center).Render("Tasks")
	return lipgloss.JoinVertical(lipgloss.Left, title, m.table.View())
}

func (m *TasksView) SetSize(w, h int) {
	m.width = w
	m.height = h
	containersW := w - taskFixedWidths
	if containersW < 10 {
		containersW = 10
	}
	m.table.SetColumns([]table.Column{
		{Title: "Task ID", Width: taskColIDW},
		{Title: "Started", Width: taskColStartedW},
		{Title: "Status", Width: taskColStatusW},
		{Title: "Containers", Width: containersW},
	})
	m.table.SetHeight(h - 2)
	m.table.SetWidth(w)
}

func toTaskTableRows(tasks []domain.ECSTask) []table.Row {
	rows := make([]table.Row, len(tasks))
	for i, t := range tasks {
		started := "—"
		if t.StartedAt != nil {
			started = t.StartedAt.Local().Format("2006-01-02 15:04:05")
		}
		names := make([]string, 0, len(t.Containers))
		for _, c := range t.Containers {
			names = append(names, c.Name)
		}
		rows[i] = table.Row{
			t.ShortID,
			started,
			t.LastStatus,
			strings.Join(names, ", "),
		}
	}
	return rows
}
