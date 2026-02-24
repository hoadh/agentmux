# System Architecture

## Overview

Agentmux is a Go TUI application for spawning, monitoring, and orchestrating Claude Code agents in parallel using DAG pipelines. The system enables users to define agent workflows in YAML, execute them with dependency management, and observe live progress through a terminal interface.

**Module:** `github.com/hoadh/agentmux`
**Go Version:** 1.24+

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

- Each agent process outputs NDJSON (one JSON object per line)
- `Parser` consumes stdout asynchronously in a goroutine
- Emits structured events (`StartMsg`, `PingMsg`, `CompleteMsg`) to manager
- Manager aggregates state and notifies TUI

### Goroutine-Per-Agent Model

- Each agent runs in its own `Process` goroutine with dedicated stdout reader
- Manager coordinates via message passing and atomic state updates
- Scheduler uses channels and mutexes to coordinate agent activation

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
- **Per-agent goroutines**: Dedicated Process readers avoid contention
- **No shared buffers**: Each component owns its state

## Configuration Schema

YAML structure:
```yaml
agents:
  agent_name:
    cmd: "command args"
    workdir: "/path"
    env:
      KEY: "value"
pipeline:
  agent_name:
    - dependency_1
    - dependency_2
```

The Config package parses this into typed structs for downstream validation and use.
