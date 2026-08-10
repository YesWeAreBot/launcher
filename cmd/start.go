package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"launcher/internal"
)

func newStartCmd() *cobra.Command {
	var daemon bool
	var app string

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the YesImBot instance",
		Long:  `Start the YesImBot instance in the current directory or the --app directory.`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			appDir, err := internal.ResolveAppDir(app)
			if err != nil {
				return err
			}
			result, err := internal.Start(internal.StartOptions{AppDir: appDir, Daemon: daemon})
			if err != nil {
				var exitErr *internal.ExitError
				if errors.As(err, &exitErr) {
					fmt.Fprintf(os.Stderr, "Koishi exited with code %d\n", exitErr.Code)
					os.Exit(exitErr.Code)
				}
				return err
			}
			// Foreground mode blocks inside Start until the child exits;
			// a returned result here means the child stopped cleanly.
			_ = result
			return nil
		},
	}

	cmd.Flags().BoolVar(&daemon, "daemon", false, "Run in background")
	cmd.Flags().StringVar(&app, "app", "", "Koishi App directory")
	return cmd
}
