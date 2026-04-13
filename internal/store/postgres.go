package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgStore struct {
	pool *pgxpool.Pool
}

func Open(ctx context.Context, url string) (Store, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &pgStore{pool: pool}, nil
}

func (s *pgStore) Close() {
	s.pool.Close()
}

func (s *pgStore) BeginIngest(ctx context.Context) (IngestTx, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	return &pgIngestTx{tx: tx}, nil
}

type pgIngestTx struct {
	tx pgx.Tx
}

func (t *pgIngestTx) Commit(ctx context.Context) error {
	return t.tx.Commit(ctx)
}

func (t *pgIngestTx) Rollback(ctx context.Context) error {
	return t.tx.Rollback(ctx)
}

func (t *pgIngestTx) ReplaceRepo(ctx context.Context, name, rootPath string) (int32, error) {
	if _, err := t.tx.Exec(ctx, `DELETE FROM repos WHERE name = $1`, name); err != nil {
		return 0, fmt.Errorf("delete existing repo: %w", err)
	}
	var id int32
	err := t.tx.QueryRow(ctx,
		`INSERT INTO repos (name, root_path) VALUES ($1, $2) RETURNING id`,
		name, rootPath,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert repo: %w", err)
	}
	return id, nil
}

func (t *pgIngestTx) InsertDocuments(ctx context.Context, repoID int32, docs []Document) (map[string]int32, error) {
	ids := make(map[string]int32, len(docs))
	if len(docs) == 0 {
		return ids, nil
	}
	rows := make([][]any, len(docs))
	for i, d := range docs {
		rows[i] = []any{repoID, d.RelativePath, d.Language}
	}
	if _, err := t.tx.CopyFrom(ctx,
		pgx.Identifier{"documents"},
		[]string{"repo_id", "relative_path", "language"},
		pgx.CopyFromRows(rows),
	); err != nil {
		return nil, fmt.Errorf("copy documents: %w", err)
	}
	r, err := t.tx.Query(ctx,
		`SELECT id, relative_path FROM documents WHERE repo_id = $1`, repoID)
	if err != nil {
		return nil, fmt.Errorf("select documents: %w", err)
	}
	defer r.Close()
	for r.Next() {
		var id int32
		var path string
		if err := r.Scan(&id, &path); err != nil {
			return nil, err
		}
		ids[path] = id
	}
	return ids, r.Err()
}

func (t *pgIngestTx) InsertSymbols(ctx context.Context, repoID int32, symbols []Symbol) (map[string]int32, error) {
	ids := make(map[string]int32, len(symbols))
	if len(symbols) == 0 {
		return ids, nil
	}
	rows := make([][]any, len(symbols))
	for i, s := range symbols {
		rows[i] = []any{repoID, s.SCIPSymbol, s.DisplayName, s.Kind, nullString(s.Signature), nullString(s.Documentation)}
	}
	if _, err := t.tx.CopyFrom(ctx,
		pgx.Identifier{"symbols"},
		[]string{"repo_id", "scip_symbol", "display_name", "kind", "signature", "documentation"},
		pgx.CopyFromRows(rows),
	); err != nil {
		return nil, fmt.Errorf("copy symbols: %w", err)
	}
	r, err := t.tx.Query(ctx,
		`SELECT id, scip_symbol FROM symbols WHERE repo_id = $1`, repoID)
	if err != nil {
		return nil, fmt.Errorf("select symbols: %w", err)
	}
	defer r.Close()
	for r.Next() {
		var id int32
		var sym string
		if err := r.Scan(&id, &sym); err != nil {
			return nil, err
		}
		ids[sym] = id
	}
	return ids, r.Err()
}

func (t *pgIngestTx) InsertOccurrences(ctx context.Context, occs []Occurrence) error {
	if len(occs) == 0 {
		return nil
	}
	rows := make([][]any, len(occs))
	for i, o := range occs {
		rows[i] = []any{o.DocumentID, o.SymbolID, o.StartLine, o.StartCol, o.EndLine, o.EndCol, o.Role}
	}
	if _, err := t.tx.CopyFrom(ctx,
		pgx.Identifier{"occurrences"},
		[]string{"document_id", "symbol_id", "start_line", "start_col", "end_line", "end_col", "role"},
		pgx.CopyFromRows(rows),
	); err != nil {
		return fmt.Errorf("copy occurrences: %w", err)
	}
	return nil
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
