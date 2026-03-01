package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var StartTime time.Time

const Version = "v0.1.0"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("agentmux", Version)
	},
}

func init() {
	StartTime = time.Now()
	rootCmd.AddCommand(versionCmd)
}
