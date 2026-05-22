package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"

	"github.com/robpowers/ecs-term/internal/domain"
)

// GetLogStreams returns log streams matching a prefix in the given log group.
func (c *ClientSet) GetLogStreams(ctx context.Context, logGroup, prefix string) ([]string, error) {
	input := &cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName:        aws.String(logGroup),
		LogStreamNamePrefix: aws.String(prefix),
		OrderBy:             "LogStreamName",
		Descending:          aws.Bool(true),
	}
	out, err := c.CWLogs.DescribeLogStreams(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("DescribeLogStreams: %w", err)
	}
	var names []string
	for _, s := range out.LogStreams {
		names = append(names, aws.ToString(s.LogStreamName))
	}
	return names, nil
}

// GetRecentLogs fetches the most recent log events from a log stream.
// If sinceTime is non-zero, only events after that time are returned.
func (c *ClientSet) GetRecentLogs(ctx context.Context, logGroup, logStream string, limit int32, sinceTime time.Time) ([]domain.LogEvent, error) {
	input := &cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  aws.String(logGroup),
		LogStreamName: aws.String(logStream),
		Limit:         aws.Int32(limit),
		StartFromHead: aws.Bool(false),
	}
	if !sinceTime.IsZero() {
		ms := sinceTime.UnixMilli()
		input.StartTime = aws.Int64(ms)
		input.StartFromHead = aws.Bool(true)
	}

	out, err := c.CWLogs.GetLogEvents(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("GetLogEvents: %w", err)
	}

	var events []domain.LogEvent
	for _, e := range out.Events {
		ts := time.Time{}
		if e.Timestamp != nil {
			ts = time.UnixMilli(*e.Timestamp)
		}
		msg := aws.ToString(e.Message)
		msg = strings.TrimRight(msg, "\n")
		events = append(events, domain.LogEvent{
			Timestamp:  ts,
			Message:    msg,
			StreamName: logStream,
		})
	}
	return events, nil
}

// BuildLogStreamName constructs the expected CloudWatch log stream name for a task container.
// Format: <prefix>/<containerName>/<taskID>
func BuildLogStreamName(prefix, containerName, taskID string) string {
	// short task ID = last segment of task ARN
	parts := strings.Split(taskID, "/")
	id := parts[len(parts)-1]
	if prefix == "" {
		return fmt.Sprintf("%s/%s", containerName, id)
	}
	return fmt.Sprintf("%s/%s/%s", prefix, containerName, id)
}
