package ingest

import (
	"testing"

	"github.com/scip-code/scip/bindings/go/scip"
)

func TestDecodeRange(t *testing.T) {
	cases := []struct {
		name                             string
		in                               []int32
		wantSL, wantSC, wantEL, wantEC int32
	}{
		{"same line", []int32{10, 4, 12}, 10, 4, 10, 12},
		{"multi line", []int32{10, 4, 14, 2}, 10, 4, 14, 2},
		{"empty", []int32{}, 0, 0, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sl, sc, el, ec := decodeRange(tc.in)
			if sl != tc.wantSL || sc != tc.wantSC || el != tc.wantEL || ec != tc.wantEC {
				t.Fatalf("got (%d,%d,%d,%d), want (%d,%d,%d,%d)",
					sl, sc, el, ec, tc.wantSL, tc.wantSC, tc.wantEL, tc.wantEC)
			}
		})
	}
}

func TestRoleFromMask(t *testing.T) {
	cases := []struct {
		mask int32
		want string
	}{
		{int32(scip.SymbolRole_Definition), "definition"},
		{int32(scip.SymbolRole_Import), "import"},
		{int32(scip.SymbolRole_WriteAccess), "write"},
		{0, "reference"},
		{int32(scip.SymbolRole_Definition) | int32(scip.SymbolRole_WriteAccess), "definition"},
	}
	for _, tc := range cases {
		if got := roleFromMask(tc.mask); got != tc.want {
			t.Errorf("mask=%d: got %q, want %q", tc.mask, got, tc.want)
		}
	}
}

func TestSymbolDisplayNameFallback(t *testing.T) {
	sym := "scip-python python rook 0.1.0 src/palace/search.py/palace_search()."
	got := symbolDisplayName(sym)
	if got != "palace_search" {
		t.Errorf("got %q, want palace_search", got)
	}
}

func TestSymbolInfoDisplayPrefersExplicit(t *testing.T) {
	si := &scip.SymbolInformation{
		Symbol:      "scip-python python rook 0.1.0 src/palace/search.py/palace_search().",
		DisplayName: "palace_search",
	}
	if got := symbolInfoDisplay(si); got != "palace_search" {
		t.Errorf("got %q", got)
	}
}

func TestTransformDocument(t *testing.T) {
	d := &scip.Document{
		RelativePath: "src/palace/search.py",
		Language:     "Python",
		Symbols: []*scip.SymbolInformation{
			{
				Symbol:        "scip-python python rook 0.1.0 src/palace/search.py/palace_search().",
				Kind:          scip.SymbolInformation_Function,
				Documentation: []string{"Search the palace.", "Returns memories."},
			},
		},
		Occurrences: []*scip.Occurrence{
			{
				Range:        []int32{41, 0, 41, 13},
				Symbol:       "scip-python python rook 0.1.0 src/palace/search.py/palace_search().",
				SymbolRoles:  int32(scip.SymbolRole_Definition),
			},
			{
				Range:       []int32{42, 4, 20},
				Symbol:      "scip-python python rook 0.1.0 src/palace/search.py/_rerank_by_recency().",
				SymbolRoles: 0,
			},
		},
	}
	r, err := transformDocument(d)
	if err != nil {
		t.Fatalf("transformDocument: %v", err)
	}
	if r.doc.RelativePath != "src/palace/search.py" || r.doc.Language != "Python" {
		t.Errorf("doc = %+v", r.doc)
	}
	if len(r.symbols) != 1 || r.symbols[0].DisplayName != "palace_search" {
		t.Errorf("symbols = %+v", r.symbols)
	}
	if r.symbols[0].Documentation != "Search the palace.\n\nReturns memories." {
		t.Errorf("doc = %q", r.symbols[0].Documentation)
	}
	if len(r.occs) != 2 {
		t.Fatalf("want 2 occs, got %d", len(r.occs))
	}
	if r.occs[0].StartLine != 41 || r.occs[0].EndCol != 13 {
		t.Errorf("occ[0] range = %+v", r.occs[0])
	}
	if r.occs[1].StartLine != 42 || r.occs[1].EndLine != 42 || r.occs[1].EndCol != 20 {
		t.Errorf("occ[1] range = %+v", r.occs[1])
	}
}
