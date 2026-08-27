package domain

import "time"

type ECSService struct {
	Name             string
	Status           string
	DesiredCount     int32
	RunningCount     int32
	PendingCount     int32
	TaskDefARN       string
	CreatedAt        time.Time
	LastDeploymentAt time.Time
	IsHealthy        bool
}
