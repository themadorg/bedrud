package cli

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"

	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/clioutput"
	"bedrud/internal/database"
	"bedrud/internal/models"
	"bedrud/internal/repository"

	"github.com/spf13/cobra"
)

func newSettingsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "settings",
		Short: "Inspect/modify runtime settings stored in the database",
	}
	cmd.AddCommand(newSettingsShowCmd(), newSettingsSetCmd(), newSettingsResetCmd())
	return cmd
}

func newSettingsShowCmd() *cobra.Command {
	var effective bool
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Print runtime settings as JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withSettingsRepo(func(repo *repository.SettingsRepository) error {
				var (
					s   *models.SystemSettings
					err error
				)
				if effective {
					s, err = repo.GetEffectiveSettings()
				} else {
					s, err = repo.GetSettings()
				}
				if err != nil {
					return err
				}
				redactSettings(s)
				if clioutput.JSON() {
					return clioutput.Success("", map[string]any{
						"effective": effective,
						"settings":  s,
					})
				}
				out, err := json.MarshalIndent(s, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(out))
				return nil
			})
		},
	}
	cmd.Flags().BoolVar(&effective, "effective", false, "Show settings merged with config defaults")
	return cmd
}

func newSettingsSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <jsonField> <value>",
		Short: "Set a single runtime setting by JSON field name",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withSettingsRepo(func(repo *repository.SettingsRepository) error {
				s, err := repo.GetSettings()
				if err != nil {
					return err
				}
				if err := setStructField(s, args[0], args[1]); err != nil {
					return err
				}
				if err := repo.SaveSettings(s); err != nil {
					return fmt.Errorf("save: %w", err)
				}
				effective, _ := repo.GetEffectiveSettings()
				if effective != nil {
					auth.ReloadProviders(effective)
				}
				return clioutput.Success(
					fmt.Sprintf("✓ Set %s = %s", args[0], args[1]),
					map[string]string{"field": args[0], "value": args[1]},
				)
			})
		},
	}
	return cmd
}

func newSettingsResetCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset runtime settings to defaults (re-runs initial migration values)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				return fmt.Errorf("destructive; re-run with --yes")
			}
			return withSettingsRepo(func(repo *repository.SettingsRepository) error {
				fresh := &models.SystemSettings{ID: 1}
				if err := repo.SaveSettings(fresh); err != nil {
					return err
				}
				return clioutput.Success(
					"✓ Settings reset to zero values; restart server or re-set fields as needed.",
					map[string]bool{"reset": true},
				)
			})
		},
	}
	cmd.Flags().BoolVar(&force, "yes", false, "Confirm reset")
	return cmd
}

func withSettingsRepo(fn func(*repository.SettingsRepository) error) error {
	cfg, err := config.Load(resolveConfigPath(defaultEtcConfig))
	if err != nil {
		return err
	}
	if err := database.Initialize(&cfg.Database); err != nil {
		return err
	}
	defer database.Close()
	if err := database.RunMigrations(); err != nil {
		return err
	}
	repo := repository.NewSettingsRepository(database.GetDB())
	repo.SetConfig(cfg)
	return fn(repo)
}

func setStructField(s *models.SystemSettings, jsonName, raw string) error {
	v := reflect.ValueOf(s).Elem()
	t := v.Type()
	for i := range t.NumField() {
		f := t.Field(i)
		tag := f.Tag.Get("json")
		name := tag
		if comma := indexComma(tag); comma >= 0 {
			name = tag[:comma]
		}
		if name != jsonName {
			continue
		}
		fv := v.Field(i)
		if !fv.CanSet() {
			return fmt.Errorf("field %q is not settable", jsonName)
		}
		return assignFromString(fv, raw)
	}
	return fmt.Errorf("unknown settings field: %s", jsonName)
}

func assignFromString(fv reflect.Value, raw string) error {
	switch fv.Kind() {
	case reflect.String:
		fv.SetString(raw)
	case reflect.Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return err
		}
		fv.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return err
		}
		fv.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return err
		}
		fv.SetUint(n)
	case reflect.Float32, reflect.Float64:
		n, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return err
		}
		fv.SetFloat(n)
	default:
		return fmt.Errorf("unsupported field kind: %s", fv.Kind())
	}
	return nil
}

func indexComma(s string) int {
	for i, r := range s {
		if r == ',' {
			return i
		}
	}
	return -1
}

func redactSettings(s *models.SystemSettings) {
	if s == nil {
		return
	}
	// Direct field writes so taint analysis (and readers) see sanitization.
	if s.GoogleClientSecret != "" {
		s.GoogleClientSecret = "***redacted***"
	}
	if s.GithubClientSecret != "" {
		s.GithubClientSecret = "***redacted***"
	}
	if s.TwitterClientSecret != "" {
		s.TwitterClientSecret = "***redacted***"
	}
	if s.JWTSecret != "" {
		s.JWTSecret = "***redacted***"
	}
	if s.SessionSecret != "" {
		s.SessionSecret = "***redacted***"
	}
	if s.LiveKitAPIKey != "" {
		s.LiveKitAPIKey = "***redacted***"
	}
	if s.LiveKitAPISecret != "" {
		s.LiveKitAPISecret = "***redacted***"
	}
	if s.ChatUploadS3AccessKey != "" {
		s.ChatUploadS3AccessKey = "***redacted***"
	}
	if s.ChatUploadS3SecretKey != "" {
		s.ChatUploadS3SecretKey = "***redacted***"
	}
	if s.EmailPassword != "" {
		s.EmailPassword = "***redacted***"
	}
}
