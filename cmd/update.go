package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"launcher/internal"
)

func newUpdateCmd() *cobra.Command {
	var app string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update and rebuild the YesImBot app",
		Long:  `Pull the latest YesImBot source, rebuild it, and refresh the Koishi App in the current directory or the --app directory.`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			appDir, err := internal.ResolveAppDir(app)
			if err != nil {
				return err
			}
			runner := internal.NewRunner()
			if err := internal.UpdateSource(appDir, runner); err != nil {
				return err
			}
			_, err = internal.Initialize(internal.InitOptions{
				Directory:   appDir,
				SkipPrompts: true,
			}, runner)
			if err != nil {
				return err
			}
			fmt.Printf("Update complete: %s\n", appDir)
			return nil
		},
	}

	cmd.Flags().StringVar(&app, "app", "", "Koishi App directory")
	return cmd
}
