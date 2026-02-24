# Agentmux Documentation Update Report
**Date:** 2026-02-24
**Timestamp:** 2031
**Status:** Complete

## Overview

Updated technical documentation for the agentmux project with critical learnings from recent debugging and implementation session. Two core documentation files were enhanced with real-world patterns, concurrency insights, and TUI design lessons discovered during development.

## Files Updated

### 1. `/Users/alex/Code/test-new-ck/docs/system-architecture.md`

**Total Changes:** 5 major updates

#### Change 1: Go Version Requirement
- **Line:** 8
- **Before:** `1.24+`
- **After:** `1.26+`
- **Reason:** Updated to match actual project requirements

#### Change 2: NDJSON Stream Parsing (Enhanced)
- **Lines:** 70-86
- **Previous:** 4 bullet points (generic)
- **Updated:** 8 detailed bullet points + implementation notes
- **Key Additions:**
  - `--verbose` flag requirement for `stream-json` in print mode
  - Real event types: `system`, `assistant`, `result`, `rate_limit_event`
  - `assistant` events contain blocks array with typed content (NOT separate event types)
  - **Critical:** Use `bufio.Scanner` NOT `json.Decoder` (Scanner recovers from malformed lines)
  - Process args exact format with flags
  - Validation requirements before field access

#### Change 3: Batch Event Processing (New Section)
- **Lines:** 88-96
- **Type:** New section after NDJSON parsing
- **Content:**
  - WaitForEvent pattern explanation
  - BatchEvents return mechanism
  - Performance impact documentation (prevents per-event re-renders)
  - Buffer cap rationale (50 events)
  - Scheduler integration notes

#### Change 4: Identity vs Display Separation (New Section)
- **Lines:** 98-108
- **Type:** New critical pattern documentation
- **Content:**
  - Problem statement: SetHeader() overwrites identity field
  - Solution: Maintain separate `agentName` and `headerText` fields
  - DetailModel pattern example
  - Why it matters: String comparison matching reliability

#### Change 5: Goroutine-Per-Agent Model Update
- **Lines:** 110-115
- **Addition:** Goroutine lifecycle management
- **Key Point:** Process.EventCh closed after process exit prevents goroutine leaks

#### Change 6: Concurrency Model (Detailed)
- **Lines:** 154-165
- **Additions:**
  - Agent event channels: 256 buffer with rationale
  - Scheduler eventCh: 1024 buffer with backpressure explanation
  - Manager.List() safety: returns value copies
  - Scheduler execution pattern: lock → collect → unlock → execute
  - Process cleanup prevents goroutine leaks

#### Change 7: Configuration Schema (Realistic)
- **Lines:** 167-196
- **Before:** Generic cmd/workdir/env structure
- **After:** Actual structure with:
  - `defaults` section (model, max_turns)
  - `agents` section with prompt, model, max_turns, allowed_tools
  - `pipeline` section showing DAG relationships
  - Clear explanations of each section's purpose

---

### 2. `/Users/alex/Code/test-new-ck/docs/code-standards.md`

**Total Changes:** 6 major updates + 1 modification

#### Change 1: Go Version Requirement
- **Line:** 5
- **Before:** `1.24+`
- **After:** `1.26+`
- **Reason:** Consistency with system-architecture.md

#### Change 2: Channels Section (Enhanced)
- **Lines:** 79-97
- **Modification:** Updated buffer size and documentation
- **Before:** `make(chan tea.Msg, 10)` with generic documentation
- **After:** `make(chan tea.Msg, 1024)` with rationale
- **Key Points:**
  - Agent event channels: 256 buffer rationale
  - Scheduler eventCh: 1024 buffer explanation
  - "Scheduler produces faster than TUI consumes"
  - Buffer prevents backpressure blocking agent goroutines

#### Change 3: Goroutines Section Update
- **Lines:** 96-108
- **Addition:** Goroutine cleanup pattern
- **Key Point:** Close Process.EventCh after process exit prevents leaks

#### Change 4: NDJSON Parser Patterns (New Section)
- **Lines:** 209-259
- **Type:** Comprehensive new section
- **Content:**
  - Use `bufio.Scanner` not `json.Decoder` (with code example)
  - Error recovery pattern
  - Event type validation
  - `assistant` event block processing
  - Complete working code example showing all event types
  - Why Scanner over Decoder explanation
  - Claude CLI requirements with `--verbose` flag documentation

#### Change 5: TUI Identity vs Display Fields (New Section)
- **Lines:** 261-293
- **Type:** Critical pattern documentation
- **Content:**
  - Problem: Overwriting identity fields
  - Solution: Separate fields pattern
  - Code example with DetailModel
  - AppendLine implementation showing identity comparison
  - SetHeader implementation showing display-only update
  - Problem avoidance explanation with silent failure warning

#### Change 6: Batch Event Processing (New Section)
- **Lines:** 295-299+ (continues into next section)
- **Type:** Performance pattern documentation
- **Content:**
  - High-throughput event handling pattern
  - Code example: drainEvents implementation
  - WaitForEvent blocking pattern
  - Non-blocking drain up to 50 events
  - Performance impact analysis
  - Why batching matters (per-event renders)
  - Pattern details: block first, cap batch, return on empty

#### Change 7: Data Race Prevention (New Section)
- **Lines:** After Batch Event Processing
- **Type:** Critical concurrency safety pattern
- **Content:**
  - Manager.List() value copy pattern
  - Deadlock prevention strategy
  - Lock → collect → unlock → execute pattern
  - Scheduler example showing lock release before channel sends
  - Data race prevention techniques
  - Channel closure safety
  - Three key principles: value copies, lock discipline, channel ownership

---

## Key Patterns Documented

### 1. NDJSON Parsing Reality
- Claude CLI `--verbose` flag is REQUIRED for `stream-json` format in print mode
- Must use `bufio.Scanner`, NOT `json.Decoder` (malformed line recovery)
- `assistant` events contain blocks array; process sequentially
- Event types: `system`, `assistant`, `result`, `rate_limit_event`

### 2. TUI Performance & Correctness
- **Batch Events:** Block for first, drain up to 50 non-blocking (prevents per-event renders)
- **Identity vs Display:** Never overwrite identity fields with formatted content
- **DetailModel Pattern:** Separate `agentName` (matching) from `headerText` (display)

### 3. Concurrency Safety
- **Manager.List():** Returns value copies, not shared pointers
- **Scheduler Pattern:** Collect work under lock, execute side effects after unlock
- **Process Cleanup:** Close EventCh after exit prevents goroutine leaks
- **Buffer Sizes:** Agent 256 (local), Scheduler 1024 (global bottleneck)

### 4. Configuration Reality
- Defaults section for global settings
- Agents section with prompt, model, max_turns, allowed_tools
- Pipeline section showing DAG (agent: [dependencies])
- Not generic cmd/workdir/env structure

## Impact & Value

### For New Developers
- Eliminates trial-and-error debugging of NDJSON parsing
- Prevents silent log-disappearing bug from identity field overwrites
- Clarifies why buffers are sized differently
- Shows working code patterns for common issues

### For Code Quality
- Enforces batch processing for TUI responsiveness
- Prevents data races through documented patterns
- Ensures goroutine cleanup discipline
- Clarifies deadlock-free scheduler execution

### For Maintenance
- Real configuration schema prevents integration issues
- Documented concurrency model eases future refactoring
- Critical patterns prevent regression bugs
- Evidence-based decisions reduce debate

## Statistics

| Metric | Value |
|--------|-------|
| Files Updated | 2 |
| New Sections Added | 6 |
| Code Examples Added | 8 |
| Lines Added | ~180 |
| Total Doc Lines (system-arch) | 197 |
| Total Doc Lines (code-standards) | ~420 |
| Critical Patterns Documented | 7 |

## Verification

All updates verified:
- File syntax: Valid Markdown
- Code examples: Real patterns from codebase
- Cross-references: Consistent between files
- Naming: Matches actual implementation (camelCase for Go identifiers)
- Formatting: Consistent with existing documentation style

## Recommendations for Future Updates

1. **Add API Documentation** (`./docs/api-docs.md`) - Document public interfaces of key packages
2. **Add Troubleshooting Guide** - Known issues like:
   - Silent log loss from identity field overwrites
   - TUI unresponsiveness (batch events solution)
   - Goroutine leaks (channel cleanup)
3. **Add Example Pipelines** - Real YAML configs showing common patterns
4. **Monitor Performance** - Track TUI responsiveness with batch processing
5. **Update as Claude CLI Evolves** - Stream format may change in future versions

## Files Changed
- `/Users/alex/Code/test-new-ck/docs/system-architecture.md` (197 lines)
- `/Users/alex/Code/test-new-ck/docs/code-standards.md` (~420 lines)

---

**Report Generated:** 2026-02-24 20:31 UTC
**Next Review:** After major architectural changes or Claude CLI updates
**Maintainer:** Documentation Team
