package cli

import (
	"fmt"
	"io"
	"strings"

	"bedrud/internal/usercli"

	"github.com/spf13/cobra"
)

func newUserCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "Manage users",
	}
	cmd.AddCommand(
		newUserCreateCmd(),
		newUserDeleteCmd(),
		newUserPromoteCmd(),
		newUserDemoteCmd(),
		newUserListCmd(),
		newUserInfoCmd(),
		newUserPasswordCmd(),
		newUserResetPasswordCmd(),
		newUserEnableCmd(),
		newUserDisableCmd(),
	)
	return cmd
}

func newUserCreateCmd() *cobra.Command {
	var email, password, name string
	var admin, passwordStdin bool
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new local user",
		Long: `Create a new local user.

--password puts the secret in the process list, where any local shell can
read it while the command runs. Prefer --password-stdin:

  printf '%s' "$PASSWORD" | bedrud user create --email you@example.com --name You --password-stdin`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if email == "" || name == "" {
				return fmt.Errorf("--email and --name are required")
			}
			password, err := resolvePasswordInput(cmd, password, passwordStdin)
			if err != nil {
				return err
			}
			return usercli.CreateUser(resolveConfigPath(defaultEtcConfig), email, password, name, admin)
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "User email")
	cmd.Flags().StringVar(&password, "password", "", "User password (visible in the process list; prefer --password-stdin)")
	cmd.Flags().BoolVar(&passwordStdin, "password-stdin", false, "Read the password from stdin")
	cmd.Flags().StringVar(&name, "name", "", "User display name")
	cmd.Flags().BoolVar(&admin, "admin", false, "Create user as superadmin")
	_ = cmd.MarkFlagRequired("email")
	return cmd
}

func newUserDeleteCmd() *cobra.Command {
	var email string
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a user and their rooms",
		RunE: func(cmd *cobra.Command, args []string) error {
			if email == "" {
				return fmt.Errorf("--email is required")
			}
			return usercli.DeleteUser(resolveConfigPath(defaultEtcConfig), email)
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "User email")
	_ = cmd.MarkFlagRequired("email")
	return cmd
}

func newUserPromoteCmd() *cobra.Command {
	var email string
	cmd := &cobra.Command{
		Use:   "promote",
		Short: "Grant superadmin access to a user",
		RunE: func(cmd *cobra.Command, args []string) error {
			if email == "" {
				return fmt.Errorf("--email is required")
			}
			return usercli.PromoteUser(resolveConfigPath(defaultEtcConfig), email, "superadmin")
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "User email")
	_ = cmd.MarkFlagRequired("email")
	return cmd
}

func newUserDemoteCmd() *cobra.Command {
	var email string
	cmd := &cobra.Command{
		Use:   "demote",
		Short: "Remove superadmin access from a user",
		RunE: func(cmd *cobra.Command, args []string) error {
			if email == "" {
				return fmt.Errorf("--email is required")
			}
			return usercli.DemoteUser(resolveConfigPath(defaultEtcConfig), email, "superadmin")
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "User email")
	_ = cmd.MarkFlagRequired("email")
	return cmd
}

func newUserListCmd() *cobra.Command {
	var page, pageSize int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List users",
		RunE: func(cmd *cobra.Command, args []string) error {
			return usercli.ListUsers(resolveConfigPath(defaultEtcConfig), page, pageSize)
		},
	}
	cmd.Flags().IntVar(&page, "page", 1, "Page number (1-indexed)")
	cmd.Flags().IntVar(&pageSize, "page-size", 50, "Users per page")
	return cmd
}

func newUserInfoCmd() *cobra.Command {
	var email string
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Show details for a user",
		RunE: func(cmd *cobra.Command, args []string) error {
			if email == "" {
				return fmt.Errorf("--email is required")
			}
			return usercli.ShowUser(resolveConfigPath(defaultEtcConfig), email)
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "User email")
	_ = cmd.MarkFlagRequired("email")
	return cmd
}

func newUserPasswordCmd() *cobra.Command {
	var email, password string
	var passwordStdin bool
	cmd := &cobra.Command{
		Use:   "password",
		Short: "Set a user's password (invalidates active sessions)",
		Long: `Set a user's password (invalidates active sessions).

--password puts the secret in the process list, where any local shell can
read it while the command runs. Prefer --password-stdin:

  printf '%s' "$NEW_PASSWORD" | bedrud user password --email you@example.com --password-stdin`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if email == "" {
				return fmt.Errorf("--email is required")
			}
			password, err := resolvePasswordInput(cmd, password, passwordStdin)
			if err != nil {
				return err
			}
			return usercli.SetUserPassword(resolveConfigPath(defaultEtcConfig), email, password)
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "User email")
	cmd.Flags().StringVar(&password, "password", "", "New password (visible in the process list; prefer --password-stdin)")
	cmd.Flags().BoolVar(&passwordStdin, "password-stdin", false, "Read the new password from stdin")
	_ = cmd.MarkFlagRequired("email")
	return cmd
}

func newUserResetPasswordCmd() *cobra.Command {
	var email string
	cmd := &cobra.Command{
		Use:   "reset-password",
		Short: "Generate a random password and print it (invalidates active sessions)",
		Long: `Generate a random password and print it (invalidates active sessions).

Only the hash is stored, so the printed password is the only copy: record it
before the terminal scrolls away. With --json it is also in data.password.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if email == "" {
				return fmt.Errorf("--email is required")
			}
			return usercli.SetUserPassword(resolveConfigPath(defaultEtcConfig), email, "")
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "User email")
	_ = cmd.MarkFlagRequired("email")
	return cmd
}

// resolvePasswordInput picks the password source, preferring stdin so the
// secret never reaches the process list. An empty flag with no --password-stdin
// is an error: silently falling back to a generated password is how an account
// ends up locked behind a value nobody printed.
func resolvePasswordInput(cmd *cobra.Command, password string, fromStdin bool) (string, error) {
	if fromStdin {
		if password != "" {
			return "", fmt.Errorf("--password and --password-stdin are mutually exclusive")
		}
		data, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return "", fmt.Errorf("read password from stdin: %w", err)
		}
		password = strings.TrimRight(string(data), "\r\n")
	}
	if password == "" {
		return "", fmt.Errorf("--password or --password-stdin is required")
	}
	return password, nil
}

func newUserEnableCmd() *cobra.Command {
	var email string
	cmd := &cobra.Command{
		Use:   "enable",
		Short: "Re-enable a disabled user",
		RunE: func(cmd *cobra.Command, args []string) error {
			if email == "" {
				return fmt.Errorf("--email is required")
			}
			return usercli.SetUserActive(resolveConfigPath(defaultEtcConfig), email, true)
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "User email")
	_ = cmd.MarkFlagRequired("email")
	return cmd
}

func newUserDisableCmd() *cobra.Command {
	var email string
	cmd := &cobra.Command{
		Use:   "disable",
		Short: "Disable a user and invalidate their sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			if email == "" {
				return fmt.Errorf("--email is required")
			}
			return usercli.SetUserActive(resolveConfigPath(defaultEtcConfig), email, false)
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "User email")
	_ = cmd.MarkFlagRequired("email")
	return cmd
}
