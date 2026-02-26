# agentmux

A Go TUI for spawning, orchestrating, and monitoring AI agents (Claude, Gemini) in parallel via DAG-based pipeline execution and stream-json event streams. Supports headless mode for CI/CD integration.

**Version:** v0.1.0 | **Go:** 1.24.2+ | **License:** MIT

## Overview

agentmux enables developers to define multi-agent workflows in YAML, execute them with automatic dependency resolution, and observe live progress in an interactive terminal interface. Each agent spawns a CLI subprocess (Claude or Gemini), parses NDJSON event streams, and reports state changes to a collaborative TUI dashboard. Agents in the same pipeline can independently target different backends.

Perfect for:
- Parallel code generation tasks (scout → planner → coder → tester → reviewer)
- Research and analysis workflows
- Complex automation pipelines
- CI/CD integration via headless mode
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
  backend: claude        # Default backend: "claude" or "gemini"
  model: "sonnet"
  max_turns: 10

agents:
  scout:
    backend: gemini      # Override: use Gemini CLI for this agent
    model: gemini-2.5-pro
    prompt: "Explore the codebase and summarize."
    max_turns: 5

  planner:
    prompt: "Create a detailed implementation plan."
    depends_on: [scout]

  coder:
    prompt: "Implement the plan."
    depends_on: [planner]
    model: "opus"
```

### Run the Pipeline

```bash
# Interactive TUI mode (default)
./agentmux run -c agentmux.yaml

# Headless mode — stream events to stdout
./agentmux run -c agentmux.yaml --headless

# Headless with NDJSON output (for scripting/CI)
./agentmux run -c agentmux.yaml --headless --format ndjson
```

**TUI mode** opens with:
- **Left sidebar**: Agent list with state indicators (● = running, ✓ = done, ✗ = failed)
- **Main panel**: Selected agent's output logs
- **Bottom bar**: Pipeline progress and help legend

**Headless mode** streams timestamped events to stdout without any TUI dependency, making it suitable for CI/CD pipelines and scripted workflows.

## Usage

### Configuration

Define agents, backends, models, and DAG dependencies in `agentmux.yaml`. Supports Claude and Gemini backends with per-agent overrides.

See [Configuration Guide](./docs/configuration.md) for the full schema, validation rules, model references, and tool lists.

### Headless Mode

Run pipelines without the TUI by passing `--headless`. Supports `--format text` (default) and `--format ndjson` for CI/CD scripting.

```bash
./agentmux run -c pipeline.yaml --headless --format ndjson | tee output.jsonl
```

Exit codes: `0` = success, `1` = failure, `130` = SIGINT, `143` = SIGTERM. See [Configuration Guide](./docs/configuration.md#headless-mode) for format examples.

### Keybindings

| Key | Action |
|-----|--------|
| `j` / `k` | Navigate agent list |
| `Tab` | Switch focus (sidebar → detail → statusbar) |
| `n` | Spawn new agent |
| `K` | Kill selected agent |
| `r` | Restart selected agent |
| `l` | Toggle log view |
| `p` | Show pipeline DAG |
| `q` | Quit |

## Documentation

| Document | Description |
|----------|-------------|
| [Configuration Guide](./docs/configuration.md) | Full config schema, backends, models, tools, headless mode |
| [System Architecture](./docs/system-architecture.md) | Design patterns, concurrency, data flow |
| [Codebase Summary](./docs/codebase-summary.md) | Package-by-package reference |
| [Code Standards](./docs/code-standards.md) | Go conventions, testing, error handling |
| [Project Overview & PDR](./docs/project-overview-pdr.md) | Vision, target users, core features |
| [Project Roadmap](./docs/project-roadmap.md) | v0.1.0 status, v0.2 features, v1.0 vision |
| [Deployment Guide](./docs/deployment-guide.md) | Build, install, troubleshooting |

## Examples

- `agentmux.yaml` — 5-agent coding pipeline (scout → planner → coder → tester → reviewer)
- `examples/parallel-analysis.yaml` — Fan-out/fan-in analysis (scout → 4 parallel → summarizer)
- `examples/mixed-backend-review.yaml` — Mixed Claude + Gemini pipeline

## Development

```bash
go build -o agentmux ./cmd/main.go   # Build
go test ./...                         # Run tests
go test -cover ./...                  # With coverage
golangci-lint run ./...               # Lint
```

For contribution guidelines, see [Code Standards](./docs/code-standards.md).

## Troubleshooting

See [Deployment Guide](./docs/deployment-guide.md) for common issues (output parsing, DAG cycles, missing logs).

## Related Projects

- [Claude Code CLI](https://github.com/anthropics/claude-code) — The agent subprocess
- [Bubbletea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Cobra](https://github.com/spf13/cobra) — CLI framework

## License

MIT

## Contributing

Contributions welcome! Please follow [Code Standards](./docs/code-standards.md) and ensure all tests pass before submitting PRs.
