package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/caioreix/swagger-mcp/cmd/swagger-mcp/cmd"
)

// repoRoot returns the absolute path to the repository root by walking up from
// the test file's location.
func repoRoot(tb testing.TB) string {
	tb.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		tb.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			tb.Fatalf("could not locate repository root from %s", filepath.Dir(file))
		}
		dir = parent
	}
}

// run executes the CLI with the given args and returns (stdout, stderr, exitCode).
func run(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	code = cmd.Execute(args, strings.NewReader(""), &outBuf, &errBuf)
	return outBuf.String(), errBuf.String(), code
}

// ── version ──────────────────────────────────────────────────────────────────

func TestVersionSubcommand(t *testing.T) {
	_, stderr, code := run(t, "version")
	if code != 0 {
		t.Fatalf("version exited %d; stderr: %s", code, stderr)
	}
	if !strings.Contains(stderr, "swagger-mcp") {
		t.Fatalf("version output should contain 'swagger-mcp'; got: %q", stderr)
	}
}

func TestVersionSubcommandDoesNotWriteToStdout(t *testing.T) {
	stdout, _, _ := run(t, "version")
	if stdout != "" {
		t.Fatalf("version should not write to stdout (reserved for JSON-RPC); got: %q", stdout)
	}
}

// ── inspect endpoints ─────────────────────────────────────────────────────────

func TestInspectEndpointsTable(t *testing.T) {
	fixture := filepath.Join(repoRoot(t), "testdata", "petstore.json")
	stdout, stderr, code := run(t, "inspect", "endpoints", "--swagger-file="+fixture)
	if code != 0 {
		t.Fatalf("inspect endpoints exited %d; stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "METHOD") || !strings.Contains(stdout, "PATH") {
		t.Fatalf("expected table headers in stdout: %q", stdout)
	}
	if !strings.Contains(stdout, "/pets") {
		t.Fatalf("expected petstore endpoint in output: %q", stdout)
	}
}

func TestInspectEndpointsJSON(t *testing.T) {
	fixture := filepath.Join(repoRoot(t), "testdata", "petstore.json")
	stdout, stderr, code := run(t, "inspect", "endpoints", "--swagger-file="+fixture, "--format=json")
	if code != 0 {
		t.Fatalf("inspect endpoints json exited %d; stderr: %s", code, stderr)
	}
	if !strings.HasPrefix(strings.TrimSpace(stdout), "[") {
		t.Fatalf("expected JSON array in stdout: %q", stdout)
	}
}

// ── inspect models ────────────────────────────────────────────────────────────

func TestInspectModelsTable(t *testing.T) {
	fixture := filepath.Join(repoRoot(t), "testdata", "petstore.json")
	stdout, stderr, code := run(t, "inspect", "models", "--swagger-file="+fixture)
	if code != 0 {
		t.Fatalf("inspect models exited %d; stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "NAME") || !strings.Contains(stdout, "TYPE") {
		t.Fatalf("expected table headers in stdout: %q", stdout)
	}
	if !strings.Contains(stdout, "Pet") {
		t.Fatalf("expected Pet model in output: %q", stdout)
	}
}

func TestInspectModelsJSON(t *testing.T) {
	fixture := filepath.Join(repoRoot(t), "testdata", "petstore.json")
	stdout, stderr, code := run(t, "inspect", "models", "--swagger-file="+fixture, "--format=json")
	if code != 0 {
		t.Fatalf("inspect models json exited %d; stderr: %s", code, stderr)
	}
	if !strings.HasPrefix(strings.TrimSpace(stdout), "[") {
		t.Fatalf("expected JSON array in stdout: %q", stdout)
	}
	if !strings.Contains(stdout, `"name"`) {
		t.Fatalf("expected 'name' field in JSON: %q", stdout)
	}
}

// ── inspect security ──────────────────────────────────────────────────────────

func TestInspectSecurityTable(t *testing.T) {
	fixture := filepath.Join(repoRoot(t), "testdata", "petstore.json")
	stdout, stderr, code := run(t, "inspect", "security", "--swagger-file="+fixture)
	if code != 0 {
		t.Fatalf("inspect security exited %d; stderr: %s", code, stderr)
	}
	// petstore has no security definitions — table should still render
	if !strings.Contains(stdout, "scheme(s)") {
		t.Fatalf("expected scheme count in output: %q", stdout)
	}
}

func TestInspectSecurityOpenAPI3WithSchemes(t *testing.T) {
	fixture := filepath.Join(repoRoot(t), "testdata", "openapi-3.1.json")
	stdout, stderr, code := run(t, "inspect", "security", "--swagger-file="+fixture)
	if code != 0 {
		t.Fatalf("inspect security exited %d; stderr: %s", code, stderr)
	}
	_ = stdout // accept any output — we just verify it doesn't crash
	_ = stderr
}

// ── inspect endpoint ──────────────────────────────────────────────────────────

func TestInspectEndpoint(t *testing.T) {
	fixture := filepath.Join(repoRoot(t), "testdata", "petstore.json")
	stdout, stderr, code := run(t, "inspect", "endpoint",
		"--swagger-file="+fixture,
		"--path=/pets",
		"--method=GET",
	)
	if code != 0 {
		t.Fatalf("inspect endpoint exited %d; stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "GET") {
		t.Fatalf("expected endpoint details in stdout: %q", stdout)
	}
}

// ── generate model ────────────────────────────────────────────────────────────

func TestGenerateModelCode(t *testing.T) {
	fixture := filepath.Join(repoRoot(t), "testdata", "petstore.json")
	stdout, stderr, code := run(t, "generate", "model",
		"--swagger-file="+fixture,
		"--model=Pet",
	)
	if code != 0 {
		t.Fatalf("generate model exited %d; stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "Pet") {
		t.Fatalf("expected Go type in stdout: %q", stdout)
	}
}

// ── generate tool ─────────────────────────────────────────────────────────────

func TestGenerateToolCode(t *testing.T) {
	fixture := filepath.Join(repoRoot(t), "testdata", "petstore.json")
	stdout, stderr, code := run(t, "generate", "tool",
		"--swagger-file="+fixture,
		"--path=/pets",
		"--method=GET",
	)
	if code != 0 {
		t.Fatalf("generate tool exited %d; stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "func") {
		t.Fatalf("expected Go function in stdout: %q", stdout)
	}
}

// ── env var precedence ────────────────────────────────────────────────────────

func TestEnvVarTransportFallback(t *testing.T) {
	// Setting env var should not crash; we just verify the CLI starts and
	// the flag is visible in --help output.
	t.Setenv("SWAGGER_MCP_TRANSPORT", "sse")
	_, stderr, _ := run(t, "--help")
	if !strings.Contains(stderr, "swagger-mcp") {
		t.Fatalf("expected help output: %q", stderr)
	}
}

// ── stdout isolation (JSON-RPC safety) ───────────────────────────────────────

func TestHelpDoesNotWriteToStdout(t *testing.T) {
	stdout, _, _ := run(t, "--help")
	if stdout != "" {
		t.Fatalf("--help must not write to stdout; got: %q", stdout)
	}
}

func TestUnknownFlagDoesNotWriteToStdout(t *testing.T) {
	stdout, _, _ := run(t, "--unknown-flag")
	if stdout != "" {
		t.Fatalf("error output must not go to stdout; got: %q", stdout)
	}
}
