# Research: DAG Scheduling & Process Management in Go
Date: 2026-02-24

## 1. DAG Topological Sort (Kahn's Algorithm)

**Data structure:** adjacency list + in-degree map.

```go
type DAG struct {
    deps   map[string][]string // node -> dependencies
    rdeps  map[string][]string // node -> dependents (reverse)
    inDeg  map[string]int
}

func TopoSort(nodes []string, deps map[string][]string) ([][]string, error) {
    inDeg := make(map[string]int, len(nodes))
    rdeps := make(map[string][]string)
    for _, n := range nodes { inDeg[n] = 0 }
    for n, ds := range deps {
        for _, d := range ds {
            inDeg[n]++        // n depends on d
            rdeps[d] = append(rdeps[d], n)
        }
    }
    // Kahn's: queue nodes with inDeg==0
    var queue []string
    for _, n := range nodes {
        if inDeg[n] == 0 { queue = append(queue, n) }
    }
    var levels [][]string
    for len(queue) > 0 {
        levels = append(levels, append([]string{}, queue...)) // current wave = parallel batch
        next := []string{}
        for _, n := range queue {
            for _, dep := range rdeps[n] {
                inDeg[dep]--
                if inDeg[dep] == 0 { next = append(next, dep) }
            }
        }
        queue = next
    }
    total := 0
    for _, l := range levels { total += len(l) }
    if total != len(nodes) { return nil, errors.New("cycle detected") }
    return levels, nil
}
```

**Parallel execution:** each `level` from `TopoSort` is a batch of independent nodes; fan-out with `sync.WaitGroup`:

```go
for _, batch := range levels {
    var wg sync.WaitGroup
    for _, node := range batch {
        wg.Add(1)
        go func(n string) { defer wg.Done(); runTask(n) }(node)
    }
    wg.Wait() // block until entire batch finishes
}
```

---

## 2. os/exec Pipe Management

**Key rules:**
- Call `StdoutPipe`/`StderrPipe` BEFORE `Start()`.
- Read pipes in goroutines to avoid deadlock (pipe buffer fills → process blocks).
- Call `Wait()` only after all readers are done (pipes are EOF'd by Wait internally).
- Close stdin pipe explicitly if used.

```go
func runProcess(ctx context.Context, name string, args []string, out io.Writer) error {
    cmd := exec.CommandContext(ctx, name, args...)
    stdout, _ := cmd.StdoutPipe()
    stderr, _ := cmd.StderrPipe()

    if err := cmd.Start(); err != nil { return err }

    var wg sync.WaitGroup
    wg.Add(2)
    go func() { defer wg.Done(); io.Copy(out, stdout) }()
    go func() { defer wg.Done(); io.Copy(out, stderr) }()
    wg.Wait()
    return cmd.Wait()
}
```

**Graceful shutdown (SIGTERM → SIGKILL):**

```go
func shutdown(cmd *exec.Cmd) error {
    if cmd.Process == nil { return nil }
    cmd.Process.Signal(syscall.SIGTERM)
    done := make(chan error, 1)
    go func() { done <- cmd.Wait() }()
    select {
    case err := <-done: return err
    case <-time.After(5 * time.Second):
        cmd.Process.Kill()
        return <-done
    }
}
```

**Avoid deadlock checklist:**
- Never use `cmd.Output()` / `cmd.CombinedOutput()` for long-lived processes.
- Always drain pipes in goroutines before `Wait()`.
- Use `exec.CommandContext` so cancellation sends SIGKILL automatically (supplement with manual SIGTERM).

---

## 3. NDJSON Streaming Parser

`json.Decoder` is the canonical approach — it handles partial reads and buffer management internally.

```go
type Event struct {
    Type    string          `json:"type"`
    Payload json.RawMessage `json:"payload"`
}

func parseNDJSON(r io.Reader, handler func(Event) error) error {
    dec := json.NewDecoder(r)
    for {
        var evt Event
        if err := dec.Decode(&evt); err != nil {
            if errors.Is(err, io.EOF) { return nil }
            // Skip malformed line: use dec.Buffered() + discard to next newline
            // In practice: log and continue
            continue
        }
        if err := handler(evt); err != nil { return err }
    }
}
```

**Type-switching on parsed events:**

```go
func dispatchEvent(evt Event) error {
    switch evt.Type {
    case "tool_use":
        var p ToolUsePayload
        return json.Unmarshal(evt.Payload, &p)
    case "text":
        var p TextPayload
        return json.Unmarshal(evt.Payload, &p)
    default:
        return nil // unknown events: ignore
    }
}
```

**Piping process stdout into NDJSON parser:**

```go
stdout, _ := cmd.StdoutPipe()
cmd.Start()
go parseNDJSON(stdout, dispatchEvent) // goroutine drains pipe
cmd.Wait()
```

**Malformed line recovery** — reset decoder on syntax errors:

```go
if _, ok := err.(*json.SyntaxError); ok {
    dec = json.NewDecoder(dec.Buffered()) // reuse buffered unread data
    // or: discard rest of line via bufio.Scanner wrapper
}
```

---

## 4. Viper YAML Config with Go Structs

**Struct definition** (use `mapstructure` tags, which Viper uses internally):

```go
type AgentConfig struct {
    Name         string   `mapstructure:"name"`
    Prompt       string   `mapstructure:"prompt"`
    DependsOn    []string `mapstructure:"depends_on"`
    AllowedTools []string `mapstructure:"allowedTools"`
    Model        string   `mapstructure:"model"`
}

type Config struct {
    Agents   []AgentConfig `mapstructure:"agents"`
    LogLevel string        `mapstructure:"log_level"`
}
```

**Loading with defaults + file override:**

```go
func LoadConfig(path string) (*Config, error) {
    v := viper.New()
    // Defaults
    v.SetDefault("log_level", "info")
    v.SetDefault("agents", []AgentConfig{})

    v.SetConfigFile(path)
    v.SetConfigType("yaml")
    if err := v.ReadInConfig(); err != nil {
        if !errors.As(err, &viper.ConfigFileNotFoundError{}) { return nil, err }
    }
    v.AutomaticEnv() // env vars override

    var cfg Config
    if err := v.Unmarshal(&cfg); err != nil { return nil, err }
    return &cfg, validate(&cfg)
}
```

**Validation pattern** (manual, no extra deps):

```go
func validate(cfg *Config) error {
    names := make(map[string]bool)
    for _, a := range cfg.Agents {
        if a.Name == "" { return fmt.Errorf("agent missing name") }
        if names[a.Name] { return fmt.Errorf("duplicate agent: %s", a.Name) }
        names[a.Name] = true
    }
    for _, a := range cfg.Agents {
        for _, dep := range a.DependsOn {
            if !names[dep] { return fmt.Errorf("agent %s: unknown dep %s", a.Name, dep) }
        }
    }
    return nil
}
```

**Sample YAML:**

```yaml
log_level: debug
agents:
  - name: planner
    prompt: "You are a planner..."
    depends_on: []
    allowedTools: [Read, Write, Glob]
  - name: coder
    prompt: "Implement the plan"
    depends_on: [planner]
    allowedTools: [Read, Write, Edit, Bash]
```

---

## Key Insights

- **DAG levels** from Kahn's map directly to parallel WaitGroup batches — no extra sync primitives needed.
- **Pipe deadlock** is the #1 pitfall with `os/exec`; always goroutine-drain both stdout AND stderr.
- **`json.Decoder`** handles streaming natively; no need for `bufio.Scanner` + `json.Unmarshal` per line (but Scanner works fine for simple cases).
- **Viper** uses `mapstructure` under the hood; ensure struct tags match YAML keys exactly (case-sensitive after `mapstructure` decode).
- `exec.CommandContext` sends SIGKILL on ctx cancel; for SIGTERM-first behavior, manage lifecycle manually.

## Unresolved Questions

- Whether Claude CLI subprocess emits NDJSON or SSE on stdout (affects parser choice).
- Viper vs plain `os.ReadFile` + `yaml.Unmarshal` — for simple configs Viper may be overkill (KISS).
- Whether DAG nodes represent agents or tasks within an agent — affects granularity of parallel batches.
