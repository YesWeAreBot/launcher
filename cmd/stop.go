package cmd

import (
	"github.com/spf13/cobra"

	"launcher/internal"
)

func newStopCmd() *cobra.Command {
	var app string

	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the running instance",
		Long:  `Stop the YesImBot instance in the current directory or the --app directory.`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			appDir, err := internal.ResolveAppDir(app)
			if err != nil {
				return err
			}
			_, err = internal.Stop(internal.StopOptions{AppDir: appDir})
			return err
		},
	}

	cmd.Flags().StringVar(&app, "app", "", "Koishi App directory")
	return cmd
}
