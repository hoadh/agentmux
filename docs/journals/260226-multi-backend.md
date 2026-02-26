# Multi-Backend Refactor: From Monolith to Registry (4 Hours, One Data Race)

**Date**: 2026-02-26 08:55
**Severity**: High (Architecture + Data Race Fix)
**Component**: agentmux backend abstraction, agent orchestration
**Status**: Resolved (Complete) + Data Race Fixed in Commit 566d9ac

## What Happened

Completed a 4-hour refactor to abstract agentmux's hardcoded Claude CLI integration into a registry-based backend system. Any agent in a DAG pipeline can now target Claude CLI or Gemini CLI via YAML config. Built a clean Backend interface, implemented Claude and Gemini backends, and wired the registry throughout the codebase.

Then the code review revealed a data race in `Manager.Get()` that TUI code was exploiting. Fixed it in a follow-up commit.

## The Brutal Truth

This refactor was *almost* perfect, and that's what made the data race sting. The code review caught it cold: `Manager.Get()` returned a raw pointer to internal mutable state while TUI goroutines read it concurrently with scheduler updates. The race detector didn't flag it during tests because the test suite doesn't exercise the real concurrent access patterns.

It's the kind of bug that would have surfaced in production under load — agent state flickering in the TUI, duration counters jumping erratically, occasional panics. The fact that code review caught it before deployment felt like dodging a bullet.

Emotionally: relieved the issue was found, frustrated it shipped in the first place, validated that code review *actually matters* (not just a checkbox).

The Gemini integration itself was satisfying — two different CLI tools with completely different argument semantics (Claude: `-p "prompt"`, Gemini: `"prompt"` positional + no tool flags) unified under one interface. That's solid abstraction work.

## Technical Details

**Architecture:**
```
internal/agent/backend/
├── backend.go      # Backend interface + registry (Get/Register)
├── claude.go       # Claude backend + init() auto-registration
└── gemini.go       # Gemini backend + init() auto-registration
```

**Backend Interface:**
```go
type Backend interface {
    Name() string
    Binary() string
    BuildArgs(prompt, model string, allowedTools []string, maxTurns int) []string
    ConvertEvent(agentName string, raw map[string]any) tea.Msg
}
```

**Registry Pattern:**
- `var registry = map[string]Backend{}` with `Register()` and `Get()` functions
- Both `claude.go` and `gemini.go` define `init()` functions that auto-register themselves
- `Manager.startAgent()` resolves backend per agent: `b, _ := backend.Get(agentCfg.Backend)`

**Key Constraint Differences (Brainstorm Doc):**
| Concern | Claude | Gemini |
|---------|--------|--------|
| Prompt arg | `-p "prompt"` | `"prompt"` (positional) |
| Tool restrict | `--allowedTools X` (CLI flag) | Config file only |
| Max turns | `--max-turns N` (CLI flag) | Config file only |
| Approval mode | N/A | `--approval-mode auto_edit` (needed for non-interactive) |
| NDJSON schema | Nested content blocks | Flat events |

**YAML Config Changes:**
```yaml
defaults:
  backend: claude              # NEW: default backend
  model: sonnet
agents:
  researcher:
    backend: gemini            # NEW: per-agent override
    model: gemini-2.5-pro
  implementer:
    backend: claude            # explicit or inherits
    depends_on: [researcher]
```

**Data Race Found & Fixed:**

**The Bug (Manager.Get()):**
```go
func (m *Manager) Get(name string) *AgentInfo {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return m.agents[name]  // Returns raw pointer to mutable state
}
```

TUI code called `Get()` and read fields (`State`, `Duration`, `Process`) *outside the lock* while scheduler goroutines mutated the same fields.

**The Fix (Commit 566d9ac):**
```go
func (m *Manager) Get(name string) *AgentInfo {
    m.mu.RLock()
    defer m.mu.RUnlock()
    info, ok := m.agents[name]
    if !ok {
        return nil
    }
    cp := *info  // Return a copy
    return &cp
}
```

This matches the pattern already used in `Manager.List()`, which correctly returns copies.

## What We Tried

1. **Adding tool restrictions to Gemini config** → Accepted but with validation warning (Gemini CLI has no tool flag)
2. **Making approval mode configurable** → Hardcoded to `auto_edit` (sufficient for non-interactive pipelines; user can configure later)
3. **Blank import of backend package** → Works but analyzed as potentially redundant (manager.go already imports backend, so init() fires transitively)
4. **Moving utilities like `truncate()` and `intFromAny()`** → Stayed in claude.go (convenience; they're shared with gemini but small enough)

## Root Cause Analysis

**Why Did the Data Race Happen?**

The TUI code is single-threaded (Bubbletea runs one main update loop) but reads shared state that scheduler goroutines mutate. The lock-free `Get()` worked in testing because unit tests don't spawn concurrent schedulers and UI updates simultaneously. Real TUI usage does. This is a classic "works in test, fails in prod" scenario.

**Why Didn't Tests Catch It?**

The test suite exercises `Manager` in isolation — either running the scheduler, or reading agent state, but not both concurrently in the same test. The race detector (`-race` flag) only catches races in executed code paths. If the test doesn't exercise concurrent mutation + read, the race remains invisible.

**Why Was Code Review Necessary?**

The refactor itself (backend abstraction) was architecturally sound. But the code review process explicitly checks for concurrency hazards, not just feature correctness. This required reading through TUI code, understanding the concurrent execution model, and spotting the unsafe pointer return.

## Lessons Learned

1. **Registry pattern with init() is idiomatic Go and minimal**: Adding a 3rd backend requires exactly 1 new file + 1 `func init()` in it. The registry is thread-safe by Go's initialization guarantees (init() runs serially before main()). No mutex needed on the registry itself.

2. **Lock-free pointer returns from thread-safe collections are tempting and wrong**: Even with a mutex protecting the read, returning the interior pointer breaks encapsulation and invites data races. Always return copies or ensure the caller knows to hold the lock.

3. **Code review caught what tests couldn't**: This validates the importance of explicit review phases. Concurrency bugs are notoriously hard to unit-test; they require either high-concurrency stress tests or careful code inspection.

4. **Constraint differences between CLI tools are significant**: Claude and Gemini CLIs have wildly different argument semantics. The Backend interface bridged this gap elegantly — each implementation knows its own tool's quirks.

5. **Warnings for unsupported features beat hard errors**: When a user specifies `allowedTools` + `backend: gemini`, the config validation emits a warning (tools are ignored on Gemini), not an error. This is user-friendly — they can use the same config file across agents with different limitations.

6. **Backward compatibility matters**: Existing YAML configs without a `backend:` field continue to work because `backend.Get("")` defaults to "claude". Zero breaking changes.

## Next Steps

- **Fix H1-H3 issues from code review**:
  - Data race in `Get()` already fixed (commit 566d9ac)
  - Redundant blank import in run.go (low priority, clean-up)
  - Spawn dialog backend selector (future feature, not blocking v1)
- **Validate Gemini NDJSON schema**: Research was web-based; real Gemini CLI output should be captured and compared
- **Address M1 redundancy**: Remove dead code in `Process.Start()` default merging (ApplyDefaults already handles this)
- **Monitor Gemini rate limits**: Free tier is 1000 req/day; users should be aware

## Impact

agentmux now supports mixed Claude+Gemini orchestration. A single YAML pipeline can route some agents to Claude (for reasoning-heavy tasks) and others to Gemini (for lighter work or parallelism). The TUI displays events identically regardless of backend. The config system validates per-backend constraints and warns users about unsupported features.

**Commits:**
- `1900108 — feat: add registry-based multi-backend support for Claude and Gemini CLI`
- `566d9ac — fix: resolve data race in Manager.Get() and improve token state sync` (follow-up)
- `63fa95f — docs: update README with multi-backend support and examples`

## Why This Matters

This refactor unblocked real-world flexibility: users can now optimize agent selection per task, run cost-effective agents on Gemini while keeping heavy lifting on Claude, or test Gemini without rebuilding the entire orchestrator. The registry pattern scales — adding Claude 3.7 or Anthropic's hypothetical agent API later is just one more file + blank import.

The data race fix ensures the TUI remains responsive and correct under concurrent load. Without it, production usage would have exposed subtle state corruption and user confusion.
