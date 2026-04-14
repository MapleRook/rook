package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/scip-code/scip/bindings/go/scip"
	"google.golang.org/protobuf/proto"
)

func main() {
	out := flag.String("out", "index.scip", "output path for the synthetic SCIP index")
	project := flag.String("project", "palace", "project name encoded in scip symbols")
	flag.Parse()

	index := &scip.Index{
		Metadata: &scip.Metadata{
			Version: scip.ProtocolVersion_UnspecifiedProtocolVersion,
			ToolInfo: &scip.ToolInfo{
				Name:    "mkscip",
				Version: "0.0.1",
			},
			ProjectRoot:          "file:///synthetic",
			TextDocumentEncoding: scip.TextEncoding_UTF8,
		},
		Documents: []*scip.Document{
			buildSearchDoc(*project),
			buildStoreDoc(*project),
		},
	}

	data, err := proto.Marshal(index)
	if err != nil {
		fmt.Fprintln(os.Stderr, "marshal:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, data, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %d bytes to %s\n", len(data), *out)
}

func buildSearchDoc(project string) *scip.Document {
	searchSym := fmt.Sprintf("scip-python python %s 0.1.0 `src/search.py`/palace_search().", project)
	storeSym := fmt.Sprintf("scip-python python %s 0.1.0 `src/store.py`/Store#", project)
	return &scip.Document{
		RelativePath: "src/search.py",
		Language:     "Python",
		Symbols: []*scip.SymbolInformation{
			{
				Symbol:        searchSym,
				DisplayName:   "palace_search",
				Kind:          scip.SymbolInformation_Function,
				Documentation: []string{"Search the palace index."},
			},
		},
		Occurrences: []*scip.Occurrence{
			{
				Symbol:      searchSym,
				Range:       []int32{10, 4, 17},
				SymbolRoles: int32(scip.SymbolRole_Definition),
			},
			{
				Symbol:      storeSym,
				Range:       []int32{3, 7, 12},
				SymbolRoles: int32(scip.SymbolRole_Import),
			},
			{
				Symbol: storeSym,
				Range:  []int32{25, 12, 17},
			},
		},
	}
}

func buildStoreDoc(project string) *scip.Document {
	storeSym := fmt.Sprintf("scip-python python %s 0.1.0 `src/store.py`/Store#", project)
	initSym := fmt.Sprintf("scip-python python %s 0.1.0 `src/store.py`/Store#__init__().", project)
	return &scip.Document{
		RelativePath: "src/store.py",
		Language:     "Python",
		Symbols: []*scip.SymbolInformation{
			{
				Symbol:        storeSym,
				DisplayName:   "Store",
				Kind:          scip.SymbolInformation_Class,
				Documentation: []string{"Backing store for the palace index."},
			},
			{
				Symbol:      initSym,
				DisplayName: "__init__",
				Kind:        scip.SymbolInformation_Method,
			},
		},
		Occurrences: []*scip.Occurrence{
			{
				Symbol:      storeSym,
				Range:       []int32{4, 6, 11},
				SymbolRoles: int32(scip.SymbolRole_Definition),
			},
			{
				Symbol:      initSym,
				Range:       []int32{7, 8, 16},
				SymbolRoles: int32(scip.SymbolRole_Definition),
			},
		},
	}
}
