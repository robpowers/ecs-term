package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	awsclient "github.com/robpowers/ecs-term/internal/aws"
	"github.com/robpowers/ecs-term/internal/config"
	"github.com/robpowers/ecs-term/internal/domain"
	"github.com/robpowers/ecs-term/internal/model"
	"github.com/robpowers/ecs-term/internal/ui"
)

// DeploymentsView shows a service's deployments (one per active/rolling-back
// revision). Reuses DescribeServiceFull/ServiceDetailMsg — Detail.Deployments
// is already populated, same call ServiceDescribeView already makes.
type DeploymentsView struct {
	viewport    viewport.Model
	deployments []domain.Deployment
	loaded      bool
	loading     bool
	err         error
	ctx         config.Context
	clients     *awsclient.ClientSet
	serviceName string
	spinner     spinner.Model
	filter      FilterBox
}

func NewDeploymentsView(ctx config.Context, clients *awsclient.ClientSet, serviceName string) DeploymentsView {
	vp := viewport.New(0, 0)
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = ui.HealthyStyle

	return DeploymentsView{
		viewport:    vp,
		loading:     true,
		ctx:         ctx,
		clients:     clients,
		serviceName: serviceName,
		spinner:     sp,
		filter:      NewFilterBox(),
	}
}

func (m *DeploymentsView) ViewID() model.ViewID { return model.ViewDeployments }

func (m *DeploymentsView) KeyHints() []string {
	return []string{"↑/k:up", "↓/j:down", "/:filter", "r:refresh", "esc:back", "q:quit", "?:help"}
}

func (m *DeploymentsView) IsCapturingInput() bool { return m.filter.Active() }

func (m *DeploymentsView) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.fetchCmd())
}

func (m *DeploymentsView) fetchCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		detail, err := m.clients.DescribeServiceFull(ctx, m.ctx.Cluster, m.serviceName)
		return model.ServiceDetailMsg{Detail: detail, Err: err}
	}
}

func (m *DeploymentsView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case model.ServiceDetailMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
			m.viewport.SetContent(ui.ErrorFgStyle.Render("Error: " + msg.Err.Error()))
			return m, nil
		}
		m.err = nil
		m.deployments = msg.Detail.Deployments
		m.loaded = true
		m.viewport.SetContent(renderDeployments(m.deployments, m.filter.Value()))
		return m, nil

	case tea.KeyMsg:
		if handled, cmd := m.filter.HandleKey(msg); handled {
			m.viewport.SetContent(renderDeployments(m.deployments, m.filter.Value()))
			return m, cmd
		}

		switch {
		case key.Matches(msg, model.GlobalKeys.Back):
			if m.filter.HandleBack() {
				m.viewport.SetContent(renderDeployments(m.deployments, m.filter.Value()))
				return m, nil
			}
			return m, func() tea.Msg { return model.NavigatePopMsg{} }
		case key.Matches(msg, model.GlobalKeys.Refresh):
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, m.fetchCmd())
		case key.Matches(msg, model.GlobalKeys.Search):
			return m, m.filter.Start()
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

func (m *DeploymentsView) View() string {
	title := ui.TitleStyle.Render("Deployments — " + m.serviceName)
	if m.loading && !m.loaded {
		return title + "\n\n  " + m.spinner.View() + " Loading deployments…"
	}
	if m.filter.Active() {
		return title + "\n" + m.viewport.View() + "\n" + m.filter.View()
	}
	return title + "\n" + m.viewport.View()
}

func (m *DeploymentsView) SetSize(w, h int) {
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

func deploymentMatches(d domain.Deployment, lowerQuery string) bool {
	fields := []string{d.ID, d.Status, d.RolloutState, d.TaskDefARN}
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f), lowerQuery) {
			return true
		}
	}
	return false
}

func renderDeployments(deployments []domain.Deployment, filter string) string {
	if len(deployments) == 0 {
		return ui.DimStyle.Render("No deployments found")
	}
	visible := deployments
	if filter != "" {
		q := strings.ToLower(filter)
		visible = make([]domain.Deployment, 0, len(deployments))
		for _, d := range deployments {
			if deploymentMatches(d, q) {
				visible = append(visible, d)
			}
		}
	}
	if len(visible) == 0 {
		return ui.DimStyle.Render(fmt.Sprintf("No matches for %q (%d deployments)", filter, len(deployments)))
	}

	var b strings.Builder
	for i, dep := range visible {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(fmt.Sprintf("%s  %s", ui.TitleStyle.Render(dep.ID), ui.StatusColor(dep.Status).Render(dep.Status)))
		if dep.RolloutState != "" {
			b.WriteString(" " + ui.StatusColor(dep.RolloutState).Render(dep.RolloutState))
		}
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("  taskdef: %s\n", dep.TaskDefARN))
		b.WriteString(fmt.Sprintf("  desired=%d running=%d pending=%d failed=%d\n",
			dep.DesiredCount, dep.RunningCount, dep.PendingCount, dep.FailedTasks))
		if dep.RolloutStateReason != "" {
			b.WriteString("  reason: " + dep.RolloutStateReason + "\n")
		}
		if !dep.CreatedAt.IsZero() {
			b.WriteString(ui.DimStyle.Render(fmt.Sprintf("  created: %s", dep.CreatedAt.Local().Format("2006-01-02 15:04:05"))) + "\n")
		}
		if !dep.UpdatedAt.IsZero() {
			b.WriteString(ui.DimStyle.Render(fmt.Sprintf("  updated: %s", dep.UpdatedAt.Local().Format("2006-01-02 15:04:05"))) + "\n")
		}
	}
	return b.String()
}
