package cmd

import (
	"github.com/spf13/cobra"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "agentmux",
	Short: "TUI for spawning and orchestrating Claude Code agents",
	Long:  "Interactive TUI to spawn, monitor, kill, and orchestrate Claude Code agents via stream-json pipes with DAG-based pipeline support.",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "agentmux.yaml", "config file path")
}
