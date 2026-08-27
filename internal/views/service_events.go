package views

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	awsclient "github.com/robpowers/ecs-term/internal/aws"
	"github.com/robpowers/ecs-term/internal/config"
	"github.com/robpowers/ecs-term/internal/domain"
	"github.com/robpowers/ecs-term/internal/model"
	"github.com/robpowers/ecs-term/internal/ui"
)

// ServiceEventsView shows a service's recent events, most-recent first.
// Content is timestamp+message, structurally identical to a LogEvent, so
// filtering behaves the same as the logs view (hide non-matching + highlight
// matches) rather than the highlight-only search used by other describe pages.
type ServiceEventsView struct {
	viewport    viewport.Model
	events      []domain.ServiceEvent
	loaded      bool
	loading     bool
	err         error
	ctx         config.Context
	clients     *awsclient.ClientSet
	serviceName string
	spinner     spinner.Model
	filter      FilterBox
	wrap        bool
}

func NewServiceEventsView(ctx config.Context, clients *awsclient.ClientSet, serviceName string) ServiceEventsView {
	vp := viewport.New(0, 0)
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = ui.HealthyStyle

	return ServiceEventsView{
		viewport:    vp,
		loading:     true,
		ctx:         ctx,
		clients:     clients,
		serviceName: serviceName,
		spinner:     sp,
		filter:      NewFilterBox(),
	}
}

func (m *ServiceEventsView) ViewID() model.ViewID { return model.ViewServiceEvents }

func (m *ServiceEventsView) KeyHints() []string {
	wrapHint := "w:wrap"
	if m.wrap {
		wrapHint = "w:no-wrap"
	}
	return []string{"↑/k:up", "↓/j:down", wrapHint, "/:filter", "r:refresh", "esc:back", "q:quit", "?:help"}
}

func (m *ServiceEventsView) IsCapturingInput() bool { return m.filter.Active() }

func (m *ServiceEventsView) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.fetchCmd())
}

func (m *ServiceEventsView) fetchCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		detail, err := m.clients.DescribeServiceFull(ctx, m.ctx.Cluster, m.serviceName)
		return model.ServiceDetailMsg{Detail: detail, Err: err}
	}
}

func (m *ServiceEventsView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case model.ServiceDetailMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
			m.viewport.SetContent(ui.ErrorFgStyle.Render("Error: " + msg.Err.Error()))
			return m, nil
		}
		m.err = nil
		events := append([]domain.ServiceEvent(nil), msg.Detail.Events...)
		sort.SliceStable(events, func(i, j int) bool { return events[i].CreatedAt.After(events[j].CreatedAt) })
		m.events = events
		m.loaded = true
		m.viewport.SetContent(renderServiceEvents(m.events, m.filter.Value(), m.wrap, m.viewport.Width))
		return m, nil

	case tea.KeyMsg:
		if handled, cmd := m.filter.HandleKey(msg); handled {
			m.viewport.SetContent(renderServiceEvents(m.events, m.filter.Value(), m.wrap, m.viewport.Width))
			return m, cmd
		}

		switch {
		case key.Matches(msg, model.GlobalKeys.Back):
			if m.filter.HandleBack() {
				m.viewport.SetContent(renderServiceEvents(m.events, m.filter.Value(), m.wrap, m.viewport.Width))
				return m, nil
			}
			return m, func() tea.Msg { return model.NavigatePopMsg{} }
		case key.Matches(msg, model.GlobalKeys.Refresh):
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, m.fetchCmd())
		case key.Matches(msg, model.GlobalKeys.Search):
			return m, m.filter.Start()
		case msg.String() == "w":
			m.wrap = !m.wrap
			m.viewport.SetContent(renderServiceEvents(m.events, m.filter.Value(), m.wrap, m.viewport.Width))
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

func (m *ServiceEventsView) View() string {
	title := ui.TitleStyle.Render("Events — " + m.serviceName)
	if m.wrap {
		title += "  " + ui.DimStyle.Render("[wrap]")
	}
	if m.loading && !m.loaded {
		return title + "\n\n  " + m.spinner.View() + " Loading events…"
	}
	if m.filter.Active() {
		return title + "\n" + m.viewport.View() + "\n" + m.filter.View()
	}
	return title + "\n" + m.viewport.View()
}

func (m *ServiceEventsView) SetSize(w, h int) {
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

func renderServiceEvents(events []domain.ServiceEvent, filter string, wrap bool, width int) string {
	if len(events) == 0 {
		return ui.DimStyle.Render("No events found")
	}
	lowerFilter := strings.ToLower(filter)
	var b strings.Builder
	matched := 0
	for _, e := range events {
		if filter != "" && !strings.Contains(strings.ToLower(e.Message), lowerFilter) {
			continue
		}
		matched++
		ts := ui.DimStyle.Render(e.CreatedAt.Local().Format("2006-01-02 15:04:05"))
		msg := e.Message
		if filter != "" {
			msg = highlightMatches(msg, filter)
		}
		line := ts + "  " + msg
		if wrap && width > 0 {
			line = lipgloss.NewStyle().Width(width).Render(line)
		}
		b.WriteString(line + "\n")
	}
	if filter != "" && matched == 0 {
		return ui.DimStyle.Render(fmt.Sprintf("No matches for %q (%d events)", filter, len(events)))
	}
	return b.String()
}
