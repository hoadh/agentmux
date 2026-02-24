# Code Review: agentmux Go TUI

## Code Review Summary

### Scope
- **Files**: 17 Go source files across 6 packages
- **LOC**: 2,303
- **Focus**: Full codebase review -- correctness, concurrency, architecture, Go idioms
- **Tests**: None (0 test files)

### Overall Assessment

**Score: 6.5 / 10**

The codebase demonstrates solid architectural instincts: clean package separation, proper use of Bubbletea's Elm-like architecture, a well-structured DAG scheduler, and sensible NDJSON stream parsing. The code compiles cleanly and passes `go vet` without warnings.

However, there are several concurrency bugs that would cause real problems under load, a few architectural mismatches with Bubbletea patterns, and zero test coverage. The issues range from data races on shared pointers to goroutine leaks and deadlock-prone channel patterns.

---

## Critical Issues (MUST Fix)

### C1. Race Condition: `Manager.List()` returns pointers to shared mutable state

**File**: `/Users/alex/Code/test-new-ck/internal/agent/manager.go` (lines 160-175)

`List()` returns `[]*AgentInfo` -- raw pointers into the manager's internal map. These are read by TUI rendering (on the main goroutine) while concurrently mutated by `SetState()`, `UpdateTokens()`, `UpdateDuration()`, and `UpdateLastEvent()` (called from event handlers that may overlap). The RWMutex only protects map access, not the AgentInfo structs themselves once the pointers escape.

```go
// CURRENT: Returns pointers into shared mutable map
func (m *Manager) List() []*AgentInfo {
    m.mu.RLock()
    defer m.mu.RUnlock()
    // ... returns m.agents[name] directly
}
```

**Impact**: Data races during rendering. `go test -race` would catch this.

**Fix**: Return deep copies, or accept that all access to AgentInfo fields must go through Manager methods (and enforce it).

```go
func (m *Manager) List() []AgentInfo {
    m.mu.RLock()
    defer m.mu.RUnlock()
    result := make([]AgentInfo, 0, len(m.agents))
    for _, info := range m.agents {
        result = append(result, *info) // copy
    }
    sort.Slice(result, func(i, j int) bool {
        return result[i].Name < result[j].Name
    })
    return result
}
```

### C2. Deadlock: `handleEvent()` holds scheduler mutex while calling `Manager.SetState()` which acquires manager mutex

**File**: `/Users/alex/Code/test-new-ck/internal/dag/scheduler.go` (lines 118-143)

`handleEvent()` acquires `s.mu.Lock()`, then calls `s.manager.SetState()` which acquires the manager's `m.mu.Lock()`. Meanwhile, `Manager.Restart()` (line 141-157 in manager.go) acquires the manager lock, then calls `Manager.Stop()` which blocks on `Process.Shutdown()` waiting on the `done` channel, which requires the scheduler event loop to process the `AgentDoneEvent` -- but the scheduler might be holding its own lock waiting for the manager.

Additionally, `blockDownstreamLocked()` sends on `s.eventCh` (a buffered channel with capacity 64) while holding `s.mu`. If `eventCh` is full, this deadlocks.

**Impact**: Potential deadlock under specific conditions (agent restart during pipeline, or many agents blocking simultaneously).

**Fix**:
- Do not send on `s.eventCh` while holding `s.mu`. Collect messages to send, release the lock, then send.
- Consider making `eventCh` unbounded or drain it in a separate goroutine.

### C3. Goroutine Leak: Agent event forwarding goroutine never terminates

**File**: `/Users/alex/Code/test-new-ck/internal/dag/scheduler.go` (lines 111-115)

```go
go func() {
    for msg := range ch {
        agentEvents <- msg
    }
}()
```

The `Process.EventCh` channel (capacity 256) is never closed. After `Process.Wait()` completes (the goroutine at process.go:80-95 sends `AgentDoneEvent` and closes `done`), the `ParseStream` goroutine exits on EOF, the wait goroutine sends the done event -- but nobody closes `EventCh`. The forwarding goroutine above blocks forever on `range ch`.

**Impact**: One leaked goroutine per agent. In a long-running session with many spawned agents, this accumulates.

**Fix**: Close `EventCh` after the process wait goroutine finishes:

```go
// In process.go, the wait goroutine:
go func() {
    err := p.Cmd.Wait()
    // ... set exit code ...
    p.EventCh <- AgentDoneEvent{AgentName: p.Name, ExitCode: code}
    close(p.EventCh) // Close after final event
    close(p.done)
}()
```

### C4. Race: `Restart()` reads `info.State` outside lock

**File**: `/Users/alex/Code/test-new-ck/internal/agent/manager.go` (lines 141-157)

```go
func (m *Manager) Restart(name string) (chan tea.Msg, error) {
    m.mu.RLock()
    info, ok := m.agents[name]
    m.mu.RUnlock()

    if !ok {
        return nil, fmt.Errorf(...)
    }

    if info.State == StateRunning {  // <-- reading State without lock
        if err := m.Stop(name); err != nil {
```

After releasing `RLock`, `info.State` can be mutated by another goroutine before the comparison.

**Fix**: Hold the lock for the entire check, or restructure so Stop+Start is atomic.

---

## High Priority Issues

### H1. Process.EventCh can block the wait goroutine

**File**: `/Users/alex/Code/test-new-ck/internal/agent/process.go` (line 93)

```go
p.EventCh <- AgentDoneEvent{AgentName: p.Name, ExitCode: code}
```

If `EventCh` (buffered at 256) is full because the consumer has stopped reading (e.g., TUI quit but cleanup races), this send blocks forever, preventing `close(p.done)`, which prevents `Shutdown()` from returning, which prevents `StopAll()` from completing.

**Impact**: Application hang on quit if an agent produced a lot of unread events.

**Fix**: Use a non-blocking send or ensure the channel is drained:

```go
select {
case p.EventCh <- AgentDoneEvent{AgentName: p.Name, ExitCode: code}:
default:
    // Channel full; consumer gone
}
```

### H2. `ParseStream` silently swallows all non-EOF errors

**File**: `/Users/alex/Code/test-new-ck/internal/agent/parser.go` (lines 58-65)

```go
if err := dec.Decode(&raw); err != nil {
    if errors.Is(err, io.EOF) {
        return
    }
    continue // silently dropped
}
```

A broken pipe, unexpected close, or persistent malformed JSON will cause an infinite loop of `continue` if `Decode` returns the same error repeatedly without advancing the reader.

**Impact**: CPU spin loop if the stream enters a bad state.

**Fix**: Add a counter or backoff for consecutive errors, or break on non-recoverable errors like `io.ErrUnexpectedEOF`:

```go
consecutiveErrors := 0
for {
    var raw map[string]any
    if err := dec.Decode(&raw); err != nil {
        if errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) {
            return
        }
        consecutiveErrors++
        if consecutiveErrors > 100 {
            ch <- ErrorEvent{AgentName: agentName, Err: fmt.Errorf("too many parse errors: %w", err)}
            return
        }
        continue
    }
    consecutiveErrors = 0
    // ...
}
```

### H3. Bubbletea pattern violation: pointer-receiver methods on value-receiver model

**File**: `/Users/alex/Code/test-new-ck/internal/tui/app.go`

Bubbletea's `Model` interface uses value receivers for `Init`, `Update`, and `View`. The `AppModel` follows this. However, many helper methods use pointer receivers (`resizeAll`, `refreshSidebar`, `refreshStats`, `rearmScheduler`, `logEvent`). When these are called from within `Update` (which receives a copy), mutations are applied to the copy, which is correct -- but only because the copy is returned. The issue is that `Init()` at line 68 calls `m.detail.SetAgent()` on a copy that is never returned. The mutation is lost.

```go
func (m AppModel) Init() tea.Cmd {
    // ...
    if name := m.sidebar.SelectedAgent(); name != "" {
        m.detail.SetAgent(name) // Mutates a copy, never returned
    }
    return tea.Batch(cmds...)
}
```

**Impact**: Initial agent selection in the detail panel is silently lost.

**Fix**: This is a structural issue with mixing value and pointer receivers in Bubbletea. Either:
- Return the model from Init (not possible with current Bubbletea API which returns only `tea.Cmd`)
- Move the initial selection into the first `WindowSizeMsg` handler
- Use `*AppModel` as the model (Bubbletea supports pointer models)

### H4. No graceful handling of spawned agent WorkDir

**File**: `/Users/alex/Code/test-new-ck/internal/tui/app.go` (lines 299-320)

`handleSpawnConfirm` creates an `AgentConfig` with an empty `WorkDir`. The `ApplyDefaults` in config.go would set it to ".", but `handleSpawnConfirm` bypasses config loading entirely -- it directly registers and starts. The process will run in whatever directory the main binary is in.

```go
cfg := config.AgentConfig{
    Prompt: msg.Prompt,
    Model:  msg.Model,
    // WorkDir not set!
}
```

**Impact**: Spawned agents may operate in an unexpected directory. Not a crash, but a correctness issue.

**Fix**: Set `WorkDir: "."` explicitly, or better, pass through `ApplyDefaults`.

---

## Medium Priority Issues

### M1. Duplicate default-merging logic

**Files**: `/Users/alex/Code/test-new-ck/internal/config/config.go` (lines 67-84) and `/Users/alex/Code/test-new-ck/internal/agent/process.go` (lines 136-165)

`ApplyDefaults` in config.go merges defaults into agent configs. Then `buildArgs` in process.go re-checks for empty model/tools and falls back to defaults again. This is redundant and creates two sources of truth for default resolution.

**Fix**: Apply defaults once at config load time and trust they are resolved.

### M2. `Scheduler.Run()` does not close `eventCh`

**File**: `/Users/alex/Code/test-new-ck/internal/dag/scheduler.go`

When `Run()` returns (either from context cancellation or pipeline completion), it does not close `s.eventCh`. The TUI's `rearmScheduler()` returns `WaitForEvent(s.scheduler.EventCh())`, which will block forever on a closed-but-not-closed channel.

**Fix**: Close `s.eventCh` at the end of `Run()` so that `WaitForEvent` receives `nil` and terminates.

### M3. `truncate()` operates on bytes, not runes

**File**: `/Users/alex/Code/test-new-ck/internal/agent/parser.go` (lines 169-174)

```go
func truncate(s string, maxLen int) string {
    if len(s) <= maxLen {
        return s
    }
    return s[:maxLen] + ...
}
```

`s[:maxLen]` slices by bytes, which can cut a multi-byte UTF-8 character in half, producing invalid UTF-8.

**Fix**: Use `[]rune` conversion or `utf8.RuneCountInString`.

### M4. Sidebar line truncation also byte-based

**File**: `/Users/alex/Code/test-new-ck/internal/tui/sidebar.go` (lines 89-92)

Same issue as M3 with `line[:maxW]`.

### M5. `StopAll` ignores errors from `Stop`

**File**: `/Users/alex/Code/test-new-ck/internal/agent/manager.go` (lines 222-235)

```go
for _, name := range running {
    m.Stop(name) // error discarded
}
```

**Fix**: At minimum, log the errors or aggregate them.

### M6. Log Writer `Close()` deletes from map during iteration

**File**: `/Users/alex/Code/test-new-ck/internal/log/writer.go` (lines 64-77)

```go
for name, f := range w.files {
    if err := f.Close(); err != nil {
        lastErr = err
    }
    delete(w.files, name) // deleting during range
}
```

While Go's spec allows deleting map keys during range iteration, this is a code smell and can confuse readers. The whole map will be garbage-collected anyway.

**Fix**: Either skip the `delete` call (the Writer is being closed) or clear the map after the loop.

---

## Low Priority Issues

### L1. `NewProcess` accepts unused parameters

**File**: `/Users/alex/Code/test-new-ck/internal/agent/process.go` (line 33)

```go
func NewProcess(name string, cfg config.AgentConfig, defaults config.AgentDefaults) *Process {
```

`cfg` and `defaults` are not used in `NewProcess`; they are passed again to `Start()`. Simplify the constructor.

### L2. Hardcoded sidebar width

**File**: `/Users/alex/Code/test-new-ck/internal/tui/styles.go` (line 9)

`SidebarWidth = 28` is hardcoded. For very narrow terminals, this leaves little room for the detail panel (minimum 10 chars enforced, but still cramped).

### L3. Agent names for spawned agents are sequential but not descriptive

**File**: `/Users/alex/Code/test-new-ck/internal/tui/app.go` (line 300)

`agent-0`, `agent-1` etc. are not helpful. Consider letting the user name agents in the spawn dialog.

### L4. `SpawnCancelMsg` is generated but never handled

**File**: `/Users/alex/Code/test-new-ck/internal/tui/spawn.go` (line 64)

The `SpawnCancelMsg` is emitted when Esc is pressed in the spawn dialog, but `AppModel.Update` has no case for it. The focus state gets stuck.

**Fix**: Handle `SpawnCancelMsg` in `Update` to restore focus to sidebar.

### L5. No version-checking on config file

**File**: `/Users/alex/Code/test-new-ck/internal/config/config.go`

The `Version` field is parsed but never checked. If someone uses `version: 2`, it is silently accepted.

---

## Edge Cases Found

1. **Empty agent map + scheduler**: If config has no agents, `Scheduler.Run()` sends `PipelineDoneMsg` on `eventCh` and returns immediately. But the TUI calls `rearmScheduler()` which blocks on `eventCh` -- since `Run` returned and nobody closed the channel, this leaks a goroutine.

2. **Rapid restart**: Calling `Restart` on an agent while it is in the middle of producing events can cause the old forwarding goroutine (in scheduler) to continue sending events from the old process mixed with events from the new process, since both write to the same `agentEvents` channel.

3. **Process `Shutdown` after context cancel**: `exec.CommandContext` kills the process when the context is cancelled. But `Shutdown()` also tries to send SIGTERM. If the context was already cancelled, the process may already be dead, and `Signal` returns an error.

4. **Pipeline view with diamond dependencies**: If agent C depends on both A and B, and both A and B depend on D, the recursive `renderNode` with the `rendered` map will only show C once. But the visual tree might be misleading about which path it belongs to.

---

## Positive Observations

1. **Clean package structure**: The separation into `agent`, `config`, `dag`, `tui`, and `log` packages is well-considered and follows Go conventions.
2. **DAG cycle detection**: Kahn's algorithm implementation is correct and provides useful error messages identifying cycle participants.
3. **Bubbletea event flow**: The pattern of `WaitForEvent` as a `tea.Cmd` that bridges goroutine-produced events into the Bubbletea message loop is correct and idiomatic.
4. **Graceful shutdown**: The SIGTERM-then-SIGKILL pattern in `Process.Shutdown()` is proper process lifecycle management.
5. **Ring buffer for output**: The detail panel's 10,000-line buffer with 20% trim is a practical approach to unbounded output.
6. **Config validation**: Self-dependency and missing-dependency checks in `Validate()` catch configuration errors early.
7. **NDJSON parsing resilience**: The parser handles alternate JSON structures for assistant messages, showing awareness of API format variations.

---

## Recommended Actions (Prioritized)

1. **Fix C1**: Return copies from `Manager.List()` and `Manager.Get()` to eliminate data races
2. **Fix C3**: Close `Process.EventCh` after the wait goroutine finishes to prevent goroutine leaks
3. **Fix C2**: Restructure scheduler to never send on channels while holding the mutex
4. **Fix H1**: Use non-blocking sends or drain `EventCh` on shutdown
5. **Fix H2**: Add consecutive error limit to `ParseStream` to prevent CPU spin
6. **Fix H3**: Move initial detail selection to `WindowSizeMsg` handler
7. **Fix L4**: Handle `SpawnCancelMsg` in `Update` -- this is a user-facing bug
8. **Add tests**: At minimum, test `dag.Graph.Validate()`, `config.LoadConfig`, `config.ApplyDefaults`, and `agent.ParseStream`
9. **Fix M2**: Close scheduler `eventCh` when `Run()` exits
10. **Fix M3/M4**: Use rune-aware truncation

---

## Metrics

| Metric | Value |
|--------|-------|
| Type Coverage | N/A (Go is statically typed; all types resolved) |
| Test Coverage | 0% (no test files) |
| Linting Issues | 0 (`go vet` clean) |
| Build Status | Passes |
| Total LOC | 2,303 |
| Files | 17 |

---

## Unresolved Questions

1. Is the `claude` CLI guaranteed to produce well-formed NDJSON, or can it produce partial lines / interleaved output? This affects the robustness requirements of `ParseStream`.
2. What is the intended behavior when a manually spawned agent (via "n" key) fails? Should it be restartable? Currently it would be, but it has no dependencies and no config persistence.
3. Should the pipeline scheduler support re-running a failed subtree? Currently, once an agent fails and its downstream is blocked, there is no mechanism to retry.
4. The `showRaw` field in `DetailModel` is toggled by the "l" key but never actually used in rendering. Is raw log view a planned feature?
