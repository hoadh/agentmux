package cmd

import (
	"fmt"
	"os"

	"github.com/hoadh/agentmux/internal/health"
	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check backend CLI availability",
	Run: func(cmd *cobra.Command, args []string) {
		results := health.CheckBackends()
		allOK := true
		for _, b := range results {
			icon := "✓"
			status := "found"
			if !b.Available {
				icon = "✗"
				status = "not found"
				allOK = false
			}
			fmt.Printf("  %s %s: %s\n", icon, b.Name, status)
		}
		if !allOK {
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)
}
