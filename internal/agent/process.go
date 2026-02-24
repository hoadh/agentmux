package agent

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hoadh/agentmux/internal/config"
)

// Process wraps an os/exec command for a claude CLI agent.
type Process struct {
	Name     string
	Cmd      *exec.Cmd
	Stdout   io.ReadCloser
	Stderr   io.ReadCloser
	EventCh  chan tea.Msg
	cancel   context.CancelFunc
	done     chan struct{}
	exitCode int
	stderr   strings.Builder
	mu       sync.Mutex
}

// NewProcess creates a new process for the given agent config.
func NewProcess(name string, cfg config.AgentConfig, defaults config.AgentDefaults) *Process {
	return &Process{
		Name:    name,
		EventCh: make(chan tea.Msg, 256),
		done:    make(chan struct{}),
	}
}

// Start launches the claude CLI process and begins parsing output.
func (p *Process) Start(cfg config.AgentConfig, defaults config.AgentDefaults) error {
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel

	args := buildArgs(cfg, defaults)
	p.Cmd = exec.CommandContext(ctx, "claude", args...)
	p.Cmd.Dir = cfg.WorkDir

	var err error
	p.Stdout, err = p.Cmd.StdoutPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("stdout pipe: %w", err)
	}

	p.Stderr, err = p.Cmd.StderrPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := p.Cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("start claude: %w", err)
	}

	// Parse stdout NDJSON in goroutine
	go ParseStream(p.Name, p.Stdout, p.EventCh)

	// Drain stderr in goroutine
	go func() {
		buf, _ := io.ReadAll(p.Stderr)
		p.mu.Lock()
		p.stderr.Write(buf)
		p.mu.Unlock()
	}()

	// Wait for process exit in goroutine
	go func() {
		err := p.Cmd.Wait()
		code := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				code = exitErr.ExitCode()
			} else {
				code = -1
			}
		}
		p.mu.Lock()
		p.exitCode = code
		p.mu.Unlock()
		select {
		case p.EventCh <- AgentDoneEvent{AgentName: p.Name, ExitCode: code}:
		default:
		}
		close(p.done)
		close(p.EventCh)
	}()

	return nil
}

// Shutdown sends SIGTERM, waits 5s, then SIGKILL if still running.
func (p *Process) Shutdown() error {
	if p.Cmd == nil || p.Cmd.Process == nil {
		return nil
	}

	// Send SIGTERM
	if err := p.Cmd.Process.Signal(syscall.SIGTERM); err != nil {
		return err
	}

	// Wait for clean exit or force kill
	select {
	case <-p.done:
		return nil
	case <-time.After(5 * time.Second):
		return p.Cmd.Process.Kill()
	}
}

// Wait blocks until the process exits and returns the exit code.
func (p *Process) Wait() int {
	<-p.done
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.exitCode
}

// StderrOutput returns captured stderr.
func (p *Process) StderrOutput() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stderr.String()
}

// buildArgs constructs CLI arguments for the claude command.
func buildArgs(cfg config.AgentConfig, defaults config.AgentDefaults) []string {
	args := []string{"-p", cfg.Prompt, "--output-format", "stream-json", "--verbose"}

	model := cfg.Model
	if model == "" {
		model = defaults.Model
	}
	if model != "" {
		args = append(args, "--model", model)
	}

	tools := cfg.AllowedTools
	if len(tools) == 0 {
		tools = defaults.AllowedTools
	}
	if len(tools) > 0 {
		for _, t := range tools {
			args = append(args, "--allowedTools", t)
		}
	}

	maxTurns := cfg.MaxTurns
	if maxTurns == 0 {
		maxTurns = defaults.MaxTurns
	}
	if maxTurns > 0 {
		args = append(args, "--max-turns", strconv.Itoa(maxTurns))
	}

	return args
}
