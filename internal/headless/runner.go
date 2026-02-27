package headless

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hoadh/agentmux/internal/agent/backend"
	"github.com/hoadh/agentmux/internal/dag"
	"github.com/hoadh/agentmux/internal/log"
	"github.com/hoadh/agentmux/internal/result"
)

// AgentManager defines the manager methods used by the headless runner.
type AgentManager interface {
	UpdateLastEvent(name string, summary string)
	UpdateDuration(name string)
	UpdateTokens(name string, input, output int)
	StopAll()
}

// PipelineScheduler abstracts the scheduler for testability.
type PipelineScheduler interface {
	Run(ctx context.Context)
	EventCh() <-chan tea.Msg
}

// Options configures headless mode.
type Options struct {
	Format     string    // "text" or "ndjson"
	Output     io.Writer // default: os.Stdout
	LogsDir    string    // directory for JSONL log files
	ResultsDir string    // directory for agent result files
}

// Runner executes a pipeline without TUI, streaming events to stdout.
type Runner struct {
	manager      AgentManager
	scheduler    PipelineScheduler
	formatter    Formatter
	output       io.Writer
	logWriter    *log.Writer
	resultWriter *result.Writer
}

// New creates a headless runner.
func New(mgr AgentManager, sched PipelineScheduler, opts Options) *Runner {
	out := opts.Output
	if out == nil {
		out = os.Stdout
	}
	var f Formatter
	switch opts.Format {
	case "ndjson":
		f = &NDJSONFormatter{}
	default:
		f = &TextFormatter{}
	}
	logWriter, err := log.NewWriter(opts.LogsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: log writer: %v\n", err)
	}
	resultWriter, err := result.NewWriter(opts.ResultsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: result writer: %v\n", err)
	}
	return &Runner{
		manager:      mgr,
		scheduler:    sched,
		formatter:    f,
		output:       out,
		logWriter:    logWriter,
		resultWriter: resultWriter,
	}
}

// Run starts the pipeline and blocks until completion. Returns an exit code.
func (r *Runner) Run(ctx context.Context, cancel context.CancelFunc) int {
	// Signal handling — buffered channel avoids data race between goroutines
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sigReceived := make(chan os.Signal, 1)
	go func() {
		select {
		case sig := <-sigCh:
			sigReceived <- sig
			cancel()
		case <-ctx.Done():
		}
		signal.Stop(sigCh)
	}()

	// Start scheduler
	go r.scheduler.Run(ctx)

	defer func() {
		r.manager.StopAll()
		r.cleanup()
	}()

	// Event loop
	eventCh := r.scheduler.EventCh()
	for {
		select {
		case <-ctx.Done():
			select {
			case sig := <-sigReceived:
				if sig == syscall.SIGINT {
					return 130
				}
				if sig == syscall.SIGTERM {
					return 143
				}
			default:
			}
			return 1
		case msg := <-eventCh:
			r.handleEvent(msg)
			if done, ok := msg.(dag.PipelineDoneMsg); ok {
				if done.Success {
					return 0
				}
				return 1
			}
		}
	}
}

// handleEvent formats and outputs the event, then updates manager state.
func (r *Runner) handleEvent(msg tea.Msg) {
	line := r.formatter.FormatEvent(msg)
	if line != "" {
		fmt.Fprintln(r.output, line)
	}
	switch evt := msg.(type) {
	case backend.AssistantEvent:
		r.manager.UpdateLastEvent(evt.AgentName, "writing...")
		r.manager.UpdateDuration(evt.AgentName)
		r.logEvent(evt.AgentName, evt)
	case backend.ToolUseEvent:
		r.manager.UpdateLastEvent(evt.AgentName, fmt.Sprintf("Tool: %s", evt.ToolName))
		r.manager.UpdateDuration(evt.AgentName)
		r.logEvent(evt.AgentName, evt)
	case backend.ToolResultEvent:
		r.logEvent(evt.AgentName, evt)
	case backend.ResultEvent:
		r.manager.UpdateTokens(evt.AgentName, evt.InputTokens, evt.OutputTokens)
		r.logEvent(evt.AgentName, evt)
		if r.resultWriter != nil && evt.Result != "" {
			if err := r.resultWriter.Write(evt.AgentName, evt.Result); err != nil {
				fmt.Fprintf(os.Stderr, "warning: write result %q: %v\n", evt.AgentName, err)
			}
		}
	case backend.AgentDoneEvent:
		r.manager.UpdateDuration(evt.AgentName)
		r.logEvent(evt.AgentName, evt)
	case backend.ErrorEvent:
		r.logEvent(evt.AgentName, evt)
	}
}

func (r *Runner) logEvent(agentName string, event interface{}) {
	if r.logWriter != nil {
		r.logWriter.Write(agentName, event)
	}
}

func (r *Runner) cleanup() {
	if r.logWriter != nil {
		r.logWriter.Close()
	}
}
