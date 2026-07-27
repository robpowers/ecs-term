package views

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/robpowers/ecs-term/internal/model"
	"github.com/robpowers/ecs-term/internal/ui"
)

// ContainerPickerView is a small centered menu used to select a container
// before opening a shell in a multi-container task.
type ContainerPickerView struct {
	taskARN    string
	containers []string
	cursor     int
	width      int
	height     int
}

func NewContainerPickerView(taskARN string, containers []string) ContainerPickerView {
	return ContainerPickerView{
		taskARN:    taskARN,
		containers: containers,
	}
}

func (m *ContainerPickerView) ViewID() model.ViewID { return model.ViewContainerPicker }

func (m *ContainerPickerView) KeyHints() []string {
	return []string{"↑/k:up", "↓/j:down", "enter:shell", "esc:cancel", "q:quit"}
}

func (m *ContainerPickerView) Init() tea.Cmd { return nil }

func (m *ContainerPickerView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, model.GlobalKeys.Back):
			return m, func() tea.Msg { return model.NavigatePopMsg{} }
		case key.Matches(msg, model.GlobalKeys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, model.GlobalKeys.Down):
			if m.cursor < len(m.containers)-1 {
				m.cursor++
			}
		case key.Matches(msg, model.GlobalKeys.Enter):
			if m.cursor < 0 || m.cursor >= len(m.containers) {
				return m, nil
			}
			name := m.containers[m.cursor]
			taskARN := m.taskARN
			return m, func() tea.Msg { return model.ContainerPickedMsg{TaskARN: taskARN, Name: name} }
		}
	}
	return m, nil
}

func (m *ContainerPickerView) View() string {
	var b strings.Builder
	b.WriteString(ui.TitleStyle.Render("Select a container") + "\n\n")
	for i, name := range m.containers {
		if i == m.cursor {
			b.WriteString("  " + ui.SelectedStyle.Render("▸ "+name) + "\n")
		} else {
			b.WriteString("    " + name + "\n")
		}
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.ColorPrimary).
		Padding(1, 2).
		Render(b.String())

	if m.width == 0 || m.height == 0 {
		return box
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m *ContainerPickerView) SetSize(w, h int) {
	m.width = w
	m.height = h
}
