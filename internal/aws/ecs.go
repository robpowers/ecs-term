package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/robpowers/ecs-term/internal/domain"
)

func (c *ClientSet) ListServices(ctx context.Context, cluster string) ([]domain.ECSService, error) {
	var arns []string
	var nextToken *string
	for {
		out, err := c.ECS.ListServices(ctx, &ecs.ListServicesInput{
			Cluster:    aws.String(cluster),
			MaxResults: aws.Int32(100),
			NextToken:  nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("ListServices: %w", err)
		}
		arns = append(arns, out.ServiceArns...)
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}

	if len(arns) == 0 {
		return nil, nil
	}

	var services []domain.ECSService
	for i := 0; i < len(arns); i += 10 {
		end := i + 10
		if end > len(arns) {
			end = len(arns)
		}
		out, err := c.ECS.DescribeServices(ctx, &ecs.DescribeServicesInput{
			Cluster:  aws.String(cluster),
			Services: arns[i:end],
			Include:  []ecstypes.ServiceField{ecstypes.ServiceFieldTags},
		})
		if err != nil {
			return nil, fmt.Errorf("DescribeServices: %w", err)
		}
		for _, svc := range out.Services {
			services = append(services, mapService(svc))
		}
	}
	return services, nil
}

func mapService(svc ecstypes.Service) domain.ECSService {
	name := aws.ToString(svc.ServiceName)
	status := aws.ToString(svc.Status)
	desired := svc.DesiredCount
	running := svc.RunningCount
	pending := svc.PendingCount
	healthy := running == desired && pending == 0 && strings.EqualFold(status, "ACTIVE")

	var created time.Time
	if svc.CreatedAt != nil {
		created = *svc.CreatedAt
	}

	return domain.ECSService{
		Name:         name,
		Status:       status,
		DesiredCount: desired,
		RunningCount: running,
		PendingCount: pending,
		TaskDefARN:   aws.ToString(svc.TaskDefinition),
		CreatedAt:    created,
		IsHealthy:    healthy,
	}
}

func (c *ClientSet) ListTasks(ctx context.Context, cluster, serviceName string) ([]domain.ECSTask, error) {
	var arns []string
	var nextToken *string
	for {
		out, err := c.ECS.ListTasks(ctx, &ecs.ListTasksInput{
			Cluster:     aws.String(cluster),
			ServiceName: aws.String(serviceName),
			NextToken:   nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("ListTasks: %w", err)
		}
		arns = append(arns, out.TaskArns...)
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}

	if len(arns) == 0 {
		return nil, nil
	}

	out, err := c.ECS.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: aws.String(cluster),
		Tasks:   arns,
		Include: []ecstypes.TaskField{ecstypes.TaskFieldTags},
	})
	if err != nil {
		return nil, fmt.Errorf("DescribeTasks: %w", err)
	}

	var tasks []domain.ECSTask
	for _, t := range out.Tasks {
		tasks = append(tasks, mapTask(t))
	}
	return tasks, nil
}

func mapTask(t ecstypes.Task) domain.ECSTask {
	arn := aws.ToString(t.TaskArn)
	shortID := arn
	if parts := strings.Split(arn, "/"); len(parts) > 0 {
		shortID = parts[len(parts)-1]
	}
	if len(shortID) > 12 {
		shortID = shortID[:12]
	}

	var containers []domain.ContainerSummary
	for _, c := range t.Containers {
		cs := domain.ContainerSummary{
			Name:   aws.ToString(c.Name),
			Status: aws.ToString(c.LastStatus),
			Health: string(c.HealthStatus),
		}
		if c.ExitCode != nil {
			cs.ExitCode = c.ExitCode
		}
		// extract log config from container overrides if available
		containers = append(containers, cs)
	}

	task := domain.ECSTask{
		TaskARN:      arn,
		ShortID:      shortID,
		LastStatus:   aws.ToString(t.LastStatus),
		HealthStatus: string(t.HealthStatus),
		StartedAt:    t.StartedAt,
		Group:        aws.ToString(t.Group),
		CPU:          aws.ToString(t.Cpu),
		Memory:       aws.ToString(t.Memory),
		Containers:   containers,
	}
	if t.StoppedReason != nil {
		task.StoppedReason = *t.StoppedReason
	}
	return task
}

func (c *ClientSet) DescribeTaskDefinition(ctx context.Context, taskDefARN string) ([]domain.ContainerDetail, error) {
	out, err := c.ECS.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(taskDefARN),
	})
	if err != nil {
		return nil, fmt.Errorf("DescribeTaskDefinition: %w", err)
	}

	var details []domain.ContainerDetail
	for _, cd := range out.TaskDefinition.ContainerDefinitions {
		detail := domain.ContainerDetail{
			Name:  aws.ToString(cd.Name),
			Image: aws.ToString(cd.Image),
		}
		if cd.Cpu != 0 {
			detail.CPU = int(cd.Cpu)
		}
		if cd.Memory != nil {
			detail.MemoryMB = int(*cd.Memory)
		}
		if cd.MemoryReservation != nil {
			detail.MemoryReserveMB = int(*cd.MemoryReservation)
		}
		for _, e := range cd.Environment {
			detail.EnvVars = append(detail.EnvVars, domain.EnvVar{
				Name:  aws.ToString(e.Name),
				Value: aws.ToString(e.Value),
			})
		}
		for _, pm := range cd.PortMappings {
			mapping := domain.PortMapping{Protocol: string(pm.Protocol)}
			if pm.ContainerPort != nil {
				mapping.ContainerPort = *pm.ContainerPort
			}
			if pm.HostPort != nil {
				mapping.HostPort = *pm.HostPort
			}
			detail.PortMappings = append(detail.PortMappings, mapping)
		}
		if cd.HealthCheck != nil {
			hc := &domain.HealthCheckConfig{
				Command: cd.HealthCheck.Command,
			}
			if cd.HealthCheck.Interval != nil {
				hc.IntervalSec = int(*cd.HealthCheck.Interval)
			}
			if cd.HealthCheck.Timeout != nil {
				hc.TimeoutSec = int(*cd.HealthCheck.Timeout)
			}
			if cd.HealthCheck.Retries != nil {
				hc.Retries = int(*cd.HealthCheck.Retries)
			}
			if cd.HealthCheck.StartPeriod != nil {
				hc.StartPeriod = int(*cd.HealthCheck.StartPeriod)
			}
			detail.HealthCheck = hc
		}

		// extract log group/stream prefix from log configuration
		if cd.LogConfiguration != nil {
			opts := cd.LogConfiguration.Options
			if opts != nil {
				if lg, ok := opts["awslogs-group"]; ok {
					_ = lg // stored on ContainerDetail if we add a field later
				}
			}
		}

		details = append(details, detail)
	}
	return details, nil
}

// GetTaskLogConfig returns the CloudWatch log group and stream prefix for a container.
func (c *ClientSet) GetTaskLogConfig(ctx context.Context, taskDefARN, containerName string) (logGroup, streamPrefix string, err error) {
	out, err := c.ECS.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(taskDefARN),
	})
	if err != nil {
		return "", "", fmt.Errorf("DescribeTaskDefinition: %w", err)
	}
	for _, cd := range out.TaskDefinition.ContainerDefinitions {
		if aws.ToString(cd.Name) != containerName {
			continue
		}
		if cd.LogConfiguration == nil || cd.LogConfiguration.Options == nil {
			return "", "", fmt.Errorf("container %q has no awslogs log configuration", containerName)
		}
		opts := cd.LogConfiguration.Options
		return opts["awslogs-group"], opts["awslogs-stream-prefix"], nil
	}
	return "", "", fmt.Errorf("container %q not found in task definition", containerName)
}
