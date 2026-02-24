# Phase 1: Project Setup & Config

## Context Links
- [Parent Plan](plan.md)
- [Brainstorm](../reports/brainstorm-260224-1523-agentmux-cli.md)
- [DAG+Config Research](research/researcher-02-dag-process-management.md)

## Overview
- **Date**: 2026-02-24
- **Priority**: P1
- **Status**: complete
- **Description**: Initialize Go module, install dependencies, create directory structure, implement YAML config parsing with validation, create example config file.

## Key Insights
<!-- Updated: Validation Session 1 - Replace Viper with yaml.v3, module path github.com/hoadh/agentmux -->
- Use `gopkg.in/yaml.v3` directly (not Viper) — simpler, fewer deps, KISS principle
- Struct tags use `yaml:"field_name"` instead of `yaml:"field_name"`
- Module path: `github.com/hoadh/agentmux`
- Agents defined as a map (not list) for named references: `agents: { planner: {...} }`
- Config validation must check: missing prompts, duplicate names, unknown deps, DAG cycles
- Keep config struct simple; defaults applied via struct field merge

## Requirements

### Functional
- Go module `github.com/hoadh/agentmux` with all dependencies
- YAML config struct matching the spec: version, defaults, agents map
- Config loader: file path from CLI flag or default `agentmux.yaml`
- Validation: required fields, dependency references exist, no self-deps
- Defaults merging: agent inherits from `defaults` block if field unset

### Non-Functional
- Config load <10ms for typical files
- Clear error messages on invalid config (line numbers if possible)

## Architecture

```
config/
└── config.go     # Types + LoadConfig() + Validate()

Config struct
├── Version   int
├── Defaults  AgentDefaults    (model, allowedTools, maxTurns)
└── Agents    map[string]AgentConfig
                ├── Prompt       string (required)
                ├── WorkDir      string (optional, default ".")
                ├── Model        string (optional, inherits default)
                ├── AllowedTools []string (optional, inherits default)
                ├── MaxTurns     int (optional, inherits default)
                └── DependsOn    []string (optional)
```

## Related Code Files

### Create
- `go.mod` — module definition
- `go.sum` — auto-generated
- `main.go` — entry point: `cmd.Execute()`
- `cmd/root.go` — Cobra root command with `--config` persistent flag
- `cmd/version.go` — `agentmux version`
- `internal/config/config.go` — types, loader, validation
- `agentmux.yaml` — example config

## Implementation Steps

1. **Init Go module**
   ```bash
   go mod init github.com/hoadh/agentmux
   ```

2. **Install dependencies**
   ```bash
   go get github.com/charmbracelet/bubbletea
   go get github.com/charmbracelet/bubbles
   go get github.com/charmbracelet/lipgloss
   go get github.com/spf13/cobra
   go get gopkg.in/yaml.v3
   ```

3. **Create directory structure**
   ```
   mkdir -p cmd internal/{agent,dag,config,tui,log}
   ```

4. **Write `main.go`** — minimal entry point calling `cmd.Execute()`

5. **Write `cmd/root.go`**
   - Root command with `--config` persistent string flag (default: `agentmux.yaml`)
   - PersistentPreRunE: load config into a package-level var (or pass via context)
   - Add `version` subcommand

6. **Write `cmd/version.go`** — prints version string (hardcoded `v0.1.0` for now)

7. **Write `internal/config/config.go`**
   - Define types:
     ```go
     type Config struct {
         Version  int                    `yaml:"version"`
         Defaults AgentDefaults          `yaml:"defaults"`
         Agents   map[string]AgentConfig `yaml:"agents"`
     }
     type AgentDefaults struct {
         Model        string   `yaml:"model"`
         AllowedTools []string `yaml:"allowedTools"`
         MaxTurns     int      `yaml:"max_turns"`
     }
     type AgentConfig struct {
         Prompt       string   `yaml:"prompt"`
         WorkDir      string   `yaml:"workdir"`
         Model        string   `yaml:"model"`
         AllowedTools []string `yaml:"allowedTools"`
         MaxTurns     int      `yaml:"max_turns"`
         DependsOn    []string `yaml:"depends_on"`
     }
     ```
   - `LoadConfig(path string) (*Config, error)` — os.ReadFile + yaml.Unmarshal
   - `ApplyDefaults(cfg *Config)` — merge defaults into each agent's empty fields
   - `Validate(cfg *Config) error` — check required fields, dep references, no self-deps

8. **Write `agentmux.yaml`** — example config with planner→coder→tester+reviewer pipeline

9. **Verify**: `go build ./...` compiles cleanly

## Todo List
- [ ] Init go module and install dependencies
- [ ] Create directory structure
- [ ] Write main.go entry point
- [ ] Write cmd/root.go with --config flag
- [ ] Write cmd/version.go
- [ ] Write internal/config/config.go (types, loader, validation)
- [ ] Write example agentmux.yaml
- [ ] Verify `go build ./...` passes

## Success Criteria
- `go build ./...` succeeds
- `./agentmux version` prints version
- `./agentmux run --config agentmux.yaml` loads config without error (run cmd is stub)
- Invalid config (missing prompt, bad dep ref) returns clear error

## Risk Assessment
- **yaml.v3 limitations**: No env var overrides or config search. Acceptable for v1; add if needed later.
- **Map ordering**: Go maps don't preserve insertion order. If agent order matters for display, sort alphabetically or add an `order` field. For now, alphabetical is fine.

## Security Considerations
- Config may contain prompts with sensitive instructions; don't log full config at debug level
- WorkDir paths should be validated (no path traversal outside project)

## Next Steps
- Phase 2: Agent process management depends on `AgentConfig` types defined here
