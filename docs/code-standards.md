# Code Standards

## Go Version & Project Layout

- **Go Version**: 1.24+
- **Project Root**: `github.com/hoadh/agentmux`
- **Structure**:
  ```
  cmd/              # CLI entry points (Cobra commands)
  internal/         # Private packages (not imported outside module)
    ├── agent/      # Process, parser, manager, state
    ├── config/     # YAML parsing and validation
    ├── dag/        # Graph and scheduler
    ├── log/        # JSONL writer
    └── tui/        # Bubbletea UI components
  main.go           # Bootstrap entry point
  go.mod, go.sum    # Dependency management
  agentmux.yaml     # Configuration file
  ```

All package logic lives in `internal/` to enforce clean API boundaries.

## File Naming

- **Go Files**: `snake_case.go` (Go standard)
  - Example: `parser.go`, `process.go`, `manager.go`, `statusbar.go`
  - Test files: `*_test.go` (e.g., `parser_test.go`)
  - Related files grouped by domain (e.g., all TUI models in `internal/tui/`)

## Error Handling

Wrap errors with context using `fmt.Errorf`:

```go
func (m *Manager) SpawnAgent(cfg *config.Agent) error {
    proc, err := NewProcess(cfg.Cmd, cfg.WorkDir)
    if err != nil {
        return fmt.Errorf("spawn agent %q: %w", cfg.Name, err)
    }
    return nil
}
```

**Pattern**: `fmt.Errorf("operation context: %w", err)`

- Always include what was being attempted
- Preserve original error with `%w` for error wrapping chains
- Never shadow errors; propagate context upward

## Concurrency

### Mutex Protection

Use `sync.RWMutex` for shared mutable state accessed from multiple goroutines:

```go
type Manager struct {
    agents map[string]*Agent
    mu     sync.RWMutex
}

func (m *Manager) Agent(name string) *Agent {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return m.agents[name]
}

func (m *Manager) UpdateState(name string, state AgentState) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.agents[name].State = state
}
```

- Lock before reading shared state from concurrent goroutines
- Lock before writing
- Always defer Unlock() immediately after Lock()

### Channels

Use buffered channels for event messaging to prevent sender blocking:

```go
type Scheduler struct {
    eventCh chan tea.Msg
}

// Buffer allows scheduler to emit events without TUI reader blocking
s.eventCh = make(chan tea.Msg, 10)
```

- Prefer buffered channels for pub-sub patterns
- Document buffer size and semantics
- Close channel only if you own it (coordinator, not workers)

### Goroutines

One goroutine per agent for subprocess I/O:

```go
go func(p *Process) {
    p.parser.Parse(p.stdout) // blocking read loop
}(proc)
```

- Each agent's Process reads stdout in dedicated goroutine
- Parser emits events back to manager via channels or callbacks
- No shared buffers between goroutine pairs; each has private readers

## Testing

- **Package**: Use Go standard `testing` package
- **Fixtures**: Store test data in `testdata/` directory per package
  - Example: `internal/config/testdata/valid.yaml`, `testdata/invalid.yaml`
- **Naming**: `TestFunctionName(t *testing.T)`
- **Coverage**: Aim for >80% on critical paths (parser, scheduler, manager)
- **Mocking**: Use interfaces for dependency injection; mock implementations in tests

Example:

```go
func TestParseValidNDJSON(t *testing.T) {
    data := readTestFile(t, "testdata/valid_output.jsonl")
    parser := NewParser(strings.NewReader(data))

    msg := parser.Next()
    if msg == nil {
        t.Fatal("expected message, got nil")
    }
}
```

## Code Organization

### Packages

Each `internal/` package is a cohesive unit:

- **Single Responsibility**: One package, one concern (e.g., `agent` = lifecycle, `dag` = orchestration)
- **Internal API**: Exported types and functions use PascalCase; unexported use camelCase
- **No Circular Imports**: Enforce via code review

### Constants & Variables

```go
// Package-level constants for configuration
const (
    DefaultTimeout = 30 * time.Second
    MaxAgents      = 100
)

var (
    ErrNoAgents = errors.New("no agents defined")
)
```

- All caps with underscores for unexported constants
- Use `var` for mutable package state (rare)
- Prefer typed constants (`const` with type)

### Structs & Methods

```go
type Agent struct {
    Name       string
    State      AgentState
    OutputLog  []string
    TokenUsage TokenUsage
}

// Receiver methods on pointers to modify state
func (a *Agent) UpdateState(s AgentState) {
    a.State = s
}

// Receiver methods on values for read-only access
func (a Agent) IsTerminal() bool {
    return a.State == StateDone || a.State == StateFailed
}
```

- Struct fields are PascalCase (exported)
- Methods use pointer receivers if mutating
- Keep struct field count <10; split if larger

## Dependency Injection

Constructor pattern for testability:

```go
func NewScheduler(g *Graph, mgr *agent.Manager) *Scheduler {
    return &Scheduler{
        graph:   g,
        manager: mgr,
        pending: make(map[string]int),
        eventCh: make(chan tea.Msg, 10),
    }
}
```

- All dependencies passed to constructors
- Avoid global state in test-critical code
- Mock via interfaces in tests

## Comments

- **Exported Functions**: Document intent, parameters, return values
- **Complex Logic**: Explain why, not what (code shows what)
- **TODO/FIXME**: Include owner or timeline: `// TODO(alexh): add timeout by v1.1`

Example:

```go
// SpawnAgent starts a subprocess for the given agent configuration and
// returns immediately. Output is consumed asynchronously.
func (m *Manager) SpawnAgent(cfg *config.Agent) error {
    // ...
}
```

## Performance Considerations

- **String Concatenation**: Use `strings.Builder` for loops, not `+`
- **Allocations**: Preallocate slices/maps with capacity when size is known
- **Goroutine Overhead**: One goroutine per agent is acceptable; don't spawn unbounded goroutines
- **Channels**: Buffered channels prevent sender blocking; size appropriately

## Security

- **Command Execution**: Use `exec.Command()` with explicit args array, not shell strings
- **Environment**: Inherit parent process env; explicitly override keys
- **File Access**: Respect working directory isolation per agent
- **Logging**: Don't log sensitive values (tokens, passwords); filter in Writer

Example:

```go
// Safe: args array prevents shell injection
cmd := exec.Command("agent", "--flag", value)

// Unsafe: shell expansion
cmd := exec.Command("sh", "-c", "agent --flag "+value)
```

## Testing Before Commit

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Lint (if using golangci-lint)
golangci-lint run ./...
```

Do not commit code that fails tests or has obvious lint errors.

## Version Control

- **Commit Message Format**: Conventional commits (feat:, fix:, refactor:, docs:, test:)
  - Example: `feat: add agent timeout configuration`
- **Atomic Commits**: One logical change per commit
- **No Secrets**: Never commit `.env`, credentials, or private keys

## Documentation in Code

- Exported package APIs should have doc comments
- Complex algorithms warrant inline comments
- Keep comments in sync with code during refactors
