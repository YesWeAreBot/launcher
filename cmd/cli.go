// Package cmd defines the yesimbot command tree (init/start/stop/status).
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is injected at build time via
// -ldflags "-X github.com/YesWeAreBot/YesImBot-launcher/cmd.version=<ver>".
var version = "0.1.0"

// Execute runs the root command and returns the process exit code.
func Execute() int {
	rootCmd := &cobra.Command{
		Use:     "yesimbot",
		Short:   "YesImBot Launcher - Manage Koishi/YesImBot instances",
		Version: version,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.AddCommand(newInitCmd(), newStartCmd(), newStopCmd(), newStatusCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "\n✗ Error: %v\n", err)
		return 1
	}
	return 0
}
