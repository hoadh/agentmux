# Code Standards

## Go Version & Project Layout

- **Go Version**: 1.24.2+ (minimum version in go.mod)
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

// Large buffer (1024) allows scheduler to emit events without TUI reader blocking.
// Scheduler produces faster than TUI can consume; backpressure would block agent goroutines.
s.eventCh = make(chan tea.Msg, 1024)
```

- **Agent event channels**: 256 buffer to prevent agent goroutines from blocking
- **Scheduler eventCh**: 1024 buffer (scheduler produces faster than TUI consumes)
- Prefer buffered channels for pub-sub patterns
- Document buffer size and semantics rationale
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
- **Goroutine Cleanup**: Close Process.EventCh after process exit to prevent goroutine leaks in `WaitForEvent` loops

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

## Backend Registry Pattern

Adding a new backend (e.g., OpenAI CLI) requires only one new file:

```go
// internal/agent/backend/openai.go
package backend

import "github.com/hoadh/agentmux/internal/log"

func init() {
    // Self-register on import
    Register("openai", &OpenAIBackend{})
}

type OpenAIBackend struct{}

func (b *OpenAIBackend) Args(cfg *config.AgentConfig) []string {
    // Build command-line args for OpenAI CLI
    args := []string{"openai", "chat"}
    if cfg.Model != "" {
        args = append(args, "--model", cfg.Model)
    }
    // ... add more flags
    return args
}

func (b *OpenAIBackend) ConvertEvent(line []byte) (BackendEvent, error) {
    // Parse NDJSON and map to shared event types
    // ...
}
```

**Why it works**: Backends share a common interface (`BackendEvent` types) and register via `init()` without any changes to existing code.

## NDJSON Parser Patterns

When consuming CLI `stream-json` output:

**Use `bufio.Scanner`, NOT `json.Decoder`:**

```go
scanner := bufio.NewScanner(reader)
for scanner.Scan() {
    line := scanner.Bytes()
    var event map[string]interface{}
    if err := json.Unmarshal(line, &event); err != nil {
        continue  // Skip malformed line
    }

    // Validate event type before accessing fields
    eventType, ok := event["type"].(string)
    if !ok {
        continue
    }

    switch eventType {
    case "assistant":
        // Process message.content blocks
        if msg, ok := event["message"].(map[string]interface{}); ok {
            if content, ok := msg["content"].([]interface{}); ok {
                for _, block := range content {
                    if b, ok := block.(map[string]interface{}); ok {
                        blockType, _ := b["type"].(string)
                        // Handle text, tool_use, tool_result
                    }
                }
            }
        }
    case "result":
        // Handle result event
    case "rate_limit_event":
        // Handle rate limit
    }
}
```

**Why Scanner over Decoder:**
- `Scanner` recovers from malformed/non-JSON lines by skipping them
- `json.Decoder` fails completely on the first bad line, losing all subsequent data
- Claude CLI output may contain stray characters or incomplete lines in edge cases

**Backend CLI Requirements:**
- **Claude**: `claude -p <prompt> --output-format stream-json --verbose [--model X] [--allowedTools T]... [--max-turns N]`
  - `--verbose` flag REQUIRED in print mode (`-p`) for proper `stream-json` format
  - Without it, format differs and parsing fails
- **Gemini**: `gemini <prompt> --output-format stream-json --approval-mode auto_edit [--model X]`
  - allowedTools and max_turns are ignored (warnings emitted at config load)
- **Extensible**: Each backend's `Args()` method constructs its own command line

## TUI Identity vs Display Fields

When a struct needs both an identity field for event matching AND a formatted display string, separate them:

```go
type DetailModel struct {
    // Identity: used for matching agent events
    agentName string

    // Display: used for rendering, updated by SetHeader()
    headerText string

    lines []string
}

func (d *DetailModel) AppendLine(agent, line string) {
    // Compare against identity field, not headerText!
    if agent == d.agentName {
        d.lines = append(d.lines, line)
    }
}

func (d *DetailModel) SetHeader(text string) {
    // Only update display field
    d.headerText = text
}
```

**Critical Pattern:** Never overwrite identity fields with formatted content.

**Problem Avoided:**
- If `agentName` were overwritten with `"researcher ● Running 2m..."`, then `AppendLine("researcher", ...)` would fail to match
- This causes logs to disappear silently

## Batch Event Processing

When processing high-throughput event streams in Bubbletea, batch events to prevent per-event re-renders:

```go
func (m *DetailModel) drainEvents(eventCh chan tea.Msg) []tea.Msg {
    batch := make([]tea.Msg, 0, 50)

    // Block for first event
    batch = append(batch, <-eventCh)

    // Drain up to 50 buffered non-blocking
    for len(batch) < 50 {
        select {
        case msg := <-eventCh:
            batch = append(batch, msg)
        default:
            return batch
        }
    }
    return batch
}
```

**Performance Impact:**
- Without batching: TUI re-renders on every single event (can be 100+ per second)
- With batching: TUI re-renders once per batch (capped at 50 events)
- Result: Dramatic responsiveness improvement, CPU drops significantly

**Pattern Details:**
- Block for at least one event (don't burn CPU spinning)
- Cap batch size at 50 to bound processing time per render cycle
- Return batch immediately if channel empties

## Data Race Prevention

Ensure concurrent access patterns prevent data races:

```go
// Manager.List() returns copies, not pointers
func (m *Manager) List() []*Info {
    m.mu.RLock()
    defer m.mu.RUnlock()

    result := make([]*Info, 0, len(m.agents))
    for _, info := range m.agents {
        cp := *info  // Value copy prevents external mutation of shared state
        result = append(result, &cp)
    }
    return result
}

// Scheduler: collect work under lock, execute side effects after unlock
func (s *Scheduler) ProcessDone(agent string) {
    s.mu.Lock()
    ready := []string{}
    for dep, agent := range s.pending {
        s.pending[dep]--
        if s.pending[dep] == 0 {
            ready = append(ready, dep)
        }
    }
    s.mu.Unlock()  // Release lock BEFORE sending on channel

    for _, a := range ready {
        s.eventCh <- AgentReadyMsg{Agent: a}  // No lock held
        s.manager.SpawnAgent(a)                 // No lock held
    }
}
```

**Deadlock Prevention:**
- Never hold a lock while sending on a channel
- If the channel receiver also holds locks, you can deadlock
- Collect work items under lock, execute side effects after unlock

**Data Race Prevention:**
- Return value copies from accessor methods, not shared pointers
- Always synchronize reads/writes of shared state under the same lock
- Close channels only from the sending end (prevents send-on-closed panic)

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
