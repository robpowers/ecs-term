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

type ServiceDescribeView struct {
	viewport    viewport.Model
	detail      domain.ECSServiceDetail
	loaded      bool
	loading     bool
	err         error
	ctx         config.Context
	clients     *awsclient.ClientSet
	serviceName string
	spinner     spinner.Model
}

func NewServiceDescribeView(ctx config.Context, clients *awsclient.ClientSet, serviceName string) ServiceDescribeView {
	vp := viewport.New(0, 0)
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = ui.HealthyStyle

	return ServiceDescribeView{
		viewport:    vp,
		loading:     true,
		ctx:         ctx,
		clients:     clients,
		serviceName: serviceName,
		spinner:     sp,
	}
}

func (m *ServiceDescribeView) ViewID() model.ViewID { return model.ViewServiceDescribe }

func (m *ServiceDescribeView) KeyHints() []string {
	return []string{"↑/k:up", "↓/j:down", "r:refresh", "esc:back", "q:quit"}
}

func (m *ServiceDescribeView) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.fetchCmd())
}

func (m *ServiceDescribeView) fetchCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		detail, err := m.clients.DescribeServiceFull(ctx, m.ctx.Cluster, m.serviceName)
		return model.ServiceDetailMsg{Detail: detail, Err: err}
	}
}

func (m *ServiceDescribeView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case model.ServiceDetailMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
			m.viewport.SetContent(ui.ErrorFgStyle.Render("Error: " + msg.Err.Error()))
			return m, nil
		}
		m.err = nil
		m.detail = msg.Detail
		m.loaded = true
		m.viewport.SetContent(renderServiceDetail(msg.Detail))
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, model.GlobalKeys.Back):
			return m, func() tea.Msg { return model.NavigatePopMsg{} }
		case key.Matches(msg, model.GlobalKeys.Refresh):
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, m.fetchCmd())
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

func (m *ServiceDescribeView) View() string {
	title := ui.TitleStyle.Render("Describe Service — " + m.serviceName)
	if m.loading && !m.loaded {
		return title + "\n\n  " + m.spinner.View() + " Loading service details…"
	}
	return title + "\n" + m.viewport.View()
}

func (m *ServiceDescribeView) SetSize(w, h int) {
	m.viewport.Width = w
	m.viewport.Height = h - 2
	if m.viewport.Height < 1 {
		m.viewport.Height = 1
	}
}

func renderServiceDetail(d domain.ECSServiceDetail) string {
	var b strings.Builder
	section := func(label string) {
		b.WriteString("\n" + ui.TitleStyle.Render(label) + "\n")
	}
	kv := func(k, v string) {
		if v == "" {
			return
		}
		b.WriteString(fmt.Sprintf("  %-20s %s\n", k+":", v))
	}

	b.WriteString(ui.TitleStyle.Render(d.Name) + "\n")
	kv("Status", d.Status)
	kv("Service ARN", d.ServiceARN)
	kv("Cluster ARN", d.ClusterARN)
	kv("Task Definition", d.TaskDefARN)
	kv("Launch Type", d.LaunchType)
	kv("Platform Version", d.PlatformVersion)
	kv("Platform Family", d.PlatformFamily)
	kv("Scheduling", d.SchedulingStrategy)
	kv("Role ARN", d.RoleARN)
	kv("Deployment Ctrl", d.DeploymentController)
	kv("Propagate Tags", d.PropagateTags)
	kv("Exec Enabled", fmt.Sprintf("%t", d.EnableExecuteCommand))
	if !d.CreatedAt.IsZero() {
		kv("Created", d.CreatedAt.Local().Format("2006-01-02 15:04:05"))
	}

	section("Counts")
	kv("Desired", fmt.Sprintf("%d", d.DesiredCount))
	kv("Running", fmt.Sprintf("%d", d.RunningCount))
	kv("Pending", fmt.Sprintf("%d", d.PendingCount))

	if d.NetworkConfig != nil {
		section("Network")
		kv("Subnets", strings.Join(d.NetworkConfig.Subnets, ", "))
		kv("Security Groups", strings.Join(d.NetworkConfig.SecurityGroups, ", "))
		kv("Assign Public IP", d.NetworkConfig.AssignPublicIP)
	}

	if len(d.LoadBalancers) > 0 {
		section("Load Balancers")
		for _, lb := range d.LoadBalancers {
			line := fmt.Sprintf("  %s → %s:%d", lb.ContainerName, targetGroupShort(lb.TargetGroupARN), lb.ContainerPort)
			if lb.LoadBalancerName != "" {
				line += " (" + lb.LoadBalancerName + ")"
			}
			b.WriteString(line + "\n")
		}
	}

	if len(d.ServiceRegistries) > 0 {
		section("Service Registries")
		for _, sr := range d.ServiceRegistries {
			b.WriteString(fmt.Sprintf("  %s  container=%s port=%d\n", sr.RegistryARN, sr.ContainerName, sr.ContainerPort))
		}
	}

	if len(d.CapacityProviderStrategy) > 0 {
		section("Capacity Providers")
		for _, cp := range d.CapacityProviderStrategy {
			b.WriteString(fmt.Sprintf("  %s  base=%d weight=%d\n", cp.Name, cp.Base, cp.Weight))
		}
	}

	if len(d.Deployments) > 0 {
		section("Deployments")
		for _, dep := range d.Deployments {
			b.WriteString(fmt.Sprintf("  %s  [%s]", dep.ID, dep.Status))
			if dep.RolloutState != "" {
				b.WriteString(" " + dep.RolloutState)
			}
			b.WriteString("\n")
			b.WriteString(fmt.Sprintf("    taskdef: %s\n", dep.TaskDefARN))
			b.WriteString(fmt.Sprintf("    desired=%d running=%d pending=%d failed=%d\n",
				dep.DesiredCount, dep.RunningCount, dep.PendingCount, dep.FailedTasks))
			if dep.RolloutStateReason != "" {
				b.WriteString("    reason: " + dep.RolloutStateReason + "\n")
			}
			if !dep.UpdatedAt.IsZero() {
				b.WriteString(ui.DimStyle.Render(fmt.Sprintf("    updated: %s", dep.UpdatedAt.Local().Format("2006-01-02 15:04:05"))) + "\n")
			}
		}
	}

	if len(d.Events) > 0 {
		section("Events")
		limit := len(d.Events)
		if limit > 20 {
			limit = 20
		}
		for i := 0; i < limit; i++ {
			ev := d.Events[i]
			ts := ui.DimStyle.Render(ev.CreatedAt.Local().Format("2006-01-02 15:04:05"))
			b.WriteString(fmt.Sprintf("  %s  %s\n", ts, ev.Message))
		}
		if len(d.Events) > limit {
			b.WriteString(ui.DimStyle.Render(fmt.Sprintf("  …and %d more\n", len(d.Events)-limit)))
		}
	}

	if len(d.Tags) > 0 {
		section("Tags")
		for _, t := range d.Tags {
			b.WriteString(fmt.Sprintf("  %-30s %s\n", t.Key, t.Value))
		}
	}

	return b.String()
}

func targetGroupShort(arn string) string {
	if i := strings.LastIndex(arn, ":targetgroup/"); i >= 0 {
		return arn[i+len(":targetgroup/"):]
	}
	if i := strings.LastIndex(arn, "/"); i >= 0 {
		return arn[i+1:]
	}
	return arn
}
