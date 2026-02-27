# Configuration Guide

## Config File (agentmux.yaml)

| Section | Purpose |
|---------|---------|
| `version` | Config format version (currently 1) |
| `vars` | Template variables: map[string]string for prompt expansion |
| `output_dir` | Root directory for logs and results (default: `.agentmux-out`) |
| `defaults` | Global defaults: backend, model, max_turns, allowedTools |
| `agents` | Named agent definitions with prompt, backend, model, tool restrictions |
| `depends_on` | DAG: each agent lists dependencies (agents that run first) |

## Template Variables

Use the `vars` section to define reusable values expanded in agent prompts via Go text/template syntax:

```yaml
vars:
  project_root: "/path/to/project"
  code_lang: "go"
  max_depth: "3"

agents:
  scanner:
    prompt: "Scan {{.project_root}} for {{.code_lang}} files up to depth {{.max_depth}}"
```

**Syntax**: `{{.var_name}}` in agent prompts is replaced with the value from `vars[var_name]`.

**Validation**: Missing variables abort config loading with a template expansion error. Values must be strings; numbers and booleans should be quoted as strings.

## Output Directory

Configure where logs and results are saved:

```yaml
output_dir: ".agentmux-out"
```

**Structure**:
```
.agentmux-out/
├── logs/         # JSONL event logs per agent
└── results/      # Markdown results per agent
```

**Priority**: CLI flag `--output-dir` > YAML `output_dir` > default `.agentmux-out`

**Example**:
```bash
agentmux run -c pipeline.yaml --output-dir ./my-results
```

## Multi-Backend Support

Each agent can target a different CLI backend. Set `backend` at the defaults or agent level:

| Backend | Binary | Notes |
|---------|--------|-------|
| `claude` (default) | `claude` | Full support: model, allowedTools, max_turns |
| `gemini` | `gemini` | Model supported; allowedTools/max_turns ignored (warning emitted) |

Adding a new backend requires only one new Go file with `init()` registration.

## Config Validation

agentmux validates your pipeline config at load time and emits warnings for backend-specific incompatibilities.

**Hard errors** (abort startup):

| Condition | Error |
|-----------|-------|
| Agent missing `prompt` | `agent "X": prompt is required` |
| Agent depends on itself | `agent "X": cannot depend on itself` |
| Agent depends on undefined agent | `agent "X": unknown dependency "Y"` |
| Unknown backend name | `unknown backend "X" (available: [claude gemini])` |

**Warnings** (printed to stderr, execution continues):

| Condition | Warning |
|-----------|---------|
| Gemini agent has `allowedTools` | `agent "X": allowedTools ignored for gemini backend` |
| Gemini agent has `max_turns > 0` | `agent "X": max_turns ignored for gemini backend` |

Warnings also trigger for values inherited from `defaults` (defaults are merged before validation).

## Models & Tools by Backend

| Feature | Claude | Gemini |
|---------|--------|--------|
| `model` | Passed as `--model` flag | Passed as `--model` flag |
| `allowedTools` | Each tool passed as `--allowedTools <tool>` | **Ignored** (warning emitted) |
| `max_turns` | Passed as `--max-turns` flag | **Ignored** (warning emitted) |

### Claude Models

| Alias | Full Model ID | Notes |
|-------|---------------|-------|
| `opus` | `claude-opus-4-6` | Most capable, adaptive reasoning |
| `sonnet` | `claude-sonnet-4-6` | Balanced speed/quality (default) |
| `haiku` | `claude-haiku-4-5` | Fastest, lightweight tasks |

### Claude Tools (`allowedTools`)

| Category | Tools |
|----------|-------|
| File Ops | `Read`, `Edit`, `Write`, `NotebookEdit`, `Glob` |
| Shell | `Bash` |
| Search | `Grep`, `WebSearch`, `WebFetch` |
| Task Mgmt | `Task`, `TodoRead`, `TodoWrite` |
| MCP | `mcp__<server>__<tool>` pattern |

Supports granular patterns: `Bash(git log *)`, `Edit(src/**)`, etc.

### Gemini Models

| Model ID | Notes |
|----------|-------|
| `gemini-2.5-pro` | Default, complex reasoning |
| `gemini-2.5-flash` | Fast inference |
| `gemini-2.5-flash-lite` | Most cost-efficient |
| `gemini-3-pro-preview` | Preview, latest generation |
| `gemini-3-flash-preview` | Preview, fast |

No short aliases — full model IDs required.

### Gemini Tools

| Category | Tools |
|----------|-------|
| Search | `google_web_search` |
| File Ops | Read, write, glob, text search/replace |
| Shell | Command execution (with confirmation) |
| Web | URL fetching, browser automation |
| Memory | Todo/memory management |
| MCP | 415+ extensions via registry |

> **Note:** Gemini CLI uses `tools.core` (allow-list) or `tools.exclude` (block-list) in config for tool filtering. The `allowedTools` field in agentmux config is ignored for Gemini agents.

## Defaults Inheritance

`ApplyDefaults()` merges global `defaults` into each agent for any unset field: `backend`, `model`, `allowedTools`, `max_turns`. The `workdir` field defaults to `"."` if empty.

**Model resolution**: If an agent has no `model`, it inherits from `defaults.model`. If still empty, no `--model` flag is passed and the backend CLI uses its own default. Model names are not validated at config time — invalid names produce runtime errors from the backend CLI.

## Headless Mode

Run pipelines without the TUI by passing `--headless`. Events stream to stdout in real time.

| Flag | Default | Description |
|------|---------|-------------|
| `--headless` | `false` | Run without TUI, stream events to stdout |
| `--format` | `text` | Output format: `text` or `ndjson` |
| `--output-dir` | `.agentmux-out` | Root directory for logs and results |

**Text format** produces human-readable lines:

```
[14:30:05] [scout] STARTED
[14:30:06] [scout] Exploring the codebase...
[14:30:10] [scout] -> Tool: bash find . -name "*.go"
[14:30:10] [scout] DONE
[14:30:10] [planner] STARTED
...
[14:30:45] [pipeline] COMPLETED
```

**NDJSON format** produces one JSON object per line for machine consumption:

```json
{"ts":"2026-02-27T14:30:05+07:00","agent":"scout","type":"agent_started"}
{"ts":"2026-02-27T14:30:06+07:00","agent":"scout","type":"assistant","data":{"text":"Exploring..."}}
{"ts":"2026-02-27T14:30:45+07:00","agent":"pipeline","type":"pipeline_done","data":{"success":true}}
```

**Exit codes**: `0` = all agents succeeded, `1` = any failure, `130` = SIGINT, `143` = SIGTERM.

**Results Export**: Agent results are saved to `{output_dir}/results/` as markdown files. Logs are saved to `{output_dir}/logs/` as JSONL.

**CI/CD example**:

```bash
./agentmux run -c pipeline.yaml --headless --format ndjson --output-dir ./ci-results | tee output.jsonl
echo "Exit: $?"
# Agent results available in ./ci-results/results/
# Event logs available in ./ci-results/logs/
```
