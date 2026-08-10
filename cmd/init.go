package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"launcher/internal"
)

func newInitCmd() *cobra.Command {
	var local string
	var build bool

	cmd := &cobra.Command{
		Use:   "init [directory]",
		Short: "Initialize or install YesImBot in a Koishi App",
		Long: `Initialize a new YesImBot app in the specified directory, or install YesImBot into an existing Koishi App.
Uses ./yesimbot-app in the current directory if no directory is specified.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var directory string
			if len(args) > 0 {
				directory = args[0]
			}
			result, err := internal.Initialize(internal.InitOptions{
				Directory: directory,
				Local:     local,
				Build:     build,
			}, internal.NewRunner())
			if err != nil {
				return err
			}

			// Ask whether to start now.
			startNow, err := internal.AskUser("\nStart YesImBot now? [Y/n] ")
			if err != nil {
				return err
			}
			if startNow {
				if err := os.Chdir(result.AppDir); err != nil {
					return fmt.Errorf("cannot enter app directory: %v", err)
				}
				_, startErr := internal.Start(internal.StartOptions{AppDir: result.AppDir, Daemon: false})
				if startErr != nil {
					var exitErr *internal.ExitError
					if errors.As(startErr, &exitErr) {
						fmt.Fprintf(os.Stderr, "Koishi exited with code %d\n", exitErr.Code)
						os.Exit(exitErr.Code)
					}
					return startErr
				}
			} else {
				fmt.Printf("\nTo start later, run:\n  cd %s && yesimbot-cli start\n", result.AppDir)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&local, "local", "", "Use a local YesImBot repository")
	cmd.Flags().BoolVar(&build, "build", false, "Force install and build for local repository")
	return cmd
}
