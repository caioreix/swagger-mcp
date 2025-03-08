# swagger-mcp

> **Bridge any REST API with AI assistants using [Model Context Protocol](https://modelcontextprotocol.io/).**

[![Go Version](https://img.shields.io/badge/go-1.26+-00ADD8?logo=go)](https://golang.org/)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](./LICENSE)
[![Go Report](https://goreportcard.com/badge/github.com/caioreix/swagger-mcp)](https://goreportcard.com/report/github.com/caioreix/swagger-mcp)

`swagger-mcp` is an MCP server that reads any Swagger/OpenAPI definition and exposes it to AI clients as structured tools — no glue code required. Point it at your API spec, connect your AI assistant, and start exploring endpoints, generating Go scaffolding, or proxying live requests instantly.

---

## ✨ Features

| Capability | Details |
|---|---|
| **Endpoint discovery** | List all API paths with HTTP methods and descriptions |
| **Model inspection** | Explore request/response schemas for any endpoint |
| **Go code generation** | Generate structs, MCP tool handlers, and complete server projects |
| **Dynamic proxy mode** | Every Swagger endpoint becomes a live MCP tool — zero code required |
| **Endpoint filtering** | Regex-based path and HTTP method include/exclude rules |
| **Authentication** | API Key, Bearer/JWT, Basic Auth, OAuth2 client credentials |
| **Multiple transports** | `stdio`, SSE, and StreamableHTTP |
| **Built-in web UI** | Interactive tool dashboard for manual testing |
| **Anti-hallucination guards** | Proxy tools include explicit LLM instructions to prevent data fabrication |
| **Smart caching** | Downloaded specs cached locally with ETag/Last-Modified revalidation |

---

## ⚡ Quick Start

### 1. Install

```bash
git clone https://github.com/caioreix/swagger-mcp.git
cd swagger-mcp
make build
```

The binary is placed at `build/swagger-mcp`.

### 2. Connect to an API

```bash
./build/swagger-mcp --swagger-url="https://petstore.swagger.io/v2/swagger.json"
```

### 3. Add to Cursor (or any MCP client)

Open **Cursor → Settings → Features → MCP → + Add New MCP Server**:

| Field | Value |
|---|---|
| Name | `My API` |
| Transport | `stdio` |
| Command | `/absolute/path/to/build/swagger-mcp --swagger-url=https://your-api/swagger.json` |

Your AI assistant can now discover endpoints, inspect models, and generate Go code against your API.

---

## 🚀 Usage

### Transport Modes

**`stdio`** (default) — communicates over stdin/stdout, ideal for Cursor and Claude Desktop:

```bash
./build/swagger-mcp --swagger-url="https://petstore.swagger.io/v2/swagger.json"
```

**`sse`** — HTTP server with Server-Sent Events:

```bash
./build/swagger-mcp \
  --transport=sse \
  --port=8080 \
  --swagger-url="https://petstore.swagger.io/v2/swagger.json"
```

Endpoints: `GET /sse` (event stream) · `POST /message?clientId=<id>` (send requests)

**`streamable-http`** — HTTP server following the MCP StreamableHTTP spec:

```bash
./build/swagger-mcp \
  --transport=streamable-http \
  --port=8080 \
  --swagger-url="https://petstore.swagger.io/v2/swagger.json"
```

Endpoints: `POST /mcp` (send/receive) · `DELETE /mcp` (terminate session)

---

### Dynamic Proxy Mode

Enable `--proxy-mode` to turn every Swagger endpoint into a callable MCP tool in real time — the server forwards each tool invocation as an HTTP request to the real API:

```bash
./build/swagger-mcp \
  --proxy-mode \
  --swagger-url="https://petstore.swagger.io/v2/swagger.json"
```

#### Filter which endpoints become tools

```bash
./build/swagger-mcp --proxy-mode \
  --swagger-url="https://petstore.swagger.io/v2/swagger.json" \
  --include-paths="^/pet.*" \
  --exclude-methods="DELETE,PUT"
```

#### Override base URL and add static headers

```bash
./build/swagger-mcp --proxy-mode \
  --swagger-url="https://petstore.swagger.io/v2/swagger.json" \
  --base-url="https://staging.example.com" \
  --headers="X-Tenant=acme,X-Source=mcp"
```

#### Authenticate with environment variables

```bash
BEARER_TOKEN=my-jwt-token ./build/swagger-mcp --proxy-mode \
  --swagger-url="https://petstore.swagger.io/v2/swagger.json"
```

---

### Web UI

Pass `--ui` to open an interactive testing dashboard in your browser:

```bash
./build/swagger-mcp \
  --transport=sse --port=8080 --ui \
  --swagger-url="https://petstore.swagger.io/v2/swagger.json"
```

Open `http://localhost:8080/` to browse tools, fill in parameters, and view JSON responses.

---

## ⚙️ Configuration Reference

### CLI Flags

```
./build/swagger-mcp --help
```

| Flag | Default | Description |
|---|---|---|
| `--swagger-url` | — | URL of the Swagger/OpenAPI definition |
| `--transport` | `stdio` | `stdio`, `sse`, or `streamable-http` |
| `--port` | `8080` | HTTP port for SSE/StreamableHTTP |
| `--ui` | `false` | Enable the built-in web UI |
| `--proxy-mode` | `false` | Dynamic proxy: each endpoint becomes an MCP tool |
| `--base-url` | — | Override the base URL from the spec |
| `--headers` | — | Static proxy headers: `Key=Value,Key2=Value2` |
| `--include-paths` | — | Regex patterns for paths to expose (comma-separated) |
| `--exclude-paths` | — | Regex patterns for paths to hide (comma-separated) |
| `--include-methods` | — | HTTP methods to expose: `GET,POST` |
| `--exclude-methods` | — | HTTP methods to hide: `DELETE,PUT` |
| `--sse-headers` | — | Headers forwarded from SSE clients to API |
| `--http-headers` | — | Headers forwarded from StreamableHTTP clients to API |

> Legacy alias `--swaggerUrl` is still accepted but deprecated.

### Environment Variables

| Variable | Default | Description |
|---|---|---|
| `LOG_LEVEL` | `info` | Log verbosity: `debug`, `info`, `warn`, `error` |
| `API_KEY` | — | API key value |
| `API_KEY_HEADER` | `X-API-Key` | Header name for the API key |
| `API_KEY_IN` | `header` | Where to send the key: `header`, `query`, or `cookie` |
| `BEARER_TOKEN` | — | Bearer / JWT token |
| `BASIC_AUTH_USER` | — | Basic auth username |
| `BASIC_AUTH_PASS` | — | Basic auth password |
| `OAUTH2_TOKEN_URL` | — | OAuth2 token endpoint URL |
| `OAUTH2_CLIENT_ID` | — | OAuth2 client ID |
| `OAUTH2_CLIENT_SECRET` | — | OAuth2 client secret |
| `OAUTH2_SCOPES` | — | OAuth2 scopes (comma-separated) |

You can place any of these in a `.env` file in your working directory — existing environment variables take precedence:

```bash
cp .env.example .env
# edit .env with your values
```

### Swagger Definition Resolution Priority

1. CLI `--swagger-url`
2. `swaggerFilePath` parameter passed directly to a tool
3. `SWAGGER_FILEPATH` in a `.swagger-mcp` file in the working directory
4. Error — no source configured

---

## 🛠️ CLI Commands

In addition to running as an MCP server, `swagger-mcp` exposes standalone commands for working with OpenAPI specs directly from the terminal.

### `serve` — Start the MCP server

Equivalent to running `swagger-mcp` with no subcommand:

```bash
swagger-mcp serve --swagger-url="https://petstore.swagger.io/v2/swagger.json" --transport=sse --port=8080
```

### `inspect` — Explore an OpenAPI spec

List all endpoints with filtering support:

```bash
swagger-mcp inspect endpoints --swagger-url="https://petstore.swagger.io/v2/swagger.json"
swagger-mcp inspect endpoints --swagger-url=... --include-paths='^/pet.*' --include-methods=GET
swagger-mcp inspect endpoints --swagger-url=... --format=json
```

Show full details (summary, tags, models) for one endpoint:

```bash
swagger-mcp inspect endpoint \
  --swagger-url="https://petstore.swagger.io/v2/swagger.json" \
  --path=/pet/{petId} \
  --method=GET
```

### `generate` — Generate Go code

Generate a complete MCP server project:

```bash
swagger-mcp generate server \
  --swagger-url="https://petstore.swagger.io/v2/swagger.json" \
  --module=github.com/acme/petstore-mcp \
  --transport=stdio,sse \
  --output=./petstore-server
```

Generate an MCP tool scaffold for a single endpoint:

```bash
swagger-mcp generate tool \
  --swagger-url="https://petstore.swagger.io/v2/swagger.json" \
  --path=/pet/{petId} \
  --method=GET
```

Generate a Go struct for a schema model:

```bash
swagger-mcp generate model \
  --swagger-url="https://petstore.swagger.io/v2/swagger.json" \
  --model=Pet
```

### `download` — Cache a spec locally

Download a spec and print the saved path (useful for scripts and `.swagger-mcp` setup):

```bash
swagger-mcp download \
  --url="https://petstore.swagger.io/v2/swagger.json" \
  --output=./swagger-cache

# Populate .swagger-mcp in one line:
echo "SWAGGER_FILEPATH=$(swagger-mcp download --url=https://petstore.swagger.io/v2/swagger.json)" > .swagger-mcp
```

---

## 📦 MCP Tools

| Tool | Description |
|---|---|
| `getSwaggerDefinition` | Download and cache a Swagger/OpenAPI document |
| `listEndpoints` | List all API paths with HTTP methods and summaries |
| `listEndpointModels` | List request/response models for a specific endpoint |
| `generateModelCode` | Generate Go structs from a schema model |
| `generateEndpointToolCode` | Generate a Go MCP tool scaffold for an endpoint |
| `generateServer` | Generate a complete, runnable Go MCP server project |
| `version` | Return the server version |

## 🗣️ MCP Prompts

| Prompt | Description |
|---|---|
| `add-endpoint` | Guided workflow: discover endpoint → inspect models → generate scaffold → implement |

Request a prompt with `prompts/get`:

```json
{
  "method": "prompts/get",
  "params": {
    "name": "add-endpoint",
    "arguments": {
      "swaggerUrl": "https://petstore.swagger.io/v2/swagger.json",
      "endpointPath": "/pet/{petId}",
      "httpMethod": "GET"
    }
  }
}
```

---

## 🏗️ Architecture

```
cmd/swagger-mcp/
├── main.go           entry point (delegates to cmd package)
└── cmd/
    ├── root.go       Cobra root command — serves the MCP server by default
    ├── serve.go      explicit serve subcommand (same as root)
    ├── generate.go   generate parent command
    ├── generate_server.go
    ├── generate_tool.go
    ├── generate_model.go
    ├── inspect.go    inspect parent command
    ├── inspect_endpoints.go
    ├── inspect_endpoint.go
    ├── download.go
    ├── version.go
    └── helpers.go    shared document loading and table printing utilities
internal/
├── app/              bootstrap, transport routing
├── codegen/          Go code generation (structs, handlers, servers)
├── config/           config struct, .env loading, auth from env
├── logging/          structured slog setup
├── mcp/              JSON-RPC 2.0, tool/prompt handlers, proxy engine
├── openapi/          spec loading, caching, parsing, filtering
└── testutil/         shared test helpers
test/
└── integration/      black-box integration test suite
testdata/             OpenAPI fixtures and golden files
```

Key design decisions:
- **Zero runtime dependencies** beyond Cobra — pure Go standard library for all business logic
- **Thin `main`** — all logic lives under `internal/`
- **Stdout reserved for JSON-RPC** — logs always go to stderr; Cobra help output too

---

## 🔧 Development

### Prerequisites

- Go `1.26+`
- Node.js / npm (optional, only for `make inspector`)

### Common Commands

```bash
make build      # compile to build/swagger-mcp
make test       # run go test ./...
make vet        # run go vet ./...
make fmt        # format all Go source files
make verify     # build + test + vet
make clean      # remove build artifacts
make inspector  # build and open the MCP Inspector (requires npm)
```

### Smoke Test

Pipe JSON-RPC directly into the binary to verify it responds correctly:

```bash
cat <<'EOF' | ./build/swagger-mcp
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"smoke","version":"1.0.0"}}}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
EOF
```

---

## 🤝 Contributing

Contributions are welcome! Here's how to get started:

1. **Fork** the repository and clone your fork
2. **Create a branch**: `git checkout -b feat/my-improvement`
3. **Make your changes** — keep commits focused and well-described
4. **Run the full check**: `make verify`
5. **Open a Pull Request** with a clear description of what you changed and why

### Guidelines

- Keep `stdout` reserved for JSON-RPC — never write non-JSON-RPC output there
- Add tests for new functionality (unit tests under the relevant `internal/` package, integration tests under `test/integration/`)
- Keep external dependencies minimal — all business logic uses only the Go standard library

---

## 🔍 Troubleshooting

**"no Swagger source available"** — provide `--swagger-url`, pass `swaggerFilePath` in the tool call, or create a `.swagger-mcp` file with `SWAGGER_FILEPATH=/path/to/swagger.json`.

**Stale cached spec** — remove the matching files from `swagger-cache/` and restart:
```bash
rm swagger-cache/<hash>.json swagger-cache/<hash>.metadata.json
```

**Seeing only startup logs** — that's expected in stdio mode. The process is waiting for JSON-RPC input on stdin. Use the smoke test above or connect an MCP client.

**Unexpected client behaviour** — check `stderr` for structured logs; `stdout` is reserved for JSON-RPC messages.

---

## 📄 License

MIT — see [LICENSE](./LICENSE) for details.

