package store_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/MapleRook/rook/internal/store"
)

func TestIngestTransactionEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	pgC, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("rook"),
		tcpostgres.WithUsername("rook"),
		tcpostgres.WithPassword("rook"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(90*time.Second),
		),
	)
	if err != nil {
		t.Skipf("testcontainers unavailable (is Docker running?): %v", err)
	}
	t.Cleanup(func() { _ = pgC.Terminate(ctx) })

	url, err := pgC.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}

	applyMigration(t, ctx, url)

	st, err := store.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	tx, err := st.BeginIngest(ctx)
	if err != nil {
		t.Fatal(err)
	}

	repoID, err := tx.ReplaceRepo(ctx, "palace", "/tmp/palace")
	if err != nil {
		t.Fatal(err)
	}

	docIDs, err := tx.InsertDocuments(ctx, repoID, []store.Document{
		{RelativePath: "src/search.py", Language: "Python"},
		{RelativePath: "src/store.py", Language: "Python"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(docIDs) != 2 {
		t.Fatalf("want 2 docs, got %d", len(docIDs))
	}

	const searchSym = "scip-python python palace 0.1.0 `src/search.py`/palace_search()."
	const storeSym = "scip-python python palace 0.1.0 `src/store.py`/Store#"

	symIDs, err := tx.InsertSymbols(ctx, repoID, []store.Symbol{
		{SCIPSymbol: searchSym, DisplayName: "palace_search", Kind: "Function", Documentation: "Search the palace."},
		{SCIPSymbol: storeSym, DisplayName: "Store", Kind: "Class"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(symIDs) != 2 {
		t.Fatalf("want 2 symbols, got %d", len(symIDs))
	}

	err = tx.InsertOccurrences(ctx, []store.Occurrence{
		{
			DocumentID: docIDs["src/search.py"],
			SymbolID:   symIDs[searchSym],
			StartLine:  41, EndLine: 41, StartCol: 4, EndCol: 17,
			Role: store.RoleDefinition,
		},
		{
			DocumentID: docIDs["src/store.py"],
			SymbolID:   symIDs[searchSym],
			StartLine:  12, EndLine: 12, StartCol: 8, EndCol: 21,
			Role: store.RoleReference,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	verify := mustConnect(t, ctx, url)
	defer verify.Close()

	var symCount, occCount, docCount int
	mustScan(t, ctx, verify, `SELECT COUNT(*) FROM symbols WHERE repo_id = $1`, &symCount, repoID)
	mustScan(t, ctx, verify, `SELECT COUNT(*) FROM documents WHERE repo_id = $1`, &docCount, repoID)
	mustScan(t, ctx, verify, `SELECT COUNT(*) FROM occurrences o JOIN symbols s ON s.id = o.symbol_id WHERE s.repo_id = $1`, &occCount, repoID)

	if symCount != 2 || docCount != 2 || occCount != 2 {
		t.Errorf("counts: symbols=%d documents=%d occurrences=%d", symCount, docCount, occCount)
	}

	var displayName, kind, doc string
	mustScan(t, ctx, verify,
		`SELECT display_name, kind, COALESCE(documentation, '') FROM symbols WHERE scip_symbol = $1`,
		[]any{&displayName, &kind, &doc}, searchSym)
	if displayName != "palace_search" || kind != "Function" || doc != "Search the palace." {
		t.Errorf("symbol row: name=%q kind=%q doc=%q", displayName, kind, doc)
	}

	// Re-ingest (ReplaceRepo should wipe the first run).
	tx2, err := st.BeginIngest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	newRepoID, err := tx2.ReplaceRepo(ctx, "palace", "/tmp/palace")
	if err != nil {
		t.Fatal(err)
	}
	if newRepoID == repoID {
		t.Errorf("ReplaceRepo returned the same id after delete")
	}
	if err := tx2.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	var leftover int
	mustScan(t, ctx, verify, `SELECT COUNT(*) FROM symbols WHERE repo_id = $1`, &leftover, repoID)
	if leftover != 0 {
		t.Errorf("old repo rows survived ReplaceRepo: %d", leftover)
	}
}

func applyMigration(t *testing.T, ctx context.Context, url string) {
	t.Helper()
	data, err := os.ReadFile("migrations/0001_init.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	up := strings.Split(string(data), "-- +goose Down")[0]
	pool := mustConnect(t, ctx, url)
	defer pool.Close()
	if _, err := pool.Exec(ctx, up); err != nil {
		t.Fatalf("apply migration: %v", err)
	}
}

func mustConnect(t *testing.T, ctx context.Context, url string) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	return pool
}

func mustScan(t *testing.T, ctx context.Context, pool *pgxpool.Pool, query string, dest any, args ...any) {
	t.Helper()
	switch d := dest.(type) {
	case []any:
		if err := pool.QueryRow(ctx, query, args...).Scan(d...); err != nil {
			t.Fatalf("scan: %v", err)
		}
	default:
		if err := pool.QueryRow(ctx, query, args...).Scan(d); err != nil {
			t.Fatalf("scan: %v", err)
		}
	}
}
