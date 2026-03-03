# System Architecture

## Overview

Agentmux is a Go TUI application for spawning, monitoring, and orchestrating AI agents (Claude, Gemini) in parallel using DAG pipelines. The system enables users to define agent workflows in YAML, execute them with dependency management, and observe live progress through a terminal interface or headless output.

**Module:** `github.com/hoadh/agentmux`
**Go Version:** 1.24.2+

## Architecture Layers

### 1. CLI Layer (`cmd/`)

- **root.go**: Cobra CLI entry point, flags, configuration loading
- **run.go**: Launch command initializing config, DAG, manager, and TUI
- **check.go**: Backend CLI availability check
- **version.go**: Version information

Cobra provides hierarchical command structure and flag parsing.

### 2. Configuration (`internal/config/`)

- **config.go**: YAML struct parsing and validation
  - Loads `agentmux.yaml` (agent definitions, pipeline DAG, global settings)
  - Defines `Config`, `AgentDefaults`, and `AgentConfig` structs
  - Validates YAML syntax, required fields, and acyclicity
  - Template variable expansion via `ExpandTemplates()` (Go text/template syntax with `{{.var_name}}`)
  - Output directory configuration via `OutputDir` field (defaults to `.agentmux-out`)
  - CLI flag precedence handling: `--var` and `--output-dir` override YAML values
  - Returns typed configuration for downstream use

### 2.1. Configuration Pipeline

1. **Load**: Parse YAML file into `Config` struct
2. **Merge defaults**: `ApplyDefaults()` merges `Defaults` section into each agent
3. **CLI overrides**: CLI `--var` flags override/add to `Vars` map; CLI `--output-dir` overrides `OutputDir`
4. **Expand templates**: `ExpandTemplates()` replaces `{{.var_name}}` in agent prompts with values from `Vars`
5. **Validate**: Check required fields, acyclicity, and backend compatibility
6. **Return**: Validated `Config` plus warning list (backend incompatibilities)

### 3. Agent Management (`internal/agent/`)

Core agent lifecycle and process management.

- **process.go** (150 LOC): `Process` spawns subprocess with configurable working directory, environment, and command; manages stdin/stdout/stderr pipes; graceful SIGTERM → SIGKILL shutdown
- **parser.go** (32 LOC): Consumes Process stdout line-by-line using bufio.Scanner; backend-agnostic via EventConverter pattern; handles malformed lines gracefully
- **manager.go** (304 LOC): `Manager` tracks agent state machine (`Pending → Running → Done|Failed|Killed|Blocked`); holds token usage, logs, and exit codes; uses `sync.RWMutex` for concurrent access; BatchEvents for TUI efficiency
- **backend/** (297 LOC): Backend interface + registry; Claude and Gemini CLI implementations with event type mapping

**Key Feature**: Multi-backend support — each agent independently targets Claude or Gemini CLI via registry pattern with `init()` self-registration.

### 4. DAG Scheduler (`internal/dag/`)

Event-driven dependency orchestration.

- **graph.go** (153 LOC): `Graph` struct holds adjacency lists (dependencies); Kahn's algorithm for acyclicity validation; topological sorting
- **scheduler.go** (212 LOC): Event-driven executor; tracks pending dependency counts per agent; decrements on completion; emits ready signals; uses `sync.Mutex` for critical sections; buffered event channel (1024 cap) prevents backpressure

### 5. TUI (`internal/tui/`)

Bubbletea composite model for interactive terminal UI.

- **app.go** (622 LOC): Root `AppModel` orchestrates sidebar, detail, statusbar, spawn modal; handles focus switching; event routing to children
- **sidebar.go** (153 LOC): Agent list with state indicators (●=running, ✓=done, ✗=failed); Vim-style nav (j/k)
- **detail.go** (222 LOC): Scrollable log viewer for selected agent; 10k-line ring buffer; auto-scroll with manual override
- **statusbar.go** (89 LOC): Progress counters and mode-specific keybinding hints
- **spawn.go** (123 LOC): Modal dialog for manual agent spawning with text inputs
- **styles.go** (83 LOC): Centralized Lipgloss palette; state icons; borders and spacing

**Design Pattern**: Identity vs Display Separation — `agentName` (identity for event matching) separate from `headerText` (formatted display string).

### 6. Headless Runner (`internal/headless/`)

Non-TUI pipeline execution for CI/CD and scripted workflows.

- **runner.go** (160 LOC): Orchestrates pipeline without TUI; signal handling (SIGINT/SIGTERM); polls agent events and formats output to stdout; exit codes reflect pipeline success/failure
- **formatter.go** (113 LOC): Text and NDJSON output formatters; text produces human-readable timestamped lines; NDJSON produces one JSON object per line for machine consumption

### 7. Logging (`internal/log/`)

- **writer.go** (77 LOC): Thread-safe JSONL event appender; lazy file creation per agent; audit trail to `~/.agentmux/logs/`

### 8. Result Writer (`internal/result/`)

- **writer.go** (34 LOC): Thread-safe markdown result saver; saves agent output to `{OutputDir}/results/{agentName}.md`

### 9. Health Check (`internal/health/`)

- **health.go** (22 LOC): Backend availability check via `exec.LookPath`; used by `agentmux check` command

## Key Design Patterns

### Event-Driven DAG Scheduling

1. DAG defines dependencies between agents
2. Scheduler tracks pending dependency counts in an in-memory map
3. When agent completes, scheduler decrements dependents' pending counts
4. Ready agents (pending count = 0) are spawned immediately
5. All state transitions emit Bubbletea messages to TUI for immediate feedback

### NDJSON Stream Parsing

Claude CLI real format (`stream-json` with `--verbose`):

- **Required Flag**: Claude CLI requires `--verbose` flag when using `--output-format stream-json` in print mode (`-p`). Without it, the format differs.
- **Event Types**: `system`, `assistant`, `result`, `rate_limit_event`
- **Message Structure**: `assistant` events contain `message.content` as an array of typed blocks:
  - Block types: `text`, `tool_use`, `tool_result` (NOT separate event types)
  - Process all blocks sequentially within an `assistant` event
- **Process Args**: `["-p", prompt, "--output-format", "stream-json", "--verbose", ...]`
- **Parser Implementation**: Uses `bufio.Scanner` for line-by-line parsing, NOT `json.Decoder`
  - `Scanner` recovers from malformed/non-JSON lines by skipping them
  - `json.Decoder` fails completely on any malformed line, causing data loss
  - Always validate event type before accessing fields
- **Async Consumption**: `Parser` consumes stdout asynchronously in a goroutine
- **Structured Events**: Emits `StartMsg`, `PingMsg`, `CompleteMsg` to manager based on event stream
- **State Aggregation**: Manager aggregates state and notifies TUI

### Batch Event Processing

High-throughput event streams require careful TUI rendering to prevent unresponsiveness:

- **WaitForEvent Pattern**: Block for the first event, then drain up to 50 buffered events non-blocking
- **BatchEvents Return**: Returns `[]tea.Msg` for the TUI to process in one render cycle
- **Performance Impact**: Batching prevents per-event re-renders, dramatically improving responsiveness
- **Buffer Cap**: Maximum 50 events per batch bounds processing time per render cycle
- **Scheduler Integration**: Scheduler's 1024-buffer event channel feeds into batch draining

### Identity vs Display Separation

Critical pattern for TUI model correctness:

- **Problem**: When `SetHeader()` overwrites an identity field with formatted content (e.g., `"researcher ● Running 2m..."`), subsequent event matching fails
- **Solution**: Maintain separate `agentName` (plain string for event matching) and `headerText` (formatted display string) fields
- **DetailModel Pattern**:
  - `agentName string` — identity field, used for `AppendLine("agentName", ...)` comparisons
  - `headerText string` — display field, updated by `SetHeader()` and rendered only
  - Both synchronized but never conflated
- **Why It Matters**: Agent event matching relies on exact string comparison; formatted strings break matching

### Goroutine-Per-Agent Model

- Each agent runs in its own `Process` goroutine with dedicated stdout reader
- Manager coordinates via message passing and atomic state updates
- Scheduler uses channels and mutexes to coordinate agent activation
- **Goroutine Lifecycle**: Process.EventCh must be closed after process exit to prevent goroutine leaks in `WaitForEvent`

### Bubbletea Composite UI

- Root `AppModel` contains child models (Sidebar, Detail, StatusBar, Spawn)
- Each child handles its own state, events, and rendering
- Parent routes Bubbletea commands and messages to children
- Global state (agent manager, scheduler) shared via pointers

## Dependencies

| Package | Use |
|---------|-----|
| `charmbracelet/bubbletea` | TUI framework, event loop, composable models |
| `charmbracelet/bubbles` | Reusable TUI components |
| `charmbracelet/lipgloss` | Terminal styling, layout |
| `spf13/cobra` | CLI framework, command structure |
| `gopkg.in/yaml.v3` | YAML configuration parsing |

## Data Flow

```
CLI (Cobra)
  ├─> run command
  │     ├─> Config (YAML parse + backend validation)
  │     ├─> Graph (DAG validation & topo sort)
  │     ├─> Manager (state tracking)
  │     ├─> Scheduler (dependency orchestration)
  │     ├─> TUI (Bubbletea root)        ← default mode
  │     │     ├─> Sidebar (agent list)
  │     │     ├─> Detail (logs)
  │     │     ├─> StatusBar (progress)
  │     │     └─> Spawn (manual trigger)
  │     └─> Headless (Runner)            ← --headless mode
  │           ├─> Text/NDJSON formatters
  │           └─> Signal handling (graceful shutdown)
  │                ├─> Process (subprocess per agent via backend)
  │                ├─> Parser (NDJSON reader)
  │                └─> Writer (JSONL audit log)
  │
  └─> check command
        └─> Backend availability check (exec.LookPath)
              └─> CLI output: ✓/✗ per backend
```

Each agent's Process spawns a subprocess and reads NDJSON output asynchronously. The Scheduler monitors completion and signals ready dependents. In TUI mode, the UI subscribes to manager events for real-time display. In headless mode, the Runner polls events and streams formatted output to stdout.

## Concurrency Model

- **Manager state**: Protected by `sync.RWMutex`
- **Scheduler critical sections**: Protected by `sync.Mutex`
- **Event channels**: Buffered channels for scheduler → TUI messaging
  - Agent event channels: 256 buffer (prevents agent goroutines from blocking)
  - Scheduler eventCh: 1024 buffer (scheduler produces faster than TUI consumes; large buffer prevents backpressure)
- **Per-agent goroutines**: Dedicated Process readers avoid contention
- **No shared buffers**: Each component owns its state
- **Manager.List() Safety**: Returns value copies (`cp := *info`), not pointers to shared state, preventing data races
- **Scheduler Execution**: Collects work items (blocked agents) under lock, executes side effects (channel sends, agent starts) after unlock to prevent deadlock
- **Process Cleanup**: Process.EventCh closed after process exit prevents goroutine leaks in `WaitForEvent` loops

## Configuration Schema

YAML structure:
```yaml
version: 1

vars:
  var_name: "value"
  project_root: "/path/to/project"
  code_lang: "go"

output_dir: ".agentmux-out"

defaults:
  backend: "claude"
  model: "sonnet"
  max_turns: 10
  allowedTools:
    - "Read"
    - "Write"

agents:
  agent_name:
    backend: "gemini"
    prompt: "Task: {{.var_name}} in {{.code_lang}}"
    model: "gemini-2.5-pro"
    max_turns: 5
    allowedTools:
      - "Bash"
    depends_on:
      - dependency_1
      - dependency_2
```

**Vars Section**: Template variables (map[string]string) expanded via Go `text/template` syntax. References in agent prompts use `{{.var_name}}` syntax.

**Output Dir**: Root directory for logs and results. Defaults to `.agentmux-out`. CLI flag `--output-dir` overrides YAML value.

**Defaults Section**: Global defaults for backend, model, max_turns, and allowedTools. Merged into each agent via `ApplyDefaults()`.

**Agents Section**: Individual agent definitions with prompt (supports template expansion), backend override, model, tool restrictions, and DAG dependencies (`depends_on`).

## CLI Flag Precedence

CLI flags override YAML values with this precedence (highest to lowest):

1. **CLI flags**: `--var`, `--output-dir`, `--config`, `--headless`, `--format`
2. **YAML config**: `vars`, `output_dir`, `defaults`, `agents`
3. **Built-in defaults**: `.agentmux-out`, `sonnet` model, `claude` backend

**Example**: Running `agentmux run -c cfg.yaml --var x=cli-val --output-dir /tmp/logs` with YAML containing `vars: {x: yaml-val}` and `output_dir: ~/.agentmux`:
- `x` becomes `cli-val` (CLI override)
- `output_dir` becomes `/tmp/logs` (CLI override)

The Config package parses this into typed structs (`Config`, `AgentDefaults`, `AgentConfig`) for downstream validation and use. See [Configuration Guide](./configuration.md) for full reference.

---

**Document Version**: v0.1.0
**Last Updated**: 2026-03-01
