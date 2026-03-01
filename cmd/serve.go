package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/hoadh/agentmux/internal/health"
	"github.com/spf13/cobra"
)

var addr string

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start HTTP server with health endpoint",
	RunE: func(cmd *cobra.Command, args []string) error {
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			report := health.Check(Version, StartTime)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(report)
		})

		fmt.Fprintf(os.Stderr, "Listening on %s\n", addr)
		return http.ListenAndServe(addr, nil)
	},
}

func init() {
	serveCmd.Flags().StringVarP(&addr, "addr", "a", ":8080", "listen address")
	rootCmd.AddCommand(serveCmd)
}
