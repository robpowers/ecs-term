package domain

import "time"

type LogEvent struct {
	Timestamp  time.Time
	Message    string
	StreamName string
}
