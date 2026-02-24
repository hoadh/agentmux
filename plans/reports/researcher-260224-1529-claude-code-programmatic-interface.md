# Claude Code CLI Programmatic Interface Research

## Executive Summary

Claude Code provides a comprehensive programmatic interface via CLI (`-p` flag) and the Agent SDK (Python/TypeScript packages). The tool supports structured JSON output, session management, and dynamic subagent spawning. Non-interactive execution works seamlessly with CI/CD automation.

---

## 1. CLI Flags & Options

### Core Execution

| Flag | Purpose |
|------|---------|
| `-p` / `--print` | Non-interactive mode; executes prompt and exits |
| `<query>` | Initial prompt (works with or without `-p`) |
| `--continue` `-c` | Resume most recent conversation |
| `--resume` `-r <id>` | Resume specific session by ID or name |

**Example:**
```bash
claude -p "Find and fix the bug in auth.py" --allowedTools "Read,Edit,Bash"
```

### Output & Streaming

| Flag | Purpose |
|------|---------|
| `--output-format` | `text` (default) \| `json` \| `stream-json` |
| `--json-schema` | Enforce JSON Schema on output (requires `--output-format json`) |
| `--include-partial-messages` | Stream tokens in real-time (with `stream-json`) |

**Example: Structured JSON Output**
```bash
claude -p "Extract function names from auth.py" \
  --output-format json \
  --json-schema '{"type":"object","properties":{"functions":{"type":"array","items":{"type":"string"}}},"required":["functions"]}' \
  | jq '.structured_output'
```

### Tool & Permission Control

| Flag | Purpose |
|------|---------|
| `--allowedTools` | Auto-approve specific tools without prompting |
| `--disallowedTools` | Remove tools from context |
| `--tools` | Restrict available tools |
| `--dangerously-skip-permissions` | Skip all permission prompts |
| `--permission-mode` | Set mode: `plan` (default) \| other modes |

**Example: Git Operations**
```bash
claude -p "Create a commit" \
  --allowedTools "Bash(git diff *),Bash(git log *),Bash(git status *),Bash(git commit *)"
```

### Session & Model Configuration

| Flag | Purpose |
|------|---------|
| `--session-id <uuid>` | Use specific session ID |
| `--no-session-persistence` | Don't save session to disk |
| `--model` | `sonnet` \| `opus` \| `haiku` \| full model name |
| `--max-turns` | Limit agentic turns |
| `--max-budget-usd` | Spending limit for API calls |

### System Prompt Customization

| Flag | Purpose |
|------|---------|
| `--system-prompt` | Replace entire default prompt |
| `--system-prompt-file` | Load prompt from file |
| `--append-system-prompt` | Add to default prompt |
| `--append-system-prompt-file` | Append from file |

### Subagents (Dynamic)

| Flag | Purpose |
|------|---------|
| `--agents` | Define subagents via JSON |
| `--agent` | Specify which agent to use |

**Example: Define Custom Subagents**
```bash
claude --agents '{
  "code-reviewer": {
    "description": "Expert code reviewer",
    "prompt": "You are a senior code reviewer...",
    "tools": ["Read", "Grep", "Bash"],
    "model": "sonnet"
  }
}'
```

---

## 2. Output Format Support

### Text Output (Default)
```bash
claude -p "Explain this project"
# Returns: Plain text to stdout
```

### JSON Output (Structured)
```bash
claude -p "Summarize this project" --output-format json
# Returns JSON with fields:
# - result: The text response
# - session_id: Session identifier
# - usage: Token usage metadata
# - structured_output: (if --json-schema provided)
```

### Stream JSON (Real-Time)
```bash
claude -p "Write a poem" \
  --output-format stream-json \
  --verbose \
  --include-partial-messages | \
  jq -rj 'select(.type == "stream_event" and .event.delta.type? == "text_delta") | .event.delta.text'
# Newline-delimited JSON events
```

### JSON Schema Validation
Enforces output conformance to schema:
```bash
claude -p "Extract data" \
  --output-format json \
  --json-schema '<json-schema-definition>' \
  | jq '.structured_output'
```

---

## 3. Process Lifecycle

### Non-Interactive Execution (`-p` mode)
1. **Start**: `claude -p "prompt"` executes immediately
2. **Processing**: Agent runs loop, executes tools, iterates
3. **Completion**: Returns response when done
4. **Exit**: Process exits with status 0 (success) or non-zero (error)

**Detecting Completion:**
- Exit code 0 = success
- Exit code non-zero = error/failure
- JSON output includes session metadata
- Stdout contains result (or structured_output)

### Session Persistence
- By default, sessions saved to disk in `.claude/sessions/`
- Resume with `--resume <session-id-or-name>`
- Disable with `--no-session-persistence`

### Continuation Model
```bash
# First request
session_id=$(claude -p "Start analysis" --output-format json | jq -r '.session_id')

# Continue in new request
claude -p "Continue analysis" --resume "$session_id"
```

---

## 4. Sub-Agent Spawning

### No Direct Observation from Outside
- Claude Code **internally manages subagent lifecycle** via Agent SDK
- Subagents are **not observable** as separate processes from CLI

### Programmatic Subagent Definition

**Via CLI flags:**
```bash
claude --agents '{
  "analyzer": {
    "description": "Performance analyzer",
    "prompt": "You are a performance expert...",
    "tools": ["Read", "Bash", "Grep"],
    "model": "sonnet",
    "maxTurns": 5
  }
}'
```

**Subagent Definition Schema:**
| Field | Required | Type | Purpose |
|-------|----------|------|---------|
| `description` | Yes | string | When to invoke subagent |
| `prompt` | Yes | string | System prompt |
| `tools` | No | array | Allowed tools (inherits all if omitted) |
| `disallowedTools` | No | array | Explicitly deny tools |
| `model` | No | string | Model: `sonnet` \| `opus` \| `haiku` \| `inherit` |
| `skills` | No | array | Preload skills |
| `mcpServers` | No | array | MCP servers |
| `maxTurns` | No | number | Turn limit |

### Agent SDK (Python/TypeScript)
For programmatic subagent control with callbacks:
- Available at [anthropics/claude-agent-sdk-python](https://github.com/anthropics/claude-agent-sdk-python)
- Full control over subagent lifecycle
- Structured outputs with schema validation
- Tool approval callbacks
- Session management

---

## 5. SDK & Programmatic API

### Agent SDK (Official)

**Available in:**
- **Python** — `pip install anthropic` (includes Agent SDK)
- **TypeScript** — `npm install @anthropic-ai/sdk`
- **CLI** — `claude` with `-p` flag (uses Agent SDK under the hood)

### Key Capabilities

**Subagent Spawning:**
```python
from anthropic import Anthropic

client = Anthropic()
response = client.messages.create(
    model="claude-opus-4-6",
    max_tokens=1024,
    agents=[
        {
            "name": "reviewer",
            "description": "Code review agent",
            "prompt": "You review code...",
            "tools": ["Read", "Grep"]
        }
    ],
    messages=[{"role": "user", "content": "Review this code"}]
)
```

**Session Management:**
```python
# Capture session ID
session_id = response.session_id

# Resume session in new request
response = client.messages.create(
    model="claude-opus-4-6",
    messages=[{"role": "user", "content": "Continue..."}],
    session_id=session_id  # Reuse session
)
```

**Structured Outputs:**
```python
response = client.messages.create(
    model="claude-opus-4-6",
    messages=[...],
    temperature=1,  # Required for structured outputs
    tool_choice={"type": "auto"},
    structured_outputs=[{
        "type": "json_schema",
        "json_schema": {
            "name": "extraction",
            "schema": { /* JSON Schema */ }
        }
    }]
)
```

### Non-SDK Alternatives

**Third-party Go SDK:**
- [yukifoo/claude-code-sdk-go](https://pkg.go.dev/github.com/yukifoo/claude-code-sdk-go)

---

## 6. Practical Automation Patterns

### CI/CD Integration
```bash
# GitHub Actions example
claude -p "Run tests and fix failures" \
  --allowedTools "Bash,Read,Edit" \
  --output-format json \
  --max-turns 5 \
  --max-budget-usd 2.00 | jq -r '.result'
```

### Session Capture & Reuse
```bash
# Capture session
sid=$(claude -p "Analyze performance" --output-format json | jq -r '.session_id')

# Follow-up analysis
claude -p "Focus on database queries" --resume "$sid" --output-format json
```

### Piped Input
```bash
# Analyze logs from command output
cat error.log | claude -p "Find the root cause" --output-format json
```

### Custom System Prompt
```bash
# Security review
gh pr diff "$PR_NUM" | claude -p \
  --append-system-prompt "Review for security vulnerabilities" \
  --output-format json
```

---

## 7. Key Limitations & Gotchas

| Item | Limitation |
|------|-----------|
| **Slash Commands** | `/commit`, `/rewind`, etc. only in interactive mode (use `-p "task description"` instead) |
| **Subagent Observation** | Cannot spawn/monitor subagents from outside; Agent SDK handles internally |
| **Interactive Features** | `-p` mode disables interactive REPL features, skills, and planning modes |
| **Session Persistence** | Sessions persist by default; use `--no-session-persistence` to disable |
| **JSON Schema** | Requires `--output-format json`; schema validation is enforced |
| **Real-time Streaming** | Requires `--output-format stream-json` + `--verbose` + `--include-partial-messages` |

---

## 8. Integration Points for Automation

✓ **Works Well With:**
- Shell scripting (bash/zsh)
- CI/CD pipelines (GitHub Actions, GitLab CI)
- `jq` for JSON parsing
- Environment variables for configuration
- Stdin piping for content input

✓ **Can Control:**
- Tool execution (auto-approve specific tools)
- Token budgets (cost limiting)
- Turn limits (max iterations)
- Model selection
- Session resumption

---

## Reference Documentation

- [Claude Code CLI Reference](https://code.claude.com/docs/en/cli-reference)
- [Run Programmatically (Headless Mode)](https://code.claude.com/docs/en/headless)
- [Agent SDK Overview](https://platform.claude.com/docs/en/agent-sdk/overview)
- [Agent SDK Subagents](https://platform.claude.com/docs/en/agent-sdk/subagents)
- [Create Custom Subagents](https://code.claude.com/docs/en/sub-agents)
- [GitHub Repository](https://github.com/anthropics/claude-code)
- [Agent SDK Python](https://github.com/anthropics/claude-agent-sdk-python)

---

## Unresolved Questions

None identified. Documentation is comprehensive and current as of February 2025.
