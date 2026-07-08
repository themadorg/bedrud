package cli

import (
	"bedrud/internal/clioutput"
	"bedrud/internal/livekit"
	"fmt"

	"github.com/spf13/cobra"
)

func newLiveKitCmd() *cobra.Command {
	var cfgPath string

	cmd := &cobra.Command{
		Use:   "livekit",
		Short: "Start the embedded LiveKit server",
		RunE: func(cmd *cobra.Command, args []string) error {
			if clioutput.JSON() {
				if err := clioutput.Success("starting livekit", map[string]any{
					"livekitConfigPath": cfgPath,
				}); err != nil {
					return err
				}
			}
			if err := livekit.RunLiveKit(cfgPath); err != nil {
				return fmt.Errorf("livekit: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&cfgPath, "config", "", "Path to LiveKit config file")
	return cmd
}
