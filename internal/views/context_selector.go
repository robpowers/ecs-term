package views

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	awsclient "github.com/robpowers/ecs-term/internal/aws"
	"github.com/robpowers/ecs-term/internal/config"
	"github.com/robpowers/ecs-term/internal/model"
	"github.com/robpowers/ecs-term/internal/ui"
)

const (
	ctxColClusterW = 25
	ctxColRegionW  = 14
	ctxColProfileW = 18
	ctxColPadding  = 2
	ctxNumCols     = 4
	ctxFixedWidths = ctxColClusterW + ctxColRegionW + ctxColProfileW + ctxColPadding*ctxNumCols
)

type ContextSelector struct {
	table   table.Model
	items   []config.Context
	clients map[string]*awsclient.ClientSet
	cfg     *config.Config
	width   int
	height  int
}

func NewContextSelector(cfg *config.Config, clients map[string]*awsclient.ClientSet) ContextSelector {
	t := table.New(
		table.WithFocused(true),
		table.WithStyles(contextTableStyles()),
	)
	km := table.DefaultKeyMap()
	km.HalfPageDown.SetKeys()
	km.HalfPageUp.SetKeys()
	t.KeyMap = km

	t.SetRows(toContextTableRows(cfg.Contexts))

	if cfg.CurrentContext != "" {
		for i, ctx := range cfg.Contexts {
			if ctx.Name == cfg.CurrentContext {
				t.SetCursor(i)
				break
			}
		}
	}

	return ContextSelector{
		table:   t,
		items:   cfg.Contexts,
		clients: clients,
		cfg:     cfg,
	}
}

func contextTableStyles() table.Styles {
	return table.Styles{
		Header:   lipgloss.NewStyle().Bold(true).Foreground(ui.ColorPrimary).Padding(0, 1),
		Cell:     lipgloss.NewStyle().Padding(0, 1),
		Selected: ui.SelectedStyle.Padding(0, 1),
	}
}

func (m *ContextSelector) ViewID() model.ViewID { return model.ViewContextSelector }

func (m *ContextSelector) KeyHints() []string {
	return []string{"↑/k:up", "↓/j:down", "enter:select", "q:quit"}
}

func (m *ContextSelector) Init() tea.Cmd { return nil }

func (m *ContextSelector) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, model.GlobalKeys.Back):
			return m, func() tea.Msg { return model.NavigatePopMsg{} }
		case key.Matches(msg, model.GlobalKeys.Enter):
			cursor := m.table.Cursor()
			if cursor < 0 || cursor >= len(m.items) {
				return m, nil
			}
			ctx := m.items[cursor]
			cs := m.clients[ctx.Name]
			sv := NewServicesView(ctx, cs)
			return m, tea.Batch(
				func() tea.Msg {
					return model.ContextSelectedMsg{
						Name:    ctx.Name,
						Region:  ctx.Region,
						Profile: ctx.AWSProfile,
						Cluster: ctx.Cluster,
					}
				},
				func() tea.Msg {
					return model.NavigatePushMsg{View: &sv}
				},
			)
		}
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *ContextSelector) View() string {
	if len(m.items) == 0 {
		return ui.DimStyle.Render("\n  No contexts configured")
	}
	title := ui.TitleStyle.Width(m.width).Align(lipgloss.Center).Render("Contexts")
	return lipgloss.JoinVertical(lipgloss.Left, title, m.table.View())
}

func (m *ContextSelector) SetSize(w, h int) {
	m.width = w
	m.height = h
	nameW := w - ctxFixedWidths
	if nameW < 10 {
		nameW = 10
	}
	m.table.SetColumns([]table.Column{
		{Title: "Name", Width: nameW},
		{Title: "Cluster", Width: ctxColClusterW},
		{Title: "Region", Width: ctxColRegionW},
		{Title: "SSO Profile", Width: ctxColProfileW},
	})
	m.table.SetHeight(h - 2)
	m.table.SetWidth(w)
}

func toContextTableRows(contexts []config.Context) []table.Row {
	rows := make([]table.Row, len(contexts))
	for i, ctx := range contexts {
		rows[i] = table.Row{
			ctx.Name,
			ctx.Cluster,
			ctx.Region,
			ctx.AWSProfile,
		}
	}
	return rows
}
