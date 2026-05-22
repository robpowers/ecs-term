package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	CurrentContext string    `yaml:"current_context"`
	Contexts       []Context `yaml:"contexts"`
}

type Context struct {
	Name            string `yaml:"name"`
	Cluster         string `yaml:"cluster"`
	Region          string `yaml:"region"`
	AWSProfile      string `yaml:"aws_profile"`
	RefreshInterval int    `yaml:"refresh_interval"`
}

func (c *Context) EffectiveRefreshInterval() int {
	if c.RefreshInterval <= 0 {
		return 30
	}
	return c.RefreshInterval
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func FindConfigFile() (string, error) {
	candidates := []string{"ecs-term.yaml"}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".ecs-term.yaml"))
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("no config file found (checked ecs-term.yaml and ~/.ecs-term.yaml)")
}

func (c *Config) Validate() error {
	if len(c.Contexts) == 0 {
		return fmt.Errorf("config must have at least one context")
	}
	seen := map[string]bool{}
	for i, ctx := range c.Contexts {
		if ctx.Name == "" {
			return fmt.Errorf("context[%d] missing name", i)
		}
		if ctx.Cluster == "" {
			return fmt.Errorf("context %q missing cluster", ctx.Name)
		}
		if ctx.Region == "" {
			return fmt.Errorf("context %q missing region", ctx.Name)
		}
		if ctx.AWSProfile == "" {
			return fmt.Errorf("context %q missing aws_profile", ctx.Name)
		}
		if seen[ctx.Name] {
			return fmt.Errorf("duplicate context name %q", ctx.Name)
		}
		seen[ctx.Name] = true
	}
	return nil
}

func (c *Config) FindContext(name string) (*Context, bool) {
	for i := range c.Contexts {
		if c.Contexts[i].Name == name {
			return &c.Contexts[i], true
		}
	}
	return nil, false
}
