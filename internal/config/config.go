package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the top-level agentmux configuration.
type Config struct {
	Version  int                    `yaml:"version"`
	Defaults AgentDefaults          `yaml:"defaults"`
	Agents   map[string]AgentConfig `yaml:"agents"`
}

// AgentDefaults provides fallback values for agent fields.
type AgentDefaults struct {
	Model        string   `yaml:"model"`
	AllowedTools []string `yaml:"allowedTools"`
	MaxTurns     int      `yaml:"max_turns"`
}

// AgentConfig defines a single agent's configuration.
type AgentConfig struct {
	Prompt       string   `yaml:"prompt"`
	WorkDir      string   `yaml:"workdir"`
	Model        string   `yaml:"model"`
	AllowedTools []string `yaml:"allowedTools"`
	MaxTurns     int      `yaml:"max_turns"`
	DependsOn    []string `yaml:"depends_on"`
}

// DefaultConfig returns a minimal empty config.
func DefaultConfig() *Config {
	return &Config{
		Version: 1,
		Agents:  make(map[string]AgentConfig),
	}
}

// LoadConfig reads and parses a YAML config file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.Agents == nil {
		cfg.Agents = make(map[string]AgentConfig)
	}

	ApplyDefaults(&cfg)

	if err := Validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// ApplyDefaults merges default values into agents with unset fields.
func ApplyDefaults(cfg *Config) {
	for name, agent := range cfg.Agents {
		if agent.Model == "" {
			agent.Model = cfg.Defaults.Model
		}
		if len(agent.AllowedTools) == 0 && len(cfg.Defaults.AllowedTools) > 0 {
			agent.AllowedTools = cfg.Defaults.AllowedTools
		}
		if agent.MaxTurns == 0 && cfg.Defaults.MaxTurns > 0 {
			agent.MaxTurns = cfg.Defaults.MaxTurns
		}
		if agent.WorkDir == "" {
			agent.WorkDir = "."
		}
		cfg.Agents[name] = agent
	}
}

// Validate checks config for required fields and dependency integrity.
func Validate(cfg *Config) error {
	for name, agent := range cfg.Agents {
		if agent.Prompt == "" {
			return fmt.Errorf("agent %q: prompt is required", name)
		}
		for _, dep := range agent.DependsOn {
			if dep == name {
				return fmt.Errorf("agent %q: cannot depend on itself", name)
			}
			if _, ok := cfg.Agents[dep]; !ok {
				return fmt.Errorf("agent %q: unknown dependency %q", name, dep)
			}
		}
	}
	return nil
}
