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

	awsclient "github.com/robpowers/ecs-term/internal/aws"
	"github.com/robpowers/ecs-term/internal/config"
	"github.com/robpowers/ecs-term/internal/domain"
	"github.com/robpowers/ecs-term/internal/model"
	"github.com/robpowers/ecs-term/internal/ui"
)

type serviceItem struct{ svc domain.ECSService }

func (i serviceItem) Title() string {
	icon := "●"
	style := ui.HealthyStyle
	if !i.svc.IsHealthy {
		style = ui.WarningStyle
	}
	if strings.ToUpper(i.svc.Status) != "ACTIVE" {
		style = ui.ErrorFgStyle
	}
	return style.Render(icon) + " " + i.svc.Name
}

func (i serviceItem) Description() string {
	return fmt.Sprintf("status: %-10s  desired: %d  running: %d  pending: %d",
		i.svc.Status, i.svc.DesiredCount, i.svc.RunningCount, i.svc.PendingCount)
}

func (i serviceItem) FilterValue() string { return i.svc.Name }

type ServicesView struct {
	list      list.Model
	items     []domain.ECSService
	loading   bool
	err       error
	ctx       config.Context
	clients   *awsclient.ClientSet
	spinner   spinner.Model
	lastFetch time.Time
}

func NewServicesView(ctx config.Context, clients *awsclient.ClientSet) ServicesView {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = ui.SelectedStyle
	delegate.Styles.SelectedDesc = ui.SelectedStyle.Foreground(ui.ColorSecondary)

	l := list.New(nil, delegate, 0, 0)
	l.Title = fmt.Sprintf("Services — %s", ctx.Cluster)
	l.Styles.Title = ui.TitleStyle
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.KeyMap.Quit.SetKeys()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = ui.HealthyStyle

	return ServicesView{
		list:    l,
		loading: true,
		ctx:     ctx,
		clients: clients,
		spinner: sp,
	}
}

func (m *ServicesView) ViewID() model.ViewID { return model.ViewServices }

func (m *ServicesView) KeyHints() []string {
	return []string{"↑/k:up", "↓/j:down", "enter:tasks", "esc:back", "r:refresh", "q:quit"}
}

func (m *ServicesView) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.fetchCmd(),
		tickEvery(time.Duration(m.ctx.EffectiveRefreshInterval())*time.Second),
	)
}

func (m *ServicesView) fetchCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		services, err := m.clients.ListServices(ctx, m.ctx.Cluster)
		return model.ServicesLoadedMsg{Services: services, Err: err}
	}
}

func (m *ServicesView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case model.RefreshTickMsg:
		return m, tea.Batch(
			tickEvery(time.Duration(m.ctx.EffectiveRefreshInterval())*time.Second),
			m.fetchCmd(),
		)

	case model.ServicesLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.err = nil
		m.items = msg.Services
		m.lastFetch = time.Now()
		listItems := make([]list.Item, len(msg.Services))
		for i, s := range msg.Services {
			listItems[i] = serviceItem{s}
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
				svc := item.(serviceItem).svc
				tv := NewTasksView(m.ctx, m.clients, svc.Name, svc.TaskDefARN)
				return m, func() tea.Msg {
					return model.NavigatePushMsg{View: &tv}
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

func (m *ServicesView) View() string {
	if m.loading && len(m.items) == 0 {
		return fmt.Sprintf("\n  %s Loading services…", m.spinner.View())
	}
	if m.err != nil && len(m.items) == 0 {
		return ui.ErrorFgStyle.Render("\n  Error: "+m.err.Error()) +
			ui.DimStyle.Render("\n  Press r to retry")
	}
	return m.list.View()
}

func (m *ServicesView) SetSize(w, h int) {
	m.list.SetSize(w, h)
}

func tickEvery(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return model.RefreshTickMsg{T: t}
	})
}

