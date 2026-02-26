# Building agentmux: A 28-Hour Journey from Concept to Multi-Agent TUI

**Date**: 2026-02-24 16:34
**Severity**: Medium (Architecture Achievement)
**Component**: agentmux CLI / Go TUI application
**Status**: Resolved (Complete)

## What Happened

Over 28 hours across 6 sequential phases, built a production-grade TUI for spawning, monitoring, and orchestrating Claude Code agents with DAG-based pipeline support. The project went from concept to working Go application with Cobra CLI, Bubbletea TUI, YAML config, streaming JSON parser, and a full DAG scheduler capable of parallel execution.

## The Brutal Truth

This was *exhausting* and occasionally maddening. Building TUIs is harder than it looks. Bubbletea is elegant but the mental model is disorienting — everything is a message-driven state machine, and UI responsiveness requires deep understanding of Go concurrency patterns. Early on, we made a critical decision to use full DAG with parallel execution instead of simple linear chains. That added 4 hours of effort but made the system genuinely useful for realistic workflows.

What was most frustrating: the parser. Claude's stream-json output looked straightforward in documentation, but the actual NDJSON structure has nested arrays, conditional fields, and edge cases that forced multiple iterations. We spent hours validating against real output before finalizing parser structs.

The satisfying part? Everything just *works*. No crashes, no hangs, clean architecture, and the alphabetical agent sorting that seemed like a trivial detail actually made the sidebar infinitely more usable than random order.

## Technical Details

**Architecture:**
```
main.go → cmd/ (Cobra CLI)
        → config/ (yaml.v3 YAML parser)
        → dag/ (directed acyclic graph + scheduler)
        → agent/ (manager + process + stream parser)
        → tui/ (Bubbletea app + sidebar + detail view + spawn modal)
        → log/ (JSONL event writer)
```

**Key Stats:**
- 28 hours total effort across 6 phases
- Phase 4 (TUI Core) was heaviest at 7h — sidebar, detail panel, focus management
- Phase 5 (Features + CLI Wiring) required careful message routing and state sync
- 6 major architectural decisions documented in validation log

**Critical Decision: YAML vs Viper**
The plan originally proposed Viper for config management. A quick analysis revealed Viper adds env var overrides, config search paths, and file watching — features agentmux doesn't need. Plain `yaml.v3` with `os.ReadFile` was 15 lines vs Viper's setup ceremony. Chose yaml.v3 per KISS principle. This saved dependency count and mental overhead with zero trade-off.

**Critical Decision: DAG Scope**
Debated whether full DAG (parallel execution of independent dependents) was worth the 4h effort vs simple linear chains. Decided full DAG because parallel tester+reviewer workflows after a coder agent are genuinely useful. This decision paid off — the system supports real-world orchestration patterns.

**Parser Challenges:**
- Claude's stream-json output has nested content blocks within `assistant` events
- Different event types (assistant, tool_use, tool_result, result) with different payloads
- Token counts buried in nested `usage` objects
- Real output validation forced multiple struct refinements before stabilizing

## What We Tried

1. **Viper for config** → Rejected (YAGNI — too many features for single config file)
2. **Simple linear agent chains** → Rejected (doesn't support realistic parallel workflows)
3. **Agent timeouts in v1** → Rejected (deferred to v1.1 — manual kill via 'k' key sufficient)
4. **Random agent order in sidebar** → Rejected (would be chaotic UX; alphabetical sorting chosen instead)
5. **Hardcoded go module path** → Rejected (updated to github.com/hoadh/agentmux during validation)

## Root Cause Analysis

**Why Did Decisions Get Made This Way?**

The validation phase was crucial. Early in planning, we had 6 open questions about architecture, scope, and dependencies. Rather than guess, we explicitly asked and documented answers with rationale. This prevented scope creep and kept the team aligned.

The YAML choice exemplifies good engineering judgment: recognize when "full-featured" is actually "over-engineered for this problem." Viper wasn't broken; it was just unnecessary friction.

The DAG decision came from real-world thinking: what do actual users want? Parallel workflows. The extra complexity was worth it.

**What Almost Went Wrong?**

The parser was on the edge of becoming a maintenance nightmare. We got lucky catching the nested structure early through real output validation. If we'd built parser structs purely from documentation, we would have had 3-4 hours of rework debugging mismatched types.

## Lessons Learned

1. **Validation phases pay dividends**: Explicitly documenting architectural decisions and rationales early prevents late surprises. The 6-question validation log was worth every minute.

2. **KISS doesn't mean "minimal features" — it means "no unnecessary features"**: Viper is great for projects needing env overrides or multiple config search paths. agentmux needed none of that. Don't use sophisticated solutions for simple problems.

3. **Stream parsing from external CLIs requires real output validation**: Documentation lies or is incomplete. Always test against real output before finalizing parser structs.

4. **TUI development is concurrency-heavy**: The hardest parts weren't UI logic or styling — they were message routing, state synchronization between goroutines (scheduler, ticker, parser), and focus management. Bubbletea shines here but demands respect.

5. **Alphabetical sorting as UX detail**: A throwaway comment about agent ordering in maps led to a real UX improvement. Sometimes the small decisions matter most.

6. **Full DAG complexity is manageable with good design**: We worried DAG scheduling would be unmaintainable. With clean separation (dag package owns graph logic, scheduler is 100 lines), it's actually elegant.

## Next Steps

- **v1 is done**: All 6 phases complete, full DAG scheduler working, TUI responsive
- **Known deferral**: Agent timeouts (max_duration config) → v1.1
- **Future enhancement**: Interactive backend selection in spawn modal (currently hardcoded to Claude)
- **Documentation**: Update README with installation, examples, and DAG pipeline syntax

## Impact

This project enabled multi-agent orchestration for Claude Code, turning the CLI tool from single-agent to pipeline-based workflow. The DAG scheduler allows complex dependency graphs; the TUI provides real-time visibility into agent progress. The codebase is clean, testable, and ready for extensions (new agent types, new backends).

**Code merged**: `2f6dd0f — feat: implement agentmux TUI for Claude Code agent orchestration`
