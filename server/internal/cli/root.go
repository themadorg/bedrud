package cli

import (
	"fmt"
	"os"
	"strings"

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
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("bedrud " + Version)
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
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
