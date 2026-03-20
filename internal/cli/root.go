package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

func SetVersion(v string) {
	version = v
}

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "deployscope",
		Short: "Kubernetes deployment monitor — cognitive layer for autonomous agents",
		Long:  "DeployScope reads Kubernetes workload state and surfaces health, ownership, and integration pointers. Mirror, not oracle.",
	}

	root.AddCommand(
		newServeCmd(),
		newStatusCmd(),
		newNamespacesCmd(),
		newInitCmd(),
		newDoctorCmd(),
		newVersionCmd(),
	)

	// Default to serve for backward compat when no subcommand given
	root.RunE = func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	}

	return root
}

func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
