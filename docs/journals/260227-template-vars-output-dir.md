# Template Variables & Configurable Output Directory — Complete

**Date**: 2026-02-27 14:35
**Severity**: Low (feature addition)
**Component**: CLI, Config, Result Output
**Status**: Resolved

## What Happened

Successfully implemented template variables in YAML configs and configurable output directories for logs and results. The feature allows users to define reusable variables in a `vars` section, expand them in agent prompts using Go template syntax (`{{.var_name}}`), and override both via CLI flags. Results are now written to configurable paths instead of hardcoded directories.

## The Brutal Truth

This was clean implementation work. No fires, no surprises, no late-night debugging sessions. The plan was solid, execution was straightforward, and code review feedback was minimal. It's refreshing when a 6-phase feature just... works.

## Technical Details

**Core additions:**
- `internal/result/writer.go`: Saves agent results as `{output_dir}/results/{agent_name}.md`
- CLI flag hierarchy: `--var key=value` and `--output-dir` override YAML config values
- Template expansion moved post-config-load to allow CLI vars to merge before processing
- Used `text/template` with `missingkey=error` for compile-time var validation

**Files touched:** 7 modified, 2 created, 3 test fixtures added. No rewrites needed.

## What We Tried

Followed the plan exactly. No detours, no failed approaches. Code review caught two logging edge cases in the headless runner; both were fixed in a single pass.

## Root Cause Analysis

None needed. The architecture decision to decouple template expansion from config loading proved correct—it enabled clean CLI var merging without side effects.

## Lessons Learned

- Separating concerns (load → merge CLI → expand) beats trying to do everything in one pass
- Write-and-close per result (vs. keep-open) was correct given single-write-per-agent pattern
- Test fixtures for YAML config variations paid off—caught edge cases early

## Next Steps

None. Feature is complete, tests pass with race detector, and it's ready for merge. Document in changelog and roadmap.
