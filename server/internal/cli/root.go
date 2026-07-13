package cli

import (
	"fmt"
	"os"
	"strings"

	"bedrud/internal/clioutput"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// Version is injected via -ldflags by mage at build time.
	Version = "dev"

	// configPath is the path to the bedrud config file, populated from the
	// --config persistent flag, BEDRUD_CONFIG env, or sensible defaults.
	configPath string
)

const (
	envPrefix         = "BEDRUD"
	defaultConfigPath = "config.yaml"
)

// NewRootCmd builds the bedrud root command and attaches all subcommands.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "bedrud",
		Short: "Bedrud - Open Source Video Meetings (All-in-One Binary)",
		Long: `Bedrud is an all-in-one binary that runs the API server, the embedded
LiveKit instance, and ships a full CLI for installation, user, room,
configuration, and certificate management.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       Version,
	}

	root.SetVersionTemplate("bedrud {{.Version}}\n")
	root.PersistentFlags().StringVar(&configPath, "config", "", "Path to bedrud config file (env: BEDRUD_CONFIG / CONFIG_PATH)")
	root.PersistentFlags().Bool("json", false, "Emit machine-readable JSON output")
	root.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		jsonFlag, _ := cmd.Flags().GetBool("json")
		if !jsonFlag {
			jsonFlag, _ = cmd.Root().PersistentFlags().GetBool("json")
		}
		clioutput.SetJSON(jsonFlag)
	}

	viper.SetEnvPrefix(envPrefix)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	viper.AutomaticEnv()
	_ = viper.BindPFlag("config", root.PersistentFlags().Lookup("config"))
	_ = viper.BindEnv("config", "BEDRUD_CONFIG", "CONFIG_PATH")

	root.AddCommand(
		newRunCmd(),
		newLiveKitCmd(),
		newInstallCmd(),
		newUninstallCmd(),
		newUpdateCmd(),
		newUpgradeCmd(),
		newCertCmd(),
		newUserCmd(),
		newRoomCmd(),
		newConfigCmd(),
		newSettingsCmd(),
		newInviteTokenCmd(),
		newDBCmd(),
		newVersionCmd(),
	)

	return root
}

// resolveConfigPath returns the chosen config path, honoring flag/env/defaults.
// fallback is used when nothing else is supplied (e.g. "config.yaml" for run,
// "/etc/bedrud/config.yaml" for installed-machine management commands).
func resolveConfigPath(fallback string) string {
	if configPath != "" {
		return configPath
	}
	if v := viper.GetString("config"); v != "" {
		return v
	}
	if v := os.Getenv("CONFIG_PATH"); v != "" {
		return v
	}
	if fallback != "" {
		return fallback
	}
	return defaultConfigPath
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		RunE: func(cmd *cobra.Command, args []string) error {
			if clioutput.JSON() {
				return clioutput.Success("", map[string]string{
					"name":    "bedrud",
					"version": Version,
				})
			}
			clioutput.Println("bedrud " + Version)
			return nil
		},
	}
}

// Execute parses argv, dispatching legacy flag-style invocations
// (`bedrud --livekit ...`, `bedrud --run ...`, `bedrud --version`) so existing
// systemd units keep working, then hands off to cobra.
func Execute(version string) {
	if version != "" {
		Version = version
	}
	if handled := dispatchLegacy(os.Args[1:]); handled {
		return
	}
	root := NewRootCmd()
	if err := root.Execute(); err != nil {
		if clioutput.JSON() {
			clioutput.EmitError(err)
		} else {
			fmt.Fprintln(os.Stderr, "Error:", err)
		}
		os.Exit(1)
	}
}
