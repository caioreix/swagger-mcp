package codegen

import (
	"fmt"
	"sort"
	"strings"

	"github.com/caioreix/swagger-mcp/internal/openapi"
)

// ServerGenParams configures complete MCP server generation.
type ServerGenParams struct {
	ModuleName     string   // Go module name (e.g., "github.com/user/my-mcp-server")
	TransportModes []string // "stdio", "sse", "streamable-http"
	ProxyMode      bool     // generate proxy handlers
	Endpoints      []string // filter to specific paths (empty = all)
}

// GenerateCompleteServer generates a complete, runnable Go MCP server project
// from an OpenAPI document. Returns a map of filename → content.
func GenerateCompleteServer(document map[string]any, params ServerGenParams) (map[string]string, error) {
	if params.ModuleName == "" {
		params.ModuleName = "github.com/generated/mcp-server"
	}
	if len(params.TransportModes) == 0 {
		params.TransportModes = []string{"stdio"}
	}

	baseURL := openapi.ExtractBaseURL(document)
	schemes := openapi.ExtractSecuritySchemes(document)
	endpoints, err := openapi.ListEndpoints(document)
	if err != nil {
		return nil, fmt.Errorf("list endpoints: %w", err)
	}

	if len(params.Endpoints) > 0 {
		endpoints = filterEndpoints(endpoints, params.Endpoints)
	}

	files := make(map[string]string)

	files["go.mod"] = generateGoMod(params.ModuleName)
	files["main.go"] = generateMainGo(params)
	files["server.go"] = generateServerGo(endpoints)
	files["tools.go"] = generateToolsGo(document, endpoints)
	files["handlers.go"] = generateHandlersGo(document, endpoints, baseURL, schemes, params.ProxyMode)
	files["helpers.go"] = generateHelpersGo(schemes)

	return files, nil
}

func filterEndpoints(endpoints []openapi.Endpoint, filter []string) []openapi.Endpoint {
	allowed := make(map[string]bool)
	for _, p := range filter {
		allowed[p] = true
	}
	var filtered []openapi.Endpoint
	for _, ep := range endpoints {
		if allowed[ep.Path] {
			filtered = append(filtered, ep)
		}
	}
	return filtered
}

func generateGoMod(moduleName string) string {
	return fmt.Sprintf("module %s\n\ngo 1.26\n", moduleName)
}

func generateMainGo(params ServerGenParams) string {
	var b strings.Builder
	b.WriteString("package main\n\n")
	b.WriteString("import (\n")
	b.WriteString("\t\"bufio\"\n")
	b.WriteString("\t\"bytes\"\n")
	b.WriteString("\t\"encoding/json\"\n")
	b.WriteString("\t\"errors\"\n")
	b.WriteString("\t\"flag\"\n")
	b.WriteString("\t\"fmt\"\n")
	b.WriteString("\t\"io\"\n")
	b.WriteString("\t\"log\"\n")

	hasHTTP := false
	for _, t := range params.TransportModes {
		if t == "sse" || t == "streamable-http" {
			hasHTTP = true
			break
		}
	}
	if hasHTTP {
		b.WriteString("\t\"net/http\"\n")
		b.WriteString("\t\"sync\"\n")
	}
	b.WriteString("\t\"os\"\n")
	b.WriteString(")\n\n")

	b.WriteString("func main() {\n")
	b.WriteString("\ttransport := flag.String(\"transport\", \"stdio\", \"Transport mode: stdio, sse, streamable-http\")\n")
	b.WriteString("\tport := flag.String(\"port\", \"8080\", \"Port for HTTP transports\")\n")
	b.WriteString("\tflag.Parse()\n\n")

	b.WriteString("\tserver := newServer()\n\n")

	b.WriteString("\tswitch *transport {\n")
	b.WriteString("\tcase \"stdio\":\n")
	b.WriteString("\t\tserveStdio(server)\n")

	if hasHTTP {
		for _, t := range params.TransportModes {
			switch t {
			case "sse":
				b.WriteString("\tcase \"sse\":\n")
				b.WriteString("\t\tserveSSE(server, *port)\n")
			case "streamable-http":
				b.WriteString("\tcase \"streamable-http\":\n")
				b.WriteString("\t\tserveStreamableHTTP(server, *port)\n")
			}
		}
	}

	b.WriteString("\tdefault:\n")
	b.WriteString("\t\tlog.Fatalf(\"unknown transport: %s\", *transport)\n")
	b.WriteString("\t}\n")
	b.WriteString("}\n\n")

	// stdio transport
	b.WriteString(generateStdioTransport())

	// SSE transport
	for _, t := range params.TransportModes {
		switch t {
		case "sse":
			b.WriteString(generateSSETransportCode())
		case "streamable-http":
			b.WriteString(generateStreamableHTTPTransportCode())
		}
	}

	return b.String()
}

func generateStdioTransport() string {
	return `func serveStdio(server *mcpServer) {
	reader := bufio.NewReader(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)
	defer writer.Flush()

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			log.Fatalf("stdin read error: %v", err)
		}
		line = bytes.TrimSpace(line)
		if len(line) > 0 {
			resp, handleErr := server.handleJSON(line)
			if handleErr != nil {
				log.Printf("handle error: %v", handleErr)
			} else if len(resp) > 0 {
				writer.Write(append(resp, '\n'))
				writer.Flush()
			}
		}
		if errors.Is(err, io.EOF) {
			return
		}
	}
}
`
}

func generateSSETransportCode() string {
	return `func serveSSE(server *mcpServer, port string) {
	var mu sync.Mutex
	clients := make(map[string]chan []byte)

	http.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		clientID := fmt.Sprintf("%d", len(clients)+1)
		ch := make(chan []byte, 64)
		mu.Lock()
		clients[clientID] = ch
		mu.Unlock()
		defer func() {
			mu.Lock()
			delete(clients, clientID)
			mu.Unlock()
		}()

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		fmt.Fprintf(w, "event: endpoint\ndata: /message?clientId=%s\n\n", clientID)
		flusher.Flush()

		for msg := range ch {
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", msg)
			flusher.Flush()
		}
	})

	http.HandleFunc("/message", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		clientID := r.URL.Query().Get("clientId")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read error", http.StatusBadRequest)
			return
		}
		resp, err := server.handleJSON(body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		mu.Lock()
		ch, ok := clients[clientID]
		mu.Unlock()
		if ok && len(resp) > 0 {
			ch <- resp
		}
		w.WriteHeader(http.StatusAccepted)
	})

	log.Printf("SSE server listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
`
}

func generateStreamableHTTPTransportCode() string {
	return `func serveStreamableHTTP(server *mcpServer, port string) {
	var mu sync.Mutex
	sessions := make(map[string]bool)

	http.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Mcp-Session-Id")
		w.Header().Set("Access-Control-Expose-Headers", "Mcp-Session-Id")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if r.Method == http.MethodDelete {
			sessionID := r.Header.Get("Mcp-Session-Id")
			mu.Lock()
			delete(sessions, sessionID)
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read error", http.StatusBadRequest)
			return
		}

		resp, err := server.handleJSON(body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		sessionID := r.Header.Get("Mcp-Session-Id")
		if sessionID == "" {
			sessionID = fmt.Sprintf("session-%d", len(sessions)+1)
		}
		mu.Lock()
		sessions[sessionID] = true
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Mcp-Session-Id", sessionID)
		w.Write(resp)
	})

	log.Printf("StreamableHTTP server listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
`
}

func generateServerGo(endpoints []openapi.Endpoint) string {
	var b strings.Builder
	b.WriteString("package main\n\n")
	b.WriteString("import (\n\t\"encoding/json\"\n\t\"fmt\"\n)\n\n")

	b.WriteString("const protocolVersion = \"2024-11-05\"\n\n")

	b.WriteString(`type jsonRPCRequest struct {
	JSONRPC string          ` + "`json:\"jsonrpc\"`" + `
	ID      any             ` + "`json:\"id,omitempty\"`" + `
	Method  string          ` + "`json:\"method\"`" + `
	Params  json.RawMessage ` + "`json:\"params,omitempty\"`" + `
}

type jsonRPCResponse struct {
	JSONRPC string ` + "`json:\"jsonrpc\"`" + `
	ID      any    ` + "`json:\"id,omitempty\"`" + `
	Result  any    ` + "`json:\"result,omitempty\"`" + `
	Error   any    ` + "`json:\"error,omitempty\"`" + `
}

type mcpServer struct{}

func newServer() *mcpServer {
	return &mcpServer{}
}

func (s *mcpServer) handleJSON(line []byte) ([]byte, error) {
	var req jsonRPCRequest
	if err := json.Unmarshal(line, &req); err != nil {
		return json.Marshal(jsonRPCResponse{JSONRPC: "2.0", Error: map[string]any{"code": -32700, "message": "parse error"}})
	}
	if req.JSONRPC != "2.0" {
		return json.Marshal(jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: map[string]any{"code": -32600, "message": "invalid JSON-RPC version"}})
	}

	switch req.Method {
	case "initialize":
		return respond(req.ID, map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "Generated MCP Server", "version": "1.0.0"},
		})
	case "notifications/initialized", "notifications/cancelled":
		return nil, nil
	case "ping":
		return respond(req.ID, map[string]any{})
	case "tools/list":
		return respond(req.ID, map[string]any{"tools": toolDefinitions()})
	case "tools/call":
		return s.handleToolCall(req.ID, req.Params)
	default:
		if req.ID == nil {
			return nil, nil
		}
		return json.Marshal(jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: map[string]any{"code": -32601, "message": fmt.Sprintf("method not found: %s", req.Method)}})
	}
}

func (s *mcpServer) handleToolCall(id any, rawParams json.RawMessage) ([]byte, error) {
	var params struct {
		Name      string         ` + "`json:\"name\"`" + `
		Arguments map[string]any ` + "`json:\"arguments\"`" + `
	}
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return json.Marshal(jsonRPCResponse{JSONRPC: "2.0", ID: id, Error: map[string]any{"code": -32602, "message": "invalid params"}})
	}
	if params.Arguments == nil {
		params.Arguments = map[string]any{}
	}
	result := callTool(params.Name, params.Arguments)
	return respond(id, result)
}

func respond(id any, result any) ([]byte, error) {
	return json.Marshal(jsonRPCResponse{JSONRPC: "2.0", ID: id, Result: result})
}
`)
	return b.String()
}

func generateToolsGo(document map[string]any, endpoints []openapi.Endpoint) string {
	var b strings.Builder
	b.WriteString("package main\n\n")

	b.WriteString("type toolDef struct {\n")
	b.WriteString("\tName        string         `json:\"name\"`\n")
	b.WriteString("\tDescription string         `json:\"description\"`\n")
	b.WriteString("\tInputSchema map[string]any `json:\"inputSchema\"`\n")
	b.WriteString("}\n\n")

	b.WriteString("func toolDefinitions() []toolDef {\n")
	b.WriteString("\treturn []toolDef{\n")

	for _, ep := range endpoints {
		toolName := generateToolName(ep.Method, ep.Path, ep.OperationID, false, false, true)
		inputSchema := generateInputSchemaForEndpoint(document, ep)
		desc := ep.Summary
		if desc == "" {
			desc = ep.Description
		}
		if desc == "" {
			desc = fmt.Sprintf("%s %s", ep.Method, ep.Path)
		}
		b.WriteString(fmt.Sprintf("\t\t{Name: %q, Description: %q, InputSchema: %s},\n",
			toolName, desc, renderGoLiteralInline(inputSchema)))
	}

	b.WriteString("\t}\n")
	b.WriteString("}\n\n")

	// callTool dispatcher
	b.WriteString("func callTool(name string, args map[string]any) map[string]any {\n")
	b.WriteString("\tswitch name {\n")
	for _, ep := range endpoints {
		toolName := generateToolName(ep.Method, ep.Path, ep.OperationID, false, false, true)
		handlerName := "Handle" + toExportedIdentifier(toolName)
		b.WriteString(fmt.Sprintf("\tcase %q:\n\t\treturn %s(args)\n", toolName, handlerName))
	}
	b.WriteString("\tdefault:\n")
	b.WriteString("\t\treturn mcpError(-32601, \"unknown tool: \"+name)\n")
	b.WriteString("\t}\n")
	b.WriteString("}\n")

	return b.String()
}

func generateInputSchemaForEndpoint(document map[string]any, ep openapi.Endpoint) map[string]any {
	operation, err := openapi.FindOperation(document, ep.Path, ep.Method)
	if err != nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	return generateInputSchema(document, operation)
}

func generateHandlersGo(document map[string]any, endpoints []openapi.Endpoint, baseURL string, schemes []openapi.SecurityScheme, proxyMode bool) string {
	var b strings.Builder
	b.WriteString("package main\n\n")

	// Collect imports needed
	imports := []string{"fmt"}
	if proxyMode {
		imports = append(imports, "bytes", "encoding/json", "io", "net/http", "net/url", "strings", "time")
		if len(schemes) > 0 {
			imports = append(imports, "os")
		}
	}
	sort.Strings(imports)
	imports = dedup(imports)

	b.WriteString("import (\n")
	for _, imp := range imports {
		b.WriteString(fmt.Sprintf("\t%q\n", imp))
	}
	b.WriteString(")\n\n")

	for _, ep := range endpoints {
		toolName := generateToolName(ep.Method, ep.Path, ep.OperationID, false, false, true)
		if proxyMode {
			operation, err := openapi.FindOperation(document, ep.Path, ep.Method)
			if err != nil {
				continue
			}
			b.WriteString(GenerateProxyHandler(toolName, ep.Method, ep.Path, baseURL, operation, schemes))
		} else {
			b.WriteString(generateHandlerWithErrors(toolName, ep.Method, ep.Path))
		}
		b.WriteString("\n\n")
	}

	return b.String()
}

func generateHelpersGo(schemes []openapi.SecurityScheme) string {
	var b strings.Builder
	b.WriteString("package main\n\n")
	b.WriteString("import \"fmt\"\n\n")

	b.WriteString(generateErrorHelpers())
	b.WriteString("\n")

	if len(schemes) > 0 {
		b.WriteString(generateAuthHelpers(schemes))
		b.WriteString("\n")
		if needsOAuth2Fetcher(schemes) {
			b.WriteString(generateOAuth2TokenFetcher())
			b.WriteString("\n")
		}
	}

	return b.String()
}

func renderGoLiteralInline(value any) string {
	switch v := value.(type) {
	case map[string]any:
		if len(v) == 0 {
			return "map[string]any{}"
		}
		keys := sortedMapKeys(v)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%q: %s", k, renderGoLiteralInline(v[k])))
		}
		return "map[string]any{" + strings.Join(parts, ", ") + "}"
	case []any:
		if len(v) == 0 {
			return "[]any{}"
		}
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, renderGoLiteralInline(item))
		}
		return "[]any{" + strings.Join(parts, ", ") + "}"
	case []string:
		if len(v) == 0 {
			return "[]string{}"
		}
		parts := make([]string, 0, len(v))
		for _, s := range v {
			parts = append(parts, fmt.Sprintf("%q", s))
		}
		return "[]string{" + strings.Join(parts, ", ") + "}"
	case string:
		return fmt.Sprintf("%q", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%v", v)
	case nil:
		return "nil"
	default:
		return fmt.Sprintf("%q", fmt.Sprint(v))
	}
}

func dedup(items []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(items))
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}
