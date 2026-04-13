# swagger-mcp

> **Bridge any REST API with AI assistants using [Model Context Protocol](https://modelcontextprotocol.io/).**

[![Go Version](https://img.shields.io/badge/go-1.26+-00ADD8?logo=go)](https://golang.org/)
[![CI](https://github.com/caioreix/swagger-mcp/actions/workflows/ci.yml/badge.svg)](https://github.com/caioreix/swagger-mcp/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](./LICENSE)
[![Go Report](https://goreportcard.com/badge/github.com/caioreix/swagger-mcp)](https://goreportcard.com/report/github.com/caioreix/swagger-mcp)

`swagger-mcp` is an MCP server that reads any Swagger/OpenAPI definition and exposes it to AI clients as structured tools — no glue code required. Point it at your API spec, connect your AI assistant, and start exploring endpoints, inspecting schemas, or proxying live requests instantly.

---

## ✨ Features

| Capability | Details |
|---|---|
| **Endpoint discovery** | List all API paths with HTTP methods and descriptions |
| **Model inspection** | Explore request/response schemas for any endpoint |
| **Dynamic proxy mode** | Every Swagger endpoint becomes a live MCP tool — zero code required |
| **Typed proxy schemas** | Proxy tools preserve OpenAPI-derived argument types instead of flattening everything to strings |
| **Structured MCP responses** | Static tools and JSON proxy responses return `structuredContent` for easier agent processing |
| **Endpoint filtering** | Regex-based path and HTTP method include/exclude rules |
| **Authentication** | API Key, Bearer/JWT, Basic Auth, OAuth2 client credentials |
| **Multiple transports** | `stdio`, StreamableHTTP, and legacy SSE |
| **Built-in web UI** | Interactive tool dashboard for manual testing |
| **Anti-hallucination guards** | Proxy tools include explicit LLM instructions to prevent data fabrication |
| **Smart caching** | Downloaded specs cached locally with ETag/Last-Modified revalidation |

---

## ⚡ Quick Start

### 1. Install

```bash
# From source
git clone https://github.com/caioreix/swagger-mcp.git
cd swagger-mcp
make build   # binary → build/swagger-mcp

# Or using Docker
docker build -t swagger-mcp .
docker run --rm swagger-mcp version
```

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

Your AI assistant can now discover endpoints, inspect models, and proxy live API calls against your API.

---

## 🚀 Usage

### Transport Modes

**`stdio`** (default) — communicates over stdin/stdout, ideal for Cursor and Claude Desktop:

```bash
./build/swagger-mcp --swagger-url="https://petstore.swagger.io/v2/swagger.json"
```

**`streamable-http`** — HTTP server following the MCP StreamableHTTP spec. Recommended for remote or multi-client deployments:

```bash
./build/swagger-mcp \
  --transport=streamable-http \
  --port=8080 \
  --swagger-url="https://petstore.swagger.io/v2/swagger.json"
```

Endpoints: `POST /mcp` (send/receive) · `DELETE /mcp` (terminate session)

**`sse`** — legacy HTTP server with Server-Sent Events. Still available for compatibility, but `streamable-http` is preferred:

```bash
./build/swagger-mcp \
  --transport=sse \
  --port=8080 \
  --swagger-url="https://petstore.swagger.io/v2/swagger.json"
```

Endpoints: `GET /sse` (event stream) · `POST /message?clientId=<id>` (send requests)

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

**Tool naming:** single-API proxy tools use a `swagger_` prefix and `snake_case` names such as `swagger_find_pets`; multi-API proxy tools use the configured API name as the prefix, such as `petstore_find_pets`.

**Proxy responses:** when the upstream API returns JSON, the tool response includes both text output and `structuredContent`.

---

### Web UI

Pass `--ui` to open an interactive testing dashboard in your browser. In stdio mode, the MCP server still talks over stdin/stdout and the UI is served on the configured HTTP port in the background:

```bash
./build/swagger-mcp --ui --port=8080 \
  --swagger-url="https://petstore.swagger.io/v2/swagger.json"
```

Open `http://localhost:8080/` to browse tools, fill in parameters, and view JSON responses.

---

## ⚙️ Configuration Reference

### CLI Flags

```
./build/swagger-mcp --help
```

| Flag | Default | Env Variable | Description |
|---|---|---|---|
| `--swagger-url` | — | `SWAGGER_MCP_SWAGGER_URL` | URL of the Swagger/OpenAPI definition |
| `--transport` | `stdio` | `SWAGGER_MCP_TRANSPORT` | `stdio`, `streamable-http`, or legacy `sse` |
| `--port` | `8080` | `SWAGGER_MCP_PORT` | HTTP port for StreamableHTTP/SSE and the stdio web UI |
| `--log-level` | `info` | `LOG_LEVEL` | Log verbosity: `debug`, `info`, `warn`, `error` |
| `--ui` | `false` | `SWAGGER_MCP_UI` | Enable the built-in web UI |
| `--proxy-mode` | `false` | `SWAGGER_MCP_PROXY_MODE` | Dynamic proxy: each endpoint becomes an MCP tool |
| `--base-url` | — | `SWAGGER_MCP_BASE_URL` | Override the base URL from the spec |
| `--headers` | — | `SWAGGER_MCP_HEADERS` | Static proxy headers: `Key=Value,Key2=Value2` |
| `--include-paths` | — | `SWAGGER_MCP_INCLUDE_PATHS` | Regex patterns for paths to expose (comma-separated) |
| `--exclude-paths` | — | `SWAGGER_MCP_EXCLUDE_PATHS` | Regex patterns for paths to hide (comma-separated) |
| `--include-methods` | — | `SWAGGER_MCP_INCLUDE_METHODS` | HTTP methods to expose: `GET,POST` |
| `--exclude-methods` | — | `SWAGGER_MCP_EXCLUDE_METHODS` | HTTP methods to hide: `DELETE,PUT` |
| `--sse-headers` | — | `SWAGGER_MCP_SSE_HEADERS` | Headers forwarded from SSE clients to API |
| `--http-headers` | — | `SWAGGER_MCP_HTTP_HEADERS` | Headers forwarded from StreamableHTTP clients to API |
| `--config` | — | `SWAGGER_MCP_CONFIG` | Path to a `swagger-mcp.yaml` multi-API config file |

> **Precedence**: CLI flags > environment variables > defaults. Legacy alias `--swaggerUrl` is still accepted but deprecated.

### Environment Variables (Authentication)

| Variable | Default | Description |
|---|---|---|
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

### Multi-API Configuration

To proxy **multiple APIs simultaneously**, create a `.swagger-mcp.yaml` file in your project root (or pass `--config path/to/file.yaml`). Each entry generates its own set of proxy tools, prefixed with the normalized API name:

```yaml
# .swagger-mcp.yaml
apis:
  - name: petstore
    swagger_url: https://petstore.swagger.io/v2/swagger.json
    base_url: https://petstore.swagger.io/v2   # optional override
    auth:
      bearer_token: ${PETSTORE_TOKEN}           # ${ENV_VAR} interpolation
    headers: "X-Tenant=acme"
    include_paths: "^/pet.*"
    exclude_methods: "DELETE"

  - name: github
    swagger_url: ./specs/github.yaml            # local file also works
    auth:
      api_key: ${GITHUB_TOKEN}
      api_key_header: Authorization
      api_key_in: header                        # "header", "query", or "cookie"

  - name: partner
    swagger_url: https://partner.example.com/openapi.yaml
    auth:
      oauth2_token_url: https://auth.partner.example.com/token
      oauth2_client_id: ${PARTNER_CLIENT_ID}
      oauth2_client_secret: ${PARTNER_CLIENT_SECRET}
      oauth2_scopes: read:data,write:data
```

**Tool naming:** proxy tools are prefixed with the normalized API name and use `snake_case` — `petstore_find_pets`, `github_list_repos`, etc. — so all APIs can coexist in the same MCP session without name conflicts.

**Credentials:** use `${ENV_VAR}` anywhere in the config file. Values are expanded at startup from environment variables (or `.env`), keeping secrets out of the YAML file.

**Config file discovery order:** `--config` flag → `SWAGGER_MCP_CONFIG` env var → `.swagger-mcp.yaml` in working directory.

**Backward compatible:** the `--proxy-mode` flag and all CLI authentication env vars continue to work exactly as before for single-API usage.

See [`swagger-mcp.example.yaml`](./swagger-mcp.example.yaml) for a fully-documented example with all authentication modes.

| Field | Description |
|---|---|
| `name` | **Required.** Unique identifier; used as tool name prefix |
| `swagger_url` | URL or local path to the Swagger/OpenAPI spec |
| `base_url` | Override the base URL from the spec |
| `auth.bearer_token` | Bearer / JWT token |
| `auth.api_key` | API key value |
| `auth.api_key_header` | Header name for the key (default: `X-API-Key`) |
| `auth.api_key_in` | `header`, `query`, or `cookie` (default: `header`) |
| `auth.basic_user` / `auth.basic_pass` | HTTP Basic auth credentials |
| `auth.oauth2_token_url` | OAuth2 token endpoint |
| `auth.oauth2_client_id` / `auth.oauth2_client_secret` | OAuth2 client credentials |
| `auth.oauth2_scopes` | OAuth2 scopes (comma-separated) |
| `headers` | Static headers for every request: `Key=Val,Key2=Val2` |
| `include_paths` | Comma-separated regex patterns for paths to expose |
| `exclude_paths` | Comma-separated regex patterns for paths to hide |
| `include_methods` | HTTP methods to expose: `GET,POST` |
| `exclude_methods` | HTTP methods to hide: `DELETE,PUT` |

---

## 🛠️ CLI Commands

In addition to running as an MCP server, `swagger-mcp` exposes standalone commands for working with OpenAPI specs directly from the terminal.

### `serve` — Start the MCP server

Equivalent to running `swagger-mcp` with no subcommand:

```bash
swagger-mcp serve --swagger-url="https://petstore.swagger.io/v2/swagger.json" --transport=streamable-http --port=8080
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

List all security schemes:

```bash
swagger-mcp inspect security --swagger-url="https://petstore.swagger.io/v2/swagger.json"
swagger-mcp inspect security --swagger-file=./api.yaml --format=json
```

List all data models:

```bash
swagger-mcp inspect models --swagger-url="https://petstore.swagger.io/v2/swagger.json"
swagger-mcp inspect models --swagger-file=./api.yaml --format=json
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
| `swagger_get_definition` | Download and cache a Swagger/OpenAPI document |
| `swagger_list_endpoints` | List all API paths with HTTP methods and summaries |
| `swagger_list_endpoint_models` | List request/response models for a specific endpoint |
| `swagger_get_version` | Return the server version |

## 🗣️ MCP Prompts

| Prompt | Description |
|---|---|
| `swagger_add_endpoint` | Guided workflow: discover endpoint → inspect models → integrate the endpoint into your project |
| `add-endpoint` | Legacy alias for `swagger_add_endpoint` |

Request a prompt with `prompts/get`:

```json
{
  "method": "prompts/get",
  "params": {
    "name": "swagger_add_endpoint",
    "arguments": {
      "swagger_url": "https://petstore.swagger.io/v2/swagger.json",
      "endpoint_path": "/pet/{petId}",
      "http_method": "GET"
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
    ├── inspect.go    inspect parent command
    ├── inspect_endpoints.go
    ├── inspect_endpoint.go
    ├── inspect_security.go
    ├── inspect_models.go
    ├── download.go
    ├── version.go
    └── helpers.go    serveOptions struct, env var helpers, document loading, table printing
internal/
├── app/              bootstrap, transport routing
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

Contributions are welcome! See [CONTRIBUTING.md](./CONTRIBUTING.md) for full setup instructions and guidelines.

Quick start:

1. **Fork** the repository and clone your fork
2. **Create a branch**: `git checkout -b feat/my-improvement`
3. **Make your changes** and add tests
4. **Run the full check**: `make verify`
5. **Open a Pull Request** with a clear description

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
