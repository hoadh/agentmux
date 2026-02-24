# agentmux Project Roadmap

## Version History & Timeline

| Version | Release | Status | Key Features |
|---------|---------|--------|--------------|
| v0.1.0 | 2026-02-25 | MVP | DAG scheduling, TUI, NDJSON parsing, single-machine |
| v0.2.0 | Q2 2026 (planned) | Planned | Agent templates, output export, session resume |
| v0.3.0 | Q4 2026 (planned) | Planned | Distributed execution, marketplace, streaming |
| v1.0.0 | 2027 H1 (planned) | Vision | Web UI, REST API, database persistence, HA |

---

## v0.1.0 — MVP (Current — 2026-02-25)

**Status**: Feature-complete; testing and final polish phase

### Completed Features

- [x] YAML-based pipeline definition (config package)
- [x] DAG validation and topological sort (dag/graph.go)
- [x] Event-driven dependency orchestration (dag/scheduler.go)
- [x] Parallel agent execution (agent/manager.go)
- [x] NDJSON stream parsing with malformed line recovery (agent/parser.go)
- [x] Interactive TUI with sidebar, detail, statusbar (tui/ package)
- [x] Real-time agent state tracking (Running, Done, Failed, Killed, Blocked)
- [x] Graceful process shutdown (SIGTERM + 5s → SIGKILL)
- [x] Audit logging to JSONL (internal/log/)
- [x] Keybindings (j/k nav, Tab focus, n spawn, K kill, r restart, q quit)
- [x] Batch event processing for TUI responsiveness
- [x] Concurrent-safe state management (sync.RWMutex, sync.Mutex)

### Known Limitations

1. **Single-machine only**: No remote execution; agents must run locally
2. **Static config**: No live reloading during pipeline execution
3. **No agent communication**: Dependency ordering only; no message passing between agents
4. **No retry logic**: Failed agents remain failed; manual restart required
5. **No timeouts**: Long-running agents block pipeline; manual kill required
6. **Event buffer cap**: 1024 events (could theoretically overflow at extreme event rates)
7. **No templates**: Each pipeline must be defined from scratch

### Metrics & Success Criteria

- [x] Compiles with Go 1.26+ without errors
- [x] >80% test coverage on critical paths (parser, scheduler, manager)
- [x] Zero orphaned processes after exit
- [x] TUI responsive at 100+ events/second (per agent)
- [x] Documentation complete (system architecture, code standards, PDR)

---

## v0.2.0 — Enhanced Developer Experience (Q2 2026 — Planned)

**Theme**: Reduce boilerplate; enable faster iteration

### Planned Features

#### 1. Agent Templates
- **Pre-built workflows**: `agentmux init --template code-review` generates pipeline config
- **Template library**: Blog pipeline, API scaffold pipeline, refactoring pipeline
- **Community templates**: Users share templates; community vetting process
- **Why**: Reduce YAML authoring time from 30 minutes → 5 minutes for common tasks

#### 2. Output Export
- **Formats**: JSON, CSV, Markdown export of agent logs and results
- **Selection**: Export single agent or entire pipeline
- **Why**: Enable integration with downstream tools (dashboards, reports, data lakes)

#### 3. Session Resume
- **Save state**: Checkpoint pipeline progress to disk after each agent completion
- **Resume on failure**: Restart from checkpoint; skip completed agents
- **Why**: Recover from transient failures without restarting entire pipeline

#### 4. Agent Restart with Backoff
- **Automatic retry**: Configurable exponential backoff (1s, 2s, 4s, 8s...)
- **Failure tracking**: Log retry attempts; eventual fail if max retries exceeded
- **Why**: Handle transient failures (rate limits, network hiccups) transparently

#### 5. UI Polish
- **Dark/light theme toggle**: Keybinding to switch themes
- **Custom colors**: Allow users to override palette in config
- **Agent filtering**: Search/filter agent list by name or state
- **Tree view**: Visualize dependency graph as ASCII tree
- **Why**: Improve accessibility and usability for diverse environments

### Implementation Strategy

- **Templates**: New config/templates/ package with YAML generators
- **Export**: New export/ package with format-specific writers
- **Session**: Extend Manager to persist state; new internal/session/ package
- **UI**: Extend TUI models with filter state; add theme manager
- **Retry**: Extend Manager with retry counter; config schema update

### Success Criteria for v0.2.0

- [ ] 5+ built-in templates covering common workflows
- [ ] Export to JSON/CSV/Markdown without data loss
- [ ] Resume from checkpoint; skip completed agents
- [ ] Automatic retry with configurable backoff
- [ ] Theme toggle and custom color support
- [ ] 100+ successful runs with v0.2.0 features

---

## v0.3.0 — Distributed Execution (Q4 2026 — Planned)

**Theme**: Scale beyond single machine

### Planned Features

#### 1. Distributed Execution (SSH Backend)
- **Remote execution**: Spawn agents on remote machines via SSH
- **Config extension**: `agent.runs_on: remote-host1` or `local` keyword
- **Return channels**: Pipe subprocess output back over SSH
- **Why**: Distribute workload across machines; parallelize I/O-bound tasks

#### 2. Agent Marketplace
- **Community registry**: GitHub-hosted YAML templates with examples
- **Discovery**: `agentmux list-templates --filter=code-gen` searches registry
- **Installation**: `agentmux install-template blog-pipeline` clones and installs
- **Why**: Accelerate adoption; reduce duplication; foster community ecosystem

#### 3. Streaming Output
- **Live export**: Write agent output to file as it executes (not just at end)
- **Formats**: Streaming JSON, NDJSON, CSV
- **Sinks**: File, HTTP endpoint, database
- **Why**: Enable real-time dashboards; reduce latency for large outputs

#### 4. Telemetry (Anonymous, Opt-In)
- **Usage metrics**: Agent execution counts, average runtime, popular templates
- **Performance profiling**: Identify bottlenecks (parsing, TUI render time)
- **Error tracking**: Anonymized crash reports
- **Why**: Guide future feature prioritization; improve reliability

### Implementation Strategy

- **SSH**: New internal/remote/ package with SSH client; extend Process to support remote exec
- **Marketplace**: Separate CLI subcommand (agentmux marketplace); registry as external service
- **Streaming**: Extend Writer; add Sink interface (FileWriter, HTTPWriter, etc.)
- **Telemetry**: Optional analytics module; clear opt-out mechanism

---

## v1.0.0 — Production-Ready (2027 H1 — Vision)

**Theme**: Enterprise readiness; ecosystem maturity

### Planned Features

#### 1. Web UI
- **Browser-based dashboard**: Monitor pipelines from any device
- **Remote execution**: Submit pipelines via web form
- **History**: View past pipeline runs with logs and metrics
- **Why**: Enable non-CLI users; support monitoring from mobile/tablet

#### 2. RESTful API
- **Pipeline submission**: POST /pipelines with config JSON
- **Status polling**: GET /pipelines/{id}/status
- **Log streaming**: WebSocket for live log streams
- **Why**: Programmatic integration with CI/CD, webhooks, custom tooling

#### 3. Persistence Layer
- **Database backend**: PostgreSQL for pipeline history, audit logs, templates
- **Query interface**: Search past runs by agent, outcome, runtime
- **Retention policy**: Configurable log retention (30/90/365 days)
- **Why**: Enable compliance, post-mortem analysis, trend tracking

#### 4. High Availability
- **Load balancing**: Multiple agentmux instances share pipelines
- **Failover**: Automatic agent restart on worker failure
- **Multi-region**: Execute agents across geographic regions
- **Why**: Support production workloads with SLA guarantees

#### 5. Enterprise Features
- **RBAC**: Role-based access control (admin, operator, viewer)
- **SSO integration**: LDAP, OAuth2 authentication
- **Webhook notifications**: Slack, PagerDuty, custom HTTP
- **Audit trail**: Immutable log of all operations with user tracking
- **Why**: Meet enterprise security and compliance requirements

### Success Metrics for v1.0.0

- [ ] Web UI used in production by 10+ teams
- [ ] REST API supports 1000+ requests/hour
- [ ] Database stores 100,000+ pipeline executions
- [ ] 99.9% uptime SLA met
- [ ] RBAC and SSO in use at 50%+ of enterprise deployments

---

## Feature Prioritization Matrix

| Feature | Effort | Impact | Priority | Version |
|---------|--------|--------|----------|---------|
| Retry logic | Low | High | High | v0.2 |
| Output export | Medium | High | High | v0.2 |
| Session resume | Medium | Medium | Medium | v0.2 |
| Templates | Medium | High | High | v0.2 |
| SSH remote exec | High | Medium | Medium | v0.3 |
| Web UI | Very High | High | Medium | v1.0 |
| REST API | High | High | Medium | v1.0 |
| Database persistence | High | Medium | Low | v1.0 |
| RBAC/SSO | High | Medium | Low | v1.0 |

---

## Dependencies & Blockers

### Current Blockers (v0.1.0)
- None identified; MVP is feature-complete

### Future Blockers (v0.2.0+)

| Item | Status | Mitigation |
|------|--------|-----------|
| SSH library selection | Planned | golang.org/x/crypto/ssh (std library) |
| Database choice | Planned | PostgreSQL (proven, widely adopted) |
| Web framework | Planned | Evaluate: gin, echo, or std http |
| API schema | Planned | OpenAPI 3.0 spec |

---

## Community & Ecosystem Goals

### v0.2.0
- **Outreach**: Blog post on multi-agent orchestration patterns
- **Examples**: 5 end-to-end example pipelines in repository
- **Community**: Set up GitHub Discussions for feature requests

### v0.3.0
- **Marketplace launch**: Host first 10 community templates
- **Talks**: Conference talk (PyCon, GopherCon adjacent)
- **Adoption**: 1000+ GitHub stars, 100+ active users

### v1.0.0
- **Enterprise partnerships**: Pilot with 3+ Fortune 500 companies
- **Case studies**: Document 5 production use cases
- **Standards**: Contribute to open-source agent orchestration standards

---

## Backlog (Deferred Beyond v1.0.0)

- **Agent composition**: Define composite agents from sub-pipelines
- **Agent communication**: Message passing between agents (not just dependency ordering)
- **Custom resource types**: GPU allocation, memory constraints per agent
- **Cost tracking**: Estimate API costs per pipeline run
- **Performance auto-tuning**: Recommend parallelism based on resource availability

---

## Release Process

### Per-Release Checklist

- [ ] All tests pass (>80% coverage)
- [ ] Documentation updated (README, architecture, API docs if applicable)
- [ ] Changelog entry with user-facing changes
- [ ] Git tag created (v0.1.0, v0.2.0, etc.)
- [ ] Binary built and released on GitHub
- [ ] Release notes published with examples
- [ ] Demo video recorded (for major releases)

### Support Timeline

| Version | End of Life |
|---------|-------------|
| v0.1.x | 2026-12-31 (10 months) |
| v0.2.x | 2027-06-30 (12 months) |
| v0.3.x | 2027-12-31 (18 months) |
| v1.0.x | TBD (ongoing) |

---

**Document Version**: v0.1.0
**Last Updated**: 2026-02-25
**Next Review**: 2026-05-25 (post-v0.2 planning)
