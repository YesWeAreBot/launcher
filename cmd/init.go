package cmd

import (
	"github.com/spf13/cobra"

	"launcher/internal"
)

func newInitCmd() *cobra.Command {
	var local string
	var build bool

	cmd := &cobra.Command{
		Use:   "init [directory]",
		Short: "Initialize a new YesImBot app",
		Long: `Initialize a new YesImBot app in the specified directory,
or ./yesimbot-app in the current directory if not specified.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var directory string
			if len(args) > 0 {
				directory = args[0]
			}
			_, err := internal.Initialize(internal.InitOptions{
				Directory: directory,
				Local:     local,
				Build:     build,
			}, internal.NewRunner())
			return err
		},
	}

	cmd.Flags().StringVar(&local, "local", "", "Use a local YesImBot repository")
	cmd.Flags().BoolVar(&build, "build", false, "Force install and build for local repository")
	return cmd
}
