# agentmux — Project Overview & PDR

## Product Vision

agentmux is an open-source Go TUI for orchestrating multi-agent AI workflows. It enables developers to define collaborative Claude Code agent pipelines in YAML, execute them with automatic dependency resolution, and monitor progress through an interactive dashboard. By abstracting the complexity of concurrent subprocess management, DAG scheduling, and event streaming, agentmux empowers teams to build, test, and iterate on complex automation tasks faster.

## Target Users

- **AI/ML Engineers**: Build and test multi-step agent workflows (research → design → code → test → review)
- **DevOps/SRE Teams**: Orchestrate parallel infrastructure analysis, remediation, and validation tasks
- **Development Teams**: Automate code generation, refactoring, and analysis pipelines
- **Researchers**: Conduct parallel experiments with different agent configurations
- **CLI Tool Builders**: Integrate agent orchestration into larger automation frameworks

## Core Features (v0.1.0 — MVP)

### 1. YAML-Based Pipeline Definition
- **What**: Define agents, their prompts, models, tool restrictions, and dependencies in `agentmux.yaml`
- **Why**: Declarative config reduces boilerplate; versioning and sharing pipelines is straightforward
- **How**: Config package parses YAML into typed structs; validation checks acyclicity and required fields

### 2. DAG-Based Dependency Orchestration
- **What**: Automatic topological sort and event-driven execution of agent dependencies
- **Why**: No manual scheduling; agents start automatically when dependencies complete
- **How**: Graph package validates acyclicity; Scheduler tracks pending counts and emits ready signals

### 3. Parallel Agent Execution
- **What**: Spawn multiple agents concurrently; no artificial serialization
- **Why**: Reduces total pipeline time by orders of magnitude
- **How**: One goroutine per agent; Manager and Scheduler coordinate via mutexes and channels

### 4. Interactive TUI Dashboard
- **What**: Real-time agent list with state indicators, log viewer, progress bar
- **Why**: Immediate feedback accelerates iteration and debugging
- **How**: Bubbletea composite UI; batch event processing (up to 50 per render) for responsiveness

### 5. Event Stream Parsing (NDJSON)
- **What**: Line-by-line consumption of Claude CLI `stream-json` output
- **Why**: Enables real-time progress visibility without buffering entire output
- **How**: bufio.Scanner for malformed line recovery; structured event type handling

### 6. Agent State Machine
- **What**: Pending → Running → Done | Failed | Killed | Blocked state transitions
- **Why**: Clear semantics for lifecycle management and error handling
- **How**: Manager tracks state and emits Bubbletea messages on transitions

### 7. Audit Logging (JSONL)
- **What**: All agent events persisted to `~/.agentmux/logs/*.jsonl`
- **Why**: Enables debugging, compliance, and post-run analysis
- **How**: Writer appends thread-safe event records with timestamps

## Non-Functional Requirements

### Performance
- **Target**: <100ms TUI latency at 100+ events/second (per agent)
- **Mechanism**: Batch event processing (50 events per render); buffered channels prevent backpressure
- **Validation**: Measure with 5+ concurrent agents; responsiveness should remain smooth

### Scalability
- **Concurrent Agents**: Support 20+ parallel agents without resource exhaustion
- **Goroutine Overhead**: ~1 goroutine per agent; acceptable for typical workflows
- **Memory**: <200MB for 20 agents with typical prompt/log sizes; profile and optimize if exceeded

### Reliability
- **Process Cleanup**: Graceful shutdown (SIGTERM + 5s → SIGKILL); no orphaned processes
- **State Consistency**: sync.RWMutex protects Manager state; sync.Mutex protects Scheduler critical sections
- **Cycle Detection**: Config validation rejects cyclic dependencies at load time

### Security
- **Command Injection**: Subprocess args use exec.Command() array form, not shell strings
- **Secret Handling**: Environment inheritance only; no logging of sensitive values (tokens, keys)
- **File Access**: Isolated working directories per agent; no symlink traversal
- **Dependency Validation**: Only trusted Claude CLI invocations; args validated

### Usability
- **Setup Time**: <5 minutes from install to first pipeline execution
- **Error Messages**: Clear, actionable feedback for config errors, cycle detection, process failures
- **Keybindings**: Standard vim-like navigation (j/k) and intuitive shortcuts (n=new, q=quit)

### Maintainability
- **Code Coverage**: >80% on critical paths (parser, scheduler, manager)
- **Test Fixtures**: YAML configs in testdata/; parser tests with realistic NDJSON
- **Comments**: Export docs; explain complex algorithms; TODOs include owner/timeline
- **File Size**: Keep code files <200 LOC; split into focused modules

## Out of Scope (v0.1.0)

- Agent templates or pre-built workflows
- Output export (JSON/CSV) — manual log parsing sufficient
- Web UI or remote execution
- Agent restart/retry logic on transient failures
- Multi-machine distributed execution

## Acceptance Criteria

### Functional
- [x] YAML config parses correctly; validation rejects invalid syntax
- [x] DAG with N agents executes in topological order; no dependency violations
- [x] Agents execute in parallel; final time ≤ max(agent time) + overhead
- [x] TUI updates in real-time; events display within 100ms
- [x] Process cleanup on quit; no orphaned processes after exit
- [x] Agent state transitions correctly (Pending → Running → Done/Failed/Killed)
- [x] Logs persist to `~/.agentmux/logs/`; no data loss on crash

### Non-Functional
- [x] Tests pass with >80% coverage on parser, scheduler, manager
- [x] Benchmark: 20 agents spawn + 100 events/agent in <5s without UI lag
- [x] Code compiles with Go 1.24.2+; no lint errors
- [ ] Documentation covers config, architecture, keybindings, troubleshooting

## Success Metrics

1. **Developer Velocity**: Time to define and execute a 5-agent pipeline: <10 minutes
2. **Reliability**: Zero orphaned processes or data loss after 100+ executions
3. **Responsiveness**: TUI remains interactive at 200+ events/second (20 agents × 10 events/sec)
4. **Community**: 100+ GitHub stars within 6 months (post-launch indicator)

## Product Roadmap

### v0.1.0 (Current — MVP)
- Core DAG orchestration, TUI, NDJSON parsing
- Single-machine execution
- Static YAML config
- **Status**: Feature-complete; testing and polish phase

### v0.2.0 (3–6 months post-v0.1)
- **Agent Templates**: Pre-defined workflows (blog pipeline, code review pipeline, research pipeline)
- **Output Export**: JSON, CSV, Markdown export of agent logs and results
- **Session Resume**: Save/load pipeline state; retry from checkpoint on failure
- **Agent Restart**: Automatic retry with exponential backoff for transient failures
- **UI Polish**: Dark/light theme toggle, custom colors, agent filtering

### v0.3.0 (6–12 months)
- **Distributed Execution**: Execute agents on remote machines (SSH backend)
- **Agent Marketplace**: Community-contributed templates and plugins
- **Streaming Output**: Real-time result streaming (e.g., write to file as agent executes)
- **Telemetry**: Anonymous usage metrics, performance profiling

### v1.0.0 (12–18 months)
- **Web UI**: Browser-based dashboard for remote execution
- **RESTful API**: Programmatic pipeline submission and monitoring
- **Persistence**: Database backend for pipeline history and audit logs
- **High Availability**: Load balancing, failover, multi-region execution
- **Enterprise Features**: RBAC, SSO, webhook integrations

## Dependencies & Constraints

### External Dependencies
- Claude & Gemini CLIs (subprocesses): Support multi-backend orchestration
- Go 1.24.2+: Minimum version required
- YAML v3: For config parsing
- Bubbletea v1.3.5+: TUI framework

### Technical Constraints
- Single-machine execution only (no native distribution in v0.1)
- Synchronous config loading (no live reloading during pipeline execution)
- No agent-to-agent message passing (only dependency ordering)

### Known Limitations
- DAG acyclicity must be manually verified in config (no runtime cycle breaking)
- Event buffering at 1024 messages (could drop events if TUI lag exceeds buffer)
- No built-in timeout; long-running agents block pipeline (manual kill required)

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|-----------|
| Claude CLI crashes | Medium | High | Restart logic; audit logs help debugging |
| TUI lag at high event rate | Medium | Medium | Batch processing; monitor event channel depth |
| Goroutine leak on crash | Low | High | Explicit EventCh close; context cancellation |
| Config syntax errors | High | Low | Validation errors are clear and actionable |
| Orphaned subprocesses | Low | High | Graceful SIGTERM + SIGKILL; process.Wait() |

## Success Story (Example Use Case)

**Scenario**: A team of 5 developers wants to auto-generate a REST API for a new microservice.

**Without agentmux**: Each developer runs Claude Code manually, copies outputs between agents, waits for bottlenecks. Total time: 4–6 hours.

**With agentmux**:
1. Define 5-agent pipeline in agentmux.yaml (10 minutes)
2. Run pipeline; agents execute in parallel (30 minutes)
3. TUI shows real-time progress; logs are persistent (debugging is fast)
4. Rerun pipeline with tweaks (5 minutes each iteration)

**Result**: Time-to-complete: <1 hour. Developers can iterate and refine.

## Next Steps

1. **v0.1.0 Release**: Polish, test, document
2. **Community Feedback**: Gather use cases and feature requests
3. **v0.2.0 Planning**: Prioritize templates and export features
4. **Ecosystem Growth**: Build example pipelines; contribute to open-source agent tooling

---

**Document Version**: v0.1.0
**Last Updated**: 2026-02-25
**Owner**: agentmux team
