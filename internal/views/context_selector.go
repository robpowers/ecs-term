package views

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	awsclient "github.com/robpowers/ecs-term/internal/aws"
	"github.com/robpowers/ecs-term/internal/config"
	"github.com/robpowers/ecs-term/internal/model"
	"github.com/robpowers/ecs-term/internal/ui"
)

type contextItem struct{ ctx config.Context }

func (i contextItem) Title() string       { return i.ctx.Name }
func (i contextItem) Description() string {
	return fmt.Sprintf("cluster: %s  region: %s  profile: %s", i.ctx.Cluster, i.ctx.Region, i.ctx.AWSProfile)
}
func (i contextItem) FilterValue() string { return i.ctx.Name }

type ContextSelector struct {
	list    list.Model
	items   []config.Context
	clients map[string]*awsclient.ClientSet // pre-built per context (set externally)
	cfg     *config.Config
}

func NewContextSelector(cfg *config.Config, clients map[string]*awsclient.ClientSet) ContextSelector {
	items := make([]list.Item, len(cfg.Contexts))
	for i, ctx := range cfg.Contexts {
		items[i] = contextItem{ctx}
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = ui.SelectedStyle
	delegate.Styles.SelectedDesc = ui.SelectedStyle.Foreground(lipgloss.Color("#333333"))

	l := list.New(items, delegate, 0, 0)
	l.Title = "Select a context"
	l.Styles.Title = ui.TitleStyle
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.KeyMap.Quit.SetKeys() // we handle q globally

	// Pre-select current_context if set
	if cfg.CurrentContext != "" {
		for i, ctx := range cfg.Contexts {
			if ctx.Name == cfg.CurrentContext {
				l.Select(i)
				break
			}
		}
	}

	return ContextSelector{
		list:    l,
		items:   cfg.Contexts,
		clients: clients,
		cfg:     cfg,
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
			if i := m.list.SelectedItem(); i != nil {
				ctx := i.(contextItem).ctx
				clients := m.clients[ctx.Name]
				sv := NewServicesView(ctx, clients)
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
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *ContextSelector) View() string {
	return m.list.View()
}

func (m *ContextSelector) SetSize(w, h int) {
	m.list.SetSize(w, h)
}
