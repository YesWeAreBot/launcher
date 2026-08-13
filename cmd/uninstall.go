package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"launcher/internal"
)

func newUninstallCmd() *cobra.Command {
	var app string
	var keepApp, noDeps, yes bool

	cmd := &cobra.Command{
		Use:   "uninstall [directory]",
		Short: "Uninstall YesImBot (and by default the Koishi App)",
		Long: `Uninstall YesImBot from the current directory, --app, or a directory argument.

By default the whole Koishi App is moved to a sibling backup directory. Pass
--keep-app to only remove YesImBot from the App and keep Koishi in place.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				if app != "" {
					return fmt.Errorf("use either a directory argument or --app, not both")
				}
				absolute, err := filepath.Abs(args[0])
				if err != nil {
					return err
				}
				app = absolute
			}

			appDir, err := internal.ResolveAppDir(app)
			if err != nil {
				return err
			}
			if err := confirmUninstall(appDir, keepApp, yes); err != nil {
				return err
			}

			result, err := internal.Uninstall(internal.UninstallOptions{
				AppDir:   appDir,
				KeepApp:  keepApp,
				SkipDeps: noDeps,
			}, internal.NewRunner())
			if err != nil {
				return err
			}

			if result.BackupDir != "" {
				fmt.Printf("Uninstalled %s; app moved to %s\n", result.AppDir, result.BackupDir)
				fmt.Printf("To restore, run: yesimbot-cli restore %s\n", result.BackupDir)
			} else if result.KeptApp {
				fmt.Printf("Removed YesImBot from %s; Koishi App kept\n", result.AppDir)
				if len(result.Removed) > 0 {
					fmt.Printf("Removed %d managed dependencies/plugin entries\n", len(result.Removed))
				}
				if result.ConfigBackupDir != "" {
					fmt.Printf("Config backup: %s\n", result.ConfigBackupDir)
					fmt.Printf("To restore, run: yesimbot-cli restore %s\n", result.ConfigBackupDir)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&app, "app", "", "Koishi App directory")
	cmd.Flags().BoolVar(&keepApp, "keep-app", false, "Keep the Koishi App and only remove YesImBot")
	cmd.Flags().BoolVar(&noDeps, "no-deps", false, "Do not run yarn install after removing dependencies")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip the uninstall confirmation prompt")
	return cmd
}

func confirmUninstall(appDir string, keepApp, yes bool) error {
	if yes {
		return nil
	}
	info, err := os.Stdin.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return fmt.Errorf("uninstall requires --yes in non-interactive mode")
	}

	action := "move the whole Koishi App to a backup directory"
	if keepApp {
		action = "remove YesImBot from the Koishi App"
	}
	fmt.Printf("Uninstall YesImBot from %s? This will stop it and %s. [y/N] ", appDir, action)
	answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && err != io.EOF {
		return err
	}
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return nil
	}
	return fmt.Errorf("uninstall cancelled")
}
