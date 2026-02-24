# agentmux Codebase Summary

Quick reference for navigating the agentmux codebase. Total: ~2,900 LOC code + ~1,223 LOC tests across 18 Go files, 7 packages.

## Module

```
github.com/hoadh/agentmux
Go 1.26+
```

## Package Overview

### cmd/ — CLI Entry Points (108 LOC)

CLI framework integration via Cobra. Entry point for all command-line operations.

| File | LOC | Exports | Purpose |
|------|-----|---------|---------|
| root.go | 22 | `Execute()` | Cobra root command, global flags, error handling |
| run.go | 65 | `runCmd` | Launch command: config load → DAG build → manager init → TUI start |
| version.go | 21 | `versionCmd` | Display version info |

**Key Types**: None (uses Config and Manager from internal packages)

**Key Functions**:
- `Execute()`: Cobra command dispatcher; entry point from main.go
- `runCmd.Run()`: Orchestrates pipeline startup (config → dag → manager → tui)

### internal/config/ — Configuration (398 LOC)

YAML parsing, validation, and type definitions.

| File | LOC | Exports | Purpose |
|------|-----|---------|---------|
| config.go | 103 | `Config`, `AgentDefaults`, `AgentConfig`, `LoadConfig()` | YAML struct definitions; parsing and validation |
| config_test.go | 295 | Test fixtures in testdata/ | Tests for valid/invalid YAML; defaults merging |

**Key Types**:
- `Config`: Root struct with `Defaults`, `Agents` map, `Pipeline` map
- `AgentDefaults`: Global model, max_turns, allowedTools
- `AgentConfig`: Individual agent with prompt, model override, dependencies, tool restrictions

**Key Functions**:
- `LoadConfig(path string) (*Config, error)`: Load and validate YAML from file
- `(c *Config) Validate() error`: Check for missing required fields, cyclic deps

**Test Fixtures**: `testdata/valid.yaml`, `testdata/invalid.yaml`, `testdata/missing_fields.yaml`

### internal/agent/ — Agent Lifecycle & I/O (1,352 LOC)

Process spawning, NDJSON parsing, state management, token tracking.

| File | LOC | Exports | Purpose |
|------|-----|---------|---------|
| process.go | 171 | `Process`, `NewProcess()` | Subprocess spawning; stdin/stdout/stderr pipes; graceful shutdown |
| parser.go | 185 | `NDJSONParser`, `Parse()` | Line-by-line NDJSON consumption; event type handling |
| manager.go | 270 | `Manager`, `AgentInfo`, `AgentState`, `TokenUsage` | Lifecycle state machine; concurrency-safe agent tracking |
| manager_test.go | 400+ | Test fixtures | Concurrent access tests; state transition validation |
| parser_test.go | 300+ | Test fixtures | NDJSON event parsing; malformed line recovery |

**Key Types**:
- `Process`: Subprocess handle with pipes and event channel
- `NDJSONParser`: Line-by-line reader; emits typed events
- `Manager`: Tracks agent state, token usage, logs; uses sync.RWMutex
- `AgentInfo`: State snapshot (name, state, tokens, exit code)
- `AgentState`: Enum (Pending, Running, Done, Failed, Killed, Blocked)
- `TokenUsage`: Input/output token counters

**Key Functions**:
- `NewProcess(cmd string, workDir string) (*Process, error)`: Spawn subprocess
- `(p *Process) Kill() error`: Graceful SIGTERM + 5s → SIGKILL shutdown
- `(p *Process) EventChannel() <-chan tea.Msg`: Subscribe to process events
- `NewManager() *Manager`: Create concurrent-safe agent tracker
- `(m *Manager) SpawnAgent(cfg *AgentConfig) error`: Start subprocess and parser
- `(m *Manager) UpdateState(name string, state AgentState)`: Transition agent state
- `(m *Manager) List() []*AgentInfo`: Return value copies (thread-safe snapshots)

**Concurrency Patterns**:
- Manager uses sync.RWMutex for all state access
- List() returns copies, not pointers, to prevent data races
- Parser runs in dedicated goroutine per agent; events sent via channel

### internal/dag/ — Dependency Orchestration (867 LOC)

DAG validation, topological sorting, event-driven scheduling.

| File | LOC | Exports | Purpose |
|------|-----|---------|---------|
| graph.go | 153 | `Graph`, `BuildFromConfig()` | Adjacency list; Kahn's cycle detection; topo sort |
| scheduler.go | 212 | `Scheduler`, `ProcessAgentDone()` | Event-driven executor; pending count tracking |
| graph_test.go | 200+ | Test fixtures | Cycle detection; topological sort validation |
| scheduler_test.go | 300+ | Test fixtures | Concurrent agent ready events; scheduling correctness |

**Key Types**:
- `Graph`: Directed acyclic graph with adjacency lists; in-degree tracking
- `Scheduler`: Manages pending dependency counts; emits ready signals
- `AgentStartedMsg`, `AgentBlockedMsg`, `PipelineDoneMsg`: Bubbletea messages

**Key Functions**:
- `BuildFromConfig(cfg *config.Config) (*Graph, error)`: Create graph from YAML pipeline
- `(g *Graph) ValidateAcyclic() error`: Kahn's algorithm; detects cycles
- `(g *Graph) TopologicalSort() []string`: Order agents by dependency
- `NewScheduler(g *Graph, mgr *agent.Manager) *Scheduler`: Create event dispatcher
- `(s *Scheduler) ProcessAgentDone(agent string)`: Decrement dependents; emit ready signals
- `(s *Scheduler) WaitForEvent() tea.Msg`: Block for first event; return batch of up to 50

**Concurrency Patterns**:
- Scheduler uses sync.Mutex for pending count map
- "Collect under lock, execute after unlock" pattern prevents deadlock
- Buffered event channel (1024) prevents backpressure on agent goroutines

### internal/tui/ — Interactive Dashboard (1,128 LOC)

Bubbletea composite UI: sidebar, detail view, statusbar, spawn modal.

| File | LOC | Exports | Purpose |
|------|-----|---------|---------|
| app.go | 513 | `AppModel`, `NewApp()` | Root model; focus management; event routing to children |
| sidebar.go | 137 | `SidebarModel` | Agent list with state indicators; selection |
| detail.go | 196 | `DetailModel` | Agent logs viewport; header with metadata |
| spawn.go | 124 | `SpawnModel` | Modal for manual agent spawn or pipeline trigger |
| statusbar.go | 83 | `StatusBarModel` | Progress bar; pipeline stats; help legend |
| styles.go | 75 | Lipgloss styles | Centralized colors, borders, spacing |

**Key Types**:
- `AppModel`: Root Bubbletea model; coordinates child models
- `SidebarModel`: List of agents with selection index
- `DetailModel`: Viewport for selected agent logs; identity vs display separation
- `SpawnModel`: Text input and button controls
- `StatusBarModel`: Progress counters and keybinding legend

**Key Functions**:
- `NewApp(mgr *agent.Manager, sch *dag.Scheduler) *AppModel`: Create root model
- `(m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd)`: Event dispatch to children
- `(m *AppModel) View() string`: Render composite layout
- `(s *SidebarModel) AppendAgent(name string)`: Add agent to list
- `(d *DetailModel) AppendLine(agent, line string)`: Add log line to selected agent
- `(d *DetailModel) SetHeader(text string)`: Update display header (not identity)

**UI Layout**:
```
┌─ HEADER (agentmux) ─────────────────────┐
├──────────────┬──────────────────────────┤
│ SIDEBAR(28w) │ DETAIL (rest)            │
│ • Agent 1    │ agent_1 (Running)        │
│ ✓ Agent 2    │ [output logs...]         │
│ ✗ Agent 3    │                          │
├──────────────┴──────────────────────────┤
│ 3/5 agents | Running: 2 | Help: ?      │
└──────────────────────────────────────────┘
```

**Keybindings**:
- `j`/`k`: Navigate sidebar
- `Tab`: Cycle focus
- `n`: Spawn agent
- `K`: Kill agent
- `r`: Restart agent
- `l`: Toggle log
- `p`: Pipeline view
- `q`: Quit

**Design Pattern**: Identity vs Display Separation
- `agentName` (identity): Exact string for event matching
- `headerText` (display): Formatted string for rendering (e.g., "agent_1 ● Running 2m")
- Both synchronized but never conflated

### internal/log/ — Event Audit Trail (78 LOC)

Persistent JSONL event logging.

| File | LOC | Exports | Purpose |
|------|-----|---------|---------|
| writer.go | 78 | `Writer`, `NewWriter()` | Append agent events to JSONL file; thread-safe |

**Key Types**:
- `Writer`: Thread-safe event appender with sync.Mutex

**Key Functions**:
- `NewWriter(logDir string) (*Writer, error)`: Create audit log writer
- `(w *Writer) WriteEvent(agent string, event interface{}) error`: Append JSONL line

**Output**: `~/.agentmux/logs/{timestamp}.jsonl` with agent name, event type, timestamp

## Dependency Graph

```
cmd/
  ├─> internal/config
  └─> internal/agent
      └─> internal/dag
          ├─> internal/tui
          └─> internal/log

internal/tui
  ├─> internal/agent    (Manager, AgentState)
  ├─> internal/dag      (Scheduler)
  └─> internal/log      (Writer)
```

## External Dependencies

| Package | Version | Use |
|---------|---------|-----|
| charmbracelet/bubbletea | v1.3.10 | TUI event loop, models, commands |
| charmbracelet/bubbles | v1.0.0 | Viewport, textinput components |
| charmbracelet/lipgloss | v1.1.0 | Styling, layout, borders |
| spf13/cobra | v1.10.2 | CLI framework, command parsing |
| gopkg.in/yaml.v3 | v3.0.1 | YAML unmarshaling |

## Testing Overview

### Test Coverage

| Package | Tests | Coverage | Focus |
|---------|-------|----------|-------|
| config | 15 | >90% | YAML parsing, validation, defaults |
| agent | 30+ | >85% | Process lifecycle, parser, concurrent access |
| dag | 20+ | >90% | Cycle detection, topo sort, scheduling |
| tui | 10+ | >70% | Model composition, event routing |

### Test Patterns

- **Fixtures**: YAML and NDJSON samples in `testdata/` per package
- **Mocking**: Interface-based dependency injection for unit tests
- **Concurrency**: sync.WaitGroup for goroutine coordination in concurrent tests
- **Coverage Target**: >80% on critical paths (parser, scheduler, manager)

### Running Tests

```bash
go test ./...              # All tests
go test -cover ./...       # With coverage
go test -race ./...        # Detect data races
go test -v ./...           # Verbose output
```

## Entry Points

| Entry Point | File | Responsibility |
|-------------|------|-----------------|
| `main()` | main.go | Load and call cmd.Execute() |
| `Execute()` | cmd/root.go | Dispatch Cobra commands |
| `runCmd.Run()` | cmd/run.go | Orchestrate pipeline startup |
| `NewApp()` | tui/app.go | Initialize TUI root model |

## File Organization Best Practices

1. **One concern per file**: agent/process.go handles subprocess only
2. **Exported types in header**: Sorted by type definition order
3. **Constructors first**: NewX() functions precede methods
4. **Receivers last**: Value and pointer receiver methods grouped by type
5. **Tests paired**: agent.go paired with agent_test.go in same directory

## Performance Characteristics

| Operation | Complexity | Notes |
|-----------|-----------|-------|
| Load config | O(n) agents | YAML parse and validate |
| Build DAG | O(n + e) agents+edges | Kahn's algorithm |
| Spawn agent | O(1) + fork | Subprocess creation overhead ~50ms |
| Event batch | O(min(50, buffered)) | TUI render cycle |
| Manager lookup | O(1) + RWMutex lock | Typical <1µs contention |

## Code Quality Metrics

- **Total LOC**: ~2,900 (code) + ~1,223 (tests)
- **Avg File Size**: ~150 LOC per file (ideal for context)
- **Max File Size**: 513 LOC (app.go) — candidate for splitting in v0.2
- **Test Count**: 70+ tests
- **Coverage**: >80% on critical paths

## Quick Navigation

**Find what you need:**
- Configuration parsing? → `internal/config/config.go`
- Agent process management? → `internal/agent/process.go`
- NDJSON parsing? → `internal/agent/parser.go`
- DAG scheduling? → `internal/dag/scheduler.go`
- TUI layout? → `internal/tui/app.go`
- Keybindings? → Search for `HandleMsg` in tui/app.go
- Tests? → Look for `_test.go` files; fixtures in `testdata/`

---

**Document Version**: v0.1.0
**Last Updated**: 2026-02-25
