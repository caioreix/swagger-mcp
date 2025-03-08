package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

func GoldenPath(tb testing.TB, name string) string {
	tb.Helper()
	return filepath.Join(RepoRoot(tb), "testdata", "golden", name)
}

func ReadGolden(tb testing.TB, name string) string {
	tb.Helper()
	content, err := os.ReadFile(GoldenPath(tb, name))
	if err != nil {
		tb.Fatalf("read golden file %s: %v", name, err)
	}
	return string(content)
}
