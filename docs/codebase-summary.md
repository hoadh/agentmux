# agentmux Codebase Summary

Quick reference for navigating the agentmux codebase. Total: ~3,305 LOC Go code + 2,619 LOC tests + 422 LOC shell across multiple Go files and 11 packages.

## Module

```
github.com/hoadh/agentmux
Go 1.24.2+
```

## Package Overview

### cmd/ — CLI Entry Points (214 LOC)

CLI framework integration via Cobra. Entry point for all command-line operations.

| File | LOC | Exports | Purpose |
|------|-----|---------|---------|
| root.go | 21 | `Execute()` | Cobra root command, global flags, error handling |
| run.go | 135 | `runCmd` | Launch command: config load → DAG build → manager init → TUI or headless start |
| serve.go | 33 | `serveCmd` | HTTP health check server: listens on configurable address, returns JSON health report |
| version.go | 25 | `versionCmd` | Display version info |

**Key Functions**:
- `Execute()`: Cobra command dispatcher; entry point from main.go
- `runCmd.Run()`: Orchestrates pipeline startup (config → dag → manager → tui)

**CLI Flags** (defined in `run.go`):
- `--config` / `-c`: Config file path (default: `agentmux.yaml`)
- `--headless`: Run without TUI
- `--format`: Output format (`text` or `ndjson`, headless only)
- `--output-dir`: Custom output directory for logs/results
- `--var`: Template variable override (repeatable, key=value format)

### internal/config/ — Configuration (175 LOC code + tests)

YAML parsing, validation, template expansion, and type definitions.

| File | LOC | Exports | Purpose |
|------|-----|---------|---------|
| config.go | 175 | `Config`, `AgentDefaults`, `AgentConfig`, `LoadConfig()`, `ExpandTemplates()` | YAML struct definitions; parsing, validation, defaults merging, and template variable expansion |

**Key Types**:
- `Config`: Root struct with `Version`, `Vars` map, `OutputDir`, `Defaults`, `Agents` map
- `AgentDefaults`: Global defaults: backend, model, max_turns, allowedTools
- `AgentConfig`: Individual agent with prompt, model, backend, dependencies, tool restrictions

**Key Functions**:
- `LoadConfig(path string) (*Config, []string, error)`: Load, validate, and expand templates from YAML file
- `ExpandTemplates(cfg *Config) error`: Expand `{{.var_name}}` placeholders in agent prompts using cfg.Vars
- `ApplyDefaults(cfg *Config)`: Merge global defaults into each agent
- `Validate(cfg *Config) ([]string, error)`: Cycle detection and field validation at load time

**Features**:
- **Template Variables**: `vars` section defines map[string]string; agent prompts use `{{.var_name}}` Go template syntax
- **Output Directory**: `output_dir` field (string) configures root dir for logs/results; defaults to `.agentmux-out`
- **Validation**: Hard errors abort startup (missing prompt, unknown backend, cyclic deps). Warnings for backend incompatibilities (e.g., Gemini ignores allowedTools).

### internal/agent/ — Agent Lifecycle & I/O (486 LOC + 297 backend LOC + tests)

Process spawning, NDJSON parsing, state management, backend selection.

| File | LOC | Purpose |
|------|-----|---------|
| manager.go | 304 | Lifecycle state machine; token tracking; sync.RWMutex for concurrency |
| process.go | 150 | Subprocess spawning; graceful SIGTERM → SIGKILL; pipes for I/O |
| parser.go | 32 | Line-by-line NDJSON reader via bufio.Scanner; backend-agnostic |
| backend/backend.go | 110 | Backend registry interface; shared event types |
| backend/claude.go | 103 | Claude CLI backend; args construction and event mapping |
| backend/gemini.go | 84 | Gemini CLI backend; args construction and event mapping |

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

### internal/tui/ — Interactive Dashboard (1,292 LOC code + tests)

Bubbletea composite UI: sidebar, detail view, statusbar, spawn modal.

| File | LOC | Purpose |
|------|-----|---------|
| app.go | 622 | Root model; focus management; event routing to children |
| sidebar.go | 153 | Agent list with state indicators; Vim navigation (j/k) |
| detail.go | 222 | Scrollable log viewport; 10k-line ring buffer with auto-scroll |
| spawn.go | 123 | Modal for manual agent spawning with text inputs |
| statusbar.go | 89 | Progress stats and mode-specific keybinding legend |
| styles.go | 83 | Lipgloss palette; state icons; colors and spacing |

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

### internal/headless/ — Non-TUI Pipeline Execution (290 LOC code + tests)

Headless (non-interactive) mode for automation and CI/CD integration.

| File | LOC | Purpose |
|------|-----|---------|
| runner.go | 177 | Main orchestration; config load, DAG build, agent spawning without TUI |
| formatter.go | 113 | Output formatting; converts agent events to structured text (JSON, plain text) |

**Design Pattern**: Headless mode reuses config, DAG, Manager, and event handling logic from TUI; replaces interactive Bubbletea with formatter-based output.

**Key Features**:
- Automatic pipeline execution without TUI interaction
- Real-time event streaming to stdout or file
- Exit codes for pipeline success/failure
- Signal handling (SIGTERM → graceful shutdown)
- Suitable for CI/CD, cron jobs, and batch processing

### internal/health/ — Health Check Server (61 LOC code)

Health status endpoint for monitoring and orchestration platforms.

| File | LOC | Purpose |
|------|-----|---------|
| health.go | 61 | Health check report: uptime, version, startup time; JSON response format |

**Key Feature**: Returns JSON health status for HTTP `/health` endpoint. Used by `agentmux serve` command to provide monitoring integration for load balancers and orchestration systems.

### internal/log/ — Event Audit Trail (77 LOC code)

Persistent JSONL event logging.

| File | LOC | Purpose |
|------|-----|---------|
| writer.go | 77 | Thread-safe JSONL event logging to `~/.agentmux/logs/` |

**Key Feature**: Lazy file creation per agent; one JSONL file per run. Each line is a structured event with timestamp, agent name, and event type. Thread-safe via sync.Mutex.

### internal/result/ — Result Export (34 LOC code)

Agent output persistence as markdown files.

| File | LOC | Purpose |
|------|-----|---------|
| writer.go | 34 | Save agent results as markdown to `{OutputDir}/results/{agentName}.md` |

**Key Feature**: Thread-safe result writer with sync.Mutex. Creates target directory if needed. One markdown file per agent per run. Overwrites existing files.

### scripts/ — Deployment & Installation (423 LOC)

| File | LOC | Language | Purpose |
|------|-----|----------|---------|
| install.sh | 138 | POSIX Shell | Interactive installer for placing agentmux executable in system PATH |
| parallel-run.sh | 285 | POSIX Shell | Batch pipeline runner: executes multiple agentmux pipelines in parallel with job limiting, isolated output directories, and summary reporting |

**install.sh Key Features**:
- Validates executable and prompts to make executable if needed
- Offers choice of installation targets: `/usr/local/bin`, `~/.local/bin`, or custom path
- Handles permission elevation (sudo) automatically
- Verifies successful installation
- Provides PATH configuration hints if needed

**Usage**: `./scripts/install.sh ./agentmux [--name custom-name]`

**parallel-run.sh Key Features**:
- Runs multiple pipelines in parallel with configurable job limiting (default: 4 parallel jobs)
- Two input modes: inline (`--` separators) or manifest file (one pipeline per line)
- Isolated output directories per pipeline; derives slug from template variables
- Per-pipeline duration tracking and pass/fail status
- Summary table with results (e.g., "1 passed, 1 failed out of 2 total")
- Safe signal handling: kills child processes on Ctrl+C; no shell eval (injection-safe)
- POSIX-compatible (sh not bash)

**Usage Examples**:
```bash
# Inline mode: run 2 pipelines sequentially with different --var values
./scripts/parallel-run.sh -c config.yaml \
  -- --var topic="dogs" \
  -- --var topic="cats"

# Manifest mode: read 1 pipeline spec per line from file
echo '--var topic="dogs"' > pipelines.txt
echo '--var topic="cats"' >> pipelines.txt
./scripts/parallel-run.sh -c config.yaml -f pipelines.txt

# Custom job limit (6 parallel, not 4)
./scripts/parallel-run.sh -c config.yaml -j 6 -f pipelines.txt
```

**CLI Flags**:
| Flag | Default | Purpose |
|------|---------|---------|
| `-c` | `agentmux.yaml` | Config file |
| `-f` | (none) | Manifest file (one spec per line) |
| `-j` | `4` | Max parallel jobs |
| `-o` | `.agentmux-out` | Base output directory |
| `-h` | — | Help |

## Dependency Graph

```
main.go → cmd/root.go → cmd/run.go
                          ├→ internal/config
                          ├→ internal/dag
                          ├→ internal/agent
                          │  ├→ internal/agent/backend (Claude, Gemini)
                          │  └→ internal/log
                          ├→ internal/result
                          ├→ internal/tui (interactive mode)
                          │  ├→ internal/agent
                          │  └→ internal/dag
                          └→ internal/headless (non-interactive mode)
                             ├→ internal/config
                             ├→ internal/dag
                             ├→ internal/agent
                             ├→ internal/log
                             └→ internal/result
```

**Key Property**: Zero circular imports; strict layering enforced. Headless and TUI are mutually exclusive modes sharing lower-level packages. Result writer is reusable across modes.

## External Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| charmbracelet/bubbletea | v0.27.x+ | TUI framework, event loop, models |
| charmbracelet/bubbles | v0.20.x+ | Viewport, textinput components |
| charmbracelet/lipgloss | v0.12.x+ | Terminal styling, layout |
| spf13/cobra | v1.8.x+ | CLI framework, command structure |
| gopkg.in/yaml.v3 | v3.x | YAML config parsing |

See `go.mod` for exact pinned versions.

## Testing Overview

### Test Coverage

| Package | Tests | Coverage | Focus |
|---------|-------|----------|-------|
| config | 15 | >90% | YAML parsing, validation, defaults |
| agent | 35+ | >85% | Process lifecycle, parser, concurrent access |
| dag | 25+ | >90% | Cycle detection, topo sort, scheduling |
| tui | 10+ | >70% | Model composition, event routing |
| headless | 15+ | >80% | Runner, formatter, event processing |

**Total**: 130+ tests across 9 packages

### Test Patterns

- **Fixtures**: YAML and NDJSON samples in `testdata/` per package
- **Mocking**: Interface-based dependency injection for unit tests
- **Concurrency**: sync.WaitGroup for goroutine coordination in concurrent tests
- **Coverage Target**: >80% on critical paths (parser, scheduler, manager, headless runner)

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
| Load config | O(n) agents | YAML parse, validate, and template expansion |
| Template expansion | O(n * m) agents × vars | Per-agent Go template execution |
| Build DAG | O(n + e) agents+edges | Kahn's algorithm |
| Spawn agent | O(1) + fork | Subprocess creation overhead ~50ms |
| Event batch | O(min(50, buffered)) | TUI render cycle |
| Manager lookup | O(1) + RWMutex lock | Typical <1µs contention |
| Write result | O(m) result size | Synchronous file I/O, thread-safe via mutex |

## Code Quality Metrics

- **Total LOC**: ~3,305 (code) + ~2,619 (tests) = ~5,924 total
- **Avg File Size**: ~80 LOC per file (excellent for context management)
- **Max File Size**: 622 LOC (app.go) — reasonable for composite TUI root model
- **Test Count**: 135+ tests across 11 packages
- **Coverage**: >80% on critical paths (parser, scheduler, manager, headless runner, config, health)

## Example Pipelines

Located in `examples/` directory (5 example configurations):

| File | Purpose |
|------|---------|
| `blog-pipeline.yaml` | Sequential 5-stage blog writing pipeline (research → outline → draft → edit → final) |
| `batch-blog-with-vars.yaml` | Batch blog generation with template variables for topic and style overrides |
| `parallel-analysis.yaml` | Fan-out/fan-in code analysis: scouts different aspects in parallel, planner synthesizes |
| `coding-pipeline.yaml` | Development workflow: scout codebase → planner designs → coder implements → tester validates → reviewer reviews |
| `mixed-backend-review.yaml` | Multi-backend example: uses both Claude and Gemini agents in same pipeline |

Example execution with template variable override:
```bash
agentmux run -c examples/batch-blog-with-vars.yaml --var topic="AI Safety" --var style="academic"
```

## Quick Navigation

**Find what you need:**
- Configuration parsing? → `internal/config/config.go`
- Template variables and CLI `--var` handling? → `internal/config/config.go` ExpandTemplates() function and cmd/run.go flag parsing
- Output directory config? → `internal/config/config.go` OutputDir field
- CLI flags (`-c`, `--var`, `--output-dir`)? → `cmd/run.go`
- Agent process management? → `internal/agent/process.go`
- NDJSON parsing? → `internal/agent/parser.go`
- DAG scheduling? → `internal/dag/scheduler.go`
- TUI layout? → `internal/tui/app.go`
- Keybindings? → Search for `HandleMsg` in `internal/tui/app.go`
- Headless mode? → `internal/headless/runner.go` and `formatter.go`
- Event logging? → `internal/log/writer.go`
- Result export? → `internal/result/writer.go`
- Installation script? → `scripts/install.sh`
- Tests? → Look for `_test.go` files; fixtures in `testdata/`

---

**Document Version**: v0.1.0
**Last Updated**: 2026-03-01
**Latest Features**: Health check server (`agentmux serve`), Template variables, Configurable output directory, Parallel run script
