package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/MapleRook/rook/internal/ingest"
	"github.com/MapleRook/rook/internal/store"
)

func newIndexCmd() *cobra.Command {
	var (
		databaseURL string
		indexer     string
		keepIndex   bool
		workers     int
		scipFile    string
	)
	cmd := &cobra.Command{
		Use:   "index <path>",
		Short: "Run a SCIP indexer against a repo and load the result into Postgres",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIndex(cmd.Context(), indexOptions{
				repoPath:    args[0],
				databaseURL: resolveDatabaseURL(databaseURL),
				indexer:     indexer,
				keepIndex:   keepIndex,
				workers:     workers,
				scipFile:    scipFile,
			})
		},
	}
	cmd.Flags().StringVar(&databaseURL, "database-url", "", "Postgres connection URL (defaults to $ROOK_DATABASE_URL)")
	cmd.Flags().StringVar(&indexer, "indexer", "auto", "SCIP indexer to run: auto | scip-python | scip-typescript")
	cmd.Flags().BoolVar(&keepIndex, "keep-index", false, "Keep the intermediate index.scip file on disk")
	cmd.Flags().IntVar(&workers, "workers", 4, "Document-level worker count for ingest")
	cmd.Flags().StringVar(&scipFile, "scip", "", "Ingest a pre-built SCIP index from this path instead of running an indexer")
	return cmd
}

type indexOptions struct {
	repoPath    string
	databaseURL string
	indexer     string
	keepIndex   bool
	workers     int
	scipFile    string
}

func runIndex(ctx context.Context, opts indexOptions) error {
	if opts.databaseURL == "" {
		return errors.New("no database URL: pass --database-url or set ROOK_DATABASE_URL")
	}
	absRepo, err := filepath.Abs(opts.repoPath)
	if err != nil {
		return fmt.Errorf("resolve repo path: %w", err)
	}
	if info, err := os.Stat(absRepo); err != nil {
		return fmt.Errorf("stat repo path: %w", err)
	} else if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", absRepo)
	}

	st, err := store.Open(ctx, opts.databaseURL)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	fmt.Printf("indexing %s\n", filepath.Base(absRepo))
	start := time.Now()

	var indexPath string
	var cleanup func()
	if opts.scipFile != "" {
		indexPath = opts.scipFile
		fmt.Printf("  using existing scip index at %s\n", indexPath)
	} else {
		runner := ingest.NewRunner(opts.indexer)
		indexPath, cleanup, err = runner.Run(ctx, absRepo)
		if err != nil {
			return fmt.Errorf("run indexer: %w", err)
		}
		if !opts.keepIndex {
			defer cleanup()
		}
		fmt.Printf("  ran %s\n", runner.Name())
	}

	result, err := ingest.Load(ctx, st, ingest.LoadOptions{
		RepoName:  filepath.Base(absRepo),
		RepoRoot:  absRepo,
		IndexPath: indexPath,
		Workers:   opts.workers,
	})
	if err != nil {
		return fmt.Errorf("load index: %w", err)
	}
	fmt.Printf("  ingested %d symbols, %d occurrences across %d files\n",
		result.Symbols, result.Occurrences, result.Documents)
	fmt.Printf("  done in %s\n", time.Since(start).Round(time.Millisecond))
	return nil
}

func resolveDatabaseURL(flag string) string {
	if flag != "" {
		return flag
	}
	return os.Getenv("ROOK_DATABASE_URL")
}
