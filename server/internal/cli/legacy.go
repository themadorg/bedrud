package cli

import (
	"bedrud/internal/clioutput"
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
	jsonMode := legacyJSONFlag(args)

	for i, arg := range args {
		switch arg {
		case "--version", "-v":
			if jsonMode {
				_ = clioutput.Success("", map[string]string{
					"name":    "bedrud",
					"version": Version,
				})
			} else {
				fmt.Println("bedrud " + Version)
			}
			return true
		case "--livekit":
			lk := flag.NewFlagSet("livekit", flag.ExitOnError)
			cfg := lk.String("config", "", "Path to LiveKit config file")
			_ = lk.Parse(legacyStripJSON(args[i+1:]))
			if jsonMode {
				_ = clioutput.Success("starting livekit", map[string]any{
					"livekitConfigPath": *cfg,
				})
			}
			if err := livekit.RunLiveKit(*cfg); err != nil {
				if jsonMode {
					clioutput.EmitError(err)
				} else {
					fmt.Fprintf(os.Stderr, "LiveKit error: %v\n", err)
				}
				os.Exit(1)
			}
			return true
		case "--run":
			rn := flag.NewFlagSet("run", flag.ExitOnError)
			cfg := rn.String("config", "", "Path to Bedrud config file")
			skip := rn.Bool("skip-migrate", false, "Skip database migrations on startup")
			_ = rn.Parse(legacyStripJSON(args[i+1:]))
			path := *cfg
			if path == "" {
				path = os.Getenv("CONFIG_PATH")
				if path == "" {
					path = defaultConfigPath
				}
			}
			if *skip {
				_ = os.Setenv("BEDRUD_SKIP_MIGRATE", "1")
			}
			if jsonMode {
				_ = clioutput.Success("starting server", map[string]any{
					"configPath":  path,
					"version":     Version,
					"skipMigrate": *skip,
				})
			}
			if err := server.Run(path, Version); err != nil {
				if jsonMode {
					clioutput.EmitError(err)
				} else {
					fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
				}
				os.Exit(1)
			}
			return true
		}
	}
	return false
}

func legacyJSONFlag(args []string) bool {
	for _, a := range args {
		if a == "--json" {
			clioutput.SetJSON(true)
			return true
		}
	}
	return false
}

func legacyStripJSON(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if a != "--json" {
			out = append(out, a)
		}
	}
	return out
}
