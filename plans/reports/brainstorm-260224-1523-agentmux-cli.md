# Brainstorm: agentmux — Claude Code Agent Manager TUI

## Problem Statement

Managing multiple Claude Code agents (planner, coder, tester, reviewer) across parallel tasks is painful. No visibility into what each agent is doing, no easy way to spawn/kill/coordinate them, and no structured way to define agent pipelines. Need an interactive TUI that makes multi-agent orchestration visual and manageable.

## Decisions Made

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Name | **agentmux** | Blend of "agent" + "tmux" |
| Stack | **Go + Bubbletea** | Fast, single binary, excellent TUI ecosystem |
| Layout | **List + Detail** | Sidebar agent list + right panel detail view |
| Output capture | **Stream-JSON (v1) + tmux attach (v1.1)** | Structured data first, raw attach later |
| Agent config | **YAML presets + ad-hoc TUI** | Reproducible pipelines + flexibility |
| Pipelines | **Basic DAG via depends_on** | Simple dependency chains in config |
| Scope | **Feature-rich v1** | Spawn, monitor, kill, log persistence, DAG, config |
| Target | **Claude Code only** | Tight integration with claude CLI |

## Architecture

```
agentmux (Go binary)
│
├── cmd/                    # CLI entry point (cobra)
├── internal/
│   ├── agent/              # Agent lifecycle management
│   │   ├── manager.go      # Spawn, track, kill agents
│   │   ├── process.go      # os/exec wrapper for claude CLI
│   │   └── parser.go       # Stream-JSON event parser
│   ├── dag/                # Dependency resolution
│   │   ├── graph.go        # DAG data structure
│   │   └── scheduler.go    # Topological sort + execution
│   ├── config/             # YAML config parsing
│   │   └── config.go       # Agent presets, pipeline defs
│   ├── tui/                # Bubbletea TUI
│   │   ├── app.go          # Root model (composite)
│   │   ├── sidebar.go      # Agent list panel
│   │   ├── detail.go       # Agent detail/output panel
│   │   ├── statusbar.go    # Bottom status bar
│   │   ├── spawn.go        # New agent dialog
│   │   └── styles.go       # Lip Gloss styles
│   └── log/                # Log persistence
│       └── writer.go       # File-based log storage
├── agentmux.yaml           # Example config
├── go.mod
└── main.go
```

### Data Flow

```
User Action (keystroke)
    │
    ▼
TUI Model.Update()
    │
    ├─► Spawn: agent.Manager.Start(config)
    │     └─► os/exec: claude -p --output-format stream-json
    │           stdout pipe → JSON parser goroutine
    │                          └─► tea.Cmd → Msg → TUI update
    │
    ├─► Kill: agent.Manager.Stop(id)
    │     └─► process.Signal(SIGTERM)
    │
    └─► Navigate: update focused agent in sidebar
```

### Stream-JSON Event Handling

Claude Code's `--output-format stream-json` emits newline-delimited JSON:
```json
{"type":"assistant","message":{"content":[{"type":"text","text":"Analyzing..."}]}}
{"type":"tool_use","tool":{"name":"Read","input":{"file_path":"src/main.go"}}}
{"type":"tool_result","content":"..."}
{"type":"result","result":"Done","session_id":"abc123","usage":{...}}
```

Parser extracts: status, current action, tool calls, token usage, completion.

### Agent Config Format

```yaml
# agentmux.yaml
version: 1
defaults:
  model: opus
  allowedTools: ["Bash", "Read", "Edit", "Write", "Glob", "Grep"]
  max_turns: 50

agents:
  planner:
    prompt: "Analyze the codebase and create implementation plan"
    workdir: ./src
    model: sonnet  # override default

  coder:
    prompt: "Implement features according to the plan"
    depends_on: [planner]
    workdir: ./src
    allowedTools: ["Bash", "Read", "Edit", "Write", "Glob", "Grep"]

  tester:
    prompt: "Write and run comprehensive tests"
    depends_on: [coder]
    workdir: ./src

  reviewer:
    prompt: "Review code quality and suggest improvements"
    depends_on: [coder]  # parallel with tester
    workdir: ./src
```

### TUI Layout

```
┌──────────────────────────────────────────────┐
│  agentmux            [3/4 running]    12:34  │
├────────────┬─────────────────────────────────┤
│ Agents     │  planner (#1)                   │
│            │  Status: ● Running  2m 14s      │
│ ● #1 plnr │  Model: sonnet                  │
│ ● #2 code │  Tokens: 12.4k in / 3.2k out    │
│ ○ #3 test │  ───────────────────────────────│
│ ◌ #4 revw │  Tool: Read src/auth.ts         │
│            │                                 │
│            │  > Analyzing authentication...  │
│            │  > Found 3 patterns to refactor │
│            │  > Generating plan document...  │
│            │  > Writing plans/phase-01.md    │
│            │                                 │
│ Legend:    │                                 │
│ ● running │  [scroll: j/k  pgup/pgdn]      │
│ ○ pending │                                 │
│ ◌ waiting │                                 │
│ ✓ done    │                                 │
│ ✗ failed  │                                 │
├────────────┴─────────────────────────────────┤
│ [n]ew [k]ill [r]estart [l]ogs [p]ipeline  q │
└──────────────────────────────────────────────┘
```

## Evaluated Approaches

### Approach 1: Pure tmux capture-pane polling
- **Pros**: Works with any CLI, familiar tmux workflow, can `tmux attach`
- **Cons**: 200ms polling delay, unstructured text, CPU overhead, hard to extract structured status
- **Verdict**: Rejected for v1. Text parsing unreliable for status extraction.

### Approach 2: Pure stream-JSON direct pipes (CHOSEN for v1)
- **Pros**: Real-time, structured data, no tmux dependency, lower complexity, rich metadata (tokens, tools)
- **Cons**: Can't `tmux attach` for raw view, tied to claude output format
- **Verdict**: Best for v1. Structured data enables rich TUI with real status info.

### Approach 3: Hybrid stream-JSON + tmux (CHOSEN for v1.1)
- **Pros**: Best of both worlds, structured + raw access
- **Cons**: More complexity, two output paths
- **Verdict**: Add tmux layer in v1.1 after core is stable.

## Implementation Considerations

### Key Libraries
- **Bubbletea** (`github.com/charmbracelet/bubbletea`) — TUI framework
- **Lip Gloss** (`github.com/charmbracelet/lipgloss`) — Layout/styling
- **Bubbles** (`github.com/charmbracelet/bubbles`) — List, Viewport, Spinner components
- **Cobra** (`github.com/spf13/cobra`) — CLI flags/commands
- **Viper** (`github.com/spf13/viper`) — YAML config parsing
- **gotmux** (`github.com/jubnzv/gotmux`) — tmux control (v1.1)

### Concurrency Model
- Each agent gets a goroutine reading stdout pipe
- JSON events parsed and sent as `tea.Msg` via channel
- Single Bubbletea event loop processes all messages (thread-safe)
- DAG scheduler runs in separate goroutine, signals TUI on state changes

### DAG Execution
- Topological sort at pipeline start
- Agents with no dependencies start immediately
- On agent completion, check dependents and start if all deps satisfied
- On agent failure, mark all downstream dependents as `blocked`

### Log Persistence
- Each agent's stream-json output written to `~/.agentmux/logs/{timestamp}-{name}.jsonl`
- Enables replay, post-mortem analysis, and audit trail
- TUI `[l]ogs` command opens log viewer with filtering

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Claude stream-json format changes | Breaks parser | Version-pin format, add format detection |
| Agent hangs (no output) | Blocks pipeline | Configurable timeout per agent |
| Many agents = high token cost | Budget overrun | `max_budget_usd` per agent, pipeline-level budget cap |
| Large output floods TUI | Unresponsive UI | Ring buffer for output (last N lines), viewport scrolling |
| DAG cycles in config | Startup crash | Cycle detection at config load time |

## Success Metrics
- Spawn 5+ agents from single command/config
- Real-time output visible within <100ms of generation
- Pipeline with 4 agents (plan→code→test+review) runs correctly
- Clean agent lifecycle: spawn, monitor, kill, restart
- Config file is intuitive — new user productive in <5 minutes

## v1 Feature Scope

### Must Have
- [x] CLI: `agentmux run` (from config) and `agentmux` (interactive TUI)
- [x] YAML config with agent presets and `depends_on`
- [x] Stream-JSON parsing for real-time status
- [x] List + Detail TUI layout
- [x] Agent lifecycle: spawn, kill, restart
- [x] DAG dependency resolution and execution
- [x] Log persistence to disk
- [x] Keybindings for all actions

### Nice to Have (v1)
- [ ] Output search/filter in detail panel
- [ ] Agent output summary (condensed view vs full log)
- [ ] Pipeline progress bar
- [ ] Token usage tracking per agent and total

### v1.1 (tmux integration)
- [ ] Optional tmux pane per agent
- [ ] `[a]ttach` to switch to tmux pane for raw view
- [ ] `tmux attach -t agentmux` for external access

### v2 (future)
- [ ] Session resume (`--resume` with saved session IDs)
- [ ] Inter-agent communication (pass output of A as context to B)
- [ ] Web dashboard alternative (optional)
- [ ] Plugin system for custom parsers

## Next Steps
1. Create detailed implementation plan with phases
2. Set up Go project with module, dependencies
3. Implement core agent process management
4. Build stream-JSON parser
5. Build TUI shell (sidebar + detail)
6. Add DAG scheduler
7. Wire everything together
8. Add config file support
9. Testing and polish
