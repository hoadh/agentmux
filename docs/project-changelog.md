# agentmux Project Changelog

## [Unreleased]

### Added
- **`-c` Shorthand**: New `-c` shorthand for `--config` flag. Usage: `agentmux run -c pipeline.yaml` (equivalent to `agentmux run --config pipeline.yaml`). Feature added 2026-02-26.
- **`--var` CLI Flag**: New repeatable `--var` flag for template variable overrides via CLI. Syntax: `agentmux run -c pipeline.yaml --var key=value --var another=value2`. CLI values override YAML `vars` section. Feature added 2026-02-26.
- **Batch Blog Example**: New example pipeline `examples/batch-blog-with-vars.yaml` demonstrates template variables and multi-agent blog generation workflow. Feature added 2026-02-26.
- **Install Script**: New `scripts/install.sh` provides interactive installation utility for placing executables in system paths. Supports multiple destination options, automatic permission handling, and PATH configuration guidance. Usage: `./scripts/install.sh <executable> [--name <name>]`. Feature added 2026-02-27.
- **Template Variables**: New `vars` section in YAML config enables Go text/template expansion in agent prompts using `{{.var_name}}` syntax. Feature added 2026-02-27.
- **Configurable Output Directory**: New `--output-dir` CLI flag and YAML `output_dir` field allow custom location for logs and results. Defaults to `.agentmux-out`. CLI flag overrides YAML config. Feature added 2026-02-27.
- **Result Export**: New `internal/result/writer.go` saves agent outputs as markdown files to `{OutputDir}/results/{agentName}.md`. Thread-safe via sync.Mutex.
- **Parallel Run Script**: New `scripts/parallel-run.sh` for batch execution of multiple pipelines in parallel with job limiting (default 4), isolated output directories, and summary reporting. Supports inline mode (`--` separators) and manifest mode (file input). POSIX-compatible, injection-safe (no eval). Feature added 2026-02-28.
- **Health Check Server**: New `agentmux serve` command starts HTTP server with `/health` JSON endpoint for monitoring integration. Configurable listen address via `-a` flag (default `:8080`). Returns version, uptime, and startup timestamp. Feature added 2026-03-01.

### Changed
- **Config Loading**: `LoadConfig()` now calls `ExpandTemplates()` automatically after validation. Returns warning list alongside Config.
- **Output Directory Structure**: Logs now save to `{OutputDir}/logs/` and results to `{OutputDir}/results/` instead of hardcoded `~/.agentmux/logs/`.

### Fixed
- N/A

---

## [v0.1.0] — 2026-02-25

### MVP Release

**Initial release of agentmux with core features:**

- YAML-based pipeline definition with global defaults inheritance
- DAG validation (cycle detection via Kahn's algorithm) and topological sorting
- Event-driven dependency scheduler with pending count tracking
- Multi-backend support (Claude & Gemini CLIs) with independent per-agent backend selection
- Parallel agent execution with concurrent-safe state management (sync.RWMutex, sync.Mutex)
- NDJSON stream parsing with bufio.Scanner (malformed line recovery)
- Interactive TUI dashboard (Bubbletea) with sidebar, detail view, statusbar, spawn modal
- Headless mode for non-interactive CI/CD execution (text + NDJSON formatters)
- Real-time agent state tracking (Pending → Running → Done/Failed/Killed/Blocked)
- Graceful process shutdown (SIGTERM + 5s → SIGKILL)
- Audit logging to JSONL files
- Batch event processing (up to 50 events per TUI render cycle) for responsiveness
- Comprehensive keybindings (j/k nav, Tab focus, n spawn, K kill, r restart, l log, p pipeline, q quit)
- >80% test coverage on critical paths
- Full documentation (system architecture, code standards, configuration guide)

### Known Limitations
- Single-machine only; no remote execution
- No live config reloading during execution
- No agent-to-agent message passing
- No automatic retry logic
- No timeout enforcement
- 1024-event buffer cap (theoretical overflow at extreme event rates)

---

## Versioning

This project follows [Semantic Versioning](https://semver.org/):

- **MAJOR** (v1.0+): Breaking changes to CLI or config schema
- **MINOR** (v0.2, v0.3): New features backward-compatible with v0.1
- **PATCH** (v0.1.1+): Bug fixes and documentation updates

## Release Frequency

- **Current**: Rapid iteration on v0.1.x (weekly releases likely)
- **v0.2.0**: Expected Q2 2026 (Agent templates, output export, session resume)
- **v0.3.0**: Expected Q4 2026 (Distributed execution, marketplace)
- **v1.0.0**: Expected 2027 H1 (Web UI, REST API, persistence)

---

**Document Version**: v0.1.0
**Last Updated**: 2026-03-01
