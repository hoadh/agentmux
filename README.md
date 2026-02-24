# agentmux

A Go TUI for spawning, orchestrating, and monitoring Claude Code agents in parallel via DAG-based pipeline execution and stream-json event streams.

**Version:** v0.1.0 | **Go:** 1.26+ | **License:** MIT

## Overview

agentmux enables developers to define multi-agent workflows in YAML, execute them with automatic dependency resolution, and observe live progress in an interactive terminal interface. Each agent spawns a Claude Code subprocess, parses NDJSON event streams, and reports state changes to a collaborative TUI dashboard.

Perfect for:
- Parallel code generation tasks (scout → planner → coder → tester → reviewer)
- Research and analysis workflows
- Complex automation pipelines
- Agent orchestration testing and debugging

## Quick Start

### Installation

```bash
# Build from source
git clone https://github.com/hoadh/agentmux.git
cd agentmux
go build -o agentmux ./cmd/main.go

# Or use pre-built binary
./agentmux --version
```

### Configure Your Pipeline

Create `agentmux.yaml`:

```yaml
defaults:
  model: "sonnet"
  max_turns: 10

agents:
  scout:
    prompt: "Explore the codebase and summarize."
    max_turns: 5

  planner:
    prompt: "Create a detailed implementation plan."
    depends_on: [scout]

  coder:
    prompt: "Implement the plan."
    depends_on: [planner]
    model: "opus"

pipeline:
  coder:
    - planner
  planner:
    - scout
```

### Run the Pipeline

```bash
./agentmux run -c agentmux.yaml
```

The TUI opens with:
- **Left sidebar**: Agent list with state indicators (● = running, ✓ = done, ✗ = failed)
- **Main panel**: Selected agent's output logs
- **Bottom bar**: Pipeline progress and help legend

## Usage

### Configuration (agentmux.yaml)

| Section | Purpose |
|---------|---------|
| `defaults` | Global defaults: model, max_turns, allowedTools |
| `agents` | Named agent definitions with prompt, model, tool restrictions |
| `pipeline` | DAG: each agent lists dependencies (agents that run first) |

See [Config Guide](./docs/configuration.md) for full schema.

### Keybindings

| Key | Action |
|-----|--------|
| `j` / `k` | Navigate agent list |
| `Tab` | Switch focus (sidebar → detail → statusbar) |
| `n` | Spawn new agent (manual trigger) |
| `K` | Kill selected agent (SIGTERM + 5s → SIGKILL) |
| `r` | Restart selected agent |
| `l` | Toggle log view |
| `p` | Show pipeline DAG |
| `q` | Quit |

## Architecture

agentmux uses a layered architecture:

```
CLI (Cobra)
  ├─> Config (YAML parse)
  ├─> DAG (dependency graph validation)
  ├─> Manager (agent lifecycle)
  ├─> Scheduler (event-driven orchestration)
  └─> TUI (Bubbletea composite UI)
        ├─> Process (subprocess per agent)
        ├─> Parser (NDJSON stream reader)
        └─> Writer (event audit log)
```

**Key Features:**
- **DAG Scheduling**: Automatic topological sort; acyclicity validation
- **NDJSON Parsing**: Line-by-line stream consumption; malformed line recovery
- **Concurrent Safety**: sync.RWMutex for state, sync.Mutex for scheduler critical sections
- **Batch Event Processing**: Up to 50 events per TUI render cycle for responsiveness
- **Identity Separation**: Distinct identity and display fields prevent silent log loss

See [System Architecture](./docs/system-architecture.md) for detailed design patterns.

## Project Structure

```
cmd/               # CLI entry points (Cobra)
internal/
  ├── agent/       # Process, parser, manager, state
  ├── config/      # YAML parsing, validation
  ├── dag/         # Graph, scheduler
  ├── log/         # JSONL writer
  └── tui/         # Bubbletea UI components
docs/              # Project documentation
main.go            # Bootstrap
go.mod, go.sum     # Dependency management
agentmux.yaml      # Example config
```

## Documentation

- [Project Overview & PDR](./docs/project-overview-pdr.md) — Vision, target users, core features
- [System Architecture](./docs/system-architecture.md) — Design patterns, concurrency, data flow
- [Code Standards](./docs/code-standards.md) — Go conventions, testing, error handling
- [Codebase Summary](./docs/codebase-summary.md) — Package-by-package reference
- [Project Roadmap](./docs/project-roadmap.md) — v0.1.0 status, v0.2 features, v1.0 vision
- [Deployment Guide](./docs/deployment-guide.md) — Build, install, troubleshooting

## Examples

See `agentmux.yaml` for a 5-agent coding pipeline (scout → planner → coder → tester → reviewer).

## Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| charmbracelet/bubbletea | v1.3.10 | TUI framework |
| charmbracelet/bubbles | v1.0.0 | Reusable components |
| charmbracelet/lipgloss | v1.1.0 | Terminal styling |
| spf13/cobra | v1.10.2 | CLI framework |
| gopkg.in/yaml.v3 | v3.0.1 | Config parsing |

## Testing

```bash
go test ./...           # Run all tests
go test -cover ./...    # With coverage
go test -v ./...        # Verbose output
```

Coverage target: >80% on critical paths (parser, scheduler, manager).

## Development

For contribution guidelines, see [Code Standards](./docs/code-standards.md).

### Build

```bash
go build -o agentmux ./cmd/main.go
```

### Lint

```bash
golangci-lint run ./...
```

## Troubleshooting

**Agent not receiving output**: Ensure `--verbose` flag is passed to Claude CLI in print mode. Without it, stream-json format differs. See [Deployment Guide](./docs/deployment-guide.md).

**DAG cycle detected**: Check config for circular dependencies in `pipeline` section.

**Logs not appearing**: Verify agent identity matches exactly (case-sensitive). Check `~/.agentmux/logs/` for audit trail.

## Related Projects

- [Claude Code CLI](https://github.com/anthropics/claude-code) — The agent subprocess
- [Bubbletea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Cobra](https://github.com/spf13/cobra) — CLI framework

## License

MIT

## Contributing

Contributions welcome! Please follow [Code Standards](./docs/code-standards.md) and ensure all tests pass before submitting PRs.
