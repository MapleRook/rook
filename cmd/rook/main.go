package main

import (
	"fmt"
	"os"

	"github.com/MapleRook/rook/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "rook:", err)
		os.Exit(1)
	}
}
