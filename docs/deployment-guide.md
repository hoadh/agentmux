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

Use the interactive install script:
```bash
./scripts/install.sh ./agentmux
```

**Features**:
- Validates executable; prompts to make executable if needed
- Offers installation targets: `/usr/local/bin`, `~/.local/bin`, or custom path
- Handles permission elevation (sudo) automatically
- Verifies successful installation
- Provides PATH configuration hints

**Install with custom name**:
```bash
./scripts/install.sh ./agentmux --name agent-orchestrator
```

Alternatively, install manually:
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
agentmux run --help    # Show run command usage
agentmux serve --help  # Show serve command usage
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

### CLI Flags Reference

| Flag | Shorthand | Type | Default | Purpose |
|------|-----------|------|---------|---------|
| `--config` | `-c` | string | `agentmux.yaml` | Path to YAML config file |
| `--var` | — | string (repeatable) | — | Template variable override; format: `key=value` |
| `--output-dir` | — | string | `.agentmux-out` | Custom root directory for logs/results |
| `--headless` | — | bool | false | Run without TUI |
| `--format` | — | string | text | Output format: `text` or `ndjson` (headless only) |

**Precedence**: CLI flags override YAML values.

### Basic Execution

```bash
# Default: config = agentmux.yaml
agentmux run

# Specify config file (shorthand)
agentmux run -c pipeline.yaml

# With template variable override
agentmux run -c pipeline.yaml --var project_root=/custom/path

# Multiple variable overrides
agentmux run -c pipeline.yaml \
  --var topic="Machine Learning" \
  --var style="academic" \
  --var output_dir="./ml-results"
```

This:
1. Loads and validates config
2. Applies CLI variable overrides
3. Builds DAG from dependencies
4. Spawns agents in topological order
5. Displays interactive TUI dashboard
6. Saves audit logs to configured output directory

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

---

## Headless Mode (Non-TUI Execution)

For automation, CI/CD integration, or batch processing without interactive terminal, use headless mode.

### Basic Usage

```bash
agentmux run -c agentmux.yaml --headless
```

This:
1. Loads and validates config
2. Builds DAG from dependencies
3. Spawns agents in topological order (same as TUI mode)
4. Streams agent output to stdout in real-time
5. Exits with success code (0) if all agents done, error code (1+) if any failed
6. Saves audit logs to `~/.agentmux/logs/` (same as TUI)

### Output Formats

#### Plain Text (Default)

```bash
agentmux run -c agentmux.yaml --headless
```

Output:
```
[2026-02-25T10:30:45Z] scout: Agent started
[2026-02-25T10:30:46Z] scout: Processing...
[2026-02-25T10:30:52Z] scout: Completed
[2026-02-25T10:30:52Z] planner: Agent started (depending on scout)
...
```

#### JSON Lines (NDJSON)

```bash
agentmux run -c agentmux.yaml --headless --output json
```

Output (one JSON object per line):
```json
{"timestamp":"2026-02-25T10:30:45Z","agent":"scout","type":"started","message":"Agent started"}
{"timestamp":"2026-02-25T10:30:46Z","agent":"scout","type":"output","message":"Processing..."}
...
```

### Integration with CI/CD

#### GitHub Actions

```yaml
name: Run Agent Pipeline
on: [push]

jobs:
  pipeline:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Install Go
        uses: actions/setup-go@v4
        with:
          go-version: 1.24

      - name: Build agentmux
        run: go build -o agentmux ./cmd/main.go

      - name: Run pipeline
        run: ./agentmux run -c agentmux.yaml --headless
        env:
          CLAUDE_API_KEY: ${{ secrets.CLAUDE_API_KEY }}
```

#### Docker

```dockerfile
FROM golang:1.24-alpine

RUN apk add --no-cache curl

# Install Claude CLI
RUN curl -fsSL https://install.claudeai.com | sh

WORKDIR /app
COPY . .

RUN go build -o agentmux ./cmd/main.go

CMD ["./agentmux", "run", "-c", "agentmux.yaml", "--headless"]
```

### Cron Job Example

```bash
#!/bin/bash
# Run daily analysis pipeline at 2 AM
# Add to crontab: 0 2 * * * /usr/local/bin/run-analysis.sh

export PATH="/usr/local/bin:$PATH"
export CLAUDE_API_KEY="your-api-key"

cd /home/user/projects/analysis
agentmux run -c agentmux.yaml --headless >> /tmp/pipeline.log 2>&1

if [ $? -eq 0 ]; then
  echo "Pipeline succeeded" | mail -s "Daily Analysis Complete" admin@example.com
else
  echo "Pipeline failed" | mail -s "Daily Analysis Failed" admin@example.com
fi
```

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Pipeline completed; all agents done |
| 1 | Config error or validation failure |
| 2 | Runtime error (agent crash, I/O error) |
| 130 | Interrupted (Ctrl+C or SIGTERM) |

### Signal Handling

- **SIGTERM**: Graceful shutdown (same as TUI; agents receive SIGTERM, 5s timeout, then SIGKILL)
- **SIGINT** (Ctrl+C): Immediate graceful shutdown
- **SIGKILL**: Forceful termination (no cleanup possible)

### Configuration

Headless mode reuses the same `agentmux.yaml` as TUI mode. See [Configuration Reference](./configuration.md) for full schema.

**No additional headless-specific config needed**. All agents run identically; only the UI mode changes.

### Troubleshooting Headless Mode

#### Pipeline runs but produces no output

**Symptom**: `agentmux run --headless` exits immediately with no output

**Cause**: Agent prompts are too simple or agents complete instantly

**Solution**: Check agent logs:
```bash
tail ~/.agentmux/logs/*.jsonl | jq '.agent, .type, .message'
```

#### Exit code 1 but no visible error

**Cause**: Config validation error

**Solution**: Run with verbose output:
```bash
agentmux run -c agentmux.yaml --headless -v
```

#### SIGTERM not killing agents

**Cause**: Child process ignoring SIGTERM (rare; usually subprocess hangs)

**Solution**: agentmux will forcefully SIGKILL after 5 seconds (same as TUI)

### Viewing Logs

**After execution**: Logs persist in JSONL format

```bash
# View audit logs
tail ~/.agentmux/logs/*.jsonl

# Parse JSONL (pretty-print with jq)
cat ~/.agentmux/logs/*.jsonl | jq '.'

# Filter by agent
cat ~/.agentmux/logs/*.jsonl | jq 'select(.agent == "scout")'
```

---

## Interactive TUI Mode

### Viewing Logs (TUI)

**Live during execution**: Select agent in sidebar; logs stream in detail panel

**After execution**: Logs persist in JSONL format

---

## Batch Execution (Multiple Pipelines in Parallel)

Run multiple agentmux pipelines in parallel with job limiting and summary reporting using `scripts/parallel-run.sh`.

### Overview

`parallel-run.sh` is a POSIX shell wrapper that:
- Spawns multiple pipelines concurrently (respects job limit)
- Isolates output directories per pipeline
- Tracks per-pipeline duration and pass/fail status
- Outputs summary table on completion
- Cleans up child processes on Ctrl+C

### Inline Mode

Run pipelines defined on the command line (separated by `--`):

```bash
./scripts/parallel-run.sh -c config.yaml \
  -- --var topic="machine-learning" \
  -- --var topic="web-development" \
  -- --var topic="devops"
```

Each `--var` line becomes a separate pipeline invocation. Output goes to `.agentmux-out/machine-learning/`, `.agentmux-out/web-development/`, etc.

### Manifest Mode

Define pipelines in a text file (one spec per line):

```bash
# pipelines.txt
--var topic="machine-learning"
--var topic="web-development"
--var topic="devops"

./scripts/parallel-run.sh -c config.yaml -f pipelines.txt
```

### Job Limiting

Control parallelism with `-j`:

```bash
# Default: 4 parallel jobs
./scripts/parallel-run.sh -c config.yaml -f pipelines.txt

# Custom limit: 8 parallel jobs
./scripts/parallel-run.sh -c config.yaml -f pipelines.txt -j 8

# Serial execution: 1 job at a time
./scripts/parallel-run.sh -c config.yaml -f pipelines.txt -j 1
```

### Output Directory

Customize base output directory with `-o`:

```bash
./scripts/parallel-run.sh -c config.yaml -f pipelines.txt -o ./batch-results
```

Creates: `./batch-results/machine-learning/`, `./batch-results/web-development/`, etc.

### Summary Report

After completion, displays:

```
=== Parallel Run Summary ===
  #    Pipeline                 Status     Duration   Output
  1    machine-learning         ✓ pass     45s        .agentmux-out/machine-learning/
  2    web-development          ✓ pass     38s        .agentmux-out/web-development/
  3    devops                   ✗ fail     29s        .agentmux-out/devops/
Results: 2 passed, 1 failed (3 total)
```

### CI/CD Integration

#### GitHub Actions Example

```yaml
name: Batch Pipeline
on: [push]

jobs:
  batch:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Install Go
        uses: actions/setup-go@v4
        with:
          go-version: 1.24

      - name: Build agentmux
        run: go build -o agentmux ./cmd/main.go

      - name: Create pipeline manifest
        run: |
          cat > pipelines.txt <<EOF
          --var topic="docs-generation"
          --var topic="code-review"
          --var topic="test-strategy"
          EOF

      - name: Run batch pipelines
        run: ./scripts/parallel-run.sh -c agentmux.yaml -f pipelines.txt -j 3
        env:
          CLAUDE_API_KEY: ${{ secrets.CLAUDE_API_KEY }}

      - name: Upload results
        if: always()
        uses: actions/upload-artifact@v3
        with:
          name: pipeline-results
          path: .agentmux-out/
```

#### Cron Job for Daily Batch Processing

```bash
#!/bin/bash
# File: /usr/local/bin/daily-batch-pipelines.sh

export PATH="/usr/local/bin:$PATH"
export CLAUDE_API_KEY="your-api-key"

cd /home/user/projects/batch-analysis

# Define pipelines
cat > pipelines.txt <<EOF
--var dataset="sales-q1"
--var dataset="sales-q2"
--var dataset="sales-q3"
--var dataset="sales-q4"
EOF

# Run up to 4 pipelines in parallel
/usr/local/bin/agentmux-parallel -c batch.yaml -f pipelines.txt -j 4 -o ./quarterly-results

# Send summary via email
if [ $? -eq 0 ]; then
  echo "Batch processing succeeded" | mail -s "Daily Batch Complete" admin@example.com
else
  echo "Batch processing failed; check logs" | mail -s "Daily Batch Failed" admin@example.com
fi
```

Add to crontab:
```bash
# Run at 2 AM daily
0 2 * * * /usr/local/bin/daily-batch-pipelines.sh >> /var/log/batch-pipelines.log 2>&1
```

---

## Health Check Server

Start an HTTP server for monitoring and orchestration platform integration.

### Basic Usage

```bash
# Start health server on default port :8080
agentmux serve

# Start on custom address
agentmux serve -a :9000
agentmux serve --addr localhost:8080
```

### Health Endpoint

The `/health` endpoint returns a JSON health report:

```bash
curl http://localhost:8080/health
```

Response:
```json
{
  "status": "healthy",
  "version": "v0.1.0",
  "uptime": "2h30m15s",
  "started_at": "2026-03-01T08:15:30Z"
}
```

### Kubernetes Integration Example

Health check for Kubernetes readiness probe:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: agentmux-monitor
spec:
  containers:
  - name: agentmux
    image: agentmux:v0.1.0
    command: ["agentmux", "serve", "--addr", ":8080"]
    ports:
    - containerPort: 8080
    readinessProbe:
      httpGet:
        path: /health
        port: 8080
      initialDelaySeconds: 2
      periodSeconds: 10
```

### Load Balancer Health Check

For health-based load balancing:

```bash
# Start agentmux serve in the background
agentmux serve -a :8080 &

# Monitor health status
watch -n 5 'curl -s http://localhost:8080/health | jq .'
```

---

## Getting Help

**Troubleshooting**: For common issues and solutions, see [Troubleshooting Guide](./troubleshooting.md).

**Resources**:
- **Documentation**: [README.md](../README.md), [System Architecture](./system-architecture.md), [Configuration Reference](./configuration.md)
- **Examples**: See `agentmux.yaml` in repository for 5-agent pipeline example
- **Issues**: Report bugs on [GitHub Issues](https://github.com/hoadh/agentmux/issues)
- **Discussions**: Ask questions on [GitHub Discussions](https://github.com/hoadh/agentmux/discussions)

---

**Document Version**: v0.1.0
**Last Updated**: 2026-03-01
**Latest Features**: Health check server, Batch execution script, Headless mode CI/CD examples
