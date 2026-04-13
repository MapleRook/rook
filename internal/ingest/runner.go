package ingest

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type Runner struct {
	chosen string
}

func NewRunner(indexer string) *Runner {
	if indexer == "" {
		indexer = "auto"
	}
	return &Runner{chosen: indexer}
}

func (r *Runner) Name() string {
	return r.chosen
}

func (r *Runner) Run(ctx context.Context, repoPath string) (string, func(), error) {
	if r.chosen == "auto" {
		r.chosen = detectIndexer(repoPath)
	}
	tmpDir, err := os.MkdirTemp("", "rook-ingest-*")
	if err != nil {
		return "", nil, fmt.Errorf("tempdir: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tmpDir) }
	indexPath := filepath.Join(tmpDir, "index.scip")

	var cmd *exec.Cmd
	switch r.chosen {
	case "scip-python":
		cmd = exec.CommandContext(ctx, "scip-python", "index", "--output", indexPath, ".")
	case "scip-typescript":
		cmd = exec.CommandContext(ctx, "scip-typescript", "index", "--output", indexPath)
	default:
		cleanup()
		return "", nil, fmt.Errorf("unknown indexer %q", r.chosen)
	}
	cmd.Dir = repoPath
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("%s failed: %w", r.chosen, err)
	}
	if _, err := os.Stat(indexPath); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("%s produced no index at %s: %w", r.chosen, indexPath, err)
	}
	return indexPath, cleanup, nil
}

func detectIndexer(repoPath string) string {
	py := []string{"pyproject.toml", "setup.py", "setup.cfg", "requirements.txt"}
	for _, f := range py {
		if _, err := os.Stat(filepath.Join(repoPath, f)); err == nil {
			return "scip-python"
		}
	}
	ts := []string{"tsconfig.json", "package.json"}
	for _, f := range ts {
		if _, err := os.Stat(filepath.Join(repoPath, f)); err == nil {
			return "scip-typescript"
		}
	}
	return "scip-python"
}
