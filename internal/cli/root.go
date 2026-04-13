package cli

import (
	"github.com/spf13/cobra"
)

func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "rook",
		Short:         "Code intelligence from SCIP indexes",
		Long:          "rook ingests SCIP indexes, stores them in Postgres, and answers questions about symbols over a CLI, GraphQL, and MCP.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(newIndexCmd())
	return cmd
}
