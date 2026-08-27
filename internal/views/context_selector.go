package views

import (
	"sort"
	"strings"

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
	visible []config.Context
	clients map[string]*awsclient.ClientSet
	cfg     *config.Config
	filter  FilterBox
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

	return ContextSelector{
		table:   t,
		items:   cfg.Contexts,
		clients: clients,
		cfg:     cfg,
		filter:  NewFilterBox(),
	}
}

func contextTableStyles() table.Styles {
	return table.Styles{
		Header:   lipgloss.NewStyle().Bold(true).Foreground(ui.ColorPrimary).Padding(0, 1),
		Cell:     lipgloss.NewStyle().Padding(0, 1),
		Selected: ui.SelectedStyle,
	}
}

func (m *ContextSelector) ViewID() model.ViewID { return model.ViewContextSelector }

func (m *ContextSelector) KeyHints() []string {
	return []string{"↑/k:up", "↓/j:down", "enter:select", "/:filter", "esc:back", "q:quit"}
}

func (m *ContextSelector) Init() tea.Cmd { return nil }

func (m *ContextSelector) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
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
		case key.Matches(msg, model.GlobalKeys.Search):
			return m, m.filter.Start()
		case key.Matches(msg, model.GlobalKeys.Enter):
			cursor := m.table.Cursor()
			if cursor < 0 || cursor >= len(m.visible) {
				return m, nil
			}
			ctx := m.visible[cursor]
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
	lines := []string{title, withHeaderRules(m.table.View(), m.width)}
	if m.filter.Active() {
		lines = append(lines, m.filter.View())
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m *ContextSelector) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.filter.SetWidth(w)
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
	m.refreshRows()
	if m.cfg.CurrentContext != "" {
		for i, ctx := range m.visible {
			if ctx.Name == m.cfg.CurrentContext {
				m.table.SetCursor(i)
				break
			}
		}
	}
	extra := 4 // title (1) + 2 header rules
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

func (m *ContextSelector) refreshRows() {
	m.visible = filterContexts(m.items, m.filter.Value())
	sort.SliceStable(m.visible, func(i, j int) bool {
		return strings.ToLower(m.visible[i].Name) < strings.ToLower(m.visible[j].Name)
	})
	m.table.SetRows(toContextTableRows(m.visible))
}

func filterContexts(items []config.Context, query string) []config.Context {
	if query == "" {
		return append([]config.Context(nil), items...)
	}
	q := strings.ToLower(query)
	out := make([]config.Context, 0, len(items))
	for _, ctx := range items {
		fields := []string{ctx.Name, ctx.Cluster, ctx.Region, ctx.AWSProfile}
		for _, f := range fields {
			if strings.Contains(strings.ToLower(f), q) {
				out = append(out, ctx)
				break
			}
		}
	}
	return out
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
