package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"launcher/internal"
)

func newSelfUpdateCmd() *cobra.Command {
	var check bool
	var channel string

	cmd := &cobra.Command{
		Use:   "self-update",
		Short: "Check or update yesimbot-cli itself",
		Long:  `Download the yesimbot-cli binary from the configured GitHub release channel.`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := internal.SelfUpdate(internal.SelfUpdateOptions{
				Channel:   channel,
				CheckOnly: check,
			})
			if err != nil {
				return err
			}
			if check {
				fmt.Printf("  CLI:        %s\n", result.Executable)
				fmt.Printf("  Channel:    %s\n", channelOrDefault(channel))
				fmt.Printf("  Asset:      %s\n", result.AssetURL)
				fmt.Println("  Status:     available")
				return nil
			}
			fmt.Printf("  Update downloaded for: %s\n", result.Executable)
			fmt.Printf("  Asset:                 %s\n", result.AssetURL)
			fmt.Println("  The replacement will finish after this process exits.")
			return nil
		},
	}

	cmd.Flags().BoolVar(&check, "check", false, "Only check whether the release asset is available")
	cmd.Flags().StringVar(&channel, "channel", internal.DefaultChannel, "GitHub release channel")
	return cmd
}

func channelOrDefault(channel string) string {
	if channel == "" {
		return internal.DefaultChannel
	}
	return channel
}
