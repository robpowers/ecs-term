package domain

import "time"

// ECSServiceDetail carries the full DescribeServices output for the
// service-describe view. Kept separate from ECSService (list row) so that
// listing many services stays lightweight.
type ECSServiceDetail struct {
	Name                     string
	Status                   string
	ServiceARN               string
	ClusterARN               string
	TaskDefARN               string
	DesiredCount             int32
	RunningCount             int32
	PendingCount             int32
	LaunchType               string
	PlatformVersion          string
	PlatformFamily           string
	SchedulingStrategy       string
	RoleARN                  string
	CreatedAt                time.Time
	Deployments              []Deployment
	Events                   []ServiceEvent
	LoadBalancers            []LoadBalancerRef
	ServiceRegistries        []ServiceRegistry
	NetworkConfig            *NetworkConfig
	CapacityProviderStrategy []CapacityProviderItem
	EnableExecuteCommand     bool
	PropagateTags            string
	DeploymentController     string
	Tags                     []Tag
}

type Deployment struct {
	ID                 string
	Status             string
	TaskDefARN         string
	DesiredCount       int32
	PendingCount       int32
	RunningCount       int32
	FailedTasks        int32
	RolloutState       string
	RolloutStateReason string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type ServiceEvent struct {
	ID        string
	CreatedAt time.Time
	Message   string
}

type LoadBalancerRef struct {
	TargetGroupARN   string
	LoadBalancerName string
	ContainerName    string
	ContainerPort    int32
}

type ServiceRegistry struct {
	RegistryARN   string
	Port          int32
	ContainerName string
	ContainerPort int32
}

type NetworkConfig struct {
	Subnets        []string
	SecurityGroups []string
	AssignPublicIP string
}

type CapacityProviderItem struct {
	Name   string
	Base   int32
	Weight int32
}

// ECSTaskDetail carries the full DescribeTasks output for the task-describe view.
type ECSTaskDetail struct {
	TaskARN               string
	TaskDefARN            string
	ClusterARN            string
	ContainerInstanceARN  string
	LastStatus            string
	DesiredStatus         string
	HealthStatus          string
	LaunchType            string
	PlatformVersion       string
	PlatformFamily        string
	Connectivity          string
	ConnectivityAt        *time.Time
	AvailabilityZone      string
	CapacityProviderName  string
	CPU                   string
	Memory                string
	Group                 string
	StartedBy             string
	EnableExecuteCommand  bool
	PullStartedAt         *time.Time
	PullStoppedAt         *time.Time
	StartedAt             *time.Time
	StoppedAt             *time.Time
	CreatedAt             *time.Time
	StoppedReason         string
	StopCode              string
	Version               int64
	Containers            []ContainerRuntime
	Attachments           []Attachment
	Tags                  []Tag
}

type ContainerRuntime struct {
	Name              string
	RuntimeID         string
	Image             string
	ImageDigest       string
	LastStatus        string
	HealthStatus      string
	ExitCode          *int32
	Reason            string
	CPU               string
	Memory            string
	MemoryReservation string
	NetworkInterfaces []NetworkInterface
}

type NetworkInterface struct {
	AttachmentID       string
	PrivateIPv4Address string
	IPv6Address        string
}

type Attachment struct {
	ID      string
	Type    string
	Status  string
	Details map[string]string
}

type Tag struct {
	Key   string
	Value string
}
