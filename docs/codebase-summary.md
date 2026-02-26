# agentmux Codebase Summary

Quick reference for navigating the agentmux codebase. Total: ~2,688 LOC code + ~1,579 LOC tests across 21 Go files, 8 packages.

## Module

```
github.com/hoadh/agentmux
Go 1.24.2+
```

## Package Overview

### cmd/ — CLI Entry Points (111 LOC)

CLI framework integration via Cobra. Entry point for all command-line operations.

| File | LOC | Exports | Purpose |
|------|-----|---------|---------|
| root.go | 22 | `Execute()` | Cobra root command, global flags, error handling |
| run.go | 68 | `runCmd` | Launch command: config load → DAG build → manager init → TUI start |
| version.go | 21 | `versionCmd` | Display version info |

**Key Functions**:
- `Execute()`: Cobra command dispatcher; entry point from main.go
- `runCmd.Run()`: Orchestrates pipeline startup (config → dag → manager → tui)

### internal/config/ — Configuration (121 LOC code + tests)

YAML parsing, validation, and type definitions.

| File | LOC | Exports | Purpose |
|------|-----|---------|---------|
| config.go | 121 | `Config`, `AgentDefaults`, `AgentConfig`, `LoadConfig()` | YAML struct definitions; parsing and validation with defaults merging |

**Key Types**:
- `Config`: Root struct with `Defaults`, `Agents` map
- `AgentDefaults`: Global defaults: backend, model, max_turns, allowedTools
- `AgentConfig`: Individual agent with prompt, model, backend, dependencies, tool restrictions

**Key Functions**:
- `LoadConfig(path string) (*Config, error)`: Load and validate YAML from file
- `(c *Config) ApplyDefaults()`: Merge global defaults into each agent
- `(c *Config) ValidateAcyclic() error`: Cycle detection at load time

**Validation**: Hard errors abort startup (missing prompt, unknown backend, cyclic deps). Warnings for backend incompatibilities (e.g., Gemini ignores allowedTools).

### internal/agent/ — Agent Lifecycle & I/O (463 LOC code + 1,579 LOC tests)

Process spawning, NDJSON parsing, state management, backend selection.

| File | LOC | Purpose |
|------|-----|---------|
| manager.go | 282 | Lifecycle state machine; token tracking; sync.RWMutex for concurrency |
| process.go | 151 | Subprocess spawning; graceful SIGTERM → SIGKILL; pipes for I/O |
| parser.go | 32 | Line-by-line NDJSON reader via bufio.Scanner; backend-agnostic |
| backend/backend.go | 111 | Backend registry interface; shared event types |
| backend/claude.go | 103 | Claude CLI backend; args construction and event mapping |
| backend/gemini.go | 85 | Gemini CLI backend; args construction and event mapping |

**Key Types**:
- `Manager`: Tracks agent state, token usage, logs; sync.RWMutex protected
- `Process`: Subprocess handle with stdout/stderr/stdin pipes
- `AgentState`: Enum (Pending, Running, Done, Failed, Killed, Blocked)
- `Backend`: Interface for CLI backends with `Args()` and `ConvertEvent()` methods
- `BackendRegistry`: Self-registering via `init()` in claude.go and gemini.go

**Multi-Backend Architecture**:
- Each agent independently selects Claude or Gemini via config `backend` field
- Registry pattern allows zero-code extension (new backend = 1 file with `init()`)
- Shared event types abstract backend-specific differences
- Claude: Full feature support (model, allowedTools, max_turns)
- Gemini: Model supported; allowedTools and max_turns ignored (warnings emitted)

**Concurrency**:
- Manager protected by sync.RWMutex; List() returns value copies
- Process lives in dedicated goroutine per agent
- Parser consumes stdout asynchronously; bufio.Scanner recovers from malformed lines

### internal/dag/ — Dependency Orchestration (364 LOC code + tests)

DAG validation, topological sorting, event-driven scheduling.

| File | LOC | Purpose |
|------|-----|---------|
| graph.go | 153 | Adjacency list; Kahn's cycle detection; topological sort |
| scheduler.go | 212 | Event-driven executor; pending count map; ready signal emission |

**Key Types**:
- `Graph`: Directed acyclic graph; adjacency lists; in-degree tracking
- `Scheduler`: Pending count map per agent; ready signal emitter

**Design Pattern**: Event-driven orchestration
1. DAG defines dependencies (who depends on whom)
2. Scheduler tracks pending count for each agent
3. On agent completion, decrement dependents' pending counts
4. When pending count reaches 0, agent is ready → emit signal
5. Manager spawns ready agents immediately
6. Buffered event channel (1024) prevents backpressure

**Concurrency**: Scheduler uses sync.Mutex; "collect under lock, execute after unlock" pattern prevents deadlock when sending on channels.

### internal/tui/ — Interactive Dashboard (1,239 LOC code + tests)

Bubbletea composite UI: sidebar, detail view, statusbar, spawn modal.

| File | LOC | Purpose |
|------|-----|---------|
| app.go | 593 | Root model; focus management; event routing to children |
| sidebar.go | 137 | Agent list with state indicators; Vim navigation (j/k) |
| detail.go | 218 | Scrollable log viewport; 10k-line ring buffer with auto-scroll |
| spawn.go | 124 | Modal for manual agent spawning with text inputs |
| statusbar.go | 90 | Progress stats and mode-specific keybinding legend |
| styles.go | 82 | Lipgloss palette; state icons; colors and spacing |

**Key Design Patterns**:

1. **Composite Model**: Root AppModel coordinates child models (sidebar, detail, statusbar, spawn)
2. **Identity vs Display Separation**: DetailModel maintains:
   - `agentName`: Plain identity string for event matching
   - `headerText`: Formatted display string for rendering
   - Never conflate; prevents silent log drops
3. **Batch Event Processing**: Collect up to 50 events per render cycle for responsiveness
4. **Ring Buffer**: DetailModel uses 10k-line ring buffer; trims 20% when full

**Keybindings**:
- `j`/`k`: Navigate agents (Vim style)
- `Tab`: Cycle focus between panels
- `n`: Spawn new agent (modal dialog)
- `K`: Kill selected agent (SIGTERM → 5s → SIGKILL)
- `r`: Restart agent
- `l`: Toggle log verbose mode
- `p`: Show pipeline DAG
- `?`: Display help
- `q`: Quit gracefully

### internal/log/ — Event Audit Trail (77 LOC code)

Persistent JSONL event logging.

| File | LOC | Purpose |
|------|-----|---------|
| writer.go | 77 | Thread-safe JSONL event logging to `~/.agentmux/logs/` |

**Key Feature**: Lazy file creation per agent; one JSONL file per run. Each line is a structured event with timestamp, agent name, and event type. Thread-safe via sync.Mutex.

## Dependency Graph

```
main.go → cmd/root.go → cmd/run.go
                          ├→ internal/config
                          ├→ internal/dag
                          ├→ internal/agent
                          │  ├→ internal/agent/backend (Claude, Gemini)
                          │  └→ internal/log
                          └→ internal/tui
                             ├→ internal/agent
                             └→ internal/dag
```

**Key Property**: Zero circular imports; strict layering enforced.

## External Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| charmbracelet/bubbletea | v1.3.5 | TUI framework, event loop, models |
| charmbracelet/bubbles | v0.21.0 | Viewport, textinput components |
| charmbracelet/lipgloss | v1.1.0 | Terminal styling, layout |
| spf13/cobra | v1.9.1 | CLI framework, command structure |
| gopkg.in/yaml.v3 | v3.x | YAML config parsing |

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

- **Total LOC**: ~2,688 (code) + ~1,579 (tests) = ~4,267 total
- **Avg File Size**: ~127 LOC per file (excellent for context)
- **Max File Size**: 593 LOC (app.go) — reasonable for root model
- **Test Count**: 70+ tests across 7 packages
- **Coverage**: >80% on critical paths (parser, scheduler, manager)

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
