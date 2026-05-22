package model

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	awsclient "github.com/robpowers/ecs-term/internal/aws"
	"github.com/robpowers/ecs-term/internal/config"
	"github.com/robpowers/ecs-term/internal/ui"
)

type AppModel struct {
	navigator  Navigator
	statusBar  ui.StatusBar
	errOverlay ui.ErrorOverlay
	width      int
	height     int
	allClients map[string]*awsclient.ClientSet
	cfg        *config.Config
}

func NewAppModel(cfg *config.Config, initialView View, clients map[string]*awsclient.ClientSet) AppModel {
	return AppModel{
		navigator:  NewNavigator(initialView),
		cfg:        cfg,
		allClients: clients,
		statusBar: ui.StatusBar{
			ContextName: "—",
			AccountID:   "—",
			Region:      "—",
			KeyHints:    initialView.KeyHints(),
		},
	}
}

// NewAppModelWithInitialPush creates an AppModel with an extra view already pushed
// on top of the context selector (used for single-context auto-navigation).
func NewAppModelWithInitialPush(cfg *config.Config, initialView View, pushed View, clients map[string]*awsclient.ClientSet) AppModel {
	m := NewAppModel(cfg, initialView, clients)
	m.navigator.Push(pushed)
	m.statusBar.KeyHints = pushed.KeyHints()
	return m
}

func (m AppModel) Init() tea.Cmd {
	return m.navigator.Current().Init()
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Error overlay intercepts Enter/Esc when visible
	if m.errOverlay.Visible {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if key.Matches(msg, GlobalKeys.Enter) || key.Matches(msg, GlobalKeys.Back) {
				m.errOverlay.Hide()
				return m, nil
			}
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.statusBar.SetWidth(msg.Width)
		contentH := msg.Height - 2
		if contentH < 1 {
			contentH = 1
		}
		m.navigator.SetSizeAll(msg.Width, contentH)
		return m, nil

	case tea.KeyMsg:
		if key.Matches(msg, GlobalKeys.Quit) {
			return m, tea.Quit
		}

	case NavigatePushMsg:
		if m.width > 0 {
			contentH := m.height - 2
			if contentH < 1 {
				contentH = 1
			}
			msg.View.SetSize(m.width, contentH)
		}
		m.navigator.Push(msg.View)
		m.statusBar.KeyHints = m.navigator.Current().KeyHints()
		return m, m.navigator.Current().Init()

	case NavigatePopMsg:
		if m.navigator.Depth() <= 1 {
			return m, tea.Quit
		}
		m.navigator.Pop()
		m.statusBar.KeyHints = m.navigator.Current().KeyHints()
		return m, m.navigator.Current().Init()

	case ContextSelectedMsg:
		m.statusBar.ContextName = msg.Name
		m.statusBar.Region = msg.Region
		m.statusBar.AccountID = "loading…"
		if cs, ok := m.allClients[msg.Name]; ok {
			return m, FetchAccountIDCmd(cs)
		}
		return m, nil

	case AccountIDMsg:
		if msg.Err == nil {
			m.statusBar.AccountID = msg.ID
		}
		return m, nil

	case ServicesLoadedMsg:
		if msg.Err != nil {
			m.errOverlay.Show(msg.Err.Error())
		} else {
			m.statusBar.LastRefresh = time.Now()
		}

	case TasksLoadedMsg:
		if msg.Err != nil {
			m.errOverlay.Show(msg.Err.Error())
		} else {
			m.statusBar.LastRefresh = time.Now()
		}

	case LogEventsMsg:
		if msg.Err != nil {
			m.errOverlay.Show(msg.Err.Error())
		} else {
			m.statusBar.LastRefresh = time.Now()
		}

	case ContainerDetailMsg:
		if msg.Err != nil {
			m.errOverlay.Show(msg.Err.Error())
		} else {
			m.statusBar.LastRefresh = time.Now()
		}
	}

	// Delegate to current view
	updated, cmd := m.navigator.Current().Update(msg)
	// The View interface returns tea.Model; we need to store it back.
	// Since views are stored as View (interface) in the stack, we
	// replace the top of the stack with the updated model.
	if v, ok := updated.(View); ok {
		m.navigator.stack[len(m.navigator.stack)-1] = v
	}
	return m, cmd
}

func (m AppModel) View() string {
	if m.width == 0 {
		return "Loading…"
	}
	contentH := m.height - 2
	if contentH < 1 {
		contentH = 1
	}
	content := m.navigator.Current().View()
	full := lipgloss.JoinVertical(lipgloss.Left,
		m.statusBar.Header(),
		content,
		m.statusBar.Footer(),
	)
	return m.errOverlay.Render(full, m.width, m.height)
}

// FetchAccountIDCmd fetches the AWS account ID and returns it as AccountIDMsg.
func FetchAccountIDCmd(clients *awsclient.ClientSet) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		id, err := clients.FetchAccountID(ctx)
		return AccountIDMsg{ID: id, Err: err}
	}
}
