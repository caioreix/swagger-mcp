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

type testBinaryCache struct {
	once     sync.Once
	path     string
	dir      string
	errBuild error
}

var testBinary testBinaryCache //nolint:gochecknoglobals // needed for TestMain

func TestMain(m *testing.M) {
	var err error
	testBinary.dir, err = os.MkdirTemp("", "swagger-mcp-bin-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create binary temp dir: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(testBinary.dir)
	os.Exit(code)
}

func compiledBinary(tb testing.TB) string {
	tb.Helper()

	testBinary.once.Do(func() {
		repoRoot := testutil.RepoRoot(tb)
		name := "swagger-mcp"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		testBinary.path = filepath.Join(testBinary.dir, name)
		command := exec.Command("go", "build", "-o", testBinary.path, "./cmd/swagger-mcp")
		command.Dir = repoRoot
		output, err := command.CombinedOutput()
		if err != nil {
			testBinary.errBuild = fmt.Errorf("build binary: %w\n%s", err, string(output))
		}
	})

	if testBinary.errBuild != nil {
		tb.Fatalf("compile binary: %v", testBinary.errBuild)
	}
	return testBinary.path
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

	if startErr := command.Start(); startErr != nil {
		tb.Fatalf("start binary: %v", startErr)
	}
	for _, line := range stdinLines {
		if _, writeErr := stdin.Write([]byte(line + "\n")); writeErr != nil {
			tb.Fatalf("write stdin: %v", writeErr)
		}
	}
	_ = stdin.Close()

	if waitErr := command.Wait(); waitErr != nil {
		tb.Fatalf("wait for binary: %v\nstderr:\n%s", waitErr, stderr.String())
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
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", contentType)
		_, _ = writer.Write(payload)
	}))
	tb.Cleanup(server.Close)
	return server
}
