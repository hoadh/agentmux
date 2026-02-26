# agentmux Troubleshooting Guide

Comprehensive guide for diagnosing and resolving common issues with agentmux pipelines.

## Troubleshooting Issues

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

**Document Version**: v0.1.0
**Last Updated**: 2026-02-27
