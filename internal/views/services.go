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

var serviceSortOptions = []sortOption{
	{"N", "name"}, {"S", "status"}, {"D", "desired"}, {"R", "running"}, {"P", "pending"}, {"L", "deployed"},
}

// Fixed widths for the non-name columns (content width, excluding cell padding).
// Each column adds 2 for cell padding (Padding(0,1) = 1 left + 1 right).
const (
	// colStatusW must comfortably exceed the ANSI-inflated width of the
	// longest colored status text (bubbles/table's cell truncation isn't
	// ANSI-aware — see ui.StatusColorSafeText's doc comment).
	colStatusW   = 24
	colCountW    = 8
	colDeployedW = 19
	colPadding   = 2
	numCols      = 6
	fixedWidths  = colStatusW + colCountW*3 + colDeployedW + colPadding*numCols
)

type ServicesView struct {
	table     table.Model
	items     []domain.ECSService
	visible   []domain.ECSService
	loading   bool
	err       error
	ctx       config.Context
	clients   *awsclient.ClientSet
	spinner   spinner.Model
	lastFetch time.Time
	filter    FilterBox
	sortKey   string
	sortAsc   bool
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
		filter:  NewFilterBox(),
		sortKey: "N",
		sortAsc: true,
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
	return []string{"↑/k:up", "↓/j:down", "enter:tasks", "d:describe", "t:cluster-tasks", "e:events", "v:deployments", "/:filter", "esc:back", "r:refresh", "q:quit"}
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
		case key.Matches(msg, model.GlobalKeys.Enter):
			cursor := m.table.Cursor()
			if cursor < 0 || cursor >= len(m.visible) {
				return m, nil
			}
			svc := m.visible[cursor]
			tv := NewTasksView(m.ctx, m.clients, svc.Name, svc.TaskDefARN)
			return m, func() tea.Msg { return model.NavigatePushMsg{View: &tv} }
		case key.Matches(msg, model.GlobalKeys.Describe):
			cursor := m.table.Cursor()
			if cursor < 0 || cursor >= len(m.visible) {
				return m, nil
			}
			svc := m.visible[cursor]
			dv := NewServiceDescribeView(m.ctx, m.clients, svc.Name)
			return m, func() tea.Msg { return model.NavigatePushMsg{View: &dv} }
		case key.Matches(msg, model.GlobalKeys.Tasks):
			cv := NewClusterTasksView(m.ctx, m.clients)
			return m, func() tea.Msg { return model.NavigatePushMsg{View: &cv} }
		case msg.String() == "e":
			cursor := m.table.Cursor()
			if cursor < 0 || cursor >= len(m.visible) {
				return m, nil
			}
			svc := m.visible[cursor]
			ev := NewServiceEventsView(m.ctx, m.clients, svc.Name)
			return m, func() tea.Msg { return model.NavigatePushMsg{View: &ev} }
		case msg.String() == "v":
			cursor := m.table.Cursor()
			if cursor < 0 || cursor >= len(m.visible) {
				return m, nil
			}
			svc := m.visible[cursor]
			dep := NewDeploymentsView(m.ctx, m.clients, svc.Name)
			return m, func() tea.Msg { return model.NavigatePushMsg{View: &dep} }
		}

		if sortKeyMatch(serviceSortOptions, msg.String()) {
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
	lines := []string{title, withHeaderRules(m.table.View(), m.width)}
	if m.filter.Active() {
		lines = append(lines, m.filter.View())
	}
	lines = append(lines, renderSortBanner(serviceSortOptions, m.sortKey, m.sortAsc))
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m *ServicesView) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.filter.SetWidth(w)
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
	// Title (1) + 2 header rules + sort banner (1) [+ filter box (1)] take
	// the rest of the rows.
	extra := 5
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

func (m *ServicesView) refreshRows() {
	m.visible = filterServices(m.items, m.filter.Value())
	sortServicesInPlace(m.visible, m.sortKey, m.sortAsc)
	m.table.SetRows(toTableRows(m.visible))
}

func filterServices(items []domain.ECSService, query string) []domain.ECSService {
	if query == "" {
		return append([]domain.ECSService(nil), items...)
	}
	q := strings.ToLower(query)
	out := make([]domain.ECSService, 0, len(items))
	for _, s := range items {
		if serviceMatches(s, q) {
			out = append(out, s)
		}
	}
	return out
}

func serviceMatches(s domain.ECSService, lowerQuery string) bool {
	fields := []string{
		s.Name,
		s.Status,
		fmt.Sprintf("%d", s.DesiredCount),
		fmt.Sprintf("%d", s.RunningCount),
		fmt.Sprintf("%d", s.PendingCount),
	}
	if !s.LastDeploymentAt.IsZero() {
		fields = append(fields, s.LastDeploymentAt.Local().Format("2006-01-02 15:04:05"))
	}
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f), lowerQuery) {
			return true
		}
	}
	return false
}

func sortServicesInPlace(items []domain.ECSService, key string, asc bool) {
	var less func(i, j int) bool
	switch key {
	case "N":
		less = func(i, j int) bool { return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name) }
	case "S":
		less = func(i, j int) bool { return items[i].Status < items[j].Status }
	case "D":
		less = func(i, j int) bool { return items[i].DesiredCount < items[j].DesiredCount }
	case "R":
		less = func(i, j int) bool { return items[i].RunningCount < items[j].RunningCount }
	case "P":
		less = func(i, j int) bool { return items[i].PendingCount < items[j].PendingCount }
	case "L":
		less = func(i, j int) bool { return items[i].LastDeploymentAt.Before(items[j].LastDeploymentAt) }
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
			ui.StatusColorSafeText(s.Status),
			fmt.Sprintf("%d", s.DesiredCount),
			fmt.Sprintf("%d", s.RunningCount),
			fmt.Sprintf("%d", s.PendingCount),
			deployed,
		}
	}
	return rows
}
