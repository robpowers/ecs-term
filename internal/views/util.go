package views

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/robpowers/ecs-term/internal/model"
)

func tickEvery(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return model.RefreshTickMsg{T: t}
	})
}
