package store

import "context"

type Document struct {
	RelativePath string
	Language     string
}

type Symbol struct {
	SCIPSymbol    string
	DisplayName   string
	Kind          string
	Signature     string
	Documentation string
}

type Occurrence struct {
	DocumentID int32
	SymbolID   int32
	StartLine  int32
	StartCol   int32
	EndLine    int32
	EndCol     int32
	Role       string
}

const (
	RoleDefinition = "definition"
	RoleReference  = "reference"
	RoleImport     = "import"
	RoleWrite      = "write"
)

type Store interface {
	BeginIngest(ctx context.Context) (IngestTx, error)
	Close()
}

type IngestTx interface {
	ReplaceRepo(ctx context.Context, name, rootPath string) (repoID int32, err error)
	InsertDocuments(ctx context.Context, repoID int32, docs []Document) (ids map[string]int32, err error)
	InsertSymbols(ctx context.Context, repoID int32, symbols []Symbol) (ids map[string]int32, err error)
	InsertOccurrences(ctx context.Context, occs []Occurrence) error
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}
