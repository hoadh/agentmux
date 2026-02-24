# System Architecture

## Overview

Agentmux is a Go TUI application for spawning, monitoring, and orchestrating Claude Code agents in parallel using DAG pipelines. The system enables users to define agent workflows in YAML, execute them with dependency management, and observe live progress through a terminal interface.

**Module:** `github.com/hoadh/agentmux`
**Go Version:** 1.26+

## Architecture Layers

### 1. CLI Layer (`cmd/`)

- **root.go**: Cobra CLI entry point, flags, configuration loading
- **run.go**: Launch command initializing config, DAG, manager, and TUI
- **version.go**: Version information

Cobra provides hierarchical command structure and flag parsing.

### 2. Configuration (`internal/config/`)

- **config.go**: YAML struct parsing and validation
  - Loads `agentmux.yaml` (agent definitions, pipeline DAG, global settings)
  - Defines `Config`, `Agent`, and `Pipeline` structs
  - Returns typed configuration for downstream use

### 3. Agent Management (`internal/agent/`)

Core agent lifecycle and process management.

- **process.go**: `Process` spawns subprocess with configurable working directory, environment, and command; manages stdin/stdout/stderr pipes
- **parser.go**: `NDJSONParser` consumes Process stdout line-by-line, emits typed NDJSON events (`StartMsg`, `PingMsg`, `CompleteMsg`, `ErrorMsg`, etc.)
- **manager.go**: `Manager` tracks agent state machine (`Pending → Running → Done|Failed|Killed|Blocked`); holds token usage, logs, and exit codes; uses `sync.RWMutex` for concurrent access; dispatches agent lifecycle events to TUI
- **Process & Parser**: Goroutine per agent reads live output asynchronously

### 4. DAG Scheduler (`internal/dag/`)

Event-driven dependency orchestration.

- **graph.go**: `Graph` struct holds adjacency lists (dependencies) and topological sorting; validates acyclicity
- **scheduler.go**: `Scheduler` tracks pending dependency counts per agent; monitors agent completion; emits `AgentStartedMsg`, `AgentBlockedMsg`, `PipelineDoneMsg` to TUI; spawns agents when deps resolve; uses `sync.Mutex` for critical section safety

### 5. TUI (`internal/tui/`)

Bubbletea composite model for interactive terminal UI.

- **app.go**: Root `AppModel` orchestrates sidebar, detail, statusbar, spawn modal; handles focus switching; integrates agent manager and scheduler
- **sidebar.go**: `SidebarModel` displays agent list with state indicators; supports selection and filtering
- **detail.go**: `DetailModel` shows selected agent logs, metadata, token usage; scrollable ANSI content
- **statusbar.go**: `StatusBarModel` displays pipeline progress, total agents, running count, legend
- **spawn.go**: `SpawnModel` modal for spawning new agents or triggering pipeline runs (manual trigger UI)
- **styles.go**: Centralized Lipgloss styles (colors, borders, spacing for light/dark themes)

Bubbletea's composite pattern allows modular UI components with local state and messages.

### 6. Logging (`internal/log/`)

- **writer.go**: `Writer` appends agent events to JSONL output file; thread-safe with `sync.Mutex`; records timestamps, agent names, event types, and output

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
  ├─> Config (YAML parse)
  ├─> Graph (DAG validation & topo sort)
  ├─> Manager (state tracking)
  ├─> Scheduler (dependency orchestration)
  └─> TUI (Bubbletea root)
        ├─> Sidebar (agent list)
        ├─> Detail (logs)
        ├─> StatusBar (progress)
        └─> Spawn (manual trigger)
             ├─> Process (subprocess per agent)
             ├─> Parser (NDJSON reader)
             └─> Writer (JSONL audit log)
```

Each agent's Process spawns a subprocess and reads NDJSON output asynchronously. The Scheduler monitors completion and signals ready dependents. The TUI subscribes to manager events and updates display in real-time.

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
defaults:
  model: "sonnet"
  max_turns: 10

agents:
  agent_name:
    prompt: "Task description"
    model: "sonnet"
    max_turns: 5
    allowed_tools:
      - "Read"
      - "Write"

pipeline:
  dependent_agent:
    - dependency_1
    - dependency_2
```

**Defaults Section**: Global defaults for model, max_turns, and other agent parameters.

**Agents Section**: Individual agent definitions with prompt, model override, tool restrictions.

**Pipeline Section**: DAG definition where each agent lists its dependencies (agents that must complete before it starts).

The Config package parses this into typed structs for downstream validation and use.
