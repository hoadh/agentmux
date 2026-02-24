---
title: "agentmux - Claude Code Agent Manager TUI"
description: "Go+Bubbletea TUI for spawning, monitoring, and orchestrating Claude Code agents with DAG pipelines"
status: complete
priority: P1
effort: 28h
branch: main
tags: [go, tui, bubbletea, claude-code, agent-orchestration, dag]
created: 2026-02-24
---

# agentmux Implementation Plan

Interactive TUI to spawn, monitor, kill, and orchestrate Claude Code agents via stream-json pipes with DAG-based pipeline support.

## Phases

| # | Phase | Effort | Status | File |
|---|-------|--------|--------|------|
| 1 | Project Setup & Config | 3h | complete | [phase-01](phase-01-project-setup-config.md) |
| 2 | Agent Process Management | 6h | complete | [phase-02](phase-02-agent-process-management.md) |
| 3 | DAG Scheduler | 4h | complete | [phase-03](phase-03-dag-scheduler.md) |
| 4 | TUI Core | 7h | complete | [phase-04](phase-04-tui-core.md) |
| 5 | TUI Features & CLI Wiring | 5h | complete | [phase-05](phase-05-tui-features-cli-wiring.md) |
| 6 | Testing & Polish | 3h | complete | [phase-06](phase-06-testing-polish.md) |

## Key Dependencies

- Go 1.22+, Bubbletea, Bubbles, Lip Gloss, Cobra, yaml.v3
- Claude Code CLI installed (`claude` binary in PATH)
- No external services; everything local

## Architecture

```
main.go → cmd/ (Cobra) → config/ (YAML+yaml.v3)
                       → dag/ (graph + scheduler)
                       → agent/ (manager + process + parser)
                       → tui/ (app + sidebar + detail + statusbar + spawn + styles)
                       → log/ (JSONL writer)
```

## Critical Path

Phase 1 → Phase 2 → Phase 3 → Phase 4 → Phase 5 → Phase 6

Phases 2 and 4 are heaviest; Phase 3 depends on Phase 2's agent types.

## Research Reports

- [Bubbletea+Cobra patterns](research/researcher-01-bubbletea-cobra-patterns.md)
- [DAG+process management](research/researcher-02-dag-process-management.md)
- [Claude Code CLI interface](../reports/researcher-260224-1529-claude-code-programmatic-interface.md)
- [Brainstorm decisions](../reports/brainstorm-260224-1523-agentmux-cli.md)

## Validation Log

### Session 1 — 2026-02-24
**Trigger:** Initial plan creation validation
**Questions asked:** 6

#### Questions & Answers

1. **[Architecture]** The plan uses Viper for YAML config parsing, but both research reports flag this as potentially over-engineered for a single config file. Viper adds env var overrides, config search paths, and file watching — none of which agentmux needs in v1. Plain `gopkg.in/yaml.v3` with manual `os.ReadFile` is ~15 lines vs Viper's setup. Which approach?
   - Options: Plain yaml.v3 (Recommended) | Viper | Start yaml.v3, migrate if needed
   - **Answer:** Plain yaml.v3 (Recommended)
   - **Rationale:** KISS principle. Single config file needs no env overrides or config search. Reduces dependency count.

2. **[Assumptions]** The parser assumes Claude's `--output-format stream-json` emits NDJSON. Have you verified the exact output format? The research found event types like 'assistant', 'tool_use', 'tool_result', 'result' — but the actual field structure matters for the parser. Should we run a quick test first?
   - Options: Test first (Recommended) | Build flexible parser | I've already tested it
   - **Answer:** Test first (Recommended)
   - **Rationale:** Parser struct definitions depend on real output. Testing first prevents rework. Add a pre-implementation step to Phase 2.

3. **[Scope]** The DAG scheduler adds 4h of effort and significant complexity. A simpler alternative: linear `depends_on` chains. Is parallel execution of independent dependents truly needed in v1?
   - Options: Full DAG (as planned) | Simple linear chains only | DAG but simpler implementation
   - **Answer:** Full DAG (as planned)
   - **Rationale:** Parallel tester+reviewer after coder is a core use case. Worth the investment.

4. **[Risk]** The plan has no agent timeout mechanism. A hung Claude process will block the pipeline forever and consume tokens. Should v1 include a `max_duration` config per agent?
   - Options: Yes, add max_duration (Recommended) | No, defer to v1.1 | Global timeout only
   - **Answer:** No, defer to v1.1
   - **Rationale:** Users can manually kill via 'k' key. Keep v1 scope tight.

5. **[Architecture]** Go module path is `github.com/USER/agentmux` (placeholder). What's the actual GitHub org/user?
   - Options: github.com/alex/agentmux | github.com/agentmux/agentmux | Just use 'agentmux'
   - **Answer:** Other
   - **Custom input:** github.com/hoadh/agentmux
   - **Rationale:** Defines import paths, go.mod, and README install instructions.

6. **[Tradeoff]** Agent config uses Go maps which don't preserve insertion order. The sidebar will display agents in unpredictable order. Options for consistent ordering?
   - Options: Alphabetical sort (Recommended) | Add 'order' field to config | Switch to YAML list with 'name' field
   - **Answer:** Alphabetical sort (Recommended)
   - **Rationale:** Simple, predictable, no config schema changes. Sort agent names for sidebar display.

#### Confirmed Decisions
- **Config library**: yaml.v3 — simpler, fewer deps, KISS
- **Stream-JSON format**: Test real output before finalizing parser structs
- **DAG scope**: Full DAG with parallel execution — core use case
- **Timeouts**: Defer to v1.1 — manual kill sufficient for v1
- **Module path**: github.com/hoadh/agentmux
- **Agent ordering**: Alphabetical sort for sidebar display

#### Action Items
- [ ] Replace Viper with yaml.v3 in Phase 1 (config parsing)
- [ ] Add pre-implementation step to Phase 2: capture real `stream-json` output
- [ ] Update module path to `github.com/hoadh/agentmux` in Phase 1
- [ ] Add alphabetical sorting note to Phase 4 sidebar implementation

#### Impact on Phases
- Phase 1: Replace Viper with yaml.v3, update module path to github.com/hoadh/agentmux
- Phase 2: Add step 0 — capture real stream-json output before writing parser structs
- Phase 4: Add alphabetical sort for sidebar agent list display
