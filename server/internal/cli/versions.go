package cli

import (
	"fmt"

	"bedrud/internal/install"

	"github.com/spf13/cobra"
)

func newVersionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "versions",
		Short: "Manage installed Bedrud versions",
		Long: `List, switch, and prune versioned binaries under /opt/bedrud
(override with BEDRUD_INSTALL_ROOT). The stable PATH entry
/usr/local/bin/bedrud is a symlink to the active version.

  bedrud versions list [--remote]
  bedrud versions current
  bedrud versions use <version>
  bedrud versions prune [--keep N] --yes
  bedrud versions remove <version> --yes
  bedrud versions path [version]
`,
	}
	cmd.AddCommand(
		newVersionsListCmd(),
		newVersionsCurrentCmd(),
		newVersionsUseCmd(),
		newVersionsPruneCmd(),
		newVersionsRemoveCmd(),
		newVersionsPathCmd(),
	)
	return cmd
}

func newVersionsListCmd() *cobra.Command {
	var remote bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List installed versions (add --remote to include GitHub stables)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return install.VersionsList(remote)
		},
	}
	cmd.Flags().BoolVar(&remote, "remote", false, "Include GitHub stable releases (metadata only)")
	return cmd
}

func newVersionsCurrentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Show the active version",
		RunE: func(cmd *cobra.Command, args []string) error {
			return install.VersionsCurrent()
		},
	}
}

func newVersionsUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <version>",
		Short: "Switch the active version (stops services, flips symlink, restarts)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return install.VersionsUse(args[0])
		},
	}
}

func newVersionsPruneCmd() *cobra.Command {
	var keep int
	var yes bool
	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Delete oldest non-active versions",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return fmt.Errorf("versions prune requires --yes to delete old versions")
			}
			return install.VersionsPrune(keep)
		},
	}
	cmd.Flags().IntVar(&keep, "keep", 5, "Number of non-active versions to keep")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Confirm deletion")
	return cmd
}

func newVersionsRemoveCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "remove <version>",
		Short: "Remove an installed version (not the active one)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return fmt.Errorf("versions remove requires --yes")
			}
			return install.VersionsRemove(args[0])
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Confirm deletion")
	return cmd
}

func newVersionsPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path [version]",
		Short: "Print the filesystem path of a version binary (default: active)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			v := ""
			if len(args) == 1 {
				v = args[0]
			}
			return install.VersionsPath(v)
		},
	}
}
