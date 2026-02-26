# agentmux Deployment Guide

Complete guide for building, installing, configuring, and running agentmux pipelines.

## Prerequisites

- **Go 1.24.2+**: Required for building from source
- **Claude CLI** and/or **Gemini CLI**: At least one required (both optional for multi-backend pipelines)
  - Claude: For Claude-backed agents
  - Gemini: For Gemini-backed agents
  - Both: Enables mixed-backend pipelines
- **Unix-like system**: Linux, macOS, or WSL2 on Windows
- **Terminal**: 80×24 minimum for TUI display

### Check Prerequisites

```bash
go version          # Should be go1.24.2 or later
claude --version    # If using Claude backend (optional)
gemini --version    # If using Gemini backend (optional)
echo $SHELL         # Should be /bin/bash, /bin/zsh, etc.
```

**Note**: You need at least one CLI backend (Claude or Gemini). For multi-backend pipelines, install both.

---

## Installation

### Option 1: Build from Source (Recommended for Development)

```bash
git clone https://github.com/hoadh/agentmux.git
cd agentmux
go build -o agentmux ./cmd/main.go
./agentmux --version
```

**Install to PATH**:
```bash
sudo mv agentmux /usr/local/bin/
# or for user-only install
mkdir -p ~/.local/bin
mv agentmux ~/.local/bin/
export PATH="$HOME/.local/bin:$PATH"  # Add to ~/.bashrc or ~/.zshrc
```

### Option 2: Pre-built Binary

Visit [GitHub Releases](https://github.com/hoadh/agentmux/releases) and download the binary for your OS:

```bash
# Linux
wget https://github.com/hoadh/agentmux/releases/download/v0.1.0/agentmux-linux-amd64
chmod +x agentmux-linux-amd64
sudo mv agentmux-linux-amd64 /usr/local/bin/agentmux

# macOS
wget https://github.com/hoadh/agentmux/releases/download/v0.1.0/agentmux-darwin-amd64
chmod +x agentmux-darwin-amd64
sudo mv agentmux-darwin-amd64 /usr/local/bin/agentmux

# macOS (Apple Silicon)
wget https://github.com/hoadh/agentmux/releases/download/v0.1.0/agentmux-darwin-arm64
chmod +x agentmux-darwin-arm64
sudo mv agentmux-darwin-arm64 /usr/local/bin/agentmux
```

### Verify Installation

```bash
agentmux --version     # Should print v0.1.0
agentmux run --help    # Should show usage
```

---

## Configuration

### Create Your First Pipeline

Create `agentmux.yaml` in your project directory:

```yaml
version: 1

defaults:
  model: "sonnet"
  max_turns: 10
  allowedTools: ["Read", "Edit", "Write", "Bash", "Glob"]

agents:
  scout:
    prompt: |
      Analyze the project structure. Identify key files, technologies used,
      and entry points. Write a summary to ANALYSIS.md.
    max_turns: 5

  planner:
    prompt: |
      Based on ANALYSIS.md, create a step-by-step implementation plan
      for refactoring the codebase. Save to PLAN.md.
    depends_on: [scout]
    max_turns: 8

  coder:
    prompt: |
      Following PLAN.md, refactor the code. Write production-ready code
      with proper error handling.
    depends_on: [planner]
    model: "opus"
    max_turns: 20

  tester:
    prompt: |
      Write comprehensive tests for all changes. Run and fix tests.
    depends_on: [coder]
    max_turns: 15
```

### Configuration Schema

#### Root Level

| Key | Type | Required | Default | Notes |
|-----|------|----------|---------|-------|
| version | int | Yes | — | Must be 1 |
| defaults | object | Yes | — | Global agent defaults |
| agents | object | Yes | — | Named agent definitions |
| pipeline | object | No | — | (Deprecated; use depends_on instead) |

#### Defaults Section

| Key | Type | Default | Notes |
|-----|------|---------|-------|
| model | string | "sonnet" | claude-3-5-sonnet-latest or opus |
| max_turns | int | 10 | Max agent iterations |
| allowedTools | array | all | Tools agent can call |

#### Agent Definition

| Key | Type | Required | Notes |
|-----|------|----------|-------|
| prompt | string | Yes | Task description for agent |
| model | string | No | Override defaults.model |
| max_turns | int | No | Override defaults.max_turns |
| allowed_tools | array | No | Restrict tools agent can use |
| depends_on | array | No | List of dependency agent names |

#### Example: Advanced Config

```yaml
version: 1

defaults:
  model: "sonnet"
  max_turns: 15
  allowedTools: ["Read", "Write", "Bash", "Edit", "Glob", "Grep"]

agents:
  researcher:
    prompt: |
      Research the topic and gather information.
      Save findings to RESEARCH.md.
    max_turns: 10

  writer:
    prompt: |
      Using RESEARCH.md, write a comprehensive article.
      Save to ARTICLE.md with proper formatting.
    depends_on: [researcher]
    model: "opus"
    max_turns: 20
    allowed_tools: ["Read", "Write"]

  editor:
    prompt: |
      Review ARTICLE.md for clarity, grammar, and structure.
      Make edits to improve quality. Save final version.
    depends_on: [writer]
    max_turns: 10
    allowed_tools: ["Read", "Edit"]

  seo:
    prompt: |
      Optimize the article for search engines.
      Add meta tags, keywords, and structure. Save to FINAL.md.
    depends_on: [editor]
    max_turns: 5
```

#### Example: Multi-Backend Pipeline

Mix Claude and Gemini agents in the same pipeline:

```yaml
version: 1

defaults:
  backend: "claude"      # Default backend
  model: "sonnet"
  max_turns: 10

agents:
  scout:
    backend: "gemini"    # Override: use Gemini for this agent
    model: "gemini-2.5-pro"
    prompt: "Analyze the codebase. Summarize architecture and key technologies."
    max_turns: 5

  planner:
    # Inherits backend: "claude" from defaults
    prompt: "Based on ANALYSIS.md, create a detailed implementation plan."
    depends_on: [scout]
    max_turns: 10

  coder:
    # Inherits backend: "claude" from defaults
    prompt: "Following the plan, implement the code."
    depends_on: [planner]
    model: "opus"
    max_turns: 20

  reviewer:
    backend: "gemini"    # Another Gemini agent
    prompt: "Review the code for quality and compliance."
    depends_on: [coder]
    max_turns: 5
```

**Validation**: agentmux validates backend names and emits warnings for unsupported features:
- Claude supports: `model`, `allowedTools`, `max_turns`
- Gemini supports: `model` only (allowedTools/max_turns ignored with warnings)

### Validation

Validate your config before running:

```bash
agentmux run -c agentmux.yaml --dry-run
```

This checks for:
- ✓ Valid YAML syntax
- ✓ Required fields (agents, defaults)
- ✓ Cyclic dependencies (detected and rejected)
- ✓ Missing dependency references

---

## Running Pipelines

### Basic Execution

```bash
agentmux run -c agentmux.yaml
```

This:
1. Loads and validates config
2. Builds DAG from dependencies
3. Spawns agents in topological order
4. Displays interactive TUI dashboard
5. Saves audit logs to `~/.agentmux/logs/`

### Interactive TUI

Once running, interact via keybindings:

| Key | Action |
|-----|--------|
| `j` / `k` | Navigate agent list up/down |
| `Tab` | Cycle focus (sidebar → detail → statusbar) |
| `n` | Open spawn dialog (manual agent trigger) |
| `K` | Kill selected agent (SIGTERM + 5s → SIGKILL) |
| `r` | Restart selected agent |
| `l` | Toggle log verbose mode |
| `p` | Show pipeline DAG tree |
| `?` | Display help legend |
| `q` | Quit (graceful shutdown) |

### Understanding Agent States

| State | Symbol | Meaning |
|-------|--------|---------|
| Pending | ○ | Waiting for dependencies |
| Running | ● | Executing subprocess |
| Done | ✓ | Completed successfully |
| Failed | ✗ | Exited with error |
| Killed | ⊗ | Manually terminated |
| Blocked | ⟳ | Dependency failed; skipped |

### Viewing Logs

**Live during execution**: Select agent in sidebar; logs stream in detail panel

**After execution**: Logs persist in JSONL format

```bash
# View audit logs
tail ~/.agentmux/logs/*.jsonl

# Parse JSONL (pretty-print with jq)
cat ~/.agentmux/logs/*.jsonl | jq '.'
```

---

## Troubleshooting

### Issue: "Agent not receiving output"

**Symptom**: Agent spawns but detail panel remains empty

**Cause**: CLI backend not invoked correctly (missing flags or binary not found)

**Solution**:

For **Claude** agents:
```bash
# Verify Claude CLI invocation includes --verbose and --output-format stream-json
claude -p "test" --output-format stream-json --verbose

# Check that Claude CLI is installed and in PATH
which claude
claude --version
```

For **Gemini** agents:
```bash
# Verify Gemini CLI invocation
gemini "test" --output-format stream-json --approval-mode auto_edit

# Check installation
which gemini
gemini --version
```

**Note**:
- Claude requires `--verbose` flag in print mode (`-p`) for `stream-json` format
- Gemini uses `--approval-mode auto_edit` for non-interactive operation
- Both CLIs must be in PATH or pipeline will fail at agent startup

### Issue: "DAG cycle detected"

**Symptom**: Error: "cycle detected in dependency graph"

**Cause**: Config has circular dependencies

**Solution**: Check `depends_on` fields for cycles:
```yaml
agents:
  agent_a:
    depends_on: [agent_b]
  agent_b:
    depends_on: [agent_a]  # ← CYCLE! Remove this
```

Visualize dependencies with:
```bash
agentmux run -c agentmux.yaml -v  # -v shows dependency tree
```

### Issue: "Config validation failed"

**Symptom**: Error: "missing required field: prompt"

**Cause**: Agent config missing required field

**Solution**: Verify all agents have `prompt` field:
```yaml
agents:
  my_agent:
    prompt: "Do something"  # ← Required!
    depends_on: [other_agent]
```

### Issue: "Process timeout / agent hangs"

**Symptom**: Agent runs forever; pipeline blocks

**Cause**: Claude CLI or agent logic enters infinite loop

**Solution**: Use interactive TUI to kill agent:
1. Navigate to agent in sidebar (j/k)
2. Press `K` to kill
3. TUI sends SIGTERM; agent has 5 seconds to exit
4. After 5s, SIGKILL forces termination
5. Pipeline resumes with blocked dependents

For v0.2.0, configurable timeouts will be added.

### Issue: "TUI rendering glitches"

**Symptom**: Terminal display corruption or missing text

**Cause**: Terminal too small or ANSI escape handling issue

**Solution**:
```bash
# Ensure terminal is at least 80×24
echo "Rows: $(tput lines), Cols: $(tput cols)"

# Try TERM override
TERM=xterm-256color agentmux run -c agentmux.yaml

# Clear terminal
clear
agentmux run -c agentmux.yaml
```

### Issue: "Logs not persisted"

**Symptom**: Audit logs missing from `~/.agentmux/logs/`

**Cause**: Log directory doesn't exist or permission denied

**Solution**:
```bash
# Create log directory if missing
mkdir -p ~/.agentmux/logs
chmod 755 ~/.agentmux/logs

# Verify permissions
ls -la ~/.agentmux/logs

# Run pipeline again
agentmux run -c agentmux.yaml
```

### Issue: "Agent state stuck in 'Pending'"

**Symptom**: Agent never transitions to Running; TUI shows ○

**Cause**: Dependency not marked as Done

**Solution**: Check if dependency agent failed:
1. Navigate to dependency agent in sidebar
2. Check state (should be ✓ Done)
3. If ✗ Failed, review its logs
4. Restart dependency with `r`

Blocked agents (⟳) appear when dependencies fail and are skipped.

### Issue: "Memory usage high with many agents"

**Symptom**: System slow; memory approaching limit

**Cause**: Large agent logs buffered in memory

**Solution**:
```bash
# Monitor memory during execution
watch 'ps aux | grep agentmux'

# Limit agents in single pipeline (recommended <20)
# For more, split into multiple sequential runs

# After execution, archive old logs
tar -czf ~/.agentmux/logs/archive-2026-02.tar.gz \
  ~/.agentmux/logs/*-2026-02-*.jsonl
rm ~/.agentmux/logs/*-2026-02-*.jsonl
```

---

## Performance Tuning

### TUI Responsiveness

If TUI feels laggy with 100+ events/second:

1. **Reduce event volume**: Simplify agent prompts (fewer tool calls)
2. **Separate agents**: Use multiple pipelines instead of one massive pipeline
3. **Monitor batch size**: Check scheduler event channel depth

### Parallel Execution

agentmux executes agents in topological order as soon as dependencies complete. To maximize parallelism:

```yaml
# GOOD: Agents independent after scout
agents:
  scout:
    prompt: "Analyze project"

  planner:
    depends_on: [scout]
    prompt: "Create plan"

  coder:
    depends_on: [planner]
    prompt: "Code"

  tester:
    depends_on: [coder]   # Only depends on coder, not scout/planner
    prompt: "Test"        # Runs in parallel with coder dependencies resolved

  reviewer:
    depends_on: [coder]   # Also only depends on coder
    prompt: "Review"      # Runs in parallel with tester
```

### Resource Allocation

Each agent is a subprocess with:
- Dedicated stdout/stderr pipes
- Independent stdout parser goroutine
- Shared TUI event channel

For 20+ agents, monitor:
```bash
# Watch goroutine count
go tool pprof http://localhost:6060/debug/pprof/goroutine

# Monitor file descriptors
lsof -p $(pgrep agentmux) | wc -l
```

---

## Advanced Configuration

### Environment Variables

Pass environment to agents via shell:

```yaml
defaults:
  model: "sonnet"

agents:
  example:
    prompt: |
      Read environment variable: $MY_VAR
    env:
      MY_VAR: "value"
      ANOTHER_VAR: "another"
```

### Working Directory

Each agent inherits parent's working directory:

```yaml
agents:
  example:
    prompt: "List current directory"
    workdir: "/path/to/specific/dir"
```

### Tool Restrictions

Limit tools to prevent accidental operations:

```yaml
defaults:
  allowedTools: ["Read", "Write", "Bash"]  # All agents can use these

agents:
  readonly_agent:
    prompt: "Analyze code"
    allowed_tools: ["Read", "Glob", "Grep"]  # This agent read-only
```

---

## Monitoring & Observability

### Real-Time Monitoring

During execution, the TUI shows:
- Agent list with state indicators
- Real-time log streaming for selected agent
- Progress bar (X/Y agents done)
- Running count

### Audit Logs

All events logged to JSONL:

```bash
cat ~/.agentmux/logs/*.jsonl | jq '.agent, .type, .timestamp'
```

Sample log entry:
```json
{
  "timestamp": "2026-02-25T10:30:45Z",
  "agent": "scout",
  "type": "agent_started",
  "event": "Agent started successfully"
}
```

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Pipeline completed (all agents Done) |
| 1 | Config error or validation failure |
| 2 | Runtime error (agent crash, I/O error) |
| 130 | User quit (Ctrl+C) |

---

## Maintenance

### Cleaning Up Old Logs

```bash
# Archive logs older than 30 days
find ~/.agentmux/logs -name "*.jsonl" -mtime +30 -exec tar -czf archive-old.tar.gz {} \;
find ~/.agentmux/logs -name "*.jsonl" -mtime +30 -delete
```

### Updating agentmux

```bash
# From source
git pull origin main
go build -o agentmux ./cmd/main.go
sudo mv agentmux /usr/local/bin/

# Or download new binary
# See Installation section above
```

---

## Getting Help

- **Documentation**: Read [README.md](../README.md), [System Architecture](./system-architecture.md)
- **Examples**: See `agentmux.yaml` in repository for 5-agent pipeline example
- **Issues**: Report bugs on [GitHub Issues](https://github.com/hoadh/agentmux/issues)
- **Discussions**: Ask questions on [GitHub Discussions](https://github.com/hoadh/agentmux/discussions)

---

**Document Version**: v0.1.0
**Last Updated**: 2026-02-25
