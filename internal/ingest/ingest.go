package ingest

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/scip-code/scip/bindings/go/scip"
	"google.golang.org/protobuf/proto"

	"github.com/MapleRook/rook/internal/store"
)

type LoadOptions struct {
	RepoName  string
	RepoRoot  string
	IndexPath string
	Workers   int
}

type Result struct {
	Documents   int
	Symbols     int
	Occurrences int
}

func Load(ctx context.Context, st store.Store, opts LoadOptions) (Result, error) {
	data, err := os.ReadFile(opts.IndexPath)
	if err != nil {
		return Result{}, fmt.Errorf("read index: %w", err)
	}
	var index scip.Index
	if err := proto.Unmarshal(data, &index); err != nil {
		return Result{}, fmt.Errorf("unmarshal scip index: %w", err)
	}

	results, err := transformDocuments(ctx, index.Documents, opts.Workers)
	if err != nil {
		return Result{}, err
	}

	docs := make([]store.Document, 0, len(results))
	symbolMap := make(map[string]store.Symbol)
	var raws []rawOcc
	for _, r := range results {
		docs = append(docs, r.doc)
		for _, s := range r.symbols {
			if _, exists := symbolMap[s.SCIPSymbol]; !exists {
				symbolMap[s.SCIPSymbol] = s
			}
		}
		raws = append(raws, r.occs...)
	}
	for _, o := range raws {
		if _, exists := symbolMap[o.SCIPSymbol]; !exists {
			symbolMap[o.SCIPSymbol] = store.Symbol{
				SCIPSymbol:  o.SCIPSymbol,
				DisplayName: symbolDisplayName(o.SCIPSymbol),
				Kind:        "Unknown",
			}
		}
	}

	symbols := make([]store.Symbol, 0, len(symbolMap))
	for _, s := range symbolMap {
		symbols = append(symbols, s)
	}

	tx, err := st.BeginIngest(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("begin: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	repoID, err := tx.ReplaceRepo(ctx, opts.RepoName, opts.RepoRoot)
	if err != nil {
		return Result{}, err
	}
	docIDs, err := tx.InsertDocuments(ctx, repoID, docs)
	if err != nil {
		return Result{}, err
	}
	symIDs, err := tx.InsertSymbols(ctx, repoID, symbols)
	if err != nil {
		return Result{}, err
	}

	finalOccs := make([]store.Occurrence, 0, len(raws))
	for _, o := range raws {
		dID, ok := docIDs[o.DocPath]
		if !ok {
			continue
		}
		sID, ok := symIDs[o.SCIPSymbol]
		if !ok {
			continue
		}
		finalOccs = append(finalOccs, store.Occurrence{
			DocumentID: dID,
			SymbolID:   sID,
			StartLine:  o.StartLine,
			StartCol:   o.StartCol,
			EndLine:    o.EndLine,
			EndCol:     o.EndCol,
			Role:       roleFromMask(o.Roles),
		})
	}
	if err := tx.InsertOccurrences(ctx, finalOccs); err != nil {
		return Result{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Result{}, err
	}
	committed = true

	return Result{
		Documents:   len(docs),
		Symbols:     len(symbols),
		Occurrences: len(finalOccs),
	}, nil
}

type rawOcc struct {
	DocPath    string
	SCIPSymbol string
	StartLine  int32
	StartCol   int32
	EndLine    int32
	EndCol     int32
	Roles      int32
}

type docResult struct {
	doc     store.Document
	symbols []store.Symbol
	occs    []rawOcc
}

func transformDocuments(ctx context.Context, documents []*scip.Document, workers int) ([]docResult, error) {
	if workers < 1 {
		workers = 1
	}
	type job struct {
		idx int
		doc *scip.Document
	}
	results := make([]docResult, len(documents))
	jobs := make(chan job)
	var wg sync.WaitGroup
	var firstErr error
	var errMu sync.Mutex

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				r, err := transformDocument(j.doc)
				if err != nil {
					errMu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					errMu.Unlock()
					continue
				}
				results[j.idx] = r
			}
		}()
	}

	var feedErr error
feed:
	for i, d := range documents {
		select {
		case <-ctx.Done():
			feedErr = ctx.Err()
			break feed
		case jobs <- job{idx: i, doc: d}:
		}
	}
	close(jobs)
	wg.Wait()

	if feedErr != nil {
		return nil, feedErr
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return results, nil
}

func transformDocument(d *scip.Document) (docResult, error) {
	doc := store.Document{
		RelativePath: d.RelativePath,
		Language:     d.Language,
	}
	syms := make([]store.Symbol, 0, len(d.Symbols))
	for _, si := range d.Symbols {
		syms = append(syms, store.Symbol{
			SCIPSymbol:    si.Symbol,
			DisplayName:   symbolInfoDisplay(si),
			Kind:          si.Kind.String(),
			Documentation: strings.Join(si.Documentation, "\n\n"),
		})
	}
	occs := make([]rawOcc, 0, len(d.Occurrences))
	for _, o := range d.Occurrences {
		sl, sc, el, ec := decodeRange(o.Range)
		occs = append(occs, rawOcc{
			DocPath:    d.RelativePath,
			SCIPSymbol: o.Symbol,
			StartLine:  sl,
			StartCol:   sc,
			EndLine:    el,
			EndCol:     ec,
			Roles:      o.SymbolRoles,
		})
	}
	return docResult{doc: doc, symbols: syms, occs: occs}, nil
}

func decodeRange(r []int32) (int32, int32, int32, int32) {
	switch len(r) {
	case 3:
		return r[0], r[1], r[0], r[2]
	case 4:
		return r[0], r[1], r[2], r[3]
	}
	return 0, 0, 0, 0
}

func roleFromMask(mask int32) string {
	switch {
	case mask&int32(scip.SymbolRole_Definition) != 0:
		return store.RoleDefinition
	case mask&int32(scip.SymbolRole_Import) != 0:
		return store.RoleImport
	case mask&int32(scip.SymbolRole_WriteAccess) != 0:
		return store.RoleWrite
	default:
		return store.RoleReference
	}
}

func symbolInfoDisplay(si *scip.SymbolInformation) string {
	if si.DisplayName != "" {
		return si.DisplayName
	}
	return symbolDisplayName(si.Symbol)
}

func symbolDisplayName(s string) string {
	sym, err := scip.ParseSymbol(s)
	if err != nil || len(sym.Descriptors) == 0 {
		return s
	}
	return sym.Descriptors[len(sym.Descriptors)-1].Name
}
