package config

import (
	"fmt"
	"os"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

// Config represents the top-level agentmux configuration.
type Config struct {
	Version    int                    `yaml:"version"`
	Vars       map[string]string      `yaml:"vars"`
	OutputDir  string                 `yaml:"output_dir"`
	Defaults   AgentDefaults          `yaml:"defaults"`
	Agents     map[string]AgentConfig `yaml:"agents"`
	AgentOrder []string               `yaml:"-"` // preserves YAML key order
}

// AgentDefaults provides fallback values for agent fields.
type AgentDefaults struct {
	Backend      string   `yaml:"backend"`
	Model        string   `yaml:"model"`
	AllowedTools []string `yaml:"allowedTools"`
	MaxTurns     int      `yaml:"max_turns"`
}

// AgentConfig defines a single agent's configuration.
type AgentConfig struct {
	Backend      string   `yaml:"backend"`
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
func LoadConfig(path string) (*Config, []string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.Agents == nil {
		cfg.Agents = make(map[string]AgentConfig)
	}

	// Extract agent key order from YAML to preserve config ordering
	cfg.AgentOrder = extractAgentOrder(data)

	ApplyDefaults(&cfg)

	if err := ExpandTemplates(&cfg); err != nil {
		return nil, nil, err
	}

	warnings, err := Validate(&cfg)
	if err != nil {
		return nil, nil, err
	}

	return &cfg, warnings, nil
}

// ExpandTemplates expands {{.var_name}} placeholders in agent prompts using cfg.Vars.
func ExpandTemplates(cfg *Config) error {
	if len(cfg.Vars) == 0 {
		return nil
	}
	for name, agent := range cfg.Agents {
		tmpl, err := template.New(name).Option("missingkey=error").Parse(agent.Prompt)
		if err != nil {
			return fmt.Errorf("agent %q: invalid template: %w", name, err)
		}
		var buf strings.Builder
		if err := tmpl.Execute(&buf, cfg.Vars); err != nil {
			return fmt.Errorf("agent %q: template expansion: %w", name, err)
		}
		agent.Prompt = buf.String()
		cfg.Agents[name] = agent
	}
	return nil
}

// extractAgentOrder parses YAML to preserve the key order of the agents map.
func extractAgentOrder(data []byte) []string {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil || len(doc.Content) == 0 {
		return nil
	}

	root := doc.Content[0] // mapping node
	for i := 0; i < len(root.Content)-1; i += 2 {
		if root.Content[i].Value == "agents" {
			agentsNode := root.Content[i+1]
			order := make([]string, 0, len(agentsNode.Content)/2)
			for j := 0; j < len(agentsNode.Content)-1; j += 2 {
				order = append(order, agentsNode.Content[j].Value)
			}
			return order
		}
	}
	return nil
}

// ApplyDefaults merges default values into agents with unset fields.
func ApplyDefaults(cfg *Config) {
	for name, agent := range cfg.Agents {
		if agent.Backend == "" {
			agent.Backend = cfg.Defaults.Backend
		}
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
// Returns warnings for non-fatal issues (e.g. unsupported backend features).
func Validate(cfg *Config) ([]string, error) {
	var warnings []string
	for name, agent := range cfg.Agents {
		if agent.Prompt == "" {
			return nil, fmt.Errorf("agent %q: prompt is required", name)
		}
		for _, dep := range agent.DependsOn {
			if dep == name {
				return nil, fmt.Errorf("agent %q: cannot depend on itself", name)
			}
			if _, ok := cfg.Agents[dep]; !ok {
				return nil, fmt.Errorf("agent %q: unknown dependency %q", name, dep)
			}
		}
		// Backend-specific warnings
		if agent.Backend == "gemini" {
			if len(agent.AllowedTools) > 0 {
				warnings = append(warnings, fmt.Sprintf(
					"agent %q: allowedTools ignored for gemini backend", name))
			}
			if agent.MaxTurns > 0 {
				warnings = append(warnings, fmt.Sprintf(
					"agent %q: max_turns ignored for gemini backend", name))
			}
		}
	}
	return warnings, nil
}
