package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

func RepoRoot(tb testing.TB) string {
	tb.Helper()

	current, err := os.Getwd()
	if err != nil {
		tb.Fatalf("get working directory: %v", err)
	}

	for {
		if _, statErr := os.Stat(filepath.Join(current, "go.mod")); statErr == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			tb.Fatalf("could not locate repository root from %s", current)
		}
		current = parent
	}
}

func FixturePath(tb testing.TB, name string) string {
	tb.Helper()
	return filepath.Join(RepoRoot(tb), "testdata", name)
}
