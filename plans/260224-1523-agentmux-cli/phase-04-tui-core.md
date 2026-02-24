# Phase 4: TUI Core

## Context Links
- [Parent Plan](plan.md)
- [Phase 2: Agent Process](phase-02-agent-process-management.md)
- [Phase 3: DAG Scheduler](phase-03-dag-scheduler.md)
- [Bubbletea Patterns Research](research/researcher-01-bubbletea-cobra-patterns.md)

## Overview
- **Date**: 2026-02-24
- **Priority**: P1
- **Status**: complete
- **Description**: Build the core Bubbletea TUI with composite model pattern: sidebar (agent list), detail panel (agent output viewport), statusbar, and Lip Gloss styling. This phase creates the visual shell; Phase 5 wires in live data.

## Key Insights
- Root model owns `focus` field; delegates Update/View to active sub-model
- ALWAYS pass `WindowSizeMsg` to ALL panels (resize gotcha)
- Viewport auto-scroll: track `autoScroll` bool; manual scroll up freezes; `G`/End re-enables
- Use bubbles `list.Model` for sidebar (built-in selection, filtering)
- Buffered channels (64+) for streaming data to prevent goroutine stalls
- `waitForMsg` pattern: blocking tea.Cmd that re-arms after each message

## Requirements

### Functional
- Composite layout: sidebar (30 chars wide) + detail (remaining width) + statusbar (1 line)
- Sidebar: list of agents with status icons (● running, ○ pending, ◌ waiting/blocked, ✓ done, ✗ failed)
- Detail: viewport showing selected agent's output with auto-scroll
- Detail header: agent name, status, duration, model, token usage
- Statusbar: keybinding hints + running/total agent count
- Tab switches focus between sidebar and detail
- j/k or arrow keys navigate within focused panel
- Responsive layout on terminal resize

### Non-Functional
- Render in <16ms (60fps) for smooth scrolling
- Support terminal sizes down to 80x24 minimum
- Ring buffer agent output at 10,000 lines to prevent memory bloat

## Architecture

```
tui/
├── app.go        # Root model: Init/Update/View, focus routing, message dispatch
├── sidebar.go    # Agent list component (wraps bubbles/list)
├── detail.go     # Agent output viewport with header + auto-scroll
├── statusbar.go  # Bottom bar: keybindings + stats
└── styles.go     # Lip Gloss style constants

Layout:
┌──────────────────────────────────────────────┐
│  agentmux            [3/4 running]    12:34  │  ← header (part of app.go View)
├────────────┬─────────────────────────────────┤
│ sidebar    │  detail                         │
│ (list)     │  (header + viewport)            │
│            │                                 │
│            │                                 │
├────────────┴─────────────────────────────────┤
│ [n]ew [k]ill [r]estart [l]ogs [p]ipe  q     │  ← statusbar
└──────────────────────────────────────────────┘
```

## Related Code Files

### Create
- `internal/tui/app.go` — root model, Init/Update/View, focus routing
- `internal/tui/sidebar.go` — sidebar component (agent list)
- `internal/tui/detail.go` — detail panel (viewport + header)
- `internal/tui/statusbar.go` — bottom status bar
- `internal/tui/styles.go` — Lip Gloss theme definitions

## Implementation Steps

### 1. Define styles (`styles.go`)

```go
var (
    SidebarWidth = 28

    // Colors
    AccentColor   = lipgloss.Color("62")   // muted blue
    SuccessColor  = lipgloss.Color("42")   // green
    ErrorColor    = lipgloss.Color("196")  // red
    WarningColor  = lipgloss.Color("214")  // orange
    MutedColor    = lipgloss.Color("241")  // gray
    PendingColor  = lipgloss.Color("248")  // light gray

    // Panel styles
    SidebarStyle = lipgloss.NewStyle().
        Width(SidebarWidth).
        BorderRight(true).
        BorderStyle(lipgloss.NormalBorder()).
        BorderForeground(MutedColor)

    SidebarFocusedStyle = SidebarStyle.
        BorderForeground(AccentColor)

    DetailStyle = lipgloss.NewStyle()

    StatusBarStyle = lipgloss.NewStyle().
        Background(AccentColor).
        Foreground(lipgloss.Color("230")).
        Padding(0, 1)

    // Status icons
    StatusIcons = map[AgentState]string{
        StateRunning: "●",
        StatePending: "○",
        StateBlocked: "◌",
        StateDone:    "✓",
        StateFailed:  "✗",
        StateKilled:  "✗",
    }

    StatusColors = map[AgentState]lipgloss.Color{
        StateRunning: AccentColor,
        StatePending: PendingColor,
        StateBlocked: WarningColor,
        StateDone:    SuccessColor,
        StateFailed:  ErrorColor,
        StateKilled:  ErrorColor,
    }
)
```

### 2. Implement sidebar (`sidebar.go`)

```go
type SidebarModel struct {
    list   list.Model
    agents []*agent.AgentInfo  // reference to manager's agent list
    width  int
    height int
}

// AgentItem implements list.Item for bubbles/list
type AgentItem struct {
    info *agent.AgentInfo
}
func (i AgentItem) Title() string       // "● planner" with colored icon
func (i AgentItem) Description() string // "Running 2m 14s" or "Pending"
func (i AgentItem) FilterValue() string // agent name

func NewSidebar(agents []*agent.AgentInfo) SidebarModel
func (s SidebarModel) Update(msg tea.Msg) (SidebarModel, tea.Cmd)
func (s SidebarModel) View() string
func (s SidebarModel) SelectedAgent() string  // returns selected agent name
func (s *SidebarModel) SetSize(w, h int)
func (s *SidebarModel) Refresh(agents []*agent.AgentInfo) // update agent list
```

<!-- Updated: Validation Session 1 - Alphabetical sort for agent list ordering -->
- Disable list filtering (not needed for small agent lists)
- Disable help view (statusbar handles keybinding display)
- Custom delegate for compact rendering: icon + name + brief status
- **Sort agents alphabetically by name** for consistent display order (Go maps don't preserve insertion order)

### 3. Implement detail panel (`detail.go`)

```go
type DetailModel struct {
    viewport   viewport.Model
    header     string
    agentName  string
    lines      []string     // output buffer
    maxLines   int          // ring buffer limit (10000)
    autoScroll bool
    width      int
    height     int
    ready      bool
}

func NewDetail() DetailModel
func (d DetailModel) Update(msg tea.Msg) (DetailModel, tea.Cmd)
func (d DetailModel) View() string
func (d *DetailModel) SetSize(w, h int)
func (d *DetailModel) SetAgent(name string, info *agent.AgentInfo)
func (d *DetailModel) AppendLine(line string)
func (d *DetailModel) SetHeader(info *agent.AgentInfo)
```

- Header (3 lines, above viewport):
  ```
  agent-name (#1)
  Status: ● Running  2m 14s  |  Model: sonnet
  Tokens: 12.4k in / 3.2k out  |  Tool: Read src/main.go
  ─────────────────────────────────────────────
  ```
- Viewport fills remaining height below header
- Auto-scroll logic from research:
  - New line arrives + autoScroll=true → `viewport.GotoBottom()`
  - User scrolls up → autoScroll=false
  - User presses `G`/End or scrolls to bottom → autoScroll=true
- Ring buffer: if `len(lines) > maxLines`, trim oldest 20%

### 4. Implement statusbar (`statusbar.go`)

```go
type StatusBarModel struct {
    width    int
    stats    PipelineStats
    mode     string  // "normal" | "spawn" | "confirm-kill"
}

type PipelineStats struct {
    Running int
    Pending int
    Done    int
    Failed  int
    Blocked int
    Total   int
    TotalTokensIn  int
    TotalTokensOut int
}

func NewStatusBar() StatusBarModel
func (s StatusBarModel) View() string
func (s *StatusBarModel) SetWidth(w int)
func (s *StatusBarModel) SetStats(stats PipelineStats)
func (s *StatusBarModel) SetMode(mode string)
```

- Normal mode: `[n]ew [k]ill [r]estart [l]ogs [p]ipe  tab:focus  q:quit  3/4 running`
- Spawn mode: `Enter:confirm  Esc:cancel  Spawning new agent...`
- Confirm-kill mode: `y:confirm  n:cancel  Kill agent "planner"?`

### 5. Implement root model (`app.go`)

```go
type focus int
const (
    focusSidebar focus = iota
    focusDetail
    focusSpawn   // Phase 5
)

type AppModel struct {
    sidebar   SidebarModel
    detail    DetailModel
    statusbar StatusBarModel
    focus     focus
    manager   *agent.Manager
    scheduler *dag.Scheduler
    width     int
    height    int
    ready     bool
}

func NewApp(mgr *agent.Manager, sched *dag.Scheduler) AppModel

func (m AppModel) Init() tea.Cmd
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd)
func (m AppModel) View() string
```

- `Init()`: return `tea.Batch(...)` with `WaitForEvent` for scheduler channel + each agent channel
- `Update()` routing:
  1. `tea.WindowSizeMsg` → resize ALL components
  2. `tea.KeyMsg`:
     - `tab` → toggle focus
     - `q`/`ctrl+c` → confirm quit (stop all agents) → `tea.Quit`
     - If focusSidebar: delegate to sidebar; on selection change → update detail
     - If focusDetail: delegate to detail (scrolling)
     - Global keys (regardless of focus): `n`, `k`, `r` (Phase 5)
  3. Agent events (`AssistantEvent`, `ToolUseEvent`, etc.):
     - Update agent info in manager
     - If event is for selected agent → `detail.AppendLine(formatted)`
     - Update sidebar refresh
     - Re-arm `WaitForEvent`
  4. Scheduler events (`AgentStartedMsg`, `AgentBlockedMsg`, `PipelineDoneMsg`):
     - Update sidebar + statusbar
- `View()`:
  ```go
  header := renderHeader(m.width, m.statusbar.stats)
  sidebar := SidebarStyle.Render(m.sidebar.View())
  detail := m.detail.View()
  body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, detail)
  statusbar := m.statusbar.View()
  return lipgloss.JoinVertical(lipgloss.Left, header, body, statusbar)
  ```

### 6. Format agent events for display

```go
// In app.go or a helpers file
func formatEvent(evt tea.Msg) string {
    switch e := evt.(type) {
    case AssistantEvent:
        return e.Text
    case ToolUseEvent:
        return fmt.Sprintf("→ Tool: %s %s", e.ToolName, e.Input)
    case ToolResultEvent:
        return fmt.Sprintf("  ← %s", truncate(e.Content, 120))
    case ResultEvent:
        return fmt.Sprintf("✓ Done (%d in / %d out tokens)", e.InputTokens, e.OutputTokens)
    case ErrorEvent:
        return fmt.Sprintf("✗ Error: %s", e.Err)
    }
    return ""
}
```

## Todo List
- [ ] Define Lip Gloss styles and theme (styles.go)
- [ ] Implement AgentItem for bubbles/list (sidebar.go)
- [ ] Implement SidebarModel with list, selection, refresh (sidebar.go)
- [ ] Implement DetailModel with viewport, header, auto-scroll (detail.go)
- [ ] Implement ring buffer for output lines (detail.go)
- [ ] Implement StatusBarModel with stats and mode display (statusbar.go)
- [ ] Implement AppModel with focus routing (app.go)
- [ ] Implement WindowSizeMsg handling for all components (app.go)
- [ ] Implement event formatting for detail panel (app.go)
- [ ] Implement agent event → detail + sidebar update flow (app.go)
- [ ] Verify `go build ./...` passes
- [ ] Manual test: launch TUI with mock data, verify layout and scrolling

## Success Criteria
- TUI renders correctly at 80x24 and larger terminals
- Tab switches focus with visual indicator (border color)
- Sidebar shows agent list with status icons and colors
- Detail panel shows output with auto-scroll and manual freeze
- Statusbar shows keybindings and pipeline stats
- Terminal resize handled gracefully (no layout break)

## Risk Assessment
- **Viewport AtBottom()**: May not exist in current bubbles version. Mitigation: check `YOffset >= TotalLineCount()-Height` manually.
- **Render performance**: Large output (10k lines) in viewport could slow rendering. Ring buffer caps this.
- **Unicode width**: Status icons (●, ✓) are variable-width in some terminals. Test with common terminals (iTerm2, Alacritty, Terminal.app).

## Security Considerations
- Agent output displayed in TUI may contain sensitive data from tool results; this is expected behavior for a local-only tool

## Next Steps
- Phase 5: Wire live agent data, add spawn dialog, keybindings, logging
