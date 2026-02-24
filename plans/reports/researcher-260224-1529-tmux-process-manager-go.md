# Research: Programmatic tmux Control from Go for Process Manager

**Date:** 2026-02-24 | **Focus:** Practical tmux control patterns for Go-based process manager

---

## Executive Summary

Go developers have **three battle-tested wrapper libraries** for tmux control and can shell out to tmux CLI (standard approach). **Control Mode is the recommended architecture** for real-time output monitoring vs. polling. Exit code detection requires `remain-on-exit` and `pane_dead_status` format variables. Polling `capture-pane` is viable for moderate frequencies (~100-500ms); Control Mode is optimal for high-frequency updates.

---

## 1. Spawning Processes in tmux

### Go Libraries (Recommended Tier)

| Library | Status | API Quality | Use Case |
|---------|--------|------------|----------|
| **gotmux** | Active | Excellent, type-safe | Production-ready, comprehensive |
| **go-tmux** | Active | Good | Lightweight, examples available |
| **gomux** | Active | Good | Simple session/window/pane ops |

**gotmux API (recommended)**:
```go
// Create session with working directory
tmux, err := gotmux.DefaultTmux()
session, err := tmux.NewSession(&gotmux.SessionOptions{
    Name:           "my-app",
    StartDirectory: "/home/user/project",
})

// Create window and pane
window, err := session.NewWindow("main")
pane, err := window.GetPaneByIndex(0)

// Send command
err = pane.SendKeys("npm start")
```

**Key Characteristics:**
- Creates real tmux sessions/windows/panes via CLI shelling
- Type-safe API with all tmux fields available (activity timestamp, attach count, cwd)
- Session persists after Go process exits (you control lifecycle)

**Under the Hood:**
All three libraries shell out to `tmux` CLI commands. There is **no native tmux C library binding for Go** — CLI is the standard approach. Libraries provide convenience wrappers around `tmux new-session`, `tmux new-window`, `tmux split-window`, etc.

### Direct CLI Approach (If No Library Fits)

```bash
# Create session
tmux new-session -d -s myapp -x 120 -y 30

# Create window with command
tmux new-window -t myapp -n build -d "make build"

# Split pane and run command
tmux send-keys -t myapp:build.0 "npm test" Enter
```

**Pros:** Full control, no dependencies
**Cons:** Manual error handling, harder to track state

---

## 2. Capturing Output

### Option A: Polling with `capture-pane` (Simpler)

**Method:**
```bash
tmux capture-pane -t session:window.pane -p
```

**Performance Baseline:**
- Safe polling frequency: **100-500ms** (10-5 captures/sec)
- No built-in refresh rate limit in `capture-pane`
- Network safe over SSH due to text-only output
- Buffer size: Full pane content (limited by tmux terminal size, typically 2000-4000 lines max)

**Limitations:**
- Inefficient for real-time (high frequency = high CPU)
- Loses intermediate output between polls
- No flow control mechanism

**Use When:**
- Polling interval ≥ 200ms acceptable
- Moderate output volume
- Simple implementation needed

### Option B: Control Mode (Recommended for Real-Time)

**Concept:** tmux Control Mode is a text protocol where the client maintains a persistent connection to tmux and receives `%output` notifications asynchronously.

**Protocol Flow:**
```
Client: tmux -CC (or -C for canonical mode)
Server: Sends %output events as pane content changes
Client: Tracks output in real-time without polling
Server: Pauses panes if client falls behind (flow control)
```

**Activation:**
```go
// Connect to tmux in control mode
cmd := exec.Command("tmux", "-CC", "attach", "-t", "myapp")
stdout := cmd.StdoutPipe()
stdin := cmd.StdinPipe()

// Read %output notifications asynchronously
// Send commands via stdin
```

**Key Advantages:**
- **True real-time** output (events arrive immediately)
- **Flow control:** tmux pauses pames when client lags (pause-after with `refresh-client -f`)
- **Bandwidth efficient:** Only deltas sent (not full pane each poll)
- **Extended-output:** Reports client latency: `%extended-output %0 1234 :` (1234ms behind)

**Pause/Resume Mechanism:**
```
Server sends: %pause <pane-id>
Client responds: refresh-client -A  # resume
# Client can call: tmux capture-pane when ready to resync
```

**Gotchas:**
- Must parse text protocol carefully (octal escaping for special chars)
- Requires persistent connection management
- More complex client implementation

**Use When:**
- Real-time monitoring required
- High-frequency updates (100+ per second)
- Network efficiency matters (SSH tunnels)

### Hybrid Approach

Use Control Mode for primary output stream but poll `capture-pane` occasionally for full-screen snapshots or state verification.

---

## 3. Sending Input to Running Processes

### `tmux send-keys` Command

**Basic Usage:**
```bash
tmux send-keys -t session:window.pane "your_command" Enter
```

**API from gotmux:**
```go
pane.SendKeys("npm start")  // Automatically adds Return
pane.SendKeys("echo test")
```

**Advanced Options:**
- `-l` flag: Send keys literally (disable reserved words like `C-c`)
  ```bash
  tmux send-keys -t pane -l "C-c"  # Sends literal "C-c" string, not interrupt
  ```
- To send actual control sequences:
  ```bash
  tmux send-keys -t pane "C-c"  # Interrupt
  tmux send-keys -t pane "C-d"  # EOF
  tmux send-keys -t pane "C-z"  # Suspend
  ```

**Common Patterns:**
```go
// Run interactive command
pane.SendKeys("psql -U user -d database")

// Send Ctrl+C to stop
cmd := exec.Command("tmux", "send-keys", "-t", "pane", "C-c")

// Send multiple keystrokes
pane.SendKeys("ls -la")
pane.SendKeys("cd /tmp")
pane.SendKeys("pwd")
```

**Reliability:**
- Sends are **non-blocking** (command queued, not guaranteed to execute)
- Use small delays between sends if sequence-critical
- No feedback on whether command executed

---

## 4. Process Lifecycle & Exit Detection

### Standard Approach: `remain-on-exit` + Format Variables

**Setup:**
```bash
# Set pane option before running command
tmux set-window-option -t session:window remain-on-exit on

# Then panes stay visible even after process exits
tmux send-keys -t pane "your_script.sh" Enter
```

**Detecting Exit:**
```bash
# Check pane_dead_status format variable
tmux display-message -t pane '#{pane_dead_status}'  # Returns exit code (e.g., "0" or "1")

# Or check if pane is dead
tmux display-message -t pane '#{?pane_dead,[DEAD],}'

# List dead panes with codes
tmux list-panes -t session:window -F '#{pane_index}: #{pane_dead_status}'
```

**In Go with gotmux:**
```go
// Enable remain-on-exit before running command
session.SetWindowOption("remain-on-exit", "on")

// Periodically check pane status
for {
    paneInfo, err := window.GetPane(0)
    if paneInfo.IsDead {
        exitCode := paneInfo.DeadStatus  // May require custom field
        fmt.Printf("Process exited with code: %d\n", exitCode)
        break
    }
    time.Sleep(500 * time.Millisecond)
}
```

**Note:** `pane_dead_status` **only works when `remain-on-exit=on`**. Without it, pane closes immediately and exit code is lost.

### Alternative: `pane-exited` Hook

```bash
# Fire a hook when process exits (only if remain-on-exit is OFF)
tmux set-hook -p pane-exited "run-shell 'echo Process exited'"

# Or set global hook and detect exit via polling
```

**Limitation:** Hooks don't capture exit code directly in standard tmux. You must use `remain-on-exit + pane_dead_status`.

### Extracting Exit Code Programmatically

No direct way to get exit code without `remain-on-exit`. For long-running processes:
```bash
# Wrapper script approach
tmux send-keys -t pane "script.sh && echo EXIT_0 || echo EXIT_\$?" Enter

# Monitor output for EXIT_X marker
```

---

## 5. Performance: Polling vs. Control Mode

### Polling `capture-pane` Benchmarks

| Frequency | Recommended Use | CPU Impact | Notes |
|-----------|-----------------|-----------|-------|
| **100ms** | Moderate updates | Low | Safe for dashboards |
| **200ms** | Slow processes | Minimal | Good balance |
| **500ms** | Status monitoring | Negligible | UI refresh limits |
| **1s+** | Logging/archival | None | Not real-time |
| **10ms** | High-frequency | High | Viable for short bursts |

**Practical Limits:**
- tmux can handle capture-pane calls at ~100/sec on modern hardware
- SSH latency becomes significant (<200ms polling over WAN)
- Terminal rendering is the bottleneck (not tmux itself)

### Control Mode Performance

- **0-latency** output delivery (event-driven)
- **Bandwidth:** ~50-200 bytes/event (vs. full pane snapshot)
- **Flow control:** Prevents buffer overflow
- **Recommended for:** Multi-pane monitoring, real-time dashboards

---

## 6. Architecture Recommendations for Go Process Manager

### Recommended Stack

```
┌─────────────────────────────┐
│   Go Process Manager        │
├─────────────────────────────┤
│  ├─ gotmux (session control)│  ← Spawn, manage lifecycle
│  ├─ Control Mode (output)   │  ← Real-time monitoring
│  ├─ capture-pane (fallback) │  ← Snapshot verification
│  └─ send-keys (input)       │  ← User commands
└─────────────────────────────┘
         │
         ↓
    tmux CLI (standard)
         │
         ↓
    User Processes (shell, scripts, apps)
```

### Implementation Pattern

```go
type ProcessManager struct {
    session *gotmux.Session

    // For output monitoring
    controlConn net.Conn
    outputChan  chan OutputEvent

    // For process tracking
    processes map[string]*ProcessHandle
}

type ProcessHandle struct {
    ID        string
    Pane      *gotmux.Pane
    ExitCode  int
    LastAlive time.Time
}

// Spawn process in tmux pane
func (pm *ProcessManager) Spawn(cmd string) (*ProcessHandle, error) {
    pane, err := pm.createPane()
    // Enable exit code detection
    pm.session.SetWindowOption("remain-on-exit", "on")
    pane.SendKeys(cmd)
    return &ProcessHandle{Pane: pane}, nil
}

// Monitor output in real-time via Control Mode
func (pm *ProcessManager) MonitorOutput(ctx context.Context) {
    for event := range pm.outputChan {
        pm.processOutput(event)
    }
}

// Poll for dead panes periodically
func (pm *ProcessManager) PollExitCodes(interval time.Duration) {
    ticker := time.NewTicker(interval)
    for range ticker.C {
        // Check pane_dead_status for each process
    }
}
```

---

## 7. Unresolved Questions & Trade-Offs

1. **Control Mode Parsing:** No Go library wraps Control Mode protocol — requires custom implementation of text protocol parsing. Feasible but not trivial.

2. **Exit Code Reliability:** `pane_dead_status` format variable availability varies by tmux version (requires reasonably recent build). Check `tmux -V` compatibility.

3. **Pane Capacity:** No documented limit on pane output buffer. Practical limit ~2000-4000 lines depending on terminal size and tmux build.

4. **Remote Execution:** All approaches work over SSH (text-based CLI), but Control Mode has lower bandwidth footprint for high-frequency monitoring.

5. **Session Cleanup:** Ensure logic to kill sessions when process manager exits to avoid orphaned tmux processes.

---

## Key References

- **gotmux:** [GitHub - GianlucaP106/gotmux](https://github.com/GianlucaP106/gotmux)
- **go-tmux:** [GitHub - jubnzv/go-tmux](https://github.com/jubnzv/go-tmux)
- **gomux:** [GitHub - wricardo/gomux](https://github.com/wricardo/gomux)
- **tmux Control Mode Wiki:** [GitHub - tmux/tmux Control Mode](https://github.com/tmux/tmux/wiki/Control-Mode)
- **tmux send-keys Guide:** [Linux Hint - tmux send-keys](https://linuxhint.com/tmux-send-keys/)
- **tmux capture-pane:** [Fig - tmux capturep](https://fig.io/manual/tmux/capturep)
- **Exit Code Detection:** [tmux-users Narkive - Get exit code from dead pane](https://tmux-users.narkive.com/NuxpYVrQ/get-exit-code-from-a-dead-pane)
- **Control Mode Flow Control:** [GitHub Issue #2217 - Control mode pause/resume](https://github.com/tmux/tmux/issues/2217)
