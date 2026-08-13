package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"launcher/internal"
)

func newStatusCmd() *cobra.Command {
	var app string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show current status",
		Long:  `Show the status of the YesImBot instance in the current directory or the --app directory.`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			appDir, err := internal.ResolveAppDir(app)
			if err != nil {
				return err
			}
			s, err := internal.Status(appDir)
			if err != nil {
				return err
			}
			printStatus(s)
			return nil
		},
	}

	cmd.Flags().StringVar(&app, "app", "", "Koishi App directory")
	return cmd
}

func printStatus(s internal.RuntimeStatus) {
	fmt.Println("YesImBot Status")
	fmt.Println("----------------------------------------")
	fmt.Printf("  Initialized:  %s\n", check(s.Initialized))

	if s.Running {
		fmt.Printf("  Running:      [OK] (PID: %d, mode: %s)\n", s.Pid, s.Mode)
	} else {
		fmt.Println("  Running:      [NO]")
	}

	fmt.Printf("  App Dir:      %s\n", s.AppDir)

	if s.SourceHead != "" {
		head := s.SourceHead
		if len(head) > 8 {
			head = head[:8]
		}
		fmt.Printf("  Source HEAD:  %s\n", head)
	}
	if s.StartedAt != "" {
		fmt.Printf("  Started At:   %s\n", s.StartedAt)
	}
	if s.StoppedAt != "" && !s.Running {
		fmt.Printf("  Stopped At:   %s\n", s.StoppedAt)
	}
	if s.UpdatedAt != "" {
		fmt.Printf("  Updated At:   %s\n", s.UpdatedAt)
	}
	if s.ConsoleURL != "" {
		fmt.Printf("  Console:      %s\n", s.ConsoleURL)
	}
	fmt.Printf("  Log File:     %s\n", s.LogFile)
}

func check(b bool) string {
	if b {
		return "[OK]"
	}
	return "[NO]"
}
