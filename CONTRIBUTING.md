# Contributing to swagger-mcp

Thank you for your interest in contributing! This guide covers setup, workflow, and expectations.

## Prerequisites

- **Go 1.26+** — see `go.mod` for the exact minimum
- **make** — all development commands are in the Makefile
- **git**

## Local Setup

```bash
git clone https://github.com/caioreix/swagger-mcp.git
cd swagger-mcp
go mod download
make build        # compile the binary → build/swagger-mcp
make verify       # build + test + vet
```

## Development Workflow

| Command | Description |
|---------|-------------|
| `make build` | Compile the binary to `build/swagger-mcp` |
| `make test` | Run all tests (`go test ./...`) |
| `make vet` | Run static analysis (`go vet ./...`) |
| `make verify` | Run build, test, and vet — run this before opening a PR |
| `make fmt` | Format source files with `gofmt` |
| `make run ARGS='...'` | Build and run with custom arguments |
| `make inspector` | Open the MCP Inspector against the local binary |

## Project Layout

```
cmd/swagger-mcp/         Entry point and Cobra CLI commands
  cmd/                   Subcommands (serve, generate, inspect, download, version)
internal/
  app/                   MCP server bootstrap & transport routing
  codegen/               Go code generation from OpenAPI specs
  config/                Config struct, .env loading, auth from env
  logging/               Structured slog setup
  mcp/                   MCP tools and protocol handlers
  openapi/               OpenAPI parsing, filtering, inspection
testdata/                Shared test fixtures (OpenAPI specs, golden files)
test/                    Integration test helpers
```

## Writing Tests

- Tests live next to the code they test (e.g., `internal/openapi/inspect_test.go`)
- Use `testutil.FixturePath(t, "file.json")` to load fixtures from `testdata/`
- Use golden files (`testutil.ReadGolden`) for output-heavy assertions
- Run `go test -race ./...` before submitting to catch data races

## Making a Pull Request

1. Fork the repo and create a branch off `main`
2. Make your changes and add tests
3. Run `make verify` — all checks must pass
4. Open a PR with a clear description of the change and why it's needed
5. Reference any related issues

## Code Style

- Standard `gofmt` formatting (enforced via `make fmt`)
- Functions and types should have doc comments when their purpose isn't obvious
- Keep CLI flag descriptions short but include the env var equivalent (e.g. `env: SWAGGER_MCP_PORT`)
- stdout is reserved for JSON-RPC in stdio transport mode — never write non-protocol output to stdout inside the serve path

## Reporting Issues

Please open a [GitHub Issue](https://github.com/caioreix/swagger-mcp/issues) with:
- A clear description of the problem
- Steps to reproduce
- Expected vs. actual behavior
- swagger-mcp version (`swagger-mcp version`) and OS
