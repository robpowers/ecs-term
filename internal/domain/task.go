package domain

import "time"

type ECSTask struct {
	TaskARN       string
	TaskDefARN    string
	ShortID       string
	LastStatus    string
	HealthStatus  string
	StartedAt     *time.Time
	StoppedReason string
	Containers    []ContainerSummary
	Group         string
	CPU           string
	Memory        string
}

type ContainerSummary struct {
	Name      string
	Status    string
	Health    string
	ExitCode  *int32
	LogGroup  string
	LogStream string
}
