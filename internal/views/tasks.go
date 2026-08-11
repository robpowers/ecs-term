package views

import (
	"context"
	"fmt"
	"sort"
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
	// taskColStatusW must comfortably exceed the ANSI-inflated width of the
	// longest colored status text — see ui.StatusColorSafe's doc comment.
	taskColStatusW  = 24
	taskColPadding  = 2
	taskNumCols     = 4
	taskFixedWidths = taskColIDW + taskColStartedW + taskColStatusW + taskColPadding*taskNumCols
)

var taskSortOptions = []sortOption{
	{"I", "id"}, {"A", "age"}, {"S", "status"}, {"C", "containers"},
}

type TasksView struct {
	table       table.Model
	items       []domain.ECSTask
	visible     []domain.ECSTask
	loading     bool
	err         error
	ctx         config.Context
	clients     *awsclient.ClientSet
	spinner     spinner.Model
	serviceName string
	taskDefARN  string
	lastFetch   time.Time
	filter      FilterBox
	sortKey     string
	sortAsc     bool
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
		filter:      NewFilterBox(),
		sortAsc:     true,
	}
}

func tasksTableStyles() table.Styles {
	return table.Styles{
		Header:   lipgloss.NewStyle().Bold(true).Foreground(ui.ColorPrimary).Padding(0, 1),
		Cell:     lipgloss.NewStyle().Padding(0, 1),
		Selected: ui.SelectedStyle,
	}
}

func (m *TasksView) ViewID() model.ViewID { return model.ViewTasks }

func (m *TasksView) KeyHints() []string {
	return []string{"↑/k:up", "↓/j:down", "l:logs", "d:describe", "c:container", "s:shell", "y:yaml", "J:json", "/:filter", "esc:back", "r:refresh", "q:quit"}
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
		switch msg.Action {
		case model.ContainerActionShell:
			return m, execShellCmd(m.ctx, msg.TaskARN, msg.Name)
		case model.ContainerActionLogs:
			lv := NewLogsView(m.ctx, m.clients, msg.TaskARN, msg.TaskDefARN, msg.Name)
			return m, func() tea.Msg { return model.NavigatePushMsg{View: &lv} }
		}
		return m, nil

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
		m.refreshRows()
		return m, nil

	case tea.KeyMsg:
		if handled, cmd := m.filter.HandleKey(msg); handled {
			m.refreshRows()
			return m, cmd
		}

		switch {
		case key.Matches(msg, model.GlobalKeys.Back):
			if m.filter.HandleBack() {
				m.refreshRows()
				return m, nil
			}
			return m, func() tea.Msg { return model.NavigatePopMsg{} }
		case key.Matches(msg, model.GlobalKeys.Refresh):
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, m.fetchCmd())
		case key.Matches(msg, model.GlobalKeys.Search):
			return m, m.filter.Start()
		case key.Matches(msg, model.GlobalKeys.Enter), key.Matches(msg, model.GlobalKeys.Logs):
			cursor := m.table.Cursor()
			if cursor < 0 || cursor >= len(m.visible) {
				return m, nil
			}
			task := m.visible[cursor]
			return m, logsForTask(m.ctx, m.clients, task, m.taskDefARN)
		case key.Matches(msg, model.GlobalKeys.Describe):
			cursor := m.table.Cursor()
			if cursor < 0 || cursor >= len(m.visible) {
				return m, nil
			}
			task := m.visible[cursor]
			dv := NewTaskDescribeView(m.ctx, m.clients, task.TaskARN)
			return m, func() tea.Msg { return model.NavigatePushMsg{View: &dv} }
		case key.Matches(msg, model.GlobalKeys.Container):
			cursor := m.table.Cursor()
			if cursor < 0 || cursor >= len(m.visible) {
				return m, nil
			}
			dv := NewContainerDetailView(m.ctx, m.clients, m.taskDefARN)
			return m, func() tea.Msg { return model.NavigatePushMsg{View: &dv} }
		case key.Matches(msg, model.GlobalKeys.Shell):
			cursor := m.table.Cursor()
			if cursor < 0 || cursor >= len(m.visible) {
				return m, nil
			}
			task := m.visible[cursor]
			return m, shellForTask(m.ctx, task)
		case msg.String() == "y":
			cursor := m.table.Cursor()
			if cursor < 0 || cursor >= len(m.visible) {
				return m, nil
			}
			task := m.visible[cursor]
			rv := NewTaskDefRawView(m.ctx, m.clients, task.TaskDefARN, "yaml")
			return m, func() tea.Msg { return model.NavigatePushMsg{View: &rv} }
		case msg.String() == "J":
			cursor := m.table.Cursor()
			if cursor < 0 || cursor >= len(m.visible) {
				return m, nil
			}
			task := m.visible[cursor]
			rv := NewTaskDefRawView(m.ctx, m.clients, task.TaskDefARN, "json")
			return m, func() tea.Msg { return model.NavigatePushMsg{View: &rv} }
		}

		if sortKeyMatch(taskSortOptions, msg.String()) {
			if m.sortKey == msg.String() {
				m.sortAsc = !m.sortAsc
			} else {
				m.sortKey = msg.String()
				m.sortAsc = true
			}
			m.refreshRows()
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
	lines := []string{title, m.table.View()}
	if m.filter.Active() {
		lines = append(lines, m.filter.View())
	}
	lines = append(lines, renderSortBanner(taskSortOptions, m.sortKey, m.sortAsc))
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m *TasksView) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.filter.SetWidth(w)
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
	extra := 3
	if m.filter.Active() {
		extra++
	}
	th := h - extra
	if th < 1 {
		th = 1
	}
	m.table.SetHeight(th)
	m.table.SetWidth(w)
}

func (m *TasksView) refreshRows() {
	m.visible = filterTasks(m.items, m.filter.Value())
	sortTasksInPlace(m.visible, m.sortKey, m.sortAsc)
	m.table.SetRows(toTaskTableRows(m.visible))
}

func filterTasks(items []domain.ECSTask, query string) []domain.ECSTask {
	if query == "" {
		return append([]domain.ECSTask(nil), items...)
	}
	q := strings.ToLower(query)
	out := make([]domain.ECSTask, 0, len(items))
	for _, t := range items {
		if taskMatches(t, q) {
			out = append(out, t)
		}
	}
	return out
}

func taskMatches(t domain.ECSTask, lowerQuery string) bool {
	names := make([]string, 0, len(t.Containers))
	for _, c := range t.Containers {
		names = append(names, c.Name)
	}
	fields := []string{t.ShortID, t.LastStatus, strings.Join(names, ", ")}
	if t.StartedAt != nil {
		fields = append(fields, t.StartedAt.Local().Format("2006-01-02 15:04:05"))
	}
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f), lowerQuery) {
			return true
		}
	}
	return false
}

func sortTasksInPlace(items []domain.ECSTask, key string, asc bool) {
	var less func(i, j int) bool
	switch key {
	case "I":
		less = func(i, j int) bool { return items[i].ShortID < items[j].ShortID }
	case "A":
		less = func(i, j int) bool {
			a, b := items[i].StartedAt, items[j].StartedAt
			if a == nil {
				return b != nil
			}
			if b == nil {
				return false
			}
			return a.Before(*b)
		}
	case "S":
		less = func(i, j int) bool { return items[i].LastStatus < items[j].LastStatus }
	case "C":
		less = func(i, j int) bool { return len(items[i].Containers) < len(items[j].Containers) }
	default:
		return
	}
	sort.SliceStable(items, func(i, j int) bool {
		if asc {
			return less(i, j)
		}
		return less(j, i)
	})
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
			ui.StatusColorSafe(t.LastStatus).Render(t.LastStatus),
			strings.Join(names, ", "),
		}
	}
	return rows
}
