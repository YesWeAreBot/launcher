package cmd

import (
	"github.com/spf13/cobra"

	"launcher/internal"
)

func newUpdateCmd() *cobra.Command {
	var app string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update the YesImBot source repository",
		Long:  `Update the YesImBot source repository in the current directory or the --app directory.`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			appDir, err := internal.ResolveAppDir(app)
			if err != nil {
				return err
			}
			return internal.UpdateSource(appDir, internal.NewRunner())
		},
	}

	cmd.Flags().StringVar(&app, "app", "", "Koishi App directory")
	return cmd
}
