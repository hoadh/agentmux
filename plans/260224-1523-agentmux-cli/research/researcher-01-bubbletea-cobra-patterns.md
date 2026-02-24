# Bubbletea + Cobra Patterns Research

Date: 2026-02-24 | Sources: charmbracelet/bubbletea, charmbracelet/bubbles docs

---

## 1. Composite Model Pattern (Sidebar + Detail + Statusbar)

Key principle: root model owns focus state, delegates Update/View to active sub-model.

```go
type panel int
const (
    sidebarPanel panel = iota
    detailPanel
)

type model struct {
    sidebar   list.Model
    detail    viewport.Model
    status    string
    focus     panel
    width     int
    height    int
    ready     bool
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmds []tea.Cmd

    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width, m.height = msg.Width, msg.Height
        sideW := 30
        m.sidebar.SetSize(sideW, m.height-2)
        if !m.ready {
            m.detail = viewport.New(m.width-sideW, m.height-2)
            m.ready = true
        } else {
            m.detail.Width = m.width - sideW
            m.detail.Height = m.height - 2
        }

    case tea.KeyMsg:
        switch msg.String() {
        case "tab":
            if m.focus == sidebarPanel {
                m.focus = detailPanel
            } else {
                m.focus = sidebarPanel
            }
            return m, nil
        case "ctrl+c", "q":
            return m, tea.Quit
        }
    }

    // Delegate to focused panel
    var cmd tea.Cmd
    switch m.focus {
    case sidebarPanel:
        m.sidebar, cmd = m.sidebar.Update(msg)
        cmds = append(cmds, cmd)
        // Sync selection → detail content
        if sel, ok := m.sidebar.SelectedItem().(myItem); ok {
            m.detail.SetContent(sel.content)
        }
    case detailPanel:
        m.detail, cmd = m.detail.Update(msg)
        cmds = append(cmds, cmd)
    }

    return m, tea.Batch(cmds...)
}

func (m model) View() string {
    if !m.ready {
        return "Initializing..."
    }
    sideView := lipgloss.NewStyle().Width(30).Render(m.sidebar.View())
    detailView := m.detail.View()
    body := lipgloss.JoinHorizontal(lipgloss.Top, sideView, detailView)
    statusBar := lipgloss.NewStyle().
        Width(m.width).Background(lipgloss.Color("62")).
        Render(m.status)
    return lipgloss.JoinVertical(lipgloss.Left, body, statusBar)
}
```

Focus indicator: apply a border style to the active panel in View().

---

## 2. Cobra + Bubbletea Integration

Pattern: Cobra parses flags → builds config struct → passes to tea.NewProgram.

```go
// main.go
func main() {
    rootCmd.Execute()
}

// cmd/run.go
var runCmd = &cobra.Command{
    Use:   "run",
    Short: "Launch TUI dashboard",
    RunE:  runTUI,
}

type Config struct {
    Namespace string
    Follow    bool
    Verbose   bool
}

func init() {
    runCmd.Flags().StringP("namespace", "n", "default", "k8s namespace")
    runCmd.Flags().BoolP("follow", "f", false, "follow log output")
    runCmd.Flags().BoolP("verbose", "v", false, "verbose output")
    rootCmd.AddCommand(runCmd)
}

func runTUI(cmd *cobra.Command, args []string) error {
    ns, _ := cmd.Flags().GetString("namespace")
    follow, _ := cmd.Flags().GetBool("follow")
    verbose, _ := cmd.Flags().GetBool("verbose")

    cfg := Config{Namespace: ns, Follow: follow, Verbose: verbose}
    m := newModel(cfg)

    p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
    _, err := p.Run()
    return err
}
```

For subcommands that don't need TUI (e.g. `logs --json`), check a flag first:
```go
func runTUI(cmd *cobra.Command, args []string) error {
    if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
        return runPlainOutput(cmd, args) // no TUI
    }
    // ... launch TUI
}
```

---

## 3. Streaming Data into Bubbletea (goroutine → tea.Cmd)

Pattern: goroutine reads io.Reader, sends typed tea.Msg via a channel, tea.Cmd polls channel.

```go
// Message types
type logLineMsg struct {
    source string
    line   string
}
type streamErrMsg struct{ err error }
type streamDoneMsg struct{ source string }

// Command: spawns goroutine, returns a Cmd that waits for next message
func streamLogs(source string, r io.Reader) tea.Cmd {
    ch := make(chan tea.Msg, 64)
    go func() {
        scanner := bufio.NewScanner(r)
        for scanner.Scan() {
            ch <- logLineMsg{source: source, line: scanner.Text()}
        }
        if err := scanner.Err(); err != nil {
            ch <- streamErrMsg{err: err}
        }
        ch <- streamDoneMsg{source: source}
        close(ch)
    }()
    return waitForMsg(ch)
}

// Recursive Cmd: re-schedules itself after each message to keep draining
func waitForMsg(ch <-chan tea.Msg) tea.Cmd {
    return func() tea.Msg {
        return <-ch
    }
}

// In model.Update:
case logLineMsg:
    m.logs[msg.source] = append(m.logs[msg.source], msg.line)
    m.viewport.SetContent(strings.Join(m.logs[msg.source], "\n"))
    if m.autoScroll {
        m.viewport.GotoBottom()
    }
    return m, waitForMsg(m.logCh) // re-arm

// Multiple concurrent streams: batch initial commands
func (m model) Init() tea.Cmd {
    return tea.Batch(
        streamLogs("stdout", m.stdoutPipe),
        streamLogs("stderr", m.stderrPipe),
    )
}
```

Key: each stream gets its own channel; `waitForMsg` is a blocking Cmd that unblocks on next item, then Update re-arms it. This is the canonical Bubbletea streaming pattern.

---

## 4. Viewport Auto-Scroll with Manual Freeze

Pattern: track `autoScroll bool` flag; any manual scroll key disables it; new content re-enables option.

```go
type model struct {
    viewport   viewport.Model
    autoScroll bool
    lines      []string
}

func newModel() model {
    vp := viewport.New(80, 24)
    vp.KeyMap = viewport.DefaultKeyMap() // pgup/pgdn/up/down built in
    return model{viewport: vp, autoScroll: true}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmd tea.Cmd

    switch msg := msg.(type) {
    case logLineMsg:
        m.lines = append(m.lines, msg.line)
        m.viewport.SetContent(strings.Join(m.lines, "\n"))
        if m.autoScroll {
            m.viewport.GotoBottom()
        }
        return m, waitForMsg(m.logCh)

    case tea.KeyMsg:
        switch msg.String() {
        case "G", "end":          // explicit go-to-bottom: re-enable
            m.autoScroll = true
            m.viewport.GotoBottom()
            return m, nil
        case "up", "k", "pgup":   // manual scroll up: freeze
            m.autoScroll = false
        }
    }

    // Always pass remaining keys to viewport for scrolling
    prevOffset := m.viewport.YOffset
    m.viewport, cmd = m.viewport.Update(msg)
    // If user scrolled at all, freeze auto-scroll
    if m.viewport.YOffset != prevOffset && m.viewport.YOffset < m.viewport.TotalLineCount()-m.viewport.Height {
        m.autoScroll = false
    }
    // If scrolled to bottom manually, re-enable
    if m.viewport.AtBottom() {
        m.autoScroll = true
    }

    return m, cmd
}

func (m model) View() string {
    scrollIndicator := ""
    if !m.autoScroll {
        scrollIndicator = " [PAUSED - G to resume]"
    }
    footer := fmt.Sprintf("%3.f%%%s", m.viewport.ScrollPercent()*100, scrollIndicator)
    return fmt.Sprintf("%s\n%s", m.viewport.View(), footer)
}
```

`viewport.AtBottom()` returns true when YOffset is at max — use it to auto-re-enable follow mode.

---

## Summary Table

| Pattern | Key API | Gotcha |
|---|---|---|
| Multi-panel focus | `m.focus` field + delegate Update | Always pass WindowSizeMsg to ALL panels |
| Cobra → BubbleTea | `cmd.Flags().GetX()` → struct → `newModel(cfg)` | Call `p.Run()` inside `RunE`, return err |
| Streaming goroutine | blocking `tea.Cmd` + `waitForMsg(ch)` re-arm | Buffer channels (64+) to avoid goroutine stalls |
| Auto-scroll viewport | `viewport.GotoBottom()` + `AtBottom()` flag | Track `autoScroll` bool; `G`/End re-enables |

---

## Unresolved Questions

1. `viewport.AtBottom()` — confirm method exists in current bubbles v0.20+; may need `m.viewport.YOffset >= m.viewport.TotalLineCount()-m.viewport.Height`.
2. For very high-frequency streams (>1000 lines/sec), consider batching lines before `SetContent` to reduce re-render thrashing.
3. Cobra `PersistentPreRunE` vs `RunE` for TUI launch — if root-level flags (e.g. `--config`) must be parsed before subcommand model init, use `PersistentPreRunE` on root.
