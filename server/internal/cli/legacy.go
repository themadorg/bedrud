package cli

import (
	"bedrud/internal/livekit"
	"bedrud/internal/server"
	"flag"
	"fmt"
	"os"
)

// dispatchLegacy handles pre-cobra invocation forms that systemd units, docs,
// and earlier installations relied on:
//
//	bedrud --version | -v
//	bedrud --livekit --config <path>
//	bedrud --run     --config <path> [--skip-migrate]
//
// Returns true when it handled the call (the process should exit on return).
func dispatchLegacy(args []string) bool {
	for i, arg := range args {
		switch arg {
		case "--version", "-v":
			fmt.Println("bedrud " + Version)
			return true
		case "--livekit":
			lk := flag.NewFlagSet("livekit", flag.ExitOnError)
			cfg := lk.String("config", "", "Path to LiveKit config file")
			_ = lk.Parse(args[i+1:])
			if err := livekit.RunLiveKit(*cfg); err != nil {
				fmt.Fprintf(os.Stderr, "LiveKit error: %v\n", err)
				os.Exit(1)
			}
			return true
		case "--run":
			rn := flag.NewFlagSet("run", flag.ExitOnError)
			cfg := rn.String("config", "", "Path to Bedrud config file")
			skip := rn.Bool("skip-migrate", false, "Skip database migrations on startup")
			_ = rn.Parse(args[i+1:])
			path := *cfg
			if path == "" {
				path = os.Getenv("CONFIG_PATH")
				if path == "" {
					path = defaultConfigPath
				}
			}
			if *skip {
				os.Setenv("BEDRUD_SKIP_MIGRATE", "1")
			}
			if err := server.Run(path); err != nil {
				fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
				os.Exit(1)
			}
			return true
		}
	}
	return false
}
