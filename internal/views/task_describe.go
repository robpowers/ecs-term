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

	awsclient "github.com/robpowers/ecs-term/internal/aws"
	"github.com/robpowers/ecs-term/internal/config"
	"github.com/robpowers/ecs-term/internal/domain"
	"github.com/robpowers/ecs-term/internal/model"
	"github.com/robpowers/ecs-term/internal/ui"
)

type TaskDescribeView struct {
	viewport viewport.Model
	detail   domain.ECSTaskDetail
	loaded   bool
	loading  bool
	err      error
	ctx      config.Context
	clients  *awsclient.ClientSet
	taskARN  string
	spinner  spinner.Model
}

func NewTaskDescribeView(ctx config.Context, clients *awsclient.ClientSet, taskARN string) TaskDescribeView {
	vp := viewport.New(0, 0)
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = ui.HealthyStyle

	return TaskDescribeView{
		viewport: vp,
		loading:  true,
		ctx:      ctx,
		clients:  clients,
		taskARN:  taskARN,
		spinner:  sp,
	}
}

func (m *TaskDescribeView) ViewID() model.ViewID { return model.ViewTaskDescribe }

func (m *TaskDescribeView) KeyHints() []string {
	return []string{"↑/k:up", "↓/j:down", "r:refresh", "esc:back", "q:quit"}
}

func (m *TaskDescribeView) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.fetchCmd())
}

func (m *TaskDescribeView) fetchCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		detail, err := m.clients.DescribeTaskFull(ctx, m.ctx.Cluster, m.taskARN)
		return model.TaskDetailMsg{Detail: detail, Err: err}
	}
}

func (m *TaskDescribeView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case model.TaskDetailMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
			m.viewport.SetContent(ui.ErrorFgStyle.Render("Error: " + msg.Err.Error()))
			return m, nil
		}
		m.err = nil
		m.detail = msg.Detail
		m.loaded = true
		m.viewport.SetContent(renderTaskDetail(msg.Detail))
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

func (m *TaskDescribeView) View() string {
	title := ui.TitleStyle.Render("Describe Task — " + shortARN(m.taskARN))
	if m.loading && !m.loaded {
		return title + "\n\n  " + m.spinner.View() + " Loading task details…"
	}
	return title + "\n" + m.viewport.View()
}

func (m *TaskDescribeView) SetSize(w, h int) {
	m.viewport.Width = w
	m.viewport.Height = h - 2
	if m.viewport.Height < 1 {
		m.viewport.Height = 1
	}
}

func renderTaskDetail(d domain.ECSTaskDetail) string {
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
	kvTime := func(k string, t *time.Time) {
		if t == nil || t.IsZero() {
			return
		}
		kv(k, t.Local().Format("2006-01-02 15:04:05"))
	}

	b.WriteString(ui.TitleStyle.Render(shortARN(d.TaskARN)) + "\n")
	kv("Task ARN", d.TaskARN)
	kv("Task Definition", d.TaskDefARN)
	kv("Cluster ARN", d.ClusterARN)
	kv("Group", d.Group)
	kv("Started By", d.StartedBy)
	kv("Launch Type", d.LaunchType)
	kv("Platform Version", d.PlatformVersion)
	kv("Platform Family", d.PlatformFamily)
	kv("Capacity Provider", d.CapacityProviderName)
	kv("Availability Zone", d.AvailabilityZone)
	if d.ContainerInstanceARN != "" {
		kv("Container Instance", d.ContainerInstanceARN)
	}
	kv("Version", fmt.Sprintf("%d", d.Version))
	kv("Exec Enabled", fmt.Sprintf("%t", d.EnableExecuteCommand))

	section("Status")
	kv("Last Status", d.LastStatus)
	kv("Desired Status", d.DesiredStatus)
	kv("Health Status", d.HealthStatus)
	kv("Connectivity", d.Connectivity)
	kvTime("Connectivity At", d.ConnectivityAt)
	if d.StoppedReason != "" {
		kv("Stopped Reason", d.StoppedReason)
	}
	if d.StopCode != "" {
		kv("Stop Code", d.StopCode)
	}

	section("Resources")
	kv("CPU", d.CPU)
	kv("Memory", d.Memory)

	section("Timeline")
	kvTime("Created", d.CreatedAt)
	kvTime("Pull Started", d.PullStartedAt)
	kvTime("Pull Stopped", d.PullStoppedAt)
	kvTime("Started", d.StartedAt)
	kvTime("Stopped", d.StoppedAt)

	if len(d.Containers) > 0 {
		section("Containers")
		for _, c := range d.Containers {
			b.WriteString("  " + ui.TitleStyle.Render(c.Name) + "\n")
			b.WriteString(fmt.Sprintf("    image:      %s\n", c.Image))
			if c.ImageDigest != "" {
				b.WriteString(fmt.Sprintf("    digest:     %s\n", c.ImageDigest))
			}
			if c.RuntimeID != "" {
				b.WriteString(fmt.Sprintf("    runtime id: %s\n", c.RuntimeID))
			}
			b.WriteString(fmt.Sprintf("    status:     %s", c.LastStatus))
			if c.HealthStatus != "" {
				b.WriteString("  health=" + c.HealthStatus)
			}
			if c.ExitCode != nil {
				b.WriteString(fmt.Sprintf("  exit=%d", *c.ExitCode))
			}
			b.WriteString("\n")
			if c.Reason != "" {
				b.WriteString("    reason:     " + c.Reason + "\n")
			}
			if c.CPU != "" || c.Memory != "" || c.MemoryReservation != "" {
				b.WriteString(fmt.Sprintf("    cpu=%s memory=%s reservation=%s\n", c.CPU, c.Memory, c.MemoryReservation))
			}
			for _, ni := range c.NetworkInterfaces {
				b.WriteString(fmt.Sprintf("    eni:        attach=%s ipv4=%s ipv6=%s\n",
					ni.AttachmentID, ni.PrivateIPv4Address, ni.IPv6Address))
			}
		}
	}

	if len(d.Attachments) > 0 {
		section("Attachments")
		for _, a := range d.Attachments {
			b.WriteString(fmt.Sprintf("  %s (%s) [%s]\n", a.Type, a.ID, a.Status))
			keys := make([]string, 0, len(a.Details))
			for k := range a.Details {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				b.WriteString(fmt.Sprintf("    %-24s %s\n", k, a.Details[k]))
			}
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

func shortARN(arn string) string {
	if i := strings.LastIndex(arn, "/"); i >= 0 {
		return arn[i+1:]
	}
	return arn
}
