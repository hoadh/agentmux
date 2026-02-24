# Research Report: Building CLI Tools in Go with Cobra and Bubbletea

**Date**: 2026-02-24
**Researcher**: AI Subagent (researcher)
**Status**: Complete

---

## Executive Summary

Go is an exceptional language for CLI development due to single-binary compilation, cross-platform support, excellent performance, and built-in concurrency. **Cobra** provides a command framework for structured, intuitive CLI interfaces. **Bubbletea** (by Charmbracelet) implements the Elm Architecture for building interactive terminal UIs. Together, they enable creating professional CLI tools: simple commands via Cobra, complex interactions via Bubbletea.

---

## 1. Why Go for CLI Tools

### Single Binary Distribution
- **No runtime required**: Compile to a single executable for any OS/architecture
- **Easy installation**: Users `curl | sh` or download from GitHub releases—no dependency hell
- **Fast startup**: Nanosecond initialization vs. Python/Node startup overhead

### Cross-Compilation
```bash
GOOS=linux GOARCH=amd64 go build -o app-linux
GOOS=darwin GOARCH=arm64 go build -o app-mac-m1
GOOS=windows GOARCH=amd64 go build -o app.exe
```

### Performance
- Compiled language: ~100x faster startup than Python/Node
- Minimal memory footprint suitable for container deployments
- Concurrent operations via goroutines (lightweight threads)

### Concurrency as First-Class
```go
// Spawn thousands of goroutines easily
go fetchAPI(url)
go processData(data)
```

---

## 2. Cobra Framework: Command Structure

### Core Concepts

**Commands = Verbs, Args = Things, Flags = Adjectives**

Pattern: `APPNAME VERB NOUN --ADJECTIVE`

Example: `kubectl get pods --namespace=default`

### Basic Command Structure

```go
package main

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
    Use:   "myapp",
    Short: "My awesome CLI tool",
    Long:  "A longer description...",
    Run: func(cmd *cobra.Command, args []string) {
        println("Root command executed")
    },
}

func main() {
    rootCmd.Execute()
}
```

### Subcommands

```go
var deployCmd = &cobra.Command{
    Use:   "deploy",
    Short: "Deploy the application",
    RunE: func(cmd *cobra.Command, args []string) error {
        environment, _ := cmd.Flags().GetString("env")
        // Handle error
        return nil
    },
}

func init() {
    rootCmd.AddCommand(deployCmd)
    deployCmd.Flags().StringP("env", "e", "staging", "Deployment environment")
}
```

### Flags: Global vs Command-Specific

```go
// PersistentFlags: inherited by all subcommands
rootCmd.PersistentFlags().String("config", "", "Config file path")

// Local flags: only for this command
deployCmd.Flags().Bool("force", false, "Force deployment")
```

### Validation with PreRunE/PostRunE

```go
var deployCmd = &cobra.Command{
    Use: "deploy",

    // Validate before execution
    PreRunE: func(cmd *cobra.Command, args []string) error {
        env, _ := cmd.Flags().GetString("env")
        if env == "" {
            return fmt.Errorf("--env flag is required")
        }
        return nil
    },

    // Main logic
    RunE: func(cmd *cobra.Command, args []string) error {
        return deployApp()
    },

    // Cleanup
    PostRunE: func(cmd *cobra.Command, args []string) error {
        return cleanup()
    },
}
```

### Organization Patterns

**Simple Layout (single cmd package)**
```
cmd/
  root.go
  deploy.go
  status.go
main.go
```

**Modular Layout (feature-based packages, recommended for large apps)**
```
cmd/
  deploy/
    cmd.go         // returns *cobra.Command
  status/
    cmd.go
main.go
```

Feature-based structure improves maintainability, parallel development, and compile times.

### Best Practices
- Use `RunE` instead of `Run` for error handling
- Define flags in `init()` functions
- Structure commands as feature packages with constructors: `NewDeployCommand()`
- Test commands with `cmd.Execute()` and capture output
- Use Viper for configuration file + environment variable integration

---

## 3. Bubbletea Framework: Elm Architecture for TUIs

### The Elm Architecture (MVU)

**Three core functions:**

1. **Model**: Struct representing application state
2. **Update(msg Msg)**: Pure function handling messages and state changes
3. **View()**: Render state to terminal (returns string)

### Core Types

```go
// Message: any value sent to Update (keyboard, timer, HTTP response, etc.)
type Msg interface{}

// Command: side effect (API call, timer, keyboard listening)
type Cmd func() Msg

// Model: your application state
type Model interface {
    Init() Cmd                    // Initial setup
    Update(msg Msg) (Model, Cmd) // Handle message
    View() string                 // Render to string
}
```

### Basic Counter Example

```go
package main

import (
    tea "github.com/charmbracelet/bubbletea"
)

type Counter struct {
    value int
}

// Initialize
func (c Counter) Init() tea.Cmd {
    return nil
}

// Handle messages
func (c Counter) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {

    // Keyboard
    case tea.KeyMsg:
        switch msg.String() {
        case "up":
            c.value++
        case "down":
            c.value--
        case "q", "ctrl+c":
            return c, tea.Quit
        }

    // Timer tick
    case TickMsg:
        c.value++
        return c, tick()
    }

    return c, nil
}

// Render
func (c Counter) View() string {
    return fmt.Sprintf("Value: %d\nPress up/down, q to quit", c.value)
}

func main() {
    p := tea.NewProgram(Counter{})
    if _, err := p.Run(); err != nil {
        log.Fatal(err)
    }
}

// Custom message
type TickMsg struct{}

func tick() tea.Cmd {
    return func() tea.Msg {
        time.Sleep(1 * time.Second)
        return TickMsg{}
    }
}
```

### Message Flow (Critical Understanding)

```
User Input
    ↓
Update() → Returns (NewModel, Cmd)
    ↓
If Cmd exists → Execute (blocks if needed)
    ↓
Cmd returns Msg
    ↓
Update() called again with Msg
    ↓
View() renders to terminal
    ↓
Loop repeats
```

**Key insight**: All state changes happen through messages. Predictable, testable, debuggable.

### Common Pattern: Async Operations

```go
type Model struct {
    loading bool
    data    string
    err     error
}

type DataLoaded string
type DataError string

func (m Model) Init() tea.Cmd {
    return fetchData() // Returns DataLoaded or DataError
}

func fetchData() tea.Cmd {
    return func() tea.Msg {
        resp, err := http.Get("https://api.example.com/data")
        if err != nil {
            return DataError(err.Error())
        }
        defer resp.Body.Close()
        data, _ := ioutil.ReadAll(resp.Body)
        return DataLoaded(string(data))
    }
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {

    case DataLoaded:
        m.loading = false
        m.data = string(msg)
        return m, nil

    case DataError:
        m.loading = false
        m.err = fmt.Errorf(string(msg))
        return m, nil
    }

    return m, nil
}
```

---

## 4. Bubbles: Reusable Components

### Provided Components

| Component | Use Case |
|-----------|----------|
| **List** | Browse/select from items, pagination, filtering |
| **TextInput** | Single-line text (forms, queries) |
| **TextArea** | Multi-line text (longer input) |
| **Spinner** | Loading indicator (async operations) |
| **Table** | Columnar data display with scrolling |
| **Viewport** | Scrollable content areas |
| **Paginator** | Navigate large datasets |

### Example: List Component

```go
import "github.com/charmbracelet/bubbles/list"

type Model struct {
    list list.Model
}

func NewModel() Model {
    items := []list.Item{
        list.NewItem("Item 1"),
        list.NewItem("Item 2"),
        list.NewItem("Item 3"),
    }

    l := list.New(items, list.NewDefaultDelegate(), 0, 0)
    l.Title = "Select an item"

    return Model{list: l}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {

    case tea.WindowSizeMsg:
        m.list.SetSize(msg.Width, msg.Height)

    case tea.KeyMsg:
        if msg.String() == "q" {
            return m, tea.Quit
        }
        if msg.String() == "enter" {
            selected := m.list.SelectedItem()
            println("Selected:", selected.String())
        }
    }

    var cmd tea.Cmd
    m.list, cmd = m.list.Update(msg)
    return m, cmd
}

func (m Model) View() string {
    return m.list.View()
}
```

### Example: TextInput Component

```go
import "github.com/charmbracelet/bubbles/textinput"

type FormModel struct {
    username textinput.Model
    password textinput.Model
    focused  int // 0 = username, 1 = password
}

func NewFormModel() FormModel {
    u := textinput.New()
    u.Placeholder = "Enter username"
    u.Focus()

    p := textinput.New()
    p.Placeholder = "Enter password"
    p.EchoMode = textinput.EchoPassword

    return FormModel{
        username: u,
        password: p,
        focused:  0,
    }
}

func (m FormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {

    case tea.KeyMsg:
        switch msg.String() {
        case "tab", "shift+tab":
            m.focused = 1 - m.focused
            if m.focused == 0 {
                m.username.Focus()
                m.password.Blur()
            } else {
                m.username.Blur()
                m.password.Focus()
            }

        case "enter":
            // Submit form
            return m, tea.Quit
        }
    }

    if m.focused == 0 {
        m.username, _ = m.username.Update(msg)
    } else {
        m.password, _ = m.password.Update(msg)
    }

    return m, nil
}

func (m FormModel) View() string {
    return fmt.Sprintf(
        "Username:\n%s\n\nPassword:\n%s",
        m.username.View(),
        m.password.View(),
    )
}
```

---

## 5. Cobra + Bubbletea Integration

### Pattern: Simple Commands → Complex Commands Use TUI

Not all workflows require a TUI. Cobra handles basic operations; Bubbletea launches for interactive tasks.

```
myapp deploy --env staging        # Cobra: quick command
myapp deploy --interactive        # Cobra: launches Bubbletea TUI
myapp select-service              # Bubbletea: selector list
myapp logs -f service-a           # Cobra: streaming output
```

### Implementation Pattern

```go
package deploy

import (
    tea "github.com/charmbracelet/bubbletea"
    "github.com/spf13/cobra"
)

// DeployModel implements tea.Model
type DeployModel struct {
    environment string
    status      string
}

func (m DeployModel) Init() tea.Cmd {
    return runDeploy(m.environment)
}

func (m DeployModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.String() == "q" {
            return m, tea.Quit
        }
    case StatusMsg:
        m.status = string(msg)
    }
    return m, nil
}

func (m DeployModel) View() string {
    return fmt.Sprintf("Deploying to %s...\n%s", m.environment, m.status)
}

// Cobra command
var DeployCmd = &cobra.Command{
    Use:   "deploy",
    Short: "Deploy application",
    RunE: func(cmd *cobra.Command, args []string) error {
        interactive, _ := cmd.Flags().GetBool("interactive")
        env, _ := cmd.Flags().GetString("env")

        if interactive {
            // Launch TUI
            model := DeployModel{environment: env}
            p := tea.NewProgram(model)
            _, err := p.Run()
            return err
        }

        // Simple deploy
        return deployApp(env)
    },
}

func init() {
    DeployCmd.Flags().StringP("env", "e", "staging", "Environment")
    DeployCmd.Flags().BoolP("interactive", "i", false, "Interactive mode")
}

type StatusMsg string

func runDeploy(env string) tea.Cmd {
    return func() tea.Msg {
        // Perform deployment, return updates
        return StatusMsg("Deployment started...")
    }
}
```

### Full Example: Service Selector

```go
package main

import (
    "github.com/charmbracelet/bubbles/list"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/spf13/cobra"
)

type ServiceModel struct {
    list list.Model
}

func (m ServiceModel) Init() tea.Cmd {
    return nil
}

func (m ServiceModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.list.SetSize(msg.Width, msg.Height)
    case tea.KeyMsg:
        if msg.String() == "q" {
            return m, tea.Quit
        }
        if msg.String() == "enter" {
            selected := m.list.SelectedItem()
            // Handle selection
            return m, tea.Quit
        }
    }
    var cmd tea.Cmd
    m.list, cmd = m.list.Update(msg)
    return m, cmd
}

func (m ServiceModel) View() string {
    return m.list.View()
}

var selectCmd = &cobra.Command{
    Use:   "select",
    Short: "Select a service",
    RunE: func(cmd *cobra.Command, args []string) error {
        items := []list.Item{
            list.NewItem("Service A"),
            list.NewItem("Service B"),
            list.NewItem("Service C"),
        }

        l := list.New(items, list.NewDefaultDelegate(), 20, 10)
        m := ServiceModel{list: l}

        p := tea.NewProgram(m)
        _, err := p.Run()
        return err
    },
}
```

---

## 6. Lipgloss: Styling Terminal Output

### Core Concepts

Lipgloss provides **declarative, CSS-like styling** for terminal text.

### Basic Styling

```go
import "github.com/charmbracelet/lipgloss"

// Define styles
var (
    titleStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("205")).
        Bold(true).
        Padding(1, 2)

    boxStyle = lipgloss.NewStyle().
        Border(lipgloss.NormalBorder()).
        BorderForeground(lipgloss.Color("63")).
        Padding(1)
)

// Use styles
func (m Model) View() string {
    title := titleStyle.Render("My App")
    content := boxStyle.Render("Content here")
    return lipgloss.JoinVertical(lipgloss.Center, title, content)
}
```

### Colors

```go
// Named colors
lipgloss.Color("red")       // ANSI color names
lipgloss.Color("205")       // ANSI 0-255 palette

// Hex colors (auto-degrades based on terminal capability)
lipgloss.Color("#FF5F00")

// Adaptive colors (light/dark theme detection)
lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
    Light:  "#EFF0EB",
    Dark:   "#000000",
})
```

### Layout Helpers

```go
// Stack vertically (center alignment)
lipgloss.JoinVertical(lipgloss.Center, "Line 1", "Line 2")

// Stack horizontally (top alignment)
lipgloss.JoinHorizontal(lipgloss.Top, "Left", "Right")

// Create grid
lipgloss.Place(20, 10, lipgloss.Center, lipgloss.Center, "Centered")
```

### Integration with Bubbletea

```go
import (
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
)

type Model struct {
    items []string
}

func (m Model) View() string {
    header := lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("99")).
        Render("Items")

    var items []string
    for _, item := range m.items {
        styledItem := lipgloss.NewStyle().
            PaddingLeft(2).
            Render(item)
        items = append(items, styledItem)
    }

    itemList := lipgloss.JoinVertical(lipgloss.Left, items...)

    return lipgloss.JoinVertical(lipgloss.Left,
        header,
        lipgloss.NewStyle().Margin(1, 0).Render(itemList),
    )
}
```

### Color Degradation (Key Feature)

Lipgloss **automatically**:
- Detects terminal color capability
- Downgrades hex colors to nearest ANSI colors if needed
- Removes color entirely for non-TTY (pipes, CI, logs)
- Supports light/dark theme detection

---

## 7. Common Mistakes Beginners Make

### Mistake 1: Ignoring Message Flow
**Problem**: Updating state directly in View instead of through Update.

```go
// ❌ WRONG
func (m Model) View() string {
    m.counter++  // State change in View!
    return fmt.Sprintf("Count: %d", m.counter)
}

// ✅ CORRECT
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.String() == "up" {
            m.counter++
        }
    }
    return m, nil
}
```

### Mistake 2: Blocking Operations in Update
**Problem**: Long-running operations block the entire UI.

```go
// ❌ WRONG
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    resp, _ := http.Get("https://...")  // Blocks!
    m.data = resp
    return m, nil
}

// ✅ CORRECT
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    return m, fetchData()  // Returns Cmd
}

func fetchData() tea.Cmd {
    return func() tea.Msg {
        resp, _ := http.Get("https://...")
        return DataLoaded(resp)
    }
}
```

### Mistake 3: Not Handling WindowSizeMsg
**Problem**: UI breaks on terminal resize.

```go
// ❌ INCOMPLETE
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // Missing WindowSizeMsg handling!
    return m, nil
}

// ✅ CORRECT
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        m.list.SetSize(msg.Width, msg.Height)
    }
    return m, nil
}
```

### Mistake 4: Monolithic View
**Problem**: 500-line View() method becomes unmaintainable.

```go
// ❌ WRONG: Everything in one function

// ✅ CORRECT: Decompose into helpers
func (m Model) View() string {
    return lipgloss.JoinVertical(lipgloss.Left,
        m.renderHeader(),
        m.renderContent(),
        m.renderFooter(),
    )
}

func (m Model) renderHeader() string {
    // ...
}
```

### Mistake 5: Logging to Stdout
**Problem**: Bubbletea takes over the terminal; logs disappear or break UI.

```go
// ❌ WRONG
fmt.Println("Debug info")  // Corrupts TUI display

// ✅ CORRECT
f, _ := os.Create("/tmp/app.log")
defer f.Close()
log.SetOutput(f)
log.Println("Debug info")
```

### Mistake 6: Not Composing Models
**Problem**: Trying to fit all logic in one Model.

```go
// ❌ WRONG: Monolithic
type Model struct {
    mode string
    // ...200 fields...
}

// ✅ CORRECT: Composed models
type Model struct {
    form    FormModel
    list    ListModel
    detail  DetailModel
    active  string  // "form" | "list" | "detail"
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch m.active {
    case "form":
        updated, cmd := m.form.Update(msg)
        m.form = updated.(FormModel)
        return m, cmd
    case "list":
        updated, cmd := m.list.Update(msg)
        m.list = updated.(ListModel)
        return m, cmd
    }
    return m, nil
}
```

### Mistake 7: Cobra String Type Assertions
**Problem**: Not properly getting flag values.

```go
// ❌ WRONG
env := cmd.Flag("env")  // Returns *pflag.Flag, not string!

// ✅ CORRECT
env, _ := cmd.Flags().GetString("env")
count, _ := cmd.Flags().GetInt("count")
force, _ := cmd.Flags().GetBool("force")
```

### Mistake 8: Missing Error Handling in RunE
**Problem**: Errors silently ignored.

```go
// ❌ WRONG
var cmd = &cobra.Command{
    Run: func(cmd *cobra.Command, args []string) {
        deployApp()  // Errors ignored!
    },
}

// ✅ CORRECT
var cmd = &cobra.Command{
    RunE: func(cmd *cobra.Command, args []string) error {
        if err := validateFlags(cmd); err != nil {
            return fmt.Errorf("validation failed: %w", err)
        }
        return deployApp()
    },
}
```

---

## 8. Code Patterns Worth Highlighting

### Pattern 1: Reusable Spinner Component

```go
type AsyncModel struct {
    spinner spinner.Model
    data    interface{}
    done    bool
}

func NewAsyncModel(task func() interface{}) AsyncModel {
    s := spinner.New()
    s.Spinner = spinner.Dot

    return AsyncModel{
        spinner: s,
        done:    false,
    }
}

func (m AsyncModel) Init() tea.Cmd {
    return tea.Batch(
        m.spinner.Tick,
        executeTask(),  // Returns DataMsg
    )
}

func (m AsyncModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case spinner.TickMsg:
        if !m.done {
            var cmd tea.Cmd
            m.spinner, cmd = m.spinner.Update(msg)
            return m, cmd
        }
    case DataMsg:
        m.data = msg
        m.done = true
        return m, nil
    }
    return m, nil
}

func (m AsyncModel) View() string {
    if m.done {
        return fmt.Sprintf("Done! Result: %v", m.data)
    }
    return m.spinner.View() + " Loading..."
}
```

### Pattern 2: Parent-Child Model Delegation

```go
type MainModel struct {
    list   list.Model
    detail detailModel
    mode   string  // "list" | "detail"
}

// Delegates keyboard input to active child
func (m MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {

    case tea.KeyMsg:
        if msg.String() == "q" {
            return m, tea.Quit
        }
        if msg.String() == "enter" && m.mode == "list" {
            m.mode = "detail"
            item := m.list.SelectedItem().(ServiceItem)
            m.detail = NewDetailModel(item)
        }
        if msg.String() == "esc" && m.mode == "detail" {
            m.mode = "list"
        }

    case tea.WindowSizeMsg:
        m.list.SetSize(msg.Width, msg.Height)
        m.detail.SetSize(msg.Width, msg.Height)
    }

    // Route to active child
    if m.mode == "list" {
        var cmd tea.Cmd
        m.list, cmd = m.list.Update(msg)
        return m, cmd
    } else {
        var cmd tea.Cmd
        m.detail, cmd = m.detail.Update(msg)
        return m, cmd
    }
}

func (m MainModel) View() string {
    if m.mode == "detail" {
        return m.detail.View()
    }
    return m.list.View()
}
```

### Pattern 3: Viper + Cobra Configuration

```go
import (
    "github.com/spf13/cobra"
    "github.com/spf13/viper"
)

func init() {
    // Bind flag to Viper
    rootCmd.PersistentFlags().String("config", "", "Config file")
    viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))

    // Environment variables
    viper.SetEnvPrefix("MYAPP")
    viper.AutomaticEnv()

    // Defaults
    viper.SetDefault("environment", "dev")
    viper.SetDefault("port", 8080)
}

var deployCmd = &cobra.Command{
    Use: "deploy",
    RunE: func(cmd *cobra.Command, args []string) error {
        env := viper.GetString("environment")
        port := viper.GetInt("port")

        // Priority: Flag > Env > Config > Default
        return deploy(env, port)
    },
}
```

### Pattern 4: Command Registration with Init Funcs

```go
// In cmd/deploy/deploy.go
func NewCommand() *cobra.Command {
    return &cobra.Command{
        Use:   "deploy",
        Short: "Deploy service",
        RunE:  runDeploy,
    }
}

func init() {
    // Avoid global state; flags set in NewCommand
}

// In main.go
import "myapp/cmd/deploy"

func main() {
    rootCmd.AddCommand(deploy.NewCommand())
    rootCmd.Execute()
}
```

---

## 9. Project Structure Reference

### Recommended Layout

```
myapp/
├── main.go
├── cmd/
│   ├── root.go                  # rootCmd definition
│   ├── deploy/
│   │   └── cmd.go              # NewCommand() returns *cobra.Command
│   ├── status/
│   │   └── cmd.go
│   └── init.go                  # AddCommand() calls
├── internal/
│   ├── deploy/
│   │   ├── service.go           # Business logic
│   │   └── models.go
│   ├── api/
│   │   ├── client.go
│   │   └── types.go
│   └── tui/
│       ├── deploy_model.go      # Bubbletea models
│       └── selector_model.go
├── go.mod
├── go.sum
└── README.md
```

### Example: cmd/deploy/cmd.go

```go
package deploy

import (
    "fmt"
    "github.com/spf13/cobra"
    "myapp/internal/deploy"
    "myapp/internal/tui"
)

func NewCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "deploy <service>",
        Short: "Deploy a service",
        Args:  cobra.ExactArgs(1),
        RunE:  runDeploy,
    }

    cmd.Flags().StringP("env", "e", "staging", "Environment")
    cmd.Flags().BoolP("interactive", "i", false, "Interactive TUI")

    return cmd
}

func runDeploy(cmd *cobra.Command, args []string) error {
    service := args[0]
    interactive, _ := cmd.Flags().GetBool("interactive")
    env, _ := cmd.Flags().GetString("env")

    if interactive {
        model := tui.NewDeployModel(service, env)
        return tui.RunProgram(model)
    }

    return deploy.Deploy(service, env)
}
```

---

## 10. Real-World Example: Deployment CLI

### Overview
A CLI tool for deploying services with:
- Non-interactive mode: `myapp deploy staging backend`
- Interactive mode: `myapp deploy --interactive` (list selection → deploy)
- Status command: `myapp status`

### File: main.go

```go
package main

import (
    "log"
    "myapp/cmd"
)

func main() {
    if err := cmd.Execute(); err != nil {
        log.Fatal(err)
    }
}
```

### File: cmd/root.go

```go
package cmd

import (
    "github.com/spf13/cobra"
    "myapp/cmd/deploy"
    "myapp/cmd/status"
)

var rootCmd = &cobra.Command{
    Use:   "myapp",
    Short: "Deployment management tool",
    Long:  "A tool for managing deployments across services",
}

func init() {
    rootCmd.AddCommand(deploy.NewCommand())
    rootCmd.AddCommand(status.NewCommand())

    rootCmd.PersistentFlags().StringP("config", "c", "", "Config file path")
}

func Execute() error {
    return rootCmd.Execute()
}
```

### File: cmd/deploy/cmd.go

```go
package deploy

import (
    "fmt"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/spf13/cobra"
    "myapp/internal/deploy"
    "myapp/internal/tui"
)

func NewCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "deploy [service]",
        Short: "Deploy a service",
        RunE:  runDeploy,
    }

    cmd.Flags().StringP("env", "e", "staging", "Environment (staging|prod)")
    cmd.Flags().BoolP("interactive", "i", false, "Interactive mode")
    cmd.Flags().Bool("dry-run", false, "Dry run (no actual deployment)")

    return cmd
}

func runDeploy(cmd *cobra.Command, args []string) error {
    interactive, _ := cmd.Flags().GetBool("interactive")
    env, _ := cmd.Flags().GetString("env")
    dryRun, _ := cmd.Flags().GetBool("dry-run")

    if interactive {
        // Launch TUI service selector
        m := tui.NewServiceSelectorModel(env, dryRun)
        p := tea.NewProgram(m)
        _, err := p.Run()
        return err
    }

    if len(args) == 0 {
        return fmt.Errorf("service name required")
    }

    service := args[0]
    return deploy.DeployService(service, env, dryRun)
}
```

### File: internal/tui/selector.go

```go
package tui

import (
    "fmt"
    "github.com/charmbracelet/bubbles/list"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "myapp/internal/deploy"
)

type ServiceSelectorModel struct {
    list      list.Model
    env       string
    dryRun    bool
    deployed  string
    error     string
}

type DeployedMsg string
type ErrorMsg string

func NewServiceSelectorModel(env string, dryRun bool) ServiceSelectorModel {
    services := []list.Item{
        list.NewItem("backend"),
        list.NewItem("frontend"),
        list.NewItem("api-gateway"),
    }

    l := list.New(services, list.NewDefaultDelegate(), 30, 10)
    l.Title = "Select service to deploy"

    return ServiceSelectorModel{
        list:   l,
        env:    env,
        dryRun: dryRun,
    }
}

func (m ServiceSelectorModel) Init() tea.Cmd {
    return nil
}

func (m ServiceSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {

    case tea.KeyMsg:
        switch msg.String() {
        case "q", "ctrl+c":
            return m, tea.Quit
        case "enter":
            service := m.list.SelectedItem().(list.Item).String()
            return m, deployService(service, m.env, m.dryRun)
        }

    case tea.WindowSizeMsg:
        m.list.SetSize(msg.Width, msg.Height)

    case DeployedMsg:
        m.deployed = string(msg)
        return m, tea.Quit

    case ErrorMsg:
        m.error = string(msg)
    }

    var cmd tea.Cmd
    m.list, cmd = m.list.Update(msg)
    return m, cmd
}

func (m ServiceSelectorModel) View() string {
    if m.deployed != "" {
        return lipgloss.NewStyle().
            Foreground(lipgloss.Color("2")).
            Bold(true).
            Render(fmt.Sprintf("✓ Deployed: %s", m.deployed))
    }

    if m.error != "" {
        return lipgloss.NewStyle().
            Foreground(lipgloss.Color("1")).
            Render(fmt.Sprintf("✗ Error: %s", m.error))
    }

    return m.list.View()
}

func deployService(service, env string, dryRun bool) tea.Cmd {
    return func() tea.Msg {
        if err := deploy.DeployService(service, env, dryRun); err != nil {
            return ErrorMsg(err.Error())
        }
        return DeployedMsg(fmt.Sprintf("%s to %s", service, env))
    }
}
```

---

## 11. Resources & Links

### Official Documentation
- [Cobra: A Commander for Modern CLI Apps](https://cobra.dev/)
- [Bubbletea Package Documentation](https://pkg.go.dev/github.com/charmbracelet/bubbletea)
- [Bubbles: TUI Components](https://github.com/charmbracelet/bubbles)
- [Lipgloss: Terminal Styling](https://github.com/charmbracelet/lipgloss)

### Tutorials & Guides
- [Charming Cobras with Bubbletea - Part 1](https://elewis.dev/charming-cobras-with-bubbletea-part-1)
- [Rapidly Building Interactive CLIs in Go with Bubbletea](https://www.inngest.com/blog/interactive-clis-with-bubbletea)
- [Intro to Bubble Tea in Go - DEV Community](https://dev.to/andyhaskell/intro-to-bubble-tea-in-go-21lg)
- [An Introduction to Charmbracelet/Bubble Tea](https://typevar.dev/articles/charmbracelet/bubbletea)
- [Go Cobra CLI Tutorial: Build a Task Manager from Scratch](https://medium.com/@sajo02/go-cobra-cli-tutorial-build-a-task-manager-from-scratch-0ad6cec11f01)
- [Creating a CLI in Go Using Cobra - JetBrains Guide](https://www.jetbrains.com/guide/go/tutorials/cli-apps-go-cobra/creating_cli/)
- [Building an Awesome Terminal UI with Go, Bubble Tea, and Lipgloss](https://www.grootan.com/blogs/building-an-awesome-terminal-user-interface-using-go-bubble-tea-and-lip-gloss/)
- [Bubble Tea: Architecture Patterns & Key Lessons](https://alexho.dev/post/bubbletea/)
- [Terminal Applications in Go](https://harrisoncramer.me/terminal-applications-in-go/)

### GitHub Examples
- [Bubbletea Official Examples](https://github.com/charmbracelet/bubbletea/tree/main/examples)
- [Bubble Shell: Interactive Shell Library](https://github.com/DomBlack/bubble-shell)
- [BOA: Cobra-styled Usage Component](https://github.com/elewis787/boa)

### Key Repositories
- [Charmbracelet Bubbletea](https://github.com/charmbracelet/bubbletea)
- [Charmbracelet Bubbles](https://github.com/charmbracelet/bubbles)
- [Charmbracelet Lipgloss](https://github.com/charmbracelet/lipgloss)
- [Spf13 Cobra](https://github.com/spf13/cobra)

---

## 12. Quick Reference: When to Use What

| Scenario | Best Tool |
|----------|-----------|
| Simple command (one-shot action) | Cobra alone |
| Multiple subcommands with flags | Cobra framework |
| User selection from list | Bubbles list + Cobra |
| Form input/validation | Bubbles textinput + Cobra |
| Real-time progress display | Bubbles spinner + Cobra |
| Complex interactive workflow | Bubbletea full program |
| Styling terminal output | Lipgloss |
| Configuration management | Cobra + Viper |

---

## Summary

**Go CLI development shines** because of:
- Single-binary cross-compilation
- Excellent command frameworks (Cobra)
- Powerful TUI libraries (Bubbletea, Bubbles, Lipgloss)
- Fast performance and low memory footprint

**The stack**:
1. **Cobra**: Command structure, flags, subcommands, help generation
2. **Bubbletea**: Elm Architecture for interactive TUIs (Model/Update/View)
3. **Bubbles**: Pre-built components (List, TextInput, Spinner, Table)
4. **Lipgloss**: CSS-like styling with auto color-degradation
5. **Viper**: Configuration file + environment variable management

**Integration pattern**: Use Cobra for command dispatch; launch Bubbletea programs for interactive workflows.

**Key insight**: Understand message flow in Bubbletea—all state changes through Update(), View() renders, no side effects in View().

---

## Unresolved Questions

- **Testing Bubbletea models**: More examples of mocking tea.Msg and testing Update/View in isolation would be valuable
- **Performance optimization**: Large lists in Bubbletea (10k+ items)—best practices for virtualization
- **Advanced layouts**: Complex dashboard layouts combining multiple Bubbletea components with responsive grid layouts
- **Remote TUI**: Can Bubbletea programs run over SSH/TTY in remote containers effectively?
- **Accessibility**: Screen reader support for TUI apps built with Bubbletea

---

**Report Compiled**: 2026-02-24 18:54 UTC
**Researcher**: AI Subagent
**Status**: Ready for implementation planning
