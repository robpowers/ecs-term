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

// fixed widths for the non-name columns (content width, excluding cell padding)
const (
	colStatusW  = 10
	colCountW   = 8
	colPadding  = 2 // lipgloss cell Padding(0,1) adds 1 left + 1 right
	numCols     = 5
	fixedWidths = colStatusW + colCountW*3 + colPadding*numCols
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
		table.WithStyles(tableStyles()),
	)
	// disable keys that conflict with global bindings
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

func tableStyles() table.Styles {
	return table.Styles{
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(ui.ColorPrimary).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(ui.ColorSecondary).
			BorderBottom(true).
			Padding(0, 1),
		Cell:     lipgloss.NewStyle().Padding(0, 1),
		Selected: ui.SelectedStyle.Padding(0, 1),
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
			row := m.table.SelectedRow()
			if row == nil {
				return m, nil
			}
			// match selected row back to the service by name
			svc := m.findService(row[0])
			if svc == nil {
				return m, nil
			}
			tv := NewTasksView(m.ctx, m.clients, svc.Name, svc.TaskDefARN)
			return m, func() tea.Msg { return model.NavigatePushMsg{View: &tv} }
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
	title := ui.TitleStyle.Render(fmt.Sprintf("Services — %s", m.ctx.Cluster))
	if m.loading && len(m.items) == 0 {
		return title + "\n\n  " + m.spinner.View() + " Loading services…"
	}
	if m.err != nil && len(m.items) == 0 {
		return title + "\n\n" +
			ui.ErrorFgStyle.Render("  Error: "+m.err.Error()) +
			ui.DimStyle.Render("\n  Press r to retry")
	}
	if len(m.items) == 0 {
		return title + "\n\n" + ui.DimStyle.Render("  No services found")
	}
	return title + "\n" + m.table.View()
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
	})
	// subtract 1 for the title line and 1 for the header border
	m.table.SetHeight(h - 2)
	m.table.SetWidth(w)
}

// findService looks up a service by name, stripping any ANSI prefix from the row.
func (m *ServicesView) findService(nameCell string) *domain.ECSService {
	plain := stripANSI(nameCell)
	for i := range m.items {
		if m.items[i].Name == plain {
			return &m.items[i]
		}
	}
	return nil
}

func toTableRows(services []domain.ECSService) []table.Row {
	rows := make([]table.Row, len(services))
	for i, s := range services {
		statusStyle := ui.StatusColor(s.Status)
		// healthy indicator prepended to name
		indicator := ui.HealthyStyle.Render("●")
		if !s.IsHealthy {
			indicator = ui.WarningStyle.Render("●")
		}
		if strings.ToUpper(s.Status) != "ACTIVE" {
			indicator = ui.ErrorFgStyle.Render("●")
		}
		rows[i] = table.Row{
			indicator + " " + s.Name,
			statusStyle.Render(s.Status),
			fmt.Sprintf("%d", s.DesiredCount),
			fmt.Sprintf("%d", s.RunningCount),
			fmt.Sprintf("%d", s.PendingCount),
		}
	}
	return rows
}

// stripANSI removes ANSI escape codes to recover the plain service name.
func stripANSI(s string) string {
	// lipgloss renders ANSI; the name starts after "● " (the indicator + space)
	// Find the last occurrence of 'm' in the ANSI prefix and strip from there.
	// Simpler: split on 'm' run and take the remainder after the reset sequence.
	for i := 0; i < len(s)-1; i++ {
		if s[i] == 0x1b && s[i+1] == '[' {
			// skip to end of this escape sequence
			for j := i + 2; j < len(s); j++ {
				if s[j] == 'm' {
					s = s[:i] + s[j+1:]
					i--
					break
				}
			}
		}
	}
	// trim the "● " indicator prefix
	s = strings.TrimPrefix(s, "● ")
	return strings.TrimSpace(s)
}
