package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	awsclient "github.com/robpowers/ecs-term/internal/aws"
	"github.com/robpowers/ecs-term/internal/config"
	"github.com/robpowers/ecs-term/internal/domain"
	"github.com/robpowers/ecs-term/internal/model"
	"github.com/robpowers/ecs-term/internal/ui"
)

type taskItem struct{ task domain.ECSTask }

func (i taskItem) Title() string {
	style := ui.StatusColor(i.task.LastStatus)
	age := ""
	if i.task.StartedAt != nil {
		d := time.Since(*i.task.StartedAt).Round(time.Second)
		age = fmt.Sprintf("  age: %s", d)
	}
	return style.Render("●") + " " + i.task.ShortID + age
}

func (i taskItem) Description() string {
	containers := make([]string, 0, len(i.task.Containers))
	for _, c := range i.task.Containers {
		containers = append(containers, c.Name+":"+c.Status)
	}
	return fmt.Sprintf("status: %-12s  containers: %s", i.task.LastStatus, strings.Join(containers, ", "))
}

func (i taskItem) FilterValue() string { return i.task.ShortID }

type TasksView struct {
	list        list.Model
	items       []domain.ECSTask
	loading     bool
	err         error
	ctx         config.Context
	clients     *awsclient.ClientSet
	spinner     spinner.Model
	serviceName string
	taskDefARN  string
	lastFetch   time.Time
}

func NewTasksView(ctx config.Context, clients *awsclient.ClientSet, serviceName, taskDefARN string) TasksView {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = ui.SelectedStyle
	delegate.Styles.SelectedDesc = ui.SelectedStyle.Foreground(lipgloss.Color("#333333"))

	l := list.New(nil, delegate, 0, 0)
	l.Title = "Tasks"
	l.Styles.Title = ui.TitleStyle
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.KeyMap.Quit.SetKeys()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = ui.HealthyStyle

	return TasksView{
		list:        l,
		loading:     true,
		ctx:         ctx,
		clients:     clients,
		spinner:     sp,
		serviceName: serviceName,
		taskDefARN:  taskDefARN,
	}
}

func (m *TasksView) ViewID() model.ViewID { return model.ViewTasks }

func (m *TasksView) KeyHints() []string {
	return []string{"↑/k:up", "↓/j:down", "enter:logs", "d:details", "esc:back", "r:refresh", "q:quit"}
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
		tasks, err := m.clients.ListTasks(ctx, m.ctx.Cluster, m.serviceName)
		return model.TasksLoadedMsg{Tasks: tasks, Err: err}
	}
}

func (m *TasksView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
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
		listItems := make([]list.Item, len(msg.Tasks))
		for i, t := range msg.Tasks {
			listItems[i] = taskItem{t}
		}
		m.list.SetItems(listItems)
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, model.GlobalKeys.Back):
			return m, func() tea.Msg { return model.NavigatePopMsg{} }
		case key.Matches(msg, model.GlobalKeys.Refresh):
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, m.fetchCmd())
		case key.Matches(msg, model.GlobalKeys.Enter):
			if item := m.list.SelectedItem(); item != nil {
				task := item.(taskItem).task
				if len(task.Containers) == 0 {
					return m, nil
				}
				// Default to first container for logs
				container := task.Containers[0]
				lv := NewLogsView(m.ctx, m.clients, task.TaskARN, m.taskDefARN, container.Name)
				return m, func() tea.Msg {
					return model.NavigatePushMsg{View: &lv}
				}
			}
		case key.Matches(msg, model.GlobalKeys.Detail):
			if item := m.list.SelectedItem(); item != nil {
				dv := NewContainerDetailView(m.ctx, m.clients, m.taskDefARN)
				return m, func() tea.Msg {
					return model.NavigatePushMsg{View: &dv}
				}
			}
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
	m.list, cmd = m.list.Update(msg)
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
	return m.list.View()
}

func (m *TasksView) SetSize(w, h int) {
	m.list.SetSize(w, h)
}
