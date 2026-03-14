# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).
Versions follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added
- `--log-level` CLI flag for the serve command (previously only `LOG_LEVEL` env var was supported)
- Environment variable support for all serve flags via `SWAGGER_MCP_*` prefix:
  `SWAGGER_MCP_TRANSPORT`, `SWAGGER_MCP_PORT`, `SWAGGER_MCP_PROXY_MODE`,
  `SWAGGER_MCP_SWAGGER_URL`, `SWAGGER_MCP_BASE_URL`, `SWAGGER_MCP_HEADERS`,
  `SWAGGER_MCP_INCLUDE_PATHS`, `SWAGGER_MCP_EXCLUDE_PATHS`,
  `SWAGGER_MCP_INCLUDE_METHODS`, `SWAGGER_MCP_EXCLUDE_METHODS`,
  `SWAGGER_MCP_SSE_HEADERS`, `SWAGGER_MCP_HTTP_HEADERS`, `SWAGGER_MCP_UI`
- `swagger-mcp inspect security` — list all security schemes from an OpenAPI spec
- `swagger-mcp inspect models` — list all data model schemas from an OpenAPI spec
- `openapi.ListSchemas()` — new exported function to enumerate top-level schemas
- GitHub Actions CI workflow (`.github/workflows/ci.yml`)
- Dockerfile for containerized deployment
- `.goreleaser.yml` for automated cross-platform binary releases
- `CONTRIBUTING.md`

### Changed
- Refactored CLI flag registration: shared `serveOptions` struct eliminates duplicate flag definitions between `root` and `serve` commands

### Removed
- `config.Load()` exported function (was dead code after Cobra migration; replaced by `config.load()` test helper)

---

## [1.0.1]

### Added
- Cobra CLI with rich help text and subcommand tree:
  - `swagger-mcp serve` — explicit serve subcommand
  - `swagger-mcp version` — display version and build info
  - `swagger-mcp generate server/tool/model` — Go code generation
  - `swagger-mcp inspect endpoints/endpoint` — endpoint inspection
  - `swagger-mcp download` — download and cache an OpenAPI spec locally
- Completely rewritten README with quick-start, config reference, and CLI documentation

### Changed
- CLI migrated from `flag.FlagSet` to Cobra
- `app.Run(args []string, ...)` signature changed to `app.Run(cfg config.Config, ...)`

---

## [1.0.0]

### Added
- Initial release: MCP server for Swagger/OpenAPI REST APIs
- Stdio, SSE, and StreamableHTTP transport modes
- Proxy mode: turn every Swagger endpoint into a live MCP tool
- Code generation tools: Go server scaffolding, endpoint tool code, model structs
- Endpoint inspection tools via MCP protocol
- Authentication: API key, Bearer token, Basic auth, OAuth2 client credentials
- `.swagger-mcp` project file for per-directory spec configuration
- `.env` file support for environment variable configuration
