package domain

type ContainerDetail struct {
	Name            string
	Image           string
	CPU             int
	MemoryMB        int
	MemoryReserveMB int
	EnvVars         []EnvVar
	HealthCheck     *HealthCheckConfig
	PortMappings    []PortMapping
}

type EnvVar struct {
	Name  string
	Value string
}

type HealthCheckConfig struct {
	Command     []string
	IntervalSec int
	TimeoutSec  int
	Retries     int
	StartPeriod int
}

type PortMapping struct {
	ContainerPort int32
	HostPort      int32
	Protocol      string
}
