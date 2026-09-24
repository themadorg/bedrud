package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"bedrud/config"
	"bedrud/internal/clioutput"
	"bedrud/internal/database"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect and modify the bedrud config file",
	}
	cmd.AddCommand(
		newConfigPathCmd(),
		newConfigShowCmd(),
		newConfigGetCmd(),
		newConfigSetCmd(),
		newConfigValidateCmd(),
	)
	return cmd
}

func newConfigPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the resolved config file path",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := resolveConfigPath(defaultEtcConfig)
			if clioutput.JSON() {
				return clioutput.Success("", map[string]string{"path": path})
			}
			fmt.Println(path)
			return nil
		},
	}
}

func newConfigShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Print the loaded configuration as YAML (env overrides included)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(resolveConfigPath(defaultEtcConfig))
			if err != nil {
				return err
			}
			masked := maskSecrets(cfg)
			if clioutput.JSON() {
				return clioutput.Success("", masked)
			}
			out, err := yaml.Marshal(masked)
			if err != nil {
				return err
			}
			fmt.Print(string(out))
			return nil
		},
	}
	return cmd
}

func newConfigGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Read a single config value by dotted key (e.g. server.port)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.New()
			v.SetConfigFile(resolveConfigPath(defaultEtcConfig))
			v.SetConfigType("yaml")
			if err := v.ReadInConfig(); err != nil {
				return fmt.Errorf("read config: %w", err)
			}
			val := v.Get(args[0])
			if val == nil {
				return fmt.Errorf("key not found: %s", args[0])
			}
			if clioutput.JSON() {
				return clioutput.Success("", map[string]any{
					"key":   args[0],
					"value": val,
				})
			}
			fmt.Println(val)
			return nil
		},
	}
	return cmd
}

func newConfigSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Write a single config value by dotted key and save to disk",
		Long: `Write a single config value by dotted key and save to disk.

Only the addressed key is rewritten: every other key keeps its exact
spelling, order and comments, and the result is parsed back before it
replaces the file on disk.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := resolveConfigPath(defaultEtcConfig)
			key, err := setConfigValue(path, args[0], args[1])
			if err != nil {
				return err
			}
			return clioutput.Success(
				fmt.Sprintf("✓ Set %s = %s in %s", key, args[1], path),
				map[string]string{"key": key, "value": args[1], "path": path},
			)
		},
	}
	return cmd
}

func newConfigValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate the config file (parses YAML and enforces required fields)",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := resolveConfigPath(defaultEtcConfig)
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}
			var problems []string
			if cfg.Auth.JWTSecret == "" {
				problems = append(problems, "auth.jwtSecret is required (set AUTH_JWT_SECRET or auth.jwtSecret)")
			} else if len(cfg.Auth.JWTSecret) < 32 {
				problems = append(problems, fmt.Sprintf("auth.jwtSecret is %d chars, recommend >= 32", len(cfg.Auth.JWTSecret)))
			}
			if cfg.Auth.SessionSecret == "" {
				problems = append(problems, "auth.sessionSecret is required")
			}
			if cfg.Database.Type == "" {
				problems = append(problems, "database.type is required (sqlite or postgres)")
			}
			if cfg.Database.Type == database.DBTypeSQLite && cfg.Database.Path == "" {
				problems = append(problems, "database.path is required for sqlite")
			}
			if cfg.Server.Port == "" {
				problems = append(problems, "server.port is required")
			}
			sort.Strings(problems)
			if len(problems) == 0 {
				return clioutput.Success(
					fmt.Sprintf("✓ Config OK: %s", path),
					map[string]any{"path": path, "problems": []string{}},
				)
			}
			if clioutput.JSON() {
				return fmt.Errorf("config validation failed: %v", problems)
			}
			fmt.Fprintf(os.Stderr, "✗ Config has %d problem(s):\n", len(problems))
			for _, p := range problems {
				fmt.Fprintln(os.Stderr, "  - "+p)
			}
			return fmt.Errorf("config validation failed")
		},
	}
	return cmd
}

func maskSecrets(cfg *config.Config) map[string]any {
	out := map[string]any{}
	bytes, _ := yaml.Marshal(cfg)
	_ = yaml.Unmarshal(bytes, &out)
	maskInPlace(out, []string{
		"jwtSecret", "sessionSecret", "apiSecret", "clientSecret", "secretKey", "accessKey", "password",
		"cloudflareAPIToken", "apiToken",
	})
	return out
}

func maskInPlace(node any, names []string) {
	m, ok := node.(map[string]any)
	if !ok {
		return
	}
	for k, v := range m {
		for _, n := range names {
			if strings.EqualFold(k, n) {
				if s, ok := v.(string); ok && s != "" {
					m[k] = "***redacted***"
				}
				break
			}
		}
		maskInPlace(v, names)
	}
}
