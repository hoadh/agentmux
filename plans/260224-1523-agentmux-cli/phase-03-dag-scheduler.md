# Phase 3: DAG Scheduler

## Context Links
- [Parent Plan](plan.md)
- [Phase 2: Agent Process](phase-02-agent-process-management.md)
- [DAG Research](research/researcher-02-dag-process-management.md)

## Overview
- **Date**: 2026-02-24
- **Priority**: P1
- **Status**: complete
- **Description**: Implement DAG data structure with cycle detection, topological sort (Kahn's), and event-driven scheduler that integrates with agent manager and Bubbletea event loop.

## Key Insights
- Kahn's algorithm produces "levels" = sets of nodes runnable in parallel
- BUT batch WaitGroup approach blocks the whole level; bad for TUI (need incremental updates)
- Better: event-driven scheduler — on each `AgentDoneEvent`, check dependents, start any whose deps are all satisfied
- Scheduler runs as goroutine, communicates with TUI via tea.Msg channel
- On agent failure, mark all downstream dependents as `Blocked`

## Requirements

### Functional
- Build DAG from config's `depends_on` fields
- Detect cycles at config load time with clear error message
- Topological sort to validate and determine initial batch (roots)
- Event-driven scheduler: start roots immediately, trigger dependents on completion
- Handle failure propagation: failed agent → block all transitive dependents
- Expose pipeline status: total/running/done/failed/blocked/pending counts

### Non-Functional
- Cycle detection in O(V+E); DAGs are tiny (5-20 nodes typically)
- Scheduler must not block Bubbletea event loop

## Architecture

```
dag/
├── graph.go      # DAG struct, AddNode/AddEdge, cycle detection, topo sort
└── scheduler.go  # Scheduler: event-driven execution engine

Scheduler flow:
  1. Build DAG from config
  2. Validate (cycle detection)
  3. Find roots (in-degree 0)
  4. Start roots via Manager.Start()
  5. Listen for AgentDoneEvent
  6. On success: decrement dependents' in-degree; start any that reach 0
  7. On failure: mark dependents as Blocked recursively
  8. On all done/failed/blocked: send PipelineDoneEvent
```

## Related Code Files

### Create
- `internal/dag/graph.go` — DAG type, AddNode, AddEdge, Validate, Roots
- `internal/dag/scheduler.go` — Scheduler type, Run, event handling

## Implementation Steps

### 1. DAG data structure (`graph.go`)

```go
type Graph struct {
    nodes  map[string]bool           // all registered nodes
    edges  map[string][]string       // node → its dependencies (what it depends ON)
    rdeps  map[string][]string       // node → its dependents (what depends on IT)
    inDeg  map[string]int            // in-degree count
}

func New() *Graph
func (g *Graph) AddNode(name string)
func (g *Graph) AddEdge(from, to string) // "from" depends on "to"
func (g *Graph) Validate() error          // cycle detection via Kahn's
func (g *Graph) Roots() []string          // nodes with inDeg == 0
func (g *Graph) Dependents(name string) []string  // who depends on name
func (g *Graph) InDegree(name string) int
```

- `Validate()`: run Kahn's, if visited count != total nodes → cycle exists
- Return error with names of nodes involved in cycle (those still with inDeg > 0 after Kahn's)

### 2. Build graph from config

```go
func BuildFromConfig(cfg *config.Config) (*Graph, error) {
    g := New()
    for name := range cfg.Agents {
        g.AddNode(name)
    }
    for name, agent := range cfg.Agents {
        for _, dep := range agent.DependsOn {
            g.AddEdge(name, dep)
        }
    }
    if err := g.Validate(); err != nil {
        return nil, err
    }
    return g, nil
}
```

### 3. Event-driven scheduler (`scheduler.go`)

```go
type SchedulerEvent interface{}  // tea.Msg marker

type AgentStartedMsg struct{ Name string }
type AgentBlockedMsg  struct{ Name string; Reason string }
type PipelineDoneMsg  struct{ Success bool }

type Scheduler struct {
    graph   *Graph
    manager *agent.Manager
    pending map[string]int    // remaining dep count (mutable copy of inDeg)
    status  map[string]agent.AgentState
    eventCh chan tea.Msg       // sends scheduler events to TUI
}

func NewScheduler(g *Graph, mgr *agent.Manager) *Scheduler

// Run starts the pipeline. Returns channel of scheduler events.
// Call in a goroutine. Blocks until pipeline complete.
func (s *Scheduler) Run(ctx context.Context) <-chan tea.Msg
```

- `Run()` logic:
  1. Init `pending` map from graph in-degrees
  2. Init `status` map: all Pending
  3. Start all roots (pending[n] == 0) via `manager.Start(n)`
  4. For each started agent, arm `WaitForEvent(ch)` in a goroutine that forwards to internal channel
  5. Loop: read from internal channel
     - `AgentDoneEvent` with ExitCode 0:
       - Mark agent Done
       - For each dependent: decrement pending count
       - If pending reaches 0 → start dependent
     - `AgentDoneEvent` with ExitCode != 0:
       - Mark agent Failed
       - Call `blockDownstream(name)` — recursively mark all transitive dependents as Blocked
     - Forward all events to `eventCh` (TUI sees them)
  6. Check if all agents are in terminal state (Done/Failed/Blocked/Killed) → send `PipelineDoneMsg`

- `blockDownstream(name)`:
  ```go
  for _, dep := range s.graph.Dependents(name) {
      if s.status[dep] == agent.StatePending {
          s.status[dep] = agent.StateBlocked
          s.eventCh <- AgentBlockedMsg{Name: dep, Reason: name + " failed"}
          s.blockDownstream(dep)
      }
  }
  ```

### 4. Integration point

The TUI's `Init()` or `run` command will:
1. Load config
2. Build DAG from config
3. Create Manager, register all agents
4. Create Scheduler
5. `go scheduler.Run(ctx)` — returns event channel
6. Arm `WaitForEvent` on scheduler's channel + each agent's channel

## Todo List
- [ ] Implement Graph struct with AddNode/AddEdge (graph.go)
- [ ] Implement Validate with Kahn's cycle detection (graph.go)
- [ ] Implement Roots/Dependents/InDegree helpers (graph.go)
- [ ] Implement BuildFromConfig (graph.go)
- [ ] Implement Scheduler struct (scheduler.go)
- [ ] Implement event-driven Run loop (scheduler.go)
- [ ] Implement blockDownstream failure propagation (scheduler.go)
- [ ] Implement PipelineDoneMsg detection (scheduler.go)
- [ ] Verify `go build ./...` passes

## Success Criteria
- Cycle in config detected with clear error at load time
- Agents with no deps start immediately
- Agent completion triggers dependents automatically
- Failed agent blocks all downstream dependents
- Pipeline completes when all agents reach terminal state
- No goroutine leaks after pipeline completion

## Risk Assessment
- **Race conditions**: Scheduler modifies pending/status maps in single goroutine (no concurrent access). Manager has its own mutex. Safe.
- **Agent hangs forever**: Scheduler should respect context cancellation. If ctx is canceled, stop all running agents. Consider per-agent timeout config (v1.1).
- **Diamond dependencies**: A depends on B and C, both depend on D. Kahn's handles this correctly; A starts only when both B and C complete.

## Security Considerations
- None specific to this phase; scheduler trusts agent manager

## Next Steps
- Phase 4: TUI will receive scheduler events and update display
- Phase 5: Wire scheduler into CLI `run` command
