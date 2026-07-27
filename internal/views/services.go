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

// Fixed widths for the non-name columns (content width, excluding cell padding).
// Each column adds 2 for cell padding (Padding(0,1) = 1 left + 1 right).
const (
	colStatusW   = 10
	colCountW    = 8
	colDeployedW = 19
	colPadding   = 2
	numCols      = 6
	fixedWidths  = colStatusW + colCountW*3 + colDeployedW + colPadding*numCols
)

type ServicesView struct {
	table     table.Model
	items     []domain.ECSService
	loading   bool
	err       error
	ctx       config.Context
	clients   *awsclient.ClientSet
	spinner   spinner.Model
	lastFetch time.Time
	width     int
	height    int
}

func NewServicesView(ctx config.Context, clients *awsclient.ClientSet) ServicesView {
	t := table.New(
		table.WithFocused(true),
		table.WithStyles(servicesTableStyles()),
	)
	km := table.DefaultKeyMap()
	km.HalfPageDown.SetKeys()
	km.HalfPageUp.SetKeys()
	t.KeyMap = km

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = ui.HealthyStyle

	return ServicesView{
		table:   t,
		loading: true,
		ctx:     ctx,
		clients: clients,
		spinner: sp,
	}
}

func servicesTableStyles() table.Styles {
	return table.Styles{
		// No border — header is 1 line, keeping height math simple.
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(ui.ColorPrimary).
			Padding(0, 1),
		Cell:     lipgloss.NewStyle().Padding(0, 1),
		Selected: ui.SelectedStyle,
	}
}

func (m *ServicesView) ViewID() model.ViewID { return model.ViewServices }

func (m *ServicesView) KeyHints() []string {
	return []string{"↑/k:up", "↓/j:down", "enter:tasks", "d:describe", "t:cluster-tasks", "esc:back", "r:refresh", "q:quit"}
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
		m.table.SetRows(toTableRows(msg.Services))
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
			svc := m.items[cursor]
			tv := NewTasksView(m.ctx, m.clients, svc.Name, svc.TaskDefARN)
			return m, func() tea.Msg { return model.NavigatePushMsg{View: &tv} }
		case key.Matches(msg, model.GlobalKeys.Describe):
			cursor := m.table.Cursor()
			if cursor < 0 || cursor >= len(m.items) {
				return m, nil
			}
			svc := m.items[cursor]
			dv := NewServiceDescribeView(m.ctx, m.clients, svc.Name)
			return m, func() tea.Msg { return model.NavigatePushMsg{View: &dv} }
		case key.Matches(msg, model.GlobalKeys.Tasks):
			cv := NewClusterTasksView(m.ctx, m.clients)
			return m, func() tea.Msg { return model.NavigatePushMsg{View: &cv} }
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

func (m *ServicesView) View() string {
	if m.loading && len(m.items) == 0 {
		return fmt.Sprintf("\n  %s Loading services…", m.spinner.View())
	}
	if m.err != nil && len(m.items) == 0 {
		return ui.ErrorFgStyle.Render("\n  Error: "+m.err.Error()) +
			ui.DimStyle.Render("\n  Press r to retry")
	}
	if len(m.items) == 0 {
		return ui.DimStyle.Render("\n  No services found")
	}
	title := ui.TitleStyle.Width(m.width).Align(lipgloss.Center).Render("Services")
	return lipgloss.JoinVertical(lipgloss.Left, title, m.table.View())
}

func (m *ServicesView) SetSize(w, h int) {
	m.width = w
	m.height = h
	nameW := w - fixedWidths
	if nameW < 10 {
		nameW = 10
	}
	m.table.SetColumns([]table.Column{
		{Title: "Name", Width: nameW},
		{Title: "Status", Width: colStatusW},
		{Title: "Desired", Width: colCountW},
		{Title: "Running", Width: colCountW},
		{Title: "Pending", Width: colCountW},
		{Title: "Last Deployed", Width: colDeployedW},
	})
	// table.View() = header (1 line) + "\n" + viewport.
	// SetHeight(h) sets viewport.Height = h - lipgloss.Height(header) = h - 1.
	// Title takes 1 line, so table gets h-1 lines → SetHeight(h-2):
	// viewport.Height = (h-2)-1 = h-3; table.View() = 1+1+(h-3) = h-1; title+table = h.
	m.table.SetHeight(h - 2)
	m.table.SetWidth(w)
}

func toTableRows(services []domain.ECSService) []table.Row {
	rows := make([]table.Row, len(services))
	for i, s := range services {
		indicator := "●"
		if !s.IsHealthy {
			indicator = "○"
		}
		if strings.ToUpper(s.Status) != "ACTIVE" {
			indicator = "✕"
		}
		deployed := "—"
		if !s.LastDeploymentAt.IsZero() {
			deployed = s.LastDeploymentAt.Local().Format("2006-01-02 15:04:05")
		}
		rows[i] = table.Row{
			indicator + " " + s.Name,
			s.Status,
			fmt.Sprintf("%d", s.DesiredCount),
			fmt.Sprintf("%d", s.RunningCount),
			fmt.Sprintf("%d", s.PendingCount),
			deployed,
		}
	}
	return rows
}
