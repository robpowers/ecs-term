package domain

type ContainerDetail struct {
	Name            string
	Image           string
	CPU             int
	MemoryMB        int
	MemoryReserveMB int
	EnvVars         []EnvVar
	Secrets         []Secret
	HealthCheck     *HealthCheckConfig
	PortMappings    []PortMapping
}

type EnvVar struct {
	Name  string
	Value string
}

// Secret references a value pulled from Secrets Manager or SSM Parameter
// Store at container start — never a plaintext value.
type Secret struct {
	Name      string
	ValueFrom string
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
