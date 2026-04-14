package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/MapleRook/rook/internal/store"
)

func newMigrateCmd() *cobra.Command {
	var databaseURL string
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Apply pending Postgres migrations",
		RunE: func(cmd *cobra.Command, _ []string) error {
			url := resolveDatabaseURL(databaseURL)
			if url == "" {
				return errors.New("no database URL: pass --database-url or set ROOK_DATABASE_URL")
			}
			if err := store.Migrate(cmd.Context(), url); err != nil {
				return err
			}
			fmt.Println("migrations up to date")
			return nil
		},
	}
	cmd.Flags().StringVar(&databaseURL, "database-url", "", "Postgres connection URL (defaults to $ROOK_DATABASE_URL)")
	return cmd
}
