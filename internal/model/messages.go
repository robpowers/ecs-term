package model

import (
	"time"

	"github.com/robpowers/ecs-term/internal/domain"
)

// Navigation
type NavigatePushMsg struct{ View View }
type NavigatePopMsg struct{}

// ContextSelectedMsg notifies AppModel of the chosen context's details.
type ContextSelectedMsg struct {
	Name    string
	Region  string
	Profile string
	Cluster string
}

// AWS data loaded
type AccountIDMsg struct {
	ID  string
	Err error
}

type ServicesLoadedMsg struct {
	Services []domain.ECSService
	Err      error
}

type TasksLoadedMsg struct {
	Tasks []domain.ECSTask
	Err   error
}

type LogEventsMsg struct {
	Events []domain.LogEvent
	Err    error
}

type ContainerDetailMsg struct {
	Details []domain.ContainerDetail
	Err     error
}

type LogConfigMsg struct {
	LogGroup    string
	StreamPrefix string
	TaskARN     string
	ContainerName string
	Err         error
}

// Refresh tick
type RefreshTickMsg struct{ T time.Time }

// Error overlay dismissed
type ErrorDismissedMsg struct{}
