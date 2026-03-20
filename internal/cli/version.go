package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version",
		RunE: func(cmd *cobra.Command, args []string) error {
			if format == "json" {
				enc := json.NewEncoder(os.Stdout)
				return enc.Encode(map[string]string{"version": version})
			}
			fmt.Printf("deployscope v%s\n", version)
			return nil
		},
	}

	cmd.Flags().StringVar(&format, "format", "text", "Output format: text, json")
	return cmd
}
