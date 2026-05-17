package cli

import (
	"bedrud/internal/usercli"
	"fmt"

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
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new local user",
		RunE: func(cmd *cobra.Command, args []string) error {
			if email == "" || password == "" || name == "" {
				return fmt.Errorf("--email, --password and --name are required")
			}
			return usercli.CreateUser(resolveConfigPath(defaultEtcConfig), email, password, name)
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "User email")
	cmd.Flags().StringVar(&password, "password", "", "User password")
	cmd.Flags().StringVar(&name, "name", "", "User display name")
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
			return usercli.PromoteUser(resolveConfigPath(defaultEtcConfig), email)
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
			return usercli.DemoteUser(resolveConfigPath(defaultEtcConfig), email)
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
	cmd := &cobra.Command{
		Use:   "password",
		Short: "Set a user's password (invalidates active sessions)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if email == "" || password == "" {
				return fmt.Errorf("--email and --password are required")
			}
			return usercli.SetUserPassword(resolveConfigPath(defaultEtcConfig), email, password)
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "User email")
	cmd.Flags().StringVar(&password, "password", "", "New password")
	_ = cmd.MarkFlagRequired("email")
	_ = cmd.MarkFlagRequired("password")
	return cmd
}

func newUserResetPasswordCmd() *cobra.Command {
	var email string
	cmd := &cobra.Command{
		Use:   "reset-password",
		Short: "Generate a random password and print it (invalidates active sessions)",
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
