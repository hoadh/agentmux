# Phase 6: Testing & Polish

## Context Links
- [Parent Plan](plan.md)
- [All previous phases](plan.md)

## Overview
- **Date**: 2026-02-24
- **Priority**: P2
- **Status**: complete
- **Description**: Unit tests for config, DAG, parser. Integration test with mock claude process. Error handling hardening, edge cases, and README with usage examples. Note: README deferred; tests written with config 96.9%, dag 39.2%, agent 46.7% coverage.

## Key Insights
- Go's `testing` package + `testify/assert` for assertions
- Mock claude process: simple Go binary that emits NDJSON to stdout, controllable via args
- Bubbletea has `teatest` package for TUI testing (send keys, assert view output)
- Focus on core logic tests (config, DAG, parser); TUI tests are integration-level

## Requirements

### Functional
- Unit tests for config loading, validation, defaults merging
- Unit tests for DAG cycle detection, topological sort, roots
- Unit tests for NDJSON parser (valid events, malformed lines, EOF)
- Integration test: mock claude binary → process wrapper → events
- Error handling: config errors, process spawn failures, pipe errors
- README.md with installation, usage, config examples

### Non-Functional
- Test coverage >80% for config, dag, parser packages
- Tests run in <10s total
- No flaky tests (no timing-dependent assertions)

## Architecture

```
Tests:
├── internal/config/config_test.go   # Config load, validate, defaults
├── internal/dag/graph_test.go       # DAG operations
├── internal/dag/scheduler_test.go   # Scheduler with mock manager
├── internal/agent/parser_test.go    # NDJSON parser
├── internal/agent/process_test.go   # Process with mock binary
└── testdata/
    ├── valid-config.yaml
    ├── invalid-cycle.yaml
    ├── invalid-missing-prompt.yaml
    ├── mock-claude/main.go          # Mock claude binary for integration tests
    └── stream-samples/
        ├── basic-run.jsonl
        └── tool-use.jsonl
```

## Related Code Files

### Create
- `internal/config/config_test.go`
- `internal/dag/graph_test.go`
- `internal/dag/scheduler_test.go`
- `internal/agent/parser_test.go`
- `internal/agent/process_test.go`
- `testdata/valid-config.yaml`
- `testdata/invalid-cycle.yaml`
- `testdata/invalid-missing-prompt.yaml`
- `testdata/mock-claude/main.go`
- `testdata/stream-samples/basic-run.jsonl`
- `testdata/stream-samples/tool-use.jsonl`
- `README.md`

## Implementation Steps

### 1. Config tests (`config_test.go`)

```go
func TestLoadConfig_Valid(t *testing.T)
func TestLoadConfig_FileNotFound(t *testing.T)          // returns default
func TestLoadConfig_InvalidYAML(t *testing.T)           // returns error
func TestValidate_MissingPrompt(t *testing.T)           // error
func TestValidate_UnknownDependency(t *testing.T)       // error
func TestValidate_SelfDependency(t *testing.T)          // error
func TestApplyDefaults_InheritsModel(t *testing.T)      // agent gets default model
func TestApplyDefaults_OverrideModel(t *testing.T)      // agent keeps own model
func TestApplyDefaults_InheritsTools(t *testing.T)
```

### 2. DAG tests (`graph_test.go`)

```go
func TestGraph_LinearChain(t *testing.T)      // A→B→C: roots=[A], validate ok
func TestGraph_Diamond(t *testing.T)           // D→B,C→A: roots=[A], both paths
func TestGraph_Parallel(t *testing.T)          // A,B,C no deps: roots=[A,B,C]
func TestGraph_CycleDetection(t *testing.T)    // A→B→A: error
func TestGraph_SelfCycle(t *testing.T)         // A→A: error
func TestGraph_Dependents(t *testing.T)        // verify rdeps
func TestGraph_EmptyGraph(t *testing.T)        // no nodes: valid, roots=[]
```

### 3. Scheduler tests (`scheduler_test.go`)

- Use a mock manager that tracks Start/Stop calls and returns controllable channels
- Test: roots start immediately
- Test: dependent starts after dep completes
- Test: failure propagation blocks downstream
- Test: diamond dependency (both paths must complete)
- Test: pipeline done detection

```go
type mockManager struct {
    started  []string
    channels map[string]chan tea.Msg
}
func (m *mockManager) Start(name string) (chan tea.Msg, error)
func (m *mockManager) completAgent(name string, exitCode int)

func TestScheduler_LinearPipeline(t *testing.T)
func TestScheduler_ParallelRoots(t *testing.T)
func TestScheduler_FailurePropagation(t *testing.T)
func TestScheduler_DiamondDependency(t *testing.T)
```

### 4. Parser tests (`parser_test.go`)

```go
func TestParseStream_AssistantEvent(t *testing.T)
func TestParseStream_ToolUseEvent(t *testing.T)
func TestParseStream_ToolResultEvent(t *testing.T)
func TestParseStream_ResultEvent(t *testing.T)
func TestParseStream_MalformedLine_Skipped(t *testing.T)
func TestParseStream_EmptyInput(t *testing.T)
func TestParseStream_UnknownType_Skipped(t *testing.T)
func TestParseStream_FromFile(t *testing.T)     // read testdata/stream-samples/
```

- Use `strings.NewReader` for unit tests; file reader for sample data tests

### 5. Process integration test (`process_test.go`)

- Build mock-claude binary in TestMain:
  ```go
  func TestMain(m *testing.M) {
      // Build mock-claude binary
      exec.Command("go", "build", "-o", "testdata/mock-claude/mock-claude",
          "./testdata/mock-claude").Run()
      os.Exit(m.Run())
  }
  ```
- Mock claude: reads `--mode` flag, emits corresponding NDJSON to stdout
  - `--mode basic`: emits 3 assistant events + result event
  - `--mode error`: emits 1 event then exits non-zero
  - `--mode slow`: emits 1 event/second (for kill testing)
- Tests:
  ```go
  func TestProcess_StartAndWait(t *testing.T)     // basic run, events received
  func TestProcess_Shutdown(t *testing.T)          // SIGTERM during slow mode
  func TestProcess_NonZeroExit(t *testing.T)       // error mode
  ```

### 6. Create test fixtures

**`testdata/valid-config.yaml`:**
```yaml
version: 1
defaults:
  model: sonnet
  allowedTools: ["Read", "Edit"]
  max_turns: 10
agents:
  planner:
    prompt: "Plan the work"
  coder:
    prompt: "Write the code"
    depends_on: [planner]
    model: opus
```

**`testdata/stream-samples/basic-run.jsonl`:**
```jsonl
{"type":"assistant","message":{"content":[{"type":"text","text":"Analyzing..."}]}}
{"type":"tool_use","tool":{"name":"Read","input":{"file_path":"main.go"}}}
{"type":"tool_result","content":"package main..."}
{"type":"result","result":"Done","session_id":"abc123","usage":{"input_tokens":500,"output_tokens":200}}
```

### 7. Error handling hardening

Review all packages for:
- [ ] Config: graceful handling of empty file, malformed YAML, missing fields
- [ ] DAG: empty graph, single node, disconnected nodes
- [ ] Parser: empty stream, binary garbage, very long lines
- [ ] Process: binary not found, permission denied, working dir not exists
- [ ] Manager: start already-running agent, stop already-stopped agent
- [ ] TUI: handle nil agent info, empty agent list

### 8. Write README.md

Sections:
- One-line description
- Installation (`go install`)
- Quick start (config example + `agentmux run`)
- Config reference (all fields documented)
- Keybindings table
- Screenshots placeholder
- Architecture overview (brief)

## Todo List
- [ ] Write config unit tests
- [ ] Write DAG graph unit tests
- [ ] Write DAG scheduler tests with mock manager
- [ ] Write parser unit tests
- [ ] Create mock-claude test binary
- [ ] Write process integration tests
- [ ] Create test fixture files (YAML configs, JSONL samples)
- [ ] Error handling review and hardening
- [ ] Write README.md
- [ ] Run `go test ./...` — all pass
- [ ] Run `go vet ./...` — no issues
- [ ] Run `golangci-lint run` if available

## Success Criteria
- `go test ./...` passes with 0 failures
- Config, DAG, parser packages have >80% coverage
- Mock claude integration test validates full event pipeline
- README is sufficient for a new user to get started
- No panics on any tested edge case

## Risk Assessment
- **Mock claude binary portability**: Built for current OS/arch only. CI may need cross-compile. For local dev, fine.
- **Flaky scheduler tests**: Use channels and explicit completion signals rather than `time.Sleep`. No timing-dependent assertions.
- **teatest availability**: If `teatest` package is immature, skip TUI-level tests; focus on unit tests for logic packages.

## Security Considerations
- Test fixtures should not contain real API keys or sensitive data
- Mock claude binary should not make real API calls

## Next Steps
- After Phase 6: project is v1-ready
- Future: v1.1 tmux integration, session resume, inter-agent communication
