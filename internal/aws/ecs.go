package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"gopkg.in/yaml.v3"

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

	var lastDeploy time.Time
	for _, dep := range svc.Deployments {
		t := dep.UpdatedAt
		if t == nil {
			t = dep.CreatedAt
		}
		if t == nil {
			continue
		}
		if t.After(lastDeploy) {
			lastDeploy = *t
		}
	}

	return domain.ECSService{
		Name:             name,
		Status:           status,
		DesiredCount:     desired,
		RunningCount:     running,
		PendingCount:     pending,
		TaskDefARN:       aws.ToString(svc.TaskDefinition),
		CreatedAt:        created,
		LastDeploymentAt: lastDeploy,
		IsHealthy:        healthy,
	}
}

// ListTasksOpts controls filtering for ListTasks.
type ListTasksOpts struct {
	// ServiceName limits to tasks belonging to a specific service. Empty means all
	// tasks in the cluster.
	ServiceName string
	// DesiredStatus filters by task desired status (e.g. "RUNNING", "STOPPED"). Empty means the AWS default.
	DesiredStatus string
}

func (c *ClientSet) ListTasks(ctx context.Context, cluster string, opts ListTasksOpts) ([]domain.ECSTask, error) {
	var arns []string
	var nextToken *string
	for {
		input := &ecs.ListTasksInput{
			Cluster:   aws.String(cluster),
			NextToken: nextToken,
		}
		if opts.ServiceName != "" {
			input.ServiceName = aws.String(opts.ServiceName)
		}
		if opts.DesiredStatus != "" {
			input.DesiredStatus = ecstypes.DesiredStatus(opts.DesiredStatus)
		}
		out, err := c.ECS.ListTasks(ctx, input)
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

	var tasks []domain.ECSTask
	for i := 0; i < len(arns); i += 100 {
		end := i + 100
		if end > len(arns) {
			end = len(arns)
		}
		out, err := c.ECS.DescribeTasks(ctx, &ecs.DescribeTasksInput{
			Cluster: aws.String(cluster),
			Tasks:   arns[i:end],
			Include: []ecstypes.TaskField{ecstypes.TaskFieldTags},
		})
		if err != nil {
			return nil, fmt.Errorf("DescribeTasks: %w", err)
		}
		for _, t := range out.Tasks {
			tasks = append(tasks, mapTask(t))
		}
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
		TaskDefARN:   aws.ToString(t.TaskDefinitionArn),
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
		for _, s := range cd.Secrets {
			detail.Secrets = append(detail.Secrets, domain.Secret{
				Name:      aws.ToString(s.Name),
				ValueFrom: aws.ToString(s.ValueFrom),
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

// GetTaskDefinitionRaw returns the full task definition marshaled as both
// JSON and YAML text. The raw SDK type never leaves this package — only the
// marshaled strings are returned.
//
// yaml.v3 marshals via reflection and can panic (reflect.Value.Interface:
// cannot return value obtained from unexported field) when it walks into the
// AWS SDK type's unexported internals. To avoid that, YAML is produced from
// a JSON round-trip instead: json.Marshal only serializes exported,
// json-tagged fields, so unmarshaling that JSON into a plain
// map[string]interface{}/[]interface{} tree gives yaml.v3 nothing but plain
// data to walk. safeMarshalYAML is still wrapped in a recover as a backstop
// so a marshaling failure surfaces as an error instead of crashing the TUI.
func (c *ClientSet) GetTaskDefinitionRaw(ctx context.Context, taskDefARN string) (jsonText, yamlText string, err error) {
	out, err := c.ECS.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(taskDefARN),
	})
	if err != nil {
		return "", "", fmt.Errorf("DescribeTaskDefinition: %w", err)
	}

	jsonBytes, err := json.MarshalIndent(out.TaskDefinition, "", "  ")
	if err != nil {
		return "", "", fmt.Errorf("marshal task definition as json: %w", err)
	}

	yamlBytes, err := safeMarshalYAMLFromJSON(jsonBytes)
	if err != nil {
		return "", "", fmt.Errorf("marshal task definition as yaml: %w", err)
	}
	return string(jsonBytes), string(yamlBytes), nil
}

// safeMarshalYAMLFromJSON re-encodes JSON as YAML via a plain interface{}
// tree (see GetTaskDefinitionRaw) and recovers from any panic in the
// underlying yaml.v3 reflection walk, converting it to an error.
func safeMarshalYAMLFromJSON(jsonBytes []byte) (yamlBytes []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("yaml marshal panicked: %v", r)
		}
	}()

	var generic interface{}
	if err := json.Unmarshal(jsonBytes, &generic); err != nil {
		return nil, fmt.Errorf("unmarshal json for yaml conversion: %w", err)
	}
	return yaml.Marshal(generic)
}

// DescribeServiceFull returns a rich, describe-style view of a single service.
func (c *ClientSet) DescribeServiceFull(ctx context.Context, cluster, serviceName string) (domain.ECSServiceDetail, error) {
	out, err := c.ECS.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String(cluster),
		Services: []string{serviceName},
		Include:  []ecstypes.ServiceField{ecstypes.ServiceFieldTags},
	})
	if err != nil {
		return domain.ECSServiceDetail{}, fmt.Errorf("DescribeServices: %w", err)
	}
	if len(out.Services) == 0 {
		return domain.ECSServiceDetail{}, fmt.Errorf("service %q not found", serviceName)
	}
	return mapServiceDetail(out.Services[0]), nil
}

func mapServiceDetail(svc ecstypes.Service) domain.ECSServiceDetail {
	d := domain.ECSServiceDetail{
		Name:               aws.ToString(svc.ServiceName),
		Status:             aws.ToString(svc.Status),
		ServiceARN:         aws.ToString(svc.ServiceArn),
		ClusterARN:         aws.ToString(svc.ClusterArn),
		TaskDefARN:         aws.ToString(svc.TaskDefinition),
		DesiredCount:       svc.DesiredCount,
		RunningCount:       svc.RunningCount,
		PendingCount:       svc.PendingCount,
		LaunchType:         string(svc.LaunchType),
		PlatformVersion:    aws.ToString(svc.PlatformVersion),
		PlatformFamily:     aws.ToString(svc.PlatformFamily),
		SchedulingStrategy: string(svc.SchedulingStrategy),
		RoleARN:            aws.ToString(svc.RoleArn),
		EnableExecuteCommand: svc.EnableExecuteCommand,
		PropagateTags:      string(svc.PropagateTags),
	}
	if svc.CreatedAt != nil {
		d.CreatedAt = *svc.CreatedAt
	}
	if svc.DeploymentController != nil {
		d.DeploymentController = string(svc.DeploymentController.Type)
	}
	for _, dep := range svc.Deployments {
		item := domain.Deployment{
			ID:                 aws.ToString(dep.Id),
			Status:             aws.ToString(dep.Status),
			TaskDefARN:         aws.ToString(dep.TaskDefinition),
			DesiredCount:       dep.DesiredCount,
			PendingCount:       dep.PendingCount,
			RunningCount:       dep.RunningCount,
			FailedTasks:        dep.FailedTasks,
			RolloutState:       string(dep.RolloutState),
			RolloutStateReason: aws.ToString(dep.RolloutStateReason),
		}
		if dep.CreatedAt != nil {
			item.CreatedAt = *dep.CreatedAt
		}
		if dep.UpdatedAt != nil {
			item.UpdatedAt = *dep.UpdatedAt
		}
		d.Deployments = append(d.Deployments, item)
	}
	for _, ev := range svc.Events {
		e := domain.ServiceEvent{
			ID:      aws.ToString(ev.Id),
			Message: aws.ToString(ev.Message),
		}
		if ev.CreatedAt != nil {
			e.CreatedAt = *ev.CreatedAt
		}
		d.Events = append(d.Events, e)
	}
	for _, lb := range svc.LoadBalancers {
		item := domain.LoadBalancerRef{
			TargetGroupARN:   aws.ToString(lb.TargetGroupArn),
			LoadBalancerName: aws.ToString(lb.LoadBalancerName),
			ContainerName:    aws.ToString(lb.ContainerName),
		}
		if lb.ContainerPort != nil {
			item.ContainerPort = *lb.ContainerPort
		}
		d.LoadBalancers = append(d.LoadBalancers, item)
	}
	for _, sr := range svc.ServiceRegistries {
		item := domain.ServiceRegistry{
			RegistryARN:   aws.ToString(sr.RegistryArn),
			ContainerName: aws.ToString(sr.ContainerName),
		}
		if sr.Port != nil {
			item.Port = *sr.Port
		}
		if sr.ContainerPort != nil {
			item.ContainerPort = *sr.ContainerPort
		}
		d.ServiceRegistries = append(d.ServiceRegistries, item)
	}
	if svc.NetworkConfiguration != nil && svc.NetworkConfiguration.AwsvpcConfiguration != nil {
		nc := svc.NetworkConfiguration.AwsvpcConfiguration
		d.NetworkConfig = &domain.NetworkConfig{
			Subnets:        nc.Subnets,
			SecurityGroups: nc.SecurityGroups,
			AssignPublicIP: string(nc.AssignPublicIp),
		}
	}
	for _, cp := range svc.CapacityProviderStrategy {
		d.CapacityProviderStrategy = append(d.CapacityProviderStrategy, domain.CapacityProviderItem{
			Name:   aws.ToString(cp.CapacityProvider),
			Base:   cp.Base,
			Weight: cp.Weight,
		})
	}
	for _, t := range svc.Tags {
		d.Tags = append(d.Tags, domain.Tag{Key: aws.ToString(t.Key), Value: aws.ToString(t.Value)})
	}
	return d
}

// DescribeTaskFull returns a rich, describe-style view of a single task.
func (c *ClientSet) DescribeTaskFull(ctx context.Context, cluster, taskARN string) (domain.ECSTaskDetail, error) {
	out, err := c.ECS.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: aws.String(cluster),
		Tasks:   []string{taskARN},
		Include: []ecstypes.TaskField{ecstypes.TaskFieldTags},
	})
	if err != nil {
		return domain.ECSTaskDetail{}, fmt.Errorf("DescribeTasks: %w", err)
	}
	if len(out.Tasks) == 0 {
		return domain.ECSTaskDetail{}, fmt.Errorf("task %q not found", taskARN)
	}
	return mapTaskDetail(out.Tasks[0]), nil
}

func mapTaskDetail(t ecstypes.Task) domain.ECSTaskDetail {
	d := domain.ECSTaskDetail{
		TaskARN:              aws.ToString(t.TaskArn),
		TaskDefARN:           aws.ToString(t.TaskDefinitionArn),
		ClusterARN:           aws.ToString(t.ClusterArn),
		ContainerInstanceARN: aws.ToString(t.ContainerInstanceArn),
		LastStatus:           aws.ToString(t.LastStatus),
		DesiredStatus:        aws.ToString(t.DesiredStatus),
		HealthStatus:         string(t.HealthStatus),
		LaunchType:           string(t.LaunchType),
		PlatformVersion:      aws.ToString(t.PlatformVersion),
		PlatformFamily:       aws.ToString(t.PlatformFamily),
		Connectivity:         string(t.Connectivity),
		ConnectivityAt:       t.ConnectivityAt,
		AvailabilityZone:     aws.ToString(t.AvailabilityZone),
		CapacityProviderName: aws.ToString(t.CapacityProviderName),
		CPU:                  aws.ToString(t.Cpu),
		Memory:               aws.ToString(t.Memory),
		Group:                aws.ToString(t.Group),
		StartedBy:            aws.ToString(t.StartedBy),
		EnableExecuteCommand: t.EnableExecuteCommand,
		PullStartedAt:        t.PullStartedAt,
		PullStoppedAt:        t.PullStoppedAt,
		StartedAt:            t.StartedAt,
		StoppedAt:            t.StoppedAt,
		CreatedAt:            t.CreatedAt,
		StoppedReason:        aws.ToString(t.StoppedReason),
		StopCode:             string(t.StopCode),
		Version:              t.Version,
	}
	for _, c := range t.Containers {
		cr := domain.ContainerRuntime{
			Name:              aws.ToString(c.Name),
			RuntimeID:         aws.ToString(c.RuntimeId),
			Image:             aws.ToString(c.Image),
			ImageDigest:       aws.ToString(c.ImageDigest),
			LastStatus:        aws.ToString(c.LastStatus),
			HealthStatus:      string(c.HealthStatus),
			ExitCode:          c.ExitCode,
			Reason:            aws.ToString(c.Reason),
			CPU:               aws.ToString(c.Cpu),
			Memory:            aws.ToString(c.Memory),
			MemoryReservation: aws.ToString(c.MemoryReservation),
		}
		for _, ni := range c.NetworkInterfaces {
			cr.NetworkInterfaces = append(cr.NetworkInterfaces, domain.NetworkInterface{
				AttachmentID:       aws.ToString(ni.AttachmentId),
				PrivateIPv4Address: aws.ToString(ni.PrivateIpv4Address),
				IPv6Address:        aws.ToString(ni.Ipv6Address),
			})
		}
		d.Containers = append(d.Containers, cr)
	}
	for _, a := range t.Attachments {
		item := domain.Attachment{
			ID:      aws.ToString(a.Id),
			Type:    aws.ToString(a.Type),
			Status:  aws.ToString(a.Status),
			Details: make(map[string]string, len(a.Details)),
		}
		for _, kv := range a.Details {
			item.Details[aws.ToString(kv.Name)] = aws.ToString(kv.Value)
		}
		d.Attachments = append(d.Attachments, item)
	}
	for _, tg := range t.Tags {
		d.Tags = append(d.Tags, domain.Tag{Key: aws.ToString(tg.Key), Value: aws.ToString(tg.Value)})
	}
	return d
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
