# Bubbletea TUI Framework Research Report

**Date:** 2026-02-24 | **Focus:** Architecture, patterns, components, concurrency for multi-pane dashboards

---

## Executive Summary

Bubbletea is a production-ready Go TUI framework (10k+ apps in use) built on The Elm Architecture. For process/dashboard monitoring with multi-pane layouts, the framework excels at:
- Declarative state management via Model/Update/View
- Thread-safe goroutine integration through Cmd abstraction
- Composable layouts with Lip Gloss for complex UIs
- Real-time streaming via channel-based message passing

**Key caveat:** Never spawn raw goroutines; use Bubble Tea's Cmd system as the concurrency boundary.

---

## 1. Architecture Patterns: Multi-Panel Layouts

### Elm Architecture Core
Every Bubbletea app implements three methods on a Model struct:

```go
type Model interface {
    Init() Cmd                      // Runs once at startup
    Update(msg Msg) (Model, Cmd)   // State machine, returns new state + side effects
    View() string                  // Renders current state
}
```

**Message flow:** Events (keypresses, timers, async I/O) → Update → View re-renders. The framework handles the main loop; you define pure state transitions.

### Multi-Pane Layout Strategies

#### Strategy 1: Composite Model Pattern (Recommended for dashboards)
Embed sub-models (sidebar, detail view, status bar) within a parent model:

```go
type Model struct {
    sidebar    SidebarModel
    detail     DetailModel
    status     StatusModel
    activePane PaneID  // Track focus
    width, height int
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // Route messages to active pane
    switch m.activePane {
    case Sidebar:
        updatedSidebar, cmd := m.sidebar.Update(msg)
        m.sidebar = updatedSidebar.(SidebarModel)
        return m, cmd
    // ... other panes
    }
}

func (m Model) View() string {
    // Use lip gloss to compose layouts
    sidebar := lipgloss.NewStyle().Width(20).Render(m.sidebar.View())
    detail := m.detail.View()
    return lipgloss.JoinHorizontal(
        lipgloss.Left,
        sidebar,
        detail,
    )
}
```

**Advantages:** Clean separation, sub-models handle their own logic, easy to reuse components.

#### Strategy 2: Dedicated Layout Libraries
- **[Panes](https://github.com/john-marinelli/panes)**: Component for switching between panes (ctrl+h/j/k/l)
- **[BubbleLayout](https://github.com/winder/bubblelayout)**: Declarative layout manager with constraint-based sizing

#### Window Resizing
Bubbletea sends `tea.WindowSizeMsg` at startup and on terminal resize. Always capture and store dimensions:

```go
case tea.WindowSizeMsg:
    m.width = msg.Width
    m.height = msg.Height
    // Recalculate pane sizes
```

---

## 2. Key Components: Bubbles + Lip Gloss

### Bubbles Library: Ready-Made Components

**Display components:**
- **Viewport**: For scrollable regions (tailing logs, long output) — maintains scroll position, handles page up/down
- **Table**: Multi-column lists with custom styling
- **Progress bar**: Visual feedback for tasks

**Input components:**
- **Textinput**: Single-line text fields (filtering, search)
- **Textarea**: Multi-line input with Unicode support
- **Filepicker**: Navigate filesystem

**Navigation:**
- **List**: Filterable item list with cycling behavior
- **Paginator**: Page-based navigation logic

**Utility:**
- **Spinner**: Loading indicator
- **Timer/Stopwatch**: Timing utilities
- **Help**: Auto-generate keybinding help

### Lip Gloss: Styling & Layout

Key functions for dashboard layouts:

```go
// Combine panes horizontally
lipgloss.JoinHorizontal(
    lipgloss.Center,  // vertical alignment
    leftPane,
    rightPane,
)

// Combine panes vertically
lipgloss.JoinVertical(
    lipgloss.Left,    // horizontal alignment
    topPane,
    bottomPane,
)

// Apply styles (colors, borders, padding)
style := lipgloss.NewStyle().
    BorderStyle(lipgloss.RoundedBorder()).
    BorderForeground(lipgloss.Color("62")).
    Padding(1, 2).
    Width(40)
```

**Critical detail:** When nesting bordered panels, manually subtract 2 from height/width for border space. Truncate text explicitly; don't rely on auto-wrapping.

---

## 3. Real-Time Updates: Streaming Data Pattern

### The Command-Message Pattern

Bubbletea executes Cmd functions in separate goroutines, capturing output as messages:

```go
// Define a Cmd that tails a file
func tailLogFile(path string) tea.Cmd {
    return func() tea.Msg {
        // Long-running operation
        file, _ := os.Open(path)
        defer file.Close()

        scanner := bufio.NewScanner(file)
        for scanner.Scan() {
            // Simulate streaming output
            return LogLineMsg{Line: scanner.Text()}
        }
        return nil
    }
}

// In Update(), handle the result:
case LogLineMsg:
    m.logs = append(m.logs, msg.Line)
    // Returning a new Cmd here enables recursive polling
    return m, tailLogFile(m.logPath)
```

### Continuous Streaming via Channels

For multiple concurrent sources (multiple tmux panes, process outputs), use Go channels:

```go
// Cmd that listens to a channel and forwards messages
func listenToChannel(ch chan MyMsg) tea.Cmd {
    return func() tea.Msg {
        return <-ch  // Blocks until message available
    }
}

// In Init(), spawn goroutines and start listening
func (m Model) Init() tea.Cmd {
    go func() {
        for output := range processStream {
            m.msgChan <- ProcessOutputMsg{Data: output}
        }
    }()

    return listenToChannel(m.msgChan)
}

// In Update(), re-invoke listenToChannel to maintain stream
case ProcessOutputMsg:
    m.buffer = append(m.buffer, msg.Data)
    return m, listenToChannel(m.msgChan)
```

### [Real-Time Example from Official Repo](https://github.com/charmbracelet/bubbletea/tree/main/examples)
The `realtime` example demonstrates Go channel communication for concurrent updates. Key insight: a single channel becomes the hub for all background goroutines to send messages safely without blocking.

---

## 4. Concurrency: Best Practices

### Architecture Pattern (Critical)

**NEVER use raw goroutines in Bubbletea.** Instead:

1. **Spawn goroutines in Init() or Cmd functions** for setup
2. **Communicate via channels** that feed into Cmd-based listeners
3. **Return Cmd from Update()** to re-listen and maintain stream

```go
// Pattern: Polling multiple sources
func pollMultipleSources(sources []string, interval time.Duration) tea.Cmd {
    return tea.Tick(interval, func(t time.Time) tea.Msg {
        // Collect data from all sources
        results := make(map[string]string)
        for _, src := range sources {
            results[src] = readSource(src)  // e.g., tmux capture-pane
        }
        return SourceUpdateMsg{Data: results}
    })
}
```

### tea.Tick for Periodic Updates
For regular polling (e.g., every 100ms to refresh tmux pane outputs):

```go
tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
    return RefreshMsg{Timestamp: t}
})
```

### Message Queue Guarantees
- Each Cmd runs in its own goroutine
- Messages are sent through a thread-safe channel (p.msgs)
- Update() is always called on the main thread
- No race conditions if you follow the Cmd pattern

---

## 5. Real-World Process Monitoring Example: tmuxwatch

### Project: [tmuxwatch](https://github.com/steipete/tmuxwatch)
A Charmbracelet-powered dashboard monitoring tmux sessions/windows/panes in real-time.

### Architecture

**Three-layer design:**
1. **CLI layer** (`cmd/tmuxwatch/`): Bubble Tea setup, flag parsing
2. **Abstraction layer** (`internal/tmux/`): Thin wrapper executing `tmux list-sessions`, `list-windows`, `list-panes`, `capture-pane`
3. **UI layer** (`internal/ui/`): Bubble Tea model split into: model, update handlers, view components

**Key pattern:** Polls tmux commands at fixed intervals → stitches pane hierarchy → renders live output per session.

### Features Demonstrating Advanced Patterns
- **Hierarchical navigation**: Collapsible sessions/windows with `/` search
- **Keyboard shortcuts**: `z`/`Z` collapse, `ctrl+m` maximize, arrow keys navigate
- **Mouse support**: Built-in Bubble Tea mouse handling
- **Pane focus tracking**: Active pane output highlighted

**No "magic":** Intentionally uses only Bubble Tea + Lip Gloss primitives for explicit, maintainable behavior.

---

## 6. Other Notable Open Source Examples

### Dashboard/Monitoring UIs
- **[gh-dash](https://github.com/dlvhdr/gh-dash)**: GitHub PR/issues dashboard with live updates
- **[SuperFile](https://github.com/MHNightCat/superfile)**: Multi-pane file manager with sidebar + preview
- **[Glow](https://github.com/charmbracelet/glow)**: Markdown renderer (simpler but demonstrates viewport/styling)

### Process Monitoring
- **AWS EKS Node Viewer**: Real-time Kubernetes node dashboard
- **CockroachDB CLI tools**: Database monitoring dashboards

All these projects follow the pattern: composite model → routed updates → Lip Gloss composed view.

---

## 7. Component Selection Matrix

| Need | Bubbles Component | Notes |
|------|------------------|-------|
| Scrollable output (logs, tailing) | `Viewport` | Maintains scroll position, handles viewport sizing automatically |
| Item selection list | `List` | Filterable, cyclic navigation, custom rendering |
| Tabular data | `Table` | Multi-column, sortable (via custom logic) |
| User input (search/filter) | `Textinput` | Single-line, validates as you type |
| Multi-line editing | `Textarea` | For configs, notes, multi-line commands |
| Status feedback | `Spinner` | Lightweight loading indicator |
| Long-running task progress | `Progress` + `Cmd` | Tick incrementally, update via Cmd return |
| File/directory picking | `Filepicker` | Built-in filesystem navigation |
| Complex layout assembly | `lipgloss.Join{Horizontal,Vertical}` | Compose sub-views, handle borders/padding |

---

## 8. Key Gotchas & Patterns

### Layout Calculations
- Subtract 2 from width/height when a pane has a border
- Always account for padding in size calculations
- Use `lipgloss.Width()` and `lipgloss.Height()` to measure rendered strings

### Responsiveness
- Return `nil` from Update if no state change and no async work
- Use `tea.Batch()` to return multiple Cmds concurrently
- Keep Update() logic fast; offload heavy computation to Cmd functions

### Streaming Without Blocking
```go
// Good: Non-blocking channel with Cmd
func (m Model) Init() tea.Cmd {
    go func() {
        for line := range m.stream {
            m.msgChan <- ProcessLineMsg{Line: line}
        }
    }()
    return listenChannel(m.msgChan)
}

// Bad: Raw goroutine modifying model directly
go func() {
    m.buffer = append(m.buffer, newLine)  // RACE CONDITION
}()
```

### Focus & Pane Routing
Track active pane as a field; route tea.KeyMsg to the focused pane's Update(). Use `tea.KeyMsg.Type` for arrow keys, not string matching.

---

## Unresolved Questions

1. **Viewport performance at scale**: How does Viewport handle 100k+ lines of buffered output? (Likely requires pagination or circular buffer strategy)
2. **Synchronized updates across panes**: When one pane fetches data, how to refresh others without explicit re-polling? (Likely requires shared model state + event broadcasting)
3. **Custom Bubbles components**: What's the boundary for forking vs. extending existing components?
4. **Alt-screen buffer mode**: When to use `tea.WithAltScreen()` vs. inline rendering?

---

## Sources

- [Bubbletea GitHub](https://github.com/charmbracelet/bubbletea)
- [Bubbles GitHub](https://github.com/charmbracelet/bubbles)
- [Lip Gloss GitHub](https://github.com/charmbracelet/lipgloss)
- [pkg.go.dev Bubbletea Docs](https://pkg.go.dev/github.com/charmbracelet/bubbletea)
- [Panes Component](https://github.com/john-marinelli/panes)
- [BubbleLayout](https://github.com/winder/bubblelayout)
- [tmuxwatch Project](https://github.com/steipete/tmuxwatch)
- [Commands in Bubble Tea (Blog)](https://charm.land/blog/commands-in-bubbletea/)
- [Building Bubble Tea Programs (Blog)](https://leg100.github.io/en/posts/building-bubbletea-programs/)
- [Terminal IRC Client with Bubble Tea (Blog)](https://sngeth.com/go/terminal/ui/bubble-tea/2025/08/17/building-terminal-ui-with-bubble-tea/)
- [Shift.foo: Multi-View Interfaces](https://shi.foo/weblog/multi-view-interfaces-in-bubble-tea)

