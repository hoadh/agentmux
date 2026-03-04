package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_Valid(t *testing.T) {
	path := filepath.Join("testdata", "valid.yaml")
	cfg, _, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg == nil {
		t.Fatal("config is nil")
	}

	if cfg.Version != 1 {
		t.Errorf("version: got %d, want 1", cfg.Version)
	}

	if len(cfg.Agents) != 3 {
		t.Errorf("agents count: got %d, want 3", len(cfg.Agents))
	}

	if _, ok := cfg.Agents["planner"]; !ok {
		t.Error("planner agent not found")
	}

	if _, ok := cfg.Agents["coder"]; !ok {
		t.Error("coder agent not found")
	}

	if cfg.Agents["coder"].Model != "opus" {
		t.Errorf("coder model: got %q, want %q", cfg.Agents["coder"].Model, "opus")
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, _, err := LoadConfig("nonexistent.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLoadConfig_MissingPrompt(t *testing.T) {
	path := filepath.Join("testdata", "missing_prompt.yaml")
	_, _, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for missing prompt")
	}
}

func TestLoadConfig_UnknownDependency(t *testing.T) {
	path := filepath.Join("testdata", "unknown_dep.yaml")
	_, _, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for unknown dependency")
	}
}

func TestLoadConfig_SelfDependency(t *testing.T) {
	path := filepath.Join("testdata", "self_dep.yaml")
	_, _, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for self-dependency")
	}
}

func TestApplyDefaults(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Defaults: AgentDefaults{
			Model:        "sonnet",
			AllowedTools: []string{"Read", "Edit"},
			MaxTurns:     10,
		},
		Agents: map[string]AgentConfig{
			"agent1": {
				Prompt: "Task 1",
			},
			"agent2": {
				Prompt:   "Task 2",
				Model:    "opus",
				MaxTurns: 20,
			},
		},
	}

	ApplyDefaults(cfg)

	// agent1 should inherit all defaults
	agent1 := cfg.Agents["agent1"]
	if agent1.Model != "sonnet" {
		t.Errorf("agent1 model: got %q, want %q", agent1.Model, "sonnet")
	}
	if agent1.MaxTurns != 10 {
		t.Errorf("agent1 max_turns: got %d, want 10", agent1.MaxTurns)
	}
	if len(agent1.AllowedTools) != 2 {
		t.Errorf("agent1 tools count: got %d, want 2", len(agent1.AllowedTools))
	}
	if agent1.WorkDir != "" {
		t.Errorf("agent1 workdir: got %q, want %q", agent1.WorkDir, "")
	}

	// agent2 should override model and max_turns
	agent2 := cfg.Agents["agent2"]
	if agent2.Model != "opus" {
		t.Errorf("agent2 model: got %q, want %q", agent2.Model, "opus")
	}
	if agent2.MaxTurns != 20 {
		t.Errorf("agent2 max_turns: got %d, want 20", agent2.MaxTurns)
	}
	if len(agent2.AllowedTools) != 2 {
		t.Errorf("agent2 tools count: got %d, want 2", len(agent2.AllowedTools))
	}
}

func TestApplyDefaults_NoDefaults(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Agents: map[string]AgentConfig{
			"agent1": {
				Prompt: "Task 1",
			},
		},
	}

	ApplyDefaults(cfg)

	agent1 := cfg.Agents["agent1"]
	if agent1.Model != "" {
		t.Errorf("agent1 model should be empty, got %q", agent1.Model)
	}
	if agent1.MaxTurns != 0 {
		t.Errorf("agent1 max_turns should be 0, got %d", agent1.MaxTurns)
	}
	if agent1.WorkDir != "" {
		t.Errorf("agent1 workdir: got %q, want %q", agent1.WorkDir, "")
	}
}

func TestValidate_Valid(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Agents: map[string]AgentConfig{
			"agent1": {
				Prompt: "Task 1",
			},
			"agent2": {
				Prompt:    "Task 2",
				DependsOn: []string{"agent1"},
			},
		},
	}

	_, err := Validate(cfg)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
}

func TestValidate_MissingPrompt(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Agents: map[string]AgentConfig{
			"agent1": {
				Prompt: "",
			},
		},
	}

	_, err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for missing prompt")
	}
}

func TestValidate_UnknownDependency(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Agents: map[string]AgentConfig{
			"agent1": {
				Prompt:    "Task 1",
				DependsOn: []string{"nonexistent"},
			},
		},
	}

	_, err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for unknown dependency")
	}
}

func TestValidate_SelfDependency(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Agents: map[string]AgentConfig{
			"agent1": {
				Prompt:    "Task 1",
				DependsOn: []string{"agent1"},
			},
		},
	}

	_, err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for self-dependency")
	}
}

func TestValidate_EmptyConfig(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Agents:  make(map[string]AgentConfig),
	}

	_, err := Validate(cfg)
	if err != nil {
		t.Fatalf("Validate should accept empty config: %v", err)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig returned nil")
	}

	if cfg.Version != 1 {
		t.Errorf("version: got %d, want 1", cfg.Version)
	}

	if cfg.Agents == nil {
		t.Fatal("Agents map is nil")
	}

	if len(cfg.Agents) != 0 {
		t.Errorf("Agents count: got %d, want 0", len(cfg.Agents))
	}
}

func TestLoadConfig_DefaultsMerging(t *testing.T) {
	path := filepath.Join("testdata", "defaults_merging.yaml")
	cfg, _, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	agent1 := cfg.Agents["agent1"]
	if agent1.Model != "sonnet" {
		t.Errorf("agent1 model: got %q, want %q", agent1.Model, "sonnet")
	}
	if agent1.MaxTurns != 5 {
		t.Errorf("agent1 max_turns: got %d, want 5", agent1.MaxTurns)
	}

	agent2 := cfg.Agents["agent2"]
	if agent2.Model != "opus" {
		t.Errorf("agent2 model: got %q, want %q", agent2.Model, "opus")
	}
	if agent2.MaxTurns != 15 {
		t.Errorf("agent2 max_turns: got %d, want 15", agent2.MaxTurns)
	}
}

func TestLoadConfig_MixedBackend(t *testing.T) {
	path := filepath.Join("testdata", "mixed_backend.yaml")
	cfg, warnings, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	researcher := cfg.Agents["researcher"]
	if researcher.Backend != "gemini" {
		t.Errorf("researcher backend: got %q, want %q", researcher.Backend, "gemini")
	}

	implementer := cfg.Agents["implementer"]
	if implementer.Backend != "claude" {
		t.Errorf("implementer backend: got %q, want %q", implementer.Backend, "claude")
	}

	// 1 warning: researcher (gemini) inherits max_turns=10 from defaults
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
}

func TestApplyDefaults_Backend(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Defaults: AgentDefaults{
			Backend: "claude",
			Model:   "sonnet",
		},
		Agents: map[string]AgentConfig{
			"agent1": {Prompt: "Task 1"},
			"agent2": {Prompt: "Task 2", Backend: "gemini"},
		},
	}

	ApplyDefaults(cfg)

	if cfg.Agents["agent1"].Backend != "claude" {
		t.Errorf("agent1 backend: got %q, want %q", cfg.Agents["agent1"].Backend, "claude")
	}
	if cfg.Agents["agent2"].Backend != "gemini" {
		t.Errorf("agent2 backend: got %q, want %q", cfg.Agents["agent2"].Backend, "gemini")
	}
}

func TestValidate_GeminiWarnings(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Agents: map[string]AgentConfig{
			"agent1": {
				Prompt:       "Task 1",
				Backend:      "gemini",
				AllowedTools: []string{"Read"},
				MaxTurns:     10,
			},
		},
	}

	warnings, err := Validate(cfg)
	if err != nil {
		t.Fatalf("Validate error: %v", err)
	}
	if len(warnings) != 2 {
		t.Fatalf("expected 2 warnings, got %d: %v", len(warnings), warnings)
	}
}

func TestValidate_ClaudeNoWarnings(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Agents: map[string]AgentConfig{
			"agent1": {
				Prompt:       "Task 1",
				Backend:      "claude",
				AllowedTools: []string{"Read"},
				MaxTurns:     10,
			},
		},
	}

	warnings, err := Validate(cfg)
	if err != nil {
		t.Fatalf("Validate error: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings for claude, got %d: %v", len(warnings), warnings)
	}
}

func TestExpandTemplates_Valid(t *testing.T) {
	cfg := &Config{
		Vars: map[string]string{"project": "myapp", "lang": "Go"},
		Agents: map[string]AgentConfig{
			"scout": {Prompt: "Analyze {{.project}} in {{.lang}}"},
		},
	}
	if err := ExpandTemplates(cfg); err != nil {
		t.Fatalf("ExpandTemplates failed: %v", err)
	}
	want := "Analyze myapp in Go"
	if cfg.Agents["scout"].Prompt != want {
		t.Errorf("got %q, want %q", cfg.Agents["scout"].Prompt, want)
	}
}

func TestExpandTemplates_UndefinedVar(t *testing.T) {
	cfg := &Config{
		Vars: map[string]string{"project": "myapp"},
		Agents: map[string]AgentConfig{
			"scout": {Prompt: "Use {{.undefined}}"},
		},
	}
	if err := ExpandTemplates(cfg); err == nil {
		t.Fatal("expected error for undefined var")
	}
}

func TestExpandTemplates_NoVars(t *testing.T) {
	cfg := &Config{
		Agents: map[string]AgentConfig{
			"scout": {Prompt: "Plain prompt"},
		},
	}
	if err := ExpandTemplates(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Agents["scout"].Prompt != "Plain prompt" {
		t.Error("prompt should be unchanged")
	}
}

func TestLoadConfig_WithVars(t *testing.T) {
	path := filepath.Join("testdata", "vars_valid.yaml")
	cfg, _, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if err := ExpandTemplates(cfg); err != nil {
		t.Fatalf("ExpandTemplates failed: %v", err)
	}
	want := "Analyze myapp written in Go"
	if cfg.Agents["scout"].Prompt != want {
		t.Errorf("got %q, want %q", cfg.Agents["scout"].Prompt, want)
	}
}

func TestLoadConfig_WithVarsUndefined(t *testing.T) {
	path := filepath.Join("testdata", "vars_undefined.yaml")
	cfg, _, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if err := ExpandTemplates(cfg); err == nil {
		t.Fatal("expected error for undefined var")
	}
}

func TestLoadConfig_WithOutputDir(t *testing.T) {
	path := filepath.Join("testdata", "output_dir.yaml")
	cfg, _, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.OutputDir != "custom-output" {
		t.Errorf("OutputDir: got %q, want %q", cfg.OutputDir, "custom-output")
	}
}

func TestLoadConfig_NilAgentsMap(t *testing.T) {
	// Create a temporary config file with no agents
	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.WriteString("version: 1\n")
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	tmpfile.Close()

	cfg, _, err := LoadConfig(tmpfile.Name())
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Agents == nil {
		t.Fatal("Agents map should not be nil after LoadConfig")
	}
}
