# Phase 5: TUI Features & CLI Wiring

## Context Links
- [Parent Plan](plan.md)
- [Phase 1: Config](phase-01-project-setup-config.md)
- [Phase 2: Agent Process](phase-02-agent-process-management.md)
- [Phase 3: DAG Scheduler](phase-03-dag-scheduler.md)
- [Phase 4: TUI Core](phase-04-tui-core.md)
- [Cobra Integration Research](research/researcher-01-bubbletea-cobra-patterns.md)

## Overview
- **Date**: 2026-02-24
- **Priority**: P1
- **Status**: complete
- **Description**: Wire everything together: Cobra CLI commands, spawn dialog, keybindings for agent lifecycle, log persistence, and the full startup flow from CLI → config → DAG → agents → TUI.

## Key Insights
- Cobra `RunE` loads config, builds DAG, creates manager+scheduler, launches `tea.NewProgram`
- Spawn dialog = overlay text input (Bubbletea `textinput` bubble)
- Keybindings should work regardless of focus for agent lifecycle (n/k/r)
- Log writer is a simple JSONL appender; one file per agent per run
- `tea.WithAltScreen()` for clean exit; `tea.WithMouseCellMotion()` optional

## Requirements

### Functional
- `agentmux run [--config path]` — load config, build DAG, launch TUI, execute pipeline
- `agentmux run` without config — launch TUI in empty state (spawn agents manually)
- `agentmux version` — print version
- Spawn dialog: `n` key → text input for prompt, optional model select, confirm → spawn ad-hoc agent
- Kill: `k` key → confirm → stop selected agent
- Restart: `r` key → restart selected agent (re-run same config)
- Logs: `l` key → toggle log view (show raw JSONL for selected agent)
- Pipeline view: `p` key → show DAG status (simple text representation)
- Log persistence: write each agent's events to `~/.agentmux/logs/{timestamp}-{name}.jsonl`
- Quit: `q` → stop all agents gracefully → exit

### Non-Functional
- Startup to TUI render <500ms
- Graceful shutdown: all agents receive SIGTERM before process exits
- Log files rotated by run (not appended across runs)

## Architecture

```
cmd/
├── root.go      # --config flag, PersistentPreRunE
├── run.go       # `agentmux run` — config → DAG → manager → scheduler → TUI
└── version.go   # prints version

tui/
├── spawn.go     # Spawn dialog overlay (textinput bubble)
└── (existing)   # app.go updated with keybindings + wiring

log/
└── writer.go    # JSONL file writer

Startup flow:
  CLI parse → LoadConfig → BuildDAG → NewManager → RegisterAgents
    → NewScheduler → NewApp(mgr, sched)
    → tea.NewProgram(app, WithAltScreen).Run()
    → scheduler.Run(ctx) as goroutine
    → TUI event loop
```

## Related Code Files

### Create
- `cmd/run.go` — run command implementation
- `internal/tui/spawn.go` — spawn dialog component
- `internal/log/writer.go` — JSONL log writer

### Modify
- `cmd/root.go` — add run subcommand
- `internal/tui/app.go` — add keybindings, spawn dialog, log toggle, pipeline view

## Implementation Steps

### 1. Implement `cmd/run.go`

```go
var runCmd = &cobra.Command{
    Use:   "run",
    Short: "Load config and launch TUI dashboard",
    RunE:  runApp,
}

func init() {
    rootCmd.AddCommand(runCmd)
}

func runApp(cmd *cobra.Command, args []string) error {
    cfgPath, _ := cmd.Flags().GetString("config")

    // 1. Load config (optional; empty config = manual-only mode)
    cfg, err := config.LoadConfig(cfgPath)
    if err != nil && cfgPath != "" {
        return fmt.Errorf("config error: %w", err)
    }
    if cfg == nil {
        cfg = config.DefaultConfig()
    }

    // 2. Build DAG
    graph, err := dag.BuildFromConfig(cfg)
    if err != nil {
        return fmt.Errorf("pipeline error: %w", err)
    }

    // 3. Create manager + register agents
    mgr := agent.NewManager(cfg.Defaults)
    for name, agentCfg := range cfg.Agents {
        mgr.Register(name, agentCfg)
    }

    // 4. Create scheduler
    sched := dag.NewScheduler(graph, mgr)

    // 5. Create and run TUI
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    app := tui.NewApp(mgr, sched, ctx, cancel)
    p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion())

    _, err = p.Run()

    // 6. Cleanup: stop all agents on exit
    mgr.StopAll()
    return err
}
```

### 2. Implement spawn dialog (`spawn.go`)

```go
type SpawnModel struct {
    promptInput textinput.Model
    modelInput  textinput.Model
    focusIndex  int     // 0=prompt, 1=model
    active      bool
    width       int
}

func NewSpawnDialog() SpawnModel
func (s SpawnModel) Update(msg tea.Msg) (SpawnModel, tea.Cmd)
func (s SpawnModel) View() string
func (s *SpawnModel) SetActive(active bool)
func (s *SpawnModel) Reset()

// SpawnConfirmMsg sent when user confirms
type SpawnConfirmMsg struct {
    Prompt string
    Model  string
}
```

- `n` key in app.go → `spawn.SetActive(true)`, set focus to `focusSpawn`
- Spawn dialog renders as overlay (centered box on top of detail panel)
- Tab cycles between prompt and model fields
- Enter confirms → sends `SpawnConfirmMsg` → app.Update creates ad-hoc agent via manager
- Esc cancels → `spawn.SetActive(false)`, restore previous focus

### 3. Add keybindings to `app.go`

Update `AppModel.Update()` for global keybindings:

```go
case tea.KeyMsg:
    // Global keys (any focus except spawn dialog)
    if m.focus != focusSpawn {
        switch msg.String() {
        case "n":
            m.spawn.SetActive(true)
            m.focus = focusSpawn
            return m, m.spawn.promptInput.Focus()
        case "k":
            // Kill selected agent
            name := m.sidebar.SelectedAgent()
            if name != "" {
                m.statusbar.SetMode("confirm-kill")
                m.confirmAction = func() tea.Cmd {
                    m.manager.Stop(name)
                    return nil
                }
            }
        case "r":
            // Restart selected agent
            name := m.sidebar.SelectedAgent()
            if name != "" {
                ch, err := m.manager.Restart(name)
                if err == nil {
                    return m, agent.WaitForEvent(ch)
                }
            }
        case "l":
            m.detail.ToggleLogView()  // raw JSONL vs formatted
        case "p":
            m.detail.ShowPipelineView(m.scheduler.Status())
        case "q", "ctrl+c":
            m.manager.StopAll()
            return m, tea.Quit
        }
    }
```

- Confirm-kill mode: `y` executes kill, `n`/Esc cancels, statusbar shows prompt

### 4. Handle SpawnConfirmMsg

```go
case SpawnConfirmMsg:
    name := fmt.Sprintf("agent-%d", m.nextAgentID)
    m.nextAgentID++
    cfg := agent.AgentConfig{
        Prompt: msg.Prompt,
        Model:  msg.Model,
    }
    m.manager.Register(name, cfg)
    ch, err := m.manager.Start(name)
    if err == nil {
        cmds = append(cmds, agent.WaitForEvent(ch))
    }
    m.spawn.Reset()
    m.focus = focusSidebar
    m.sidebar.Refresh(m.manager.List())
```

### 5. Implement log writer (`writer.go`)

```go
type Writer struct {
    dir     string        // ~/.agentmux/logs/
    files   map[string]*os.File
    mu      sync.Mutex
}

func NewWriter() (*Writer, error)   // creates dir if not exists
func (w *Writer) Write(agentName string, event interface{}) error
func (w *Writer) Close() error

// Write serializes event as JSON line and appends to agent's log file
// File: ~/.agentmux/logs/{YYYYMMDD-HHMMSS}-{agentName}.jsonl
```

- Writer created at app startup, passed to AppModel
- On each agent event in `app.Update()`, call `writer.Write(agentName, event)`
- `Close()` flushes and closes all files; called on quit

### 6. Wire log writer into app.go

```go
// In NewApp:
logWriter, _ := log.NewWriter()
app.logWriter = logWriter

// In Update, after processing agent events:
if logWriter != nil {
    logWriter.Write(agentName, msg)
}

// In quit handler:
m.logWriter.Close()
```

### 7. Pipeline status view

When `p` is pressed, detail panel shows a simple text DAG:

```
Pipeline Status
═══════════════
planner  ● Running  (2m 14s)
  └─► coder    ○ Pending
        ├─► tester   ○ Pending
        └─► reviewer ○ Pending
```

- Simple recursive renderer using graph adjacency and agent status
- Displayed in detail viewport (replaces agent output temporarily)
- Any other key returns to normal agent detail view

## Todo List
- [ ] Implement cmd/run.go (config → DAG → manager → scheduler → TUI)
- [ ] Wire run subcommand into root.go
- [ ] Implement SpawnModel dialog (spawn.go)
- [ ] Add global keybindings to app.go (n/k/r/l/p/q)
- [ ] Implement confirm-kill flow in app.go
- [ ] Handle SpawnConfirmMsg for ad-hoc agents (app.go)
- [ ] Implement log.Writer JSONL persistence (writer.go)
- [ ] Wire log writer into app event handling
- [ ] Implement pipeline status view (app.go or detail.go)
- [ ] Implement Init() with scheduler.Run + WaitForEvent arming
- [ ] Test full startup flow: `agentmux run --config agentmux.yaml`
- [ ] Test empty startup: `agentmux run` → spawn via dialog

## Success Criteria
- `agentmux run --config agentmux.yaml` launches TUI and starts pipeline
- `agentmux run` launches empty TUI; `n` opens spawn dialog
- Kill/restart selected agent works
- Log files appear in `~/.agentmux/logs/` with valid JSONL
- `q` stops all agents gracefully and exits
- Pipeline view shows DAG structure with statuses

## Risk Assessment
- **Spawn dialog focus management**: Textinput focus must be explicitly managed. If bubbletea focus is tricky, start with single-field prompt-only dialog (KISS).
- **Concurrent agent channel draining**: Each agent's event channel needs its own `WaitForEvent` cmd. On 10+ agents, this means 10+ concurrent blocking cmds. Bubbletea handles this fine (each is a separate goroutine).
- **Log disk space**: JSONL files can grow large for long-running agents. Ring buffer in-memory; full log on disk. Acceptable for v1.

## Security Considerations
- Log files contain agent output (potentially sensitive code/data). Written to user's home dir with default permissions.
- Spawn dialog prompts are user-provided; no injection risk (passed directly to claude CLI as -p argument).

## Next Steps
- Phase 6: Testing and polish
