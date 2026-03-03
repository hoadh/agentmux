# Code Standards

## Go Version & Project Layout

- **Go Version**: 1.24.2+ (minimum version in go.mod)
- **Project Root**: `github.com/hoadh/agentmux`
- **Structure**:
  ```
  cmd/              # CLI entry points (Cobra commands: run, check, version)
  internal/         # Private packages (not imported outside module)
    ├── agent/      # Process, parser, manager, state, backend registry
    ├── backend/    # Claude & Gemini CLI implementations
    ├── config/     # YAML parsing and validation
    ├── dag/        # Graph and scheduler
    ├── health/     # Backend availability check
    ├── headless/   # Non-TUI runner and formatters
    ├── log/        # JSONL event writer
    ├── result/     # Markdown result writer
    └── tui/        # Bubbletea UI components (sidebar, detail, statusbar, spawn)
  scripts/          # Deployment helpers (install.sh, parallel-run.sh)
  examples/         # Example pipeline configurations
  main.go           # Bootstrap entry point
  go.mod, go.sum    # Dependency management
  agentmux.yaml     # Default configuration file
  ```

All package logic lives in `internal/` to enforce clean API boundaries. TUI and headless modes are mutually exclusive frontends sharing core packages.

## File Naming

- **Go Files**: `snake_case.go` (Go standard)
  - Example: `parser.go`, `process.go`, `manager.go`, `statusbar.go`
  - Test files: `*_test.go` (e.g., `parser_test.go`)
  - Related files grouped by domain (e.g., all TUI models in `internal/tui/`)

- **Shell Scripts**: `lowercase-with-dashes.sh` (e.g., `install.sh`)
  - POSIX-compliant; no bash-specific features unless documented

## Template Variables

### YAML Vars Section

Define reusable variables in YAML for prompt expansion:

```yaml
vars:
  project_root: "/path/to/project"
  code_lang: "go"
  max_depth: "3"

agents:
  scanner:
    prompt: "Scan {{.project_root}} for {{.code_lang}} files to depth {{.max_depth}}"
```

**Pattern**: `{{.var_name}}` uses Go `text/template` syntax; values are always strings.

### CLI Flag Handling

Override vars via CLI `--var` flag in `cmd/run.go`:

```go
func init() {
    runCmd.Flags().StringArrayVar(
        &varOverrides, "var", []string{},
        "Template variable override (repeatable, format: key=value)",
    )
}

// In runCmd.Run():
for _, varStr := range varOverrides {
    parts := strings.Split(varStr, "=")
    if len(parts) == 2 {
        cfg.Vars[parts[0]] = parts[1]  // Override YAML var
    }
}
```

**Precedence**: CLI `--var` overrides YAML `vars` section. New variables added via CLI merged with YAML.

### Template Expansion (ExpandTemplates)

Located in `internal/config/config.go`:

```go
func ExpandTemplates(cfg *Config) error {
    t := template.New("config")
    // Parse and execute {{.var_name}} in each agent prompt
    for _, agent := range cfg.Agents {
        tmpl, err := t.Parse(agent.Prompt)
        if err != nil {
            return fmt.Errorf("template parse error: %w", err)
        }
        var buf strings.Builder
        if err := tmpl.Execute(&buf, cfg.Vars); err != nil {
            return fmt.Errorf("template expansion error: %w", err)
        }
        agent.Prompt = buf.String()
    }
    return nil
}
```

**Error Handling**: Missing variables abort with template expansion error. All var values must be strings.

## CLI Flag Conventions

### Flag Naming

- **Short flags**: Single letter (e.g., `-c`, `-v`)
- **Long flags**: Kebab-case (e.g., `--config`, `--output-dir`, `--var`)
- **Bool flags**: Verb present for enabled state (e.g., `--headless` means "run headless")

### Repeatable Flags

For `--var`, use `StringArrayVar`:

```go
runCmd.Flags().StringArrayVar(&varOverrides, "var", []string{}, "...")
```

Users invoke as:
```bash
agentmux run --var key1=val1 --var key2=val2 --var key3=val3
```

### Flag Precedence

Establish clear precedence in documentation and code:

1. **CLI flag** (highest priority)
2. **YAML value** (medium priority)
3. **Default** (lowest priority)

Example:
```go
// --output-dir overrides YAML output_dir, defaults to .agentmux-out
outputDir := cliOutputDir
if outputDir == "" {
    outputDir = cfg.OutputDir
}
if outputDir == "" {
    outputDir = ".agentmux-out"
}
```

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

## Headless Mode Patterns

When implementing non-TUI pipeline execution:

### Runner Structure

```go
// internal/headless/runner.go
type Runner struct {
    cfg       *config.Config
    dag       *dag.Graph
    scheduler *dag.Scheduler
    manager   *agent.Manager
    formatter *Formatter
}

func (r *Runner) Run(ctx context.Context) error {
    // 1. Validate config
    if err := r.cfg.ValidateAcyclic(); err != nil {
        return err
    }

    // 2. Build DAG (same as TUI)
    graph, err := dag.NewGraph(r.cfg)
    if err != nil {
        return err
    }

    // 3. Initialize scheduler (same as TUI)
    r.scheduler = dag.NewScheduler(graph, r.manager)

    // 4. Main event loop (instead of Bubbletea, listen to channels)
    for {
        select {
        case msg := <-r.scheduler.EventCh:
            // Format and output to stdout/file
            r.formatter.Format(msg)

        case <-ctx.Done():
            // Handle SIGTERM/SIGINT
            return r.manager.Shutdown(5 * time.Second)
        }
    }
}
```

### Signal Handling Pattern

```go
func (r *Runner) setupSignalHandling(ctx context.Context) context.Context {
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

    ctx, cancel := context.WithCancel(ctx)
    go func() {
        <-sigCh
        cancel()
    }()

    return ctx
}
```

### Formatter Interface

```go
// internal/headless/formatter.go
type Formatter interface {
    Format(msg tea.Msg) error  // Convert Bubbletea message to output
    Close() error               // Flush buffers
}

type PlainTextFormatter struct {
    w io.Writer
}

func (f *PlainTextFormatter) Format(msg tea.Msg) error {
    switch m := msg.(type) {
    case agent.StateChangeMsg:
        fmt.Fprintf(f.w, "[%s] %s: %s\n", time.Now().Format(time.RFC3339), m.Agent, m.NewState)
    case agent.OutputMsg:
        fmt.Fprintf(f.w, "[%s] %s: %s\n", time.Now().Format(time.RFC3339), m.Agent, m.Text)
    }
    return nil
}
```

**Pattern**: Reuse agent event types and Scheduler; only replace TUI rendering with formatter-based output.

## Backend Registry Pattern

Each backend self-registers via `init()` on import. No changes to Manager or config needed.

### Backend Interface

```go
// internal/agent/backend/backend.go
type Backend interface {
    // Args constructs CLI invocation args from config
    Args(cfg *config.AgentConfig) []string

    // ConvertEvent maps backend-specific NDJSON to shared event types
    ConvertEvent(line []byte) (BackendEvent, error)
}

type BackendEvent struct {
    Type    string      // "started", "output", "result", "error"
    Payload interface{} // Event-specific data
}

// Global registry
var registry = make(map[string]Backend)

func Register(name string, backend Backend) {
    registry[name] = backend
}

func Get(name string) (Backend, error) {
    b, ok := registry[name]
    if !ok {
        return nil, fmt.Errorf("unknown backend: %s", name)
    }
    return b, nil
}
```

### Adding a New Backend

Adding a new backend (e.g., OpenAI CLI) requires only one new file:

```go
// internal/agent/backend/openai.go
package backend

import "github.com/hoadh/agentmux/internal/config"

func init() {
    // Self-register on import (no config changes needed)
    Register("openai", &OpenAIBackend{})
}

type OpenAIBackend struct{}

// Args constructs: openai chat --model X --stream-json ...
func (b *OpenAIBackend) Args(cfg *config.AgentConfig) []string {
    args := []string{"openai", "chat"}
    if cfg.Model != "" {
        args = append(args, "--model", cfg.Model)
    }
    args = append(args, "--stream-json")
    return args
}

// ConvertEvent maps NDJSON to BackendEvent
func (b *OpenAIBackend) ConvertEvent(line []byte) (BackendEvent, error) {
    var data map[string]interface{}
    if err := json.Unmarshal(line, &data); err != nil {
        return BackendEvent{}, err
    }

    // Map OpenAI event types to BackendEvent
    switch data["type"] {
    case "content_block_delta":
        return BackendEvent{Type: "output", Payload: data["delta"]}, nil
    case "message_stop":
        return BackendEvent{Type: "result", Payload: data["message"]}, nil
    default:
        return BackendEvent{}, nil
    }
}
```

### Why It Works

1. **Self-registration**: `init()` runs when backend package imported; no registration code needed in Manager
2. **Shared event types**: All backends emit `BackendEvent` with consistent `Type` and `Payload`
3. **Config-driven backend selection**: Manager calls `Get()` based on `agent.backend` field; no hardcoded conditionals
4. **Parser-agnostic**: Parser feeds lines to backend; backend interprets format
5. **Zero coupling**: New backend doesn't touch config, manager, or parser

**Feature Matrix** (configurable per backend):

| Backend | Args() | ConvertEvent() | Supports Model | Supports Tools | Supports Turns |
|---------|--------|---|---|---|---|
| Claude | ✓ | ✓ | ✓ | ✓ | ✓ |
| Gemini | ✓ | ✓ | ✓ | ✗ | ✗ |
| OpenAI | ✓ (example) | ✓ (example) | ✓ | TBD | TBD |

See [Configuration Reference](./configuration.md) for feature warnings emitted at config load time.

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
