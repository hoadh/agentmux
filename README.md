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

### Configuration (agentmux.yaml)

| Section | Purpose |
|---------|---------|
| `defaults` | Global defaults: backend, model, max_turns, allowedTools |
| `agents` | Named agent definitions with prompt, backend, model, tool restrictions |
| `depends_on` | DAG: each agent lists dependencies (agents that run first) |

#### Multi-Backend Support

Each agent can target a different CLI backend. Set `backend` at the defaults or agent level:

| Backend | Binary | Notes |
|---------|--------|-------|
| `claude` (default) | `claude` | Full support: model, allowedTools, max_turns |
| `gemini` | `gemini` | Model supported; allowedTools/max_turns ignored (warning emitted) |

Adding a new backend requires only one new Go file with `init()` registration.

#### Config Validation

agentmux validates your pipeline config at load time and emits warnings for backend-specific incompatibilities.

**Hard errors** (abort startup):

| Condition | Error |
|-----------|-------|
| Agent missing `prompt` | `agent "X": prompt is required` |
| Agent depends on itself | `agent "X": cannot depend on itself` |
| Agent depends on undefined agent | `agent "X": unknown dependency "Y"` |
| Unknown backend name | `unknown backend "X" (available: [claude gemini])` |

**Warnings** (printed to stderr, execution continues):

| Condition | Warning |
|-----------|---------|
| Gemini agent has `allowedTools` | `agent "X": allowedTools ignored for gemini backend` |
| Gemini agent has `max_turns > 0` | `agent "X": max_turns ignored for gemini backend` |

Warnings also trigger for values inherited from `defaults` (defaults are merged before validation).

#### Models & Tools by Backend

| Feature | Claude | Gemini |
|---------|--------|--------|
| `model` | Passed as `--model` flag | Passed as `--model` flag |
| `allowedTools` | Each tool passed as `--allowedTools <tool>` | **Ignored** (warning emitted) |
| `max_turns` | Passed as `--max-turns` flag | **Ignored** (warning emitted) |

##### Claude Models

| Alias | Full Model ID | Notes |
|-------|---------------|-------|
| `opus` | `claude-opus-4-6` | Most capable, adaptive reasoning |
| `sonnet` | `claude-sonnet-4-6` | Balanced speed/quality (default) |
| `haiku` | `claude-haiku-4-5` | Fastest, lightweight tasks |

##### Claude Tools (`allowedTools`)

| Category | Tools |
|----------|-------|
| File Ops | `Read`, `Edit`, `Write`, `NotebookEdit`, `Glob` |
| Shell | `Bash` |
| Search | `Grep`, `WebSearch`, `WebFetch` |
| Task Mgmt | `Task`, `TodoRead`, `TodoWrite` |
| MCP | `mcp__<server>__<tool>` pattern |

Supports granular patterns: `Bash(git log *)`, `Edit(src/**)`, etc.

##### Gemini Models

| Model ID | Notes |
|----------|-------|
| `gemini-2.5-pro` | Default, complex reasoning |
| `gemini-2.5-flash` | Fast inference |
| `gemini-2.5-flash-lite` | Most cost-efficient |
| `gemini-3-pro-preview` | Preview, latest generation |
| `gemini-3-flash-preview` | Preview, fast |

No short aliases — full model IDs required.

##### Gemini Tools

| Category | Tools |
|----------|-------|
| Search | `google_web_search` |
| File Ops | Read, write, glob, text search/replace |
| Shell | Command execution (with confirmation) |
| Web | URL fetching, browser automation |
| Memory | Todo/memory management |
| MCP | 415+ extensions via registry |

> **Note:** Gemini CLI uses `tools.core` (allow-list) or `tools.exclude` (block-list) in config for tool filtering. The `allowedTools` field in agentmux config is ignored for Gemini agents.

**Model resolution**: If an agent has no `model`, it inherits from `defaults.model`. If still empty, no `--model` flag is passed and the backend CLI uses its own default. Model names are not validated at config time — invalid names produce runtime errors from the backend CLI.

**Defaults inheritance**: `ApplyDefaults()` merges global `defaults` into each agent for any unset field: `backend`, `model`, `allowedTools`, `max_turns`. The `workdir` field defaults to `"."` if empty.

See [Config Guide](./docs/configuration.md) for full schema.

### Headless Mode

Run pipelines without the TUI by passing `--headless`. Events stream to stdout in real time.

| Flag | Default | Description |
|------|---------|-------------|
| `--headless` | `false` | Run without TUI, stream events to stdout |
| `--format` | `text` | Output format: `text` or `ndjson` |

**Text format** produces human-readable lines:

```
[14:30:05] [scout] STARTED
[14:30:06] [scout] Exploring the codebase...
[14:30:10] [scout] -> Tool: bash find . -name "*.go"
[14:30:10] [scout] DONE
[14:30:10] [planner] STARTED
...
[14:30:45] [pipeline] COMPLETED
```

**NDJSON format** produces one JSON object per line for machine consumption:

```json
{"ts":"2026-02-27T14:30:05+07:00","agent":"scout","type":"agent_started"}
{"ts":"2026-02-27T14:30:06+07:00","agent":"scout","type":"assistant","data":{"text":"Exploring..."}}
{"ts":"2026-02-27T14:30:45+07:00","agent":"pipeline","type":"pipeline_done","data":{"success":true}}
```

**Exit codes**: `0` = all agents succeeded, `1` = any failure, `130` = SIGINT, `143` = SIGTERM.

**CI/CD example**:

```bash
./agentmux run -c pipeline.yaml --headless --format ndjson | tee output.jsonl
echo "Exit: $?"
```

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
  ├─> Config (YAML parse + backend validation)
  ├─> DAG (dependency graph validation)
  ├─> Manager (agent lifecycle + backend resolution)
  ├─> Scheduler (event-driven orchestration)
  ├─> Backend (registry: Claude, Gemini, extensible)
  ├─> TUI (Bubbletea composite UI)
  │     ├─> Process (subprocess per agent via backend)
  │     ├─> Parser (NDJSON stream reader + backend converter)
  │     └─> Writer (event audit log)
  └─> Headless (non-TUI runner for CI/CD)
        ├─> Text/NDJSON formatters
        └─> Signal handling (graceful shutdown)
```

**Key Features:**
- **Multi-Backend**: Registry-based abstraction; agents choose Claude or Gemini independently
- **DAG Scheduling**: Automatic topological sort; acyclicity validation
- **NDJSON Parsing**: Line-by-line stream consumption; backend-specific event conversion
- **Concurrent Safety**: sync.RWMutex for state, sync.Mutex for scheduler critical sections
- **Batch Event Processing**: Up to 50 events per TUI render cycle for responsiveness
- **Headless Mode**: Run pipelines without TUI; text or NDJSON output for CI/CD
- **Extensible**: Adding a new backend = 1 new file with `init()` registration

See [System Architecture](./docs/system-architecture.md) for detailed design patterns.

## Project Structure

```
cmd/               # CLI entry points (Cobra)
internal/
  ├── agent/       # Process, parser, manager, state
  │   └── backend/ # Backend interface, registry, Claude & Gemini impls
  ├── config/      # YAML parsing, validation, backend warnings
  ├── dag/         # Graph, scheduler
  ├── headless/    # Non-TUI runner, text/NDJSON formatters
  ├── log/         # JSONL writer
  └── tui/         # Bubbletea UI components
examples/          # Pipeline examples (single & mixed backend)
docs/              # Project documentation
main.go            # Bootstrap
go.mod, go.sum     # Dependency management
```

## Documentation

- [Project Overview & PDR](./docs/project-overview-pdr.md) — Vision, target users, core features
- [System Architecture](./docs/system-architecture.md) — Design patterns, concurrency, data flow
- [Code Standards](./docs/code-standards.md) — Go conventions, testing, error handling
- [Codebase Summary](./docs/codebase-summary.md) — Package-by-package reference
- [Project Roadmap](./docs/project-roadmap.md) — v0.1.0 status, v0.2 features, v1.0 vision
- [Deployment Guide](./docs/deployment-guide.md) — Build, install, troubleshooting

## Examples

- `agentmux.yaml` — 5-agent coding pipeline (scout → planner → coder → tester → reviewer)
- `examples/parallel-analysis.yaml` — Fan-out/fan-in analysis (scout → 4 parallel → summarizer)
- `examples/mixed-backend-review.yaml` — Mixed Claude + Gemini pipeline

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
