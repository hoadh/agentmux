# Phase 2: Agent Process Management

## Context Links
- [Parent Plan](plan.md)
- [Phase 1: Config](phase-01-project-setup-config.md)
- [DAG+Process Research](research/researcher-02-dag-process-management.md)
- [Claude CLI Research](../reports/researcher-260224-1529-claude-code-programmatic-interface.md)
- [Bubbletea Streaming Patterns](research/researcher-01-bubbletea-cobra-patterns.md)

## Overview
- **Date**: 2026-02-24
- **Priority**: P1
- **Status**: complete
- **Description**: Implement os/exec wrapper for claude CLI, stream-JSON parser for NDJSON output, and agent manager for lifecycle control (spawn, kill, restart, status tracking).

## Key Insights
- Call `StdoutPipe()` BEFORE `Start()`; drain pipes in goroutines to avoid deadlock
- `json.Decoder` handles streaming natively; no need for bufio.Scanner per line
- Graceful shutdown: SIGTERM first, then SIGKILL after 5s timeout
- `exec.CommandContext` sends SIGKILL on cancel; manage SIGTERM manually
- Claude `stream-json` emits NDJSON lines: `{"type":"assistant",...}`, `{"type":"tool_use",...}`, `{"type":"result",...}`
- Each agent goroutine reads pipe → parses JSON → sends `tea.Msg` via channel
- Buffer channels (64+) to prevent goroutine stalls

## Requirements

### Functional
- Spawn claude CLI process with configurable flags (-p, --output-format, --model, --allowedTools, --max-turns)
- Parse NDJSON stream into typed Go events (AssistantMsg, ToolUse, ToolResult, Result)
- Track agent state: Pending, Running, Done, Failed, Killed
- Agent manager: Start(name), Stop(name), Restart(name), List(), Get(name)
- Send parsed events as tea.Msg to Bubbletea event loop
- Capture stderr for error diagnostics

### Non-Functional
- Event delivery <50ms from claude output to TUI message
- Handle malformed JSON lines gracefully (skip + log, don't crash)
- Support 10+ concurrent agents without goroutine leaks

## Architecture

```
agent/
├── process.go    # Process struct: os/exec wrapper, pipe setup, shutdown
├── parser.go     # NDJSON parser: json.Decoder → typed events
└── manager.go    # Manager: registry, lifecycle, tea.Msg bridge

Flow:
  Manager.Start(name, cfg)
    → Process.Start() — exec claude, setup pipes
    → goroutine: Parser.Stream(stdout) → channel → tea.Msg
    → goroutine: drain stderr → buffer

  Manager.Stop(name)
    → Process.Shutdown() — SIGTERM → wait 5s → SIGKILL
    → close channel

  Manager.Restart(name)
    → Stop(name) → Start(name, same cfg)
```

## Related Code Files

### Create
- `internal/agent/process.go` — Process struct, Start/Shutdown/buildArgs
- `internal/agent/parser.go` — Event types, ParseStream function
- `internal/agent/manager.go` — Manager struct, lifecycle methods, tea.Msg types

## Implementation Steps

<!-- Updated: Validation Session 1 - Test real stream-json output before writing parser -->
### 0. Capture real stream-json output (PRE-IMPLEMENTATION)

Before writing parser structs, run this command and save the output:
```bash
claude -p "Say hello and read one file" --output-format stream-json --verbose 2>/dev/null > testdata/stream-samples/real-output.jsonl
```
Analyze the actual JSON structure to confirm/adjust event type definitions below. Key things to verify:
- Event type field name and values (is it `"type"` or something else?)
- Nested structure of assistant messages (content array? text field?)
- Tool use event structure (tool name, input format)
- Result event structure (session_id, usage fields, token counts)
- Any event types not anticipated (e.g., "system", "progress", "thinking")

**Update the structs below based on real output before proceeding.**

### 1. Define event types (`parser.go`)

```go
// Raw NDJSON event from claude stream-json
type StreamEvent struct {
    Type string          `json:"type"`
    // Fields vary by type; use json.RawMessage for flexibility
}

// Parsed event types for TUI consumption
type AssistantEvent struct {
    AgentName string
    Text      string
}
type ToolUseEvent struct {
    AgentName string
    ToolName  string
    Input     string // truncated preview
}
type ToolResultEvent struct {
    AgentName string
    Content   string // truncated preview
}
type ResultEvent struct {
    AgentName  string
    Result     string
    SessionID  string
    InputTokens  int
    OutputTokens int
}
type ErrorEvent struct {
    AgentName string
    Err       error
}
type AgentDoneEvent struct {
    AgentName string
    ExitCode  int
}
```

### 2. Implement NDJSON parser (`parser.go`)

```go
func ParseStream(agentName string, r io.Reader, ch chan<- tea.Msg) {
    dec := json.NewDecoder(r)
    for {
        var raw map[string]interface{}
        if err := dec.Decode(&raw); err != nil {
            if errors.Is(err, io.EOF) { return }
            // malformed line: log + continue
            continue
        }
        msg := convertEvent(agentName, raw)
        if msg != nil { ch <- msg }
    }
}
```

- `convertEvent` switches on `raw["type"]` string:
  - `"assistant"` → extract text from `message.content[0].text` → `AssistantEvent`
  - `"tool_use"` → extract `tool.name` + truncated `tool.input` → `ToolUseEvent`
  - `"tool_result"` → extract truncated content → `ToolResultEvent`
  - `"result"` → extract result, session_id, usage stats → `ResultEvent`
  - unknown → skip

### 3. Implement process wrapper (`process.go`)

```go
type Process struct {
    Name     string
    Cmd      *exec.Cmd
    Stdout   io.ReadCloser
    Stderr   io.ReadCloser
    EventCh  chan tea.Msg
    cancel   context.CancelFunc
    done     chan struct{}
    exitCode int
}

func NewProcess(name string, cfg AgentConfig, defaults AgentDefaults) *Process
func (p *Process) Start() error
func (p *Process) Shutdown() error  // SIGTERM → 5s → SIGKILL
func (p *Process) Wait() int        // blocks until exit, returns exit code
```

- `Start()`:
  1. Build args: `["claude", "-p", cfg.Prompt, "--output-format", "stream-json", ...]`
  2. Create context with cancel
  3. `cmd.StdoutPipe()`, `cmd.StderrPipe()` BEFORE `cmd.Start()`
  4. Set `cmd.Dir = cfg.WorkDir`
  5. `cmd.Start()`
  6. Spawn goroutine: `ParseStream(name, stdout, eventCh)`
  7. Spawn goroutine: drain stderr into buffer
  8. Spawn goroutine: `cmd.Wait()` → send `AgentDoneEvent` → close done

- `Shutdown()`:
  1. `cmd.Process.Signal(syscall.SIGTERM)`
  2. Select: `<-done` (clean exit) or `<-time.After(5s)` → `cmd.Process.Kill()`

- `buildArgs()`:
  1. Start with `["-p", prompt, "--output-format", "stream-json"]`
  2. Append `--model` if set
  3. Append `--allowedTools` joined by comma if set
  4. Append `--max-turns` if set
  5. Append `--verbose` always (richer stream-json output)

### 4. Implement agent manager (`manager.go`)

```go
type AgentState int
const (
    StatePending AgentState = iota
    StateRunning
    StateDone
    StateFailed
    StateKilled
    StateBlocked  // DAG: deps failed
)

type AgentInfo struct {
    Name      string
    State     AgentState
    Config    AgentConfig
    Process   *Process
    StartedAt time.Time
    Duration  time.Duration
    Tokens    TokenUsage
    LastEvent string // summary of last event for sidebar
}

type TokenUsage struct {
    Input  int
    Output int
}

type Manager struct {
    agents   map[string]*AgentInfo
    defaults AgentDefaults
    mu       sync.RWMutex
}

func NewManager(defaults AgentDefaults) *Manager
func (m *Manager) Register(name string, cfg AgentConfig)
func (m *Manager) Start(name string) (chan tea.Msg, error)
func (m *Manager) Stop(name string) error
func (m *Manager) Restart(name string) (chan tea.Msg, error)
func (m *Manager) List() []*AgentInfo
func (m *Manager) Get(name string) *AgentInfo
func (m *Manager) StopAll() // graceful shutdown of all agents
```

- `Start(name)`:
  1. Lock, get agent info, check state is Pending/Done/Failed/Killed
  2. Create Process, call `Start()`
  3. Update state to Running, record StartedAt
  4. Return event channel (TUI will arm `waitForMsg` on it)
  5. Unlock

- `Stop(name)`:
  1. Lock, get agent info, check state is Running
  2. Call `Process.Shutdown()`
  3. Update state to Killed
  4. Unlock

### 5. Define tea.Msg bridge

The TUI needs a `waitForMsg` cmd pattern to drain each agent's event channel:

```go
// In manager.go or a shared msgs.go
func WaitForEvent(ch <-chan tea.Msg) tea.Cmd {
    return func() tea.Msg {
        msg, ok := <-ch
        if !ok { return nil }
        return msg
    }
}
```

TUI calls `WaitForEvent(ch)` for each started agent; on receiving a msg, it re-arms.

## Todo List
- [ ] Define all event types (parser.go)
- [ ] Implement ParseStream NDJSON parser (parser.go)
- [ ] Implement convertEvent with type switching (parser.go)
- [ ] Implement Process struct with Start/Shutdown/Wait (process.go)
- [ ] Implement buildArgs for claude CLI flags (process.go)
- [ ] Implement Manager with Register/Start/Stop/Restart/List (manager.go)
- [ ] Implement WaitForEvent tea.Cmd bridge (manager.go)
- [ ] Verify `go build ./...` passes
- [ ] Manual test: spawn single agent, verify events arrive

## Success Criteria
- Can spawn a claude process, parse stream-json events, receive typed Go structs
- Can kill a running agent with SIGTERM graceful shutdown
- Can restart a previously stopped agent
- Malformed JSON lines skipped without crash
- No goroutine leaks on agent stop (verified with runtime.NumGoroutine)

## Risk Assessment
- **Claude stream-json format changes**: Parser uses loose map[string]interface{} → typed extraction. Unknown event types are silently skipped. Low risk.
- **Pipe deadlock**: Both stdout and stderr drained in goroutines. Covered.
- **Channel overflow**: Buffered channel (256). If TUI is too slow to drain, events queue. Could add ring buffer if needed (YAGNI for now).
- **SIGTERM not supported on Windows**: Go `syscall.SIGTERM` works on darwin/linux. Windows support deferred.

## Security Considerations
- Agent prompts may contain sensitive data; don't log full prompts
- `--dangerously-skip-permissions` should NOT be used by default; rely on `--allowedTools`
- WorkDir validated in Phase 1 config

## Next Steps
- Phase 3: DAG scheduler will call `Manager.Start()` and listen for `AgentDoneEvent`
- Phase 4: TUI will consume event channels via `WaitForEvent`
