package integration_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/caioreix/swagger-mcp/internal/testutil"
)

var (
	buildOnce  sync.Once
	binaryPath string
	binaryDir  string
	buildErr   error
)

func TestMain(m *testing.M) {
	code := m.Run()
	if binaryDir != "" {
		_ = os.RemoveAll(binaryDir)
	}
	os.Exit(code)
}

func compiledBinary(tb testing.TB) string {
	tb.Helper()

	buildOnce.Do(func() {
		repoRoot := testutil.RepoRoot(tb)
		var err error
		binaryDir, err = os.MkdirTemp("", "swagger-mcp-bin-*")
		if err != nil {
			buildErr = fmt.Errorf("create binary temp dir: %w", err)
			return
		}
		name := "swagger-mcp"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		binaryPath = filepath.Join(binaryDir, name)
		command := exec.Command("go", "build", "-o", binaryPath, "./cmd/swagger-mcp")
		command.Dir = repoRoot
		output, err := command.CombinedOutput()
		if err != nil {
			buildErr = fmt.Errorf("build binary: %w\n%s", err, string(output))
		}
	})

	if buildErr != nil {
		tb.Fatalf("compile binary: %v", buildErr)
	}
	return binaryPath
}

func runBinary(tb testing.TB, args []string, stdinLines []string, env map[string]string) (string, string) {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	command := exec.CommandContext(ctx, compiledBinary(tb), args...)
	command.Dir = tb.TempDir()
	command.Env = append([]string{}, os.Environ()...)
	for key, value := range env {
		command.Env = append(command.Env, key+"="+value)
	}

	stdin, err := command.StdinPipe()
	if err != nil {
		tb.Fatalf("stdin pipe: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Start(); err != nil {
		tb.Fatalf("start binary: %v", err)
	}
	for _, line := range stdinLines {
		if _, err := stdin.Write([]byte(line + "\n")); err != nil {
			tb.Fatalf("write stdin: %v", err)
		}
	}
	_ = stdin.Close()

	if err := command.Wait(); err != nil {
		tb.Fatalf("wait for binary: %v\nstderr:\n%s", err, stderr.String())
	}
	if ctx.Err() != nil {
		tb.Fatalf("binary timed out: %v", ctx.Err())
	}

	return stdout.String(), stderr.String()
}

func fixtureServer(tb testing.TB, fixtureName, contentType string) *httptest.Server {
	tb.Helper()
	payload, err := os.ReadFile(testutil.FixturePath(tb, fixtureName))
	if err != nil {
		tb.Fatalf("read fixture %s: %v", fixtureName, err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", contentType)
		_, _ = writer.Write(payload)
	}))
	tb.Cleanup(server.Close)
	return server
}
