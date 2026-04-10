package app

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

const webUIHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Swagger MCP - Tool Tester</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #0d1117; color: #c9d1d9; }
.header { background: #161b22; border-bottom: 1px solid #30363d; padding: 16px 24px; display: flex; align-items: center; gap: 12px; }
.header h1 { font-size: 20px; color: #58a6ff; }
.header .version { font-size: 12px; color: #8b949e; background: #21262d; padding: 2px 8px; border-radius: 12px; }
.layout { display: flex; height: calc(100vh - 57px); }
.sidebar { width: 280px; border-right: 1px solid #30363d; overflow-y: auto; background: #0d1117; }
.sidebar h2 { font-size: 12px; text-transform: uppercase; color: #8b949e; padding: 16px 16px 8px; letter-spacing: 0.5px; }
.tool-item { padding: 10px 16px; cursor: pointer; border-bottom: 1px solid #21262d; transition: background 0.15s; }
.tool-item:hover { background: #161b22; }
.tool-item.active { background: #1f6feb22; border-left: 3px solid #58a6ff; }
.tool-item .name { font-size: 14px; font-weight: 600; color: #c9d1d9; }
.tool-item .desc { font-size: 12px; color: #8b949e; margin-top: 4px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.main { flex: 1; overflow-y: auto; padding: 24px; }
.section { margin-bottom: 24px; }
.section h3 { font-size: 14px; color: #8b949e; margin-bottom: 12px; text-transform: uppercase; letter-spacing: 0.5px; }
.form-group { margin-bottom: 16px; }
.form-group label { display: block; font-size: 13px; color: #c9d1d9; margin-bottom: 6px; font-weight: 600; }
.form-group label .required { color: #f85149; margin-left: 2px; }
.form-group label .type { color: #8b949e; font-weight: 400; font-size: 12px; }
.form-group input, .form-group select, .form-group textarea { width: 100%; padding: 8px 12px; background: #0d1117; border: 1px solid #30363d; border-radius: 6px; color: #c9d1d9; font-size: 14px; font-family: inherit; }
.form-group input:focus, .form-group select:focus, .form-group textarea:focus { outline: none; border-color: #58a6ff; box-shadow: 0 0 0 3px #1f6feb33; }
.form-group textarea { min-height: 80px; resize: vertical; }
.form-group .hint { font-size: 12px; color: #8b949e; margin-top: 4px; }
.btn { padding: 8px 20px; border-radius: 6px; border: 1px solid #30363d; font-size: 14px; font-weight: 600; cursor: pointer; transition: all 0.15s; }
.btn-primary { background: #238636; color: #fff; border-color: #238636; }
.btn-primary:hover { background: #2ea043; }
.btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }
.btn-secondary { background: #21262d; color: #c9d1d9; }
.btn-secondary:hover { background: #30363d; }
.response-area { background: #161b22; border: 1px solid #30363d; border-radius: 6px; padding: 16px; font-family: "SFMono-Regular", Consolas, monospace; font-size: 13px; white-space: pre-wrap; word-break: break-word; max-height: 400px; overflow-y: auto; line-height: 1.5; }
.response-area.error { border-color: #f8514966; color: #f85149; }
.response-area.success { border-color: #23863666; }
.toolbar { display: flex; gap: 8px; align-items: center; margin-bottom: 16px; }
.status { font-size: 12px; color: #8b949e; }
.status.loading { color: #d29922; }
.empty-state { text-align: center; padding: 60px 24px; color: #8b949e; }
.empty-state h2 { font-size: 24px; margin-bottom: 8px; color: #c9d1d9; }
.tabs { display: flex; gap: 0; border-bottom: 1px solid #30363d; margin-bottom: 16px; }
.tab { padding: 8px 16px; font-size: 13px; color: #8b949e; cursor: pointer; border-bottom: 2px solid transparent; }
.tab.active { color: #58a6ff; border-bottom-color: #58a6ff; }
</style>
</head>
<body>
<div class="header">
<h1>🔧 Swagger MCP</h1>
<span class="version" id="version">loading...</span>
</div>
<div class="layout">
<div class="sidebar">
<h2>Tools</h2>
<div id="tool-list"></div>
</div>
<div class="main" id="main-content">
<div class="empty-state">
<h2>Select a tool</h2>
<p>Choose a tool from the sidebar to test it</p>
</div>
</div>
</div>
<script>
let tools = [];
let currentTool = null;
let history = [];

async function rpc(method, params) {
  const resp = await fetch('/api/rpc', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({jsonrpc: '2.0', id: Date.now(), method, params})
  });
  const data = await resp.json();
  if (data.error) throw new Error(data.error.message);
  return data.result;
}

async function init() {
  try {
    const toolsResult = await rpc('tools/list', {});
    tools = toolsResult.tools || [];
    renderToolList();
    const versionResult = await rpc('tools/call', {name: 'version', arguments: {}});
    const content = versionResult.content || [];
    if (content.length > 0) {
      try { document.getElementById('version').textContent = 'v' + JSON.parse(content[0].text).version; } catch(e) {}
    }
  } catch(e) { console.error('Init error:', e); }
}

function renderToolList() {
  const list = document.getElementById('tool-list');
  list.innerHTML = tools.map((t, i) =>
    '<div class="tool-item' + (currentTool === i ? ' active' : '') + '" onclick="selectTool(' + i + ')">' +
    '<div class="name">' + esc(t.name) + '</div>' +
    '<div class="desc">' + esc(t.description || '').substring(0, 80) + '</div></div>'
  ).join('');
}

function selectTool(index) {
  currentTool = index;
  renderToolList();
  renderToolForm(tools[index]);
}

function renderToolForm(tool) {
  const main = document.getElementById('main-content');
  const schema = tool.inputSchema || {};
  const props = schema.properties || {};
  const required = schema.required || [];

  let html = '<div class="section"><h3>' + esc(tool.name) + '</h3>';
  html += '<p style="color:#8b949e;font-size:13px;margin-bottom:16px">' + esc(tool.description || '') + '</p>';

  const keys = Object.keys(props);
  if (keys.length > 0) {
    keys.forEach(key => {
      const prop = props[key];
      const isReq = required.includes(key);
      const type = prop.type || 'string';
      html += '<div class="form-group">';
      html += '<label>' + esc(key) + (isReq ? '<span class="required">*</span>' : '') + ' <span class="type">' + type + '</span></label>';
      if (type === 'boolean') {
        html += '<select id="field-' + key + '"><option value="">-- not set --</option><option value="true">true</option><option value="false">false</option></select>';
      } else if (prop.enum) {
        html += '<select id="field-' + key + '"><option value="">-- select --</option>';
        prop.enum.forEach(v => { html += '<option value="' + esc(v) + '">' + esc(v) + '</option>'; });
        html += '</select>';
      } else {
        html += '<input type="text" id="field-' + key + '" placeholder="' + esc(prop.description || '') + '">';
      }
      if (prop.description) html += '<div class="hint">' + esc(prop.description) + '</div>';
      html += '</div>';
    });
  }

  html += '<div class="toolbar">';
  html += '<button class="btn btn-primary" id="exec-btn" onclick="executeTool()">Execute</button>';
  html += '<button class="btn btn-secondary" onclick="clearResponse()">Clear</button>';
  html += '<span class="status" id="status"></span></div></div>';
  html += '<div class="section"><h3>Response</h3>';
  html += '<div class="response-area" id="response">Click Execute to call this tool</div></div>';
  main.innerHTML = html;
}

async function executeTool() {
  const tool = tools[currentTool];
  const schema = tool.inputSchema || {};
  const props = schema.properties || {};
  const args = {};

  Object.keys(props).forEach(key => {
    const el = document.getElementById('field-' + key);
    if (!el || el.value === '') return;
    const type = props[key].type;
    if (type === 'boolean') args[key] = el.value === 'true';
    else if (type === 'integer' || type === 'number') args[key] = Number(el.value);
    else args[key] = el.value;
  });

  const btn = document.getElementById('exec-btn');
  const status = document.getElementById('status');
  const respEl = document.getElementById('response');
  btn.disabled = true;
  status.className = 'status loading';
  status.textContent = 'Executing...';

  try {
    const result = await rpc('tools/call', {name: tool.name, arguments: args});
    const content = result.content || [];
    let text = content.map(c => c.text || '').join('\n');
    try { text = JSON.stringify(JSON.parse(text), null, 2); } catch(e) {}
    respEl.textContent = text;
    respEl.className = result.isError ? 'response-area error' : 'response-area success';
    status.className = 'status';
    status.textContent = result.isError ? 'Error' : 'Success';
    history.push({tool: tool.name, args, result: text, time: new Date().toISOString()});
  } catch(e) {
    respEl.textContent = 'Error: ' + e.message;
    respEl.className = 'response-area error';
    status.className = 'status';
    status.textContent = 'Failed';
  }
  btn.disabled = false;
}

function clearResponse() {
  const el = document.getElementById('response');
  if (el) { el.textContent = 'Click Execute to call this tool'; el.className = 'response-area'; }
}

function esc(s) { const d = document.createElement('div'); d.textContent = s; return d.innerHTML; }

init();
</script>
</body>
</html>`

func serveWebUI(handler jsonHandler, logger *slog.Logger, port string) int {
	uiLogger := componentLogger(logger, "app.webui")
	mux := http.NewServeMux()

	// Serve the web UI
	mux.HandleFunc("GET /", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, webUIHTML)
	})

	// JSON-RPC proxy endpoint for the UI
	mux.HandleFunc("POST /api/rpc", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSONError(w, "failed to read body")
			return
		}

		resp, err := handler.HandleJSON(body)
		if err != nil {
			writeJSONError(w, err.Error())
			return
		}

		if len(resp) > 0 {
			_, _ = w.Write(resp)
		} else {
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":null}`))
		}
	})

	mux.HandleFunc("OPTIONS /api/rpc", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusNoContent)
	})

	uiLogger.Info("starting web UI", "port", port, "url", fmt.Sprintf("http://localhost:%s", port))
	server := &http.Server{ //nolint:gosec // timeout configured by caller
		Addr:    ":" + port,
		Handler: mux,
	}
	if err := server.ListenAndServe(); err != nil {
		uiLogger.Error("web UI server error", "error", err)
		return 1
	}
	return 0
}

func writeJSONError(w http.ResponseWriter, message string) {
	resp := map[string]any{
		"jsonrpc": "2.0",
		"error":   map[string]any{"code": -32603, "message": message},
	}
	data, _ := json.Marshal(resp)
	_, _ = w.Write(data)
}

// startWebUIBackground starts the web UI on a separate goroutine.
func startWebUIBackground(handler jsonHandler, logger *slog.Logger, port string) {
	go func() {
		uiLogger := componentLogger(logger, "app.webui")
		uiLogger.Info("starting web UI in background", "port", port, "url", fmt.Sprintf("http://localhost:%s", port))
		_ = serveWebUI(handler, logger, port)
	}()
}
