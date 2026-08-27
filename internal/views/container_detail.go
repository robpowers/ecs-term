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

type ContainerDetailView struct {
	viewport   viewport.Model
	details    []domain.ContainerDetail
	loading    bool
	err        error
	ctx        config.Context
	clients    *awsclient.ClientSet
	taskDefARN string
	spinner    spinner.Model
	filter     FilterBox
}

func NewContainerDetailView(ctx config.Context, clients *awsclient.ClientSet, taskDefARN string) ContainerDetailView {
	vp := viewport.New(0, 0)
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = ui.HealthyStyle

	return ContainerDetailView{
		viewport:   vp,
		loading:    true,
		ctx:        ctx,
		clients:    clients,
		taskDefARN: taskDefARN,
		spinner:    sp,
		filter:     NewFilterBox(),
	}
}

func (m *ContainerDetailView) ViewID() model.ViewID { return model.ViewContainerDetail }

func (m *ContainerDetailView) KeyHints() []string {
	return []string{"↑/k:up", "↓/j:down", "/:search", "esc:back", "q:quit", "?:help"}
}

func (m *ContainerDetailView) IsCapturingInput() bool { return m.filter.Active() }

func (m *ContainerDetailView) renderContent() string {
	text := renderDetails(m.details)
	if f := m.filter.Value(); f != "" {
		text = highlightMatches(text, f)
	}
	return text
}

func (m *ContainerDetailView) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.fetchCmd())
}

func (m *ContainerDetailView) fetchCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		details, err := m.clients.DescribeTaskDefinition(ctx, m.taskDefARN)
		return model.ContainerDetailMsg{Details: details, Err: err}
	}
}

func (m *ContainerDetailView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case model.ContainerDetailMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
			m.viewport.SetContent(ui.ErrorFgStyle.Render("Error: " + msg.Err.Error()))
			return m, nil
		}
		m.err = nil
		m.details = msg.Details
		m.viewport.SetContent(m.renderContent())
		return m, nil

	case tea.KeyMsg:
		if handled, cmd := m.filter.HandleKey(msg); handled {
			m.viewport.SetContent(m.renderContent())
			return m, cmd
		}
		switch {
		case key.Matches(msg, model.GlobalKeys.Back):
			if m.filter.HandleBack() {
				m.viewport.SetContent(m.renderContent())
				return m, nil
			}
			return m, func() tea.Msg { return model.NavigatePopMsg{} }
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

func (m *ContainerDetailView) View() string {
	title := ui.TitleStyle.Render("Container Definitions")
	if m.loading {
		return title + "\n\n  " + m.spinner.View() + " Loading task definition…"
	}
	if m.filter.Active() {
		return title + "\n" + m.viewport.View() + "\n" + m.filter.View()
	}
	return title + "\n" + m.viewport.View()
}

func (m *ContainerDetailView) SetSize(w, h int) {
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

func renderDetails(details []domain.ContainerDetail) string {
	var b strings.Builder
	for i, d := range details {
		if i > 0 {
			b.WriteString(strings.Repeat("─", 60) + "\n\n")
		}
		b.WriteString(ui.TitleStyle.Render(d.Name) + "\n")
		b.WriteString(fmt.Sprintf("  Image:   %s\n", d.Image))
		if d.CPU > 0 {
			b.WriteString(fmt.Sprintf("  CPU:     %d units\n", d.CPU))
		}
		if d.MemoryMB > 0 {
			b.WriteString(fmt.Sprintf("  Memory:  %d MB\n", d.MemoryMB))
		}
		if d.MemoryReserveMB > 0 {
			b.WriteString(fmt.Sprintf("  MemRes:  %d MB\n", d.MemoryReserveMB))
		}

		if len(d.PortMappings) > 0 {
			b.WriteString("\n  " + ui.DimStyle.Render("Port Mappings") + "\n")
			for _, pm := range d.PortMappings {
				b.WriteString(fmt.Sprintf("    %d → %d (%s)\n", pm.ContainerPort, pm.HostPort, pm.Protocol))
			}
		}

		if d.HealthCheck != nil {
			hc := d.HealthCheck
			b.WriteString("\n  " + ui.DimStyle.Render("Health Check") + "\n")
			b.WriteString(fmt.Sprintf("    Command:  %s\n", strings.Join(hc.Command, " ")))
			b.WriteString(fmt.Sprintf("    Interval: %ds  Timeout: %ds  Retries: %d\n",
				hc.IntervalSec, hc.TimeoutSec, hc.Retries))
			if hc.StartPeriod > 0 {
				b.WriteString(fmt.Sprintf("    StartPeriod: %ds\n", hc.StartPeriod))
			}
		}

		if len(d.EnvVars) > 0 {
			b.WriteString("\n  " + ui.DimStyle.Render("Environment Variables") + "\n")
			for _, e := range d.EnvVars {
				b.WriteString(fmt.Sprintf("    %-30s = %s\n", e.Name, e.Value))
			}
		}

		if len(d.Secrets) > 0 {
			b.WriteString("\n  " + ui.DimStyle.Render("Secrets") + "\n")
			for _, s := range d.Secrets {
				b.WriteString(fmt.Sprintf("    %-30s ← %s\n", s.Name, s.ValueFrom))
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}
