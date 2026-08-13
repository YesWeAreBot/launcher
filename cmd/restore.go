package cmd

import (
	"github.com/spf13/cobra"

	"launcher/internal"
)

func newRestoreCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restore <backup-directory>",
		Short: "Restore a YesImBot uninstall backup",
		Long: `Restore a whole Koishi App backup created by uninstall, or restore
package.json and koishi.yml from an uninstall --keep-app backup.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return internal.RestoreBackup(args[0])
		},
	}
}
