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
	// Replace indicates the receiver should overwrite its buffer with Events
	// rather than appending. Used for initial loads and manual refreshes;
	// tail-poll results leave Replace false so new events are appended.
	Replace bool
	Err     error
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

type ServiceDetailMsg struct {
	Detail domain.ECSServiceDetail
	Err    error
}

type TaskDetailMsg struct {
	Detail domain.ECSTaskDetail
	Err    error
}

type ClusterTasksLoadedMsg struct {
	Tasks []domain.ECSTask
	Err   error
}

// ContainerPickedMsg is emitted by ContainerPickerView after the user selects
// a container to shell into.
type ContainerPickedMsg struct {
	TaskARN string
	Name    string
}

// ExecFinishedMsg is emitted after `aws ecs execute-command` returns control
// to the TUI.
type ExecFinishedMsg struct{ Err error }

// Refresh tick
type RefreshTickMsg struct{ T time.Time }

// Error overlay dismissed
type ErrorDismissedMsg struct{}
