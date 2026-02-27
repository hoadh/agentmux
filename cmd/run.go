package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hoadh/agentmux/internal/agent"
	_ "github.com/hoadh/agentmux/internal/agent/backend"
	"github.com/hoadh/agentmux/internal/config"
	"github.com/hoadh/agentmux/internal/dag"
	"github.com/hoadh/agentmux/internal/headless"
	"github.com/hoadh/agentmux/internal/tui"
	"github.com/spf13/cobra"
)

var (
	headlessMode bool
	outputFormat string
	outputDir    string
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Load config and run pipeline (TUI or headless)",
	RunE:  runApp,
}

func init() {
	runCmd.Flags().BoolVar(&headlessMode, "headless", false, "run pipeline without TUI, streaming events to stdout")
	runCmd.Flags().StringVar(&outputFormat, "format", "text", "output format: text or ndjson (requires --headless)")
	runCmd.Flags().StringVar(&outputDir, "output-dir", ".agentmux-out", "output directory for logs and results")
	rootCmd.AddCommand(runCmd)
}

func runApp(cmd *cobra.Command, args []string) error {
	// Validate format flag early before expensive config/DAG work
	if outputFormat != "text" && outputFormat != "ndjson" {
		return fmt.Errorf("invalid format %q: must be 'text' or 'ndjson'", outputFormat)
	}

	cfgPath, _ := cmd.Flags().GetString("config")

	// Load config (optional; empty config = manual-only mode)
	cfg, warnings, err := config.LoadConfig(cfgPath)
	if err != nil && cfgPath != "agentmux.yaml" {
		return fmt.Errorf("config error: %w", err)
	}
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}

	// Resolve output directory: CLI flag > YAML config > default
	if cmd.Flags().Changed("output-dir") {
		cfg.OutputDir = outputDir
	}
	if cfg.OutputDir == "" {
		cfg.OutputDir = ".agentmux-out"
	}
	logsDir := filepath.Join(cfg.OutputDir, "logs")
	resultsDir := filepath.Join(cfg.OutputDir, "results")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("create logs dir: %w", err)
	}
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		return fmt.Errorf("create results dir: %w", err)
	}

	// Build DAG
	graph, err := dag.BuildFromConfig(cfg)
	if err != nil {
		return fmt.Errorf("pipeline error: %w", err)
	}

	// Create manager and register agents in config order
	mgr := agent.NewManager(cfg.Defaults)
	mgr.SetOrder(cfg.AgentOrder)
	for name, agentCfg := range cfg.Agents {
		mgr.Register(name, agentCfg)
	}

	// Create scheduler
	sched := dag.NewScheduler(graph, mgr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if headlessMode {
		opts := headless.Options{
			Format:     outputFormat,
			LogsDir:    logsDir,
			ResultsDir: resultsDir,
		}
		runner := headless.New(mgr, sched, opts)
		if exitCode := runner.Run(ctx, cancel); exitCode != 0 {
			os.Exit(exitCode)
		}
		return nil
	}

	// TUI mode
	app := tui.NewApp(mgr, sched, graph, ctx, cancel, logsDir, resultsDir)
	p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion())

	_, err = p.Run()

	// Cleanup: stop all agents on exit
	mgr.StopAll()
	return err
}
