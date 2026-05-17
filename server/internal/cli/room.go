package cli

import (
	"bedrud/internal/roomcli"
	"fmt"

	"github.com/spf13/cobra"
)

func newRoomCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "room",
		Short: "Manage meetings/rooms",
	}
	cmd.AddCommand(
		newRoomListCmd(),
		newRoomInfoCmd(),
		newRoomCloseCmd(),
		newRoomSuspendCmd(),
		newRoomReactivateCmd(),
		newRoomKickCmd(),
	)
	return cmd
}

func newRoomListCmd() *cobra.Command {
	var page, pageSize int
	var activeOnly bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List rooms",
		RunE: func(cmd *cobra.Command, args []string) error {
			return roomcli.ListRooms(resolveConfigPath(defaultEtcConfig), page, pageSize, activeOnly)
		},
	}
	cmd.Flags().IntVar(&page, "page", 1, "Page number")
	cmd.Flags().IntVar(&pageSize, "page-size", 50, "Rooms per page")
	cmd.Flags().BoolVar(&activeOnly, "active", false, "Only show active rooms (ignores pagination)")
	return cmd
}

func newRoomInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info <room-id-or-name>",
		Short: "Show details for a room",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return roomcli.ShowRoom(resolveConfigPath(defaultEtcConfig), args[0])
		},
	}
	return cmd
}

func newRoomCloseCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "close <room-id-or-name>",
		Short: "Close a room (cascade delete: LiveKit, uploads, DB)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				return fmt.Errorf("destructive operation; re-run with --yes to confirm")
			}
			return roomcli.CloseRoom(resolveConfigPath(defaultEtcConfig), args[0])
		},
	}
	cmd.Flags().BoolVar(&force, "yes", false, "Confirm destructive operation")
	return cmd
}

func newRoomSuspendCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "suspend <room-id-or-name>",
		Short: "Suspend a room (disconnects participants, keeps DB record)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return roomcli.SuspendRoom(resolveConfigPath(defaultEtcConfig), args[0])
		},
	}
	return cmd
}

func newRoomReactivateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reactivate <room-id-or-name>",
		Short: "Reactivate a previously suspended room",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return roomcli.ReactivateRoom(resolveConfigPath(defaultEtcConfig), args[0])
		},
	}
	return cmd
}

func newRoomKickCmd() *cobra.Command {
	var identity string
	cmd := &cobra.Command{
		Use:   "kick <room-id-or-name>",
		Short: "Kick a participant from a room",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if identity == "" {
				return fmt.Errorf("--identity is required")
			}
			return roomcli.KickParticipant(resolveConfigPath(defaultEtcConfig), args[0], identity)
		},
	}
	cmd.Flags().StringVar(&identity, "identity", "", "Participant identity (user ID or name)")
	_ = cmd.MarkFlagRequired("identity")
	return cmd
}
