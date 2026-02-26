package cmd

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hoadh/agentmux/internal/agent"
	_ "github.com/hoadh/agentmux/internal/agent/backend"
	"github.com/hoadh/agentmux/internal/config"
	"github.com/hoadh/agentmux/internal/dag"
	"github.com/hoadh/agentmux/internal/tui"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Load config and launch TUI dashboard",
	RunE:  runApp,
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func runApp(cmd *cobra.Command, args []string) error {
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

	// Create and run TUI
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := tui.NewApp(mgr, sched, graph, ctx, cancel)
	p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion())

	_, err = p.Run()

	// Cleanup: stop all agents on exit
	mgr.StopAll()
	return err
}
