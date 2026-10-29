package main

import (
	"os/exec"
	"strings"
	"testing"

	"mygo-harness/internal/llm"
)

// fixturePy 与 internal/mcp/client_test.go 的夹具相同（跨包测试复制一份，
// 避免为测试污染非测试文件）。
const fixturePy = `
import sys, json
for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    try:
        req = json.loads(line)
    except Exception:
        continue
    m = req.get("method", "")
    rid = req.get("id")
    if rid is None:
        continue
    if m == "initialize":
        result = {"protocolVersion": "2024-11-05", "capabilities": {"tools": {}},
                  "serverInfo": {"name": "fixture", "version": "0.0.1"}}
    elif m == "tools/list":
        result = {"tools": [{"name": "echo", "description": "回声工具",
                             "inputSchema": {"type": "object"}}]}
    elif m == "tools/call":
        args = req["params"].get("arguments", {})
        result = {"content": [{"type": "text", "text": "echo: " + json.dumps(args, ensure_ascii=False)}],
                  "isError": args.get("fail") is True}
    else:
        continue
    print(json.dumps({"jsonrpc": "2.0", "id": rid, "result": result}, ensure_ascii=False), flush=True)
`

// TestMCPServerConfigRoundtrip: mcp.json load/save.
func TestMCPServerConfigRoundtrip(t *testing.T) {
	dataDir := t.TempDir()
	if len(loadMCPServers(dataDir)) != 0 {
		t.Fatal("missing file should give empty servers")
	}
	servers := map[string]MCPServerConfig{
		"sqlite": {Command: "uvx", Args: []string{"mcp-server-sqlite"}, Env: map[string]string{"K": "V"}},
	}
	if err := saveMCPServers(dataDir, servers); err != nil {
		t.Fatal(err)
	}
	got := loadMCPServers(dataDir)
	if len(got) != 1 || got["sqlite"].Command != "uvx" || len(got["sqlite"].Args) != 1 || got["sqlite"].Env["K"] != "V" {
		t.Fatalf("roundtrip: %+v", got)
	}
}

// REQ-005 AC-3: enabled servers mount specs (mutating); disabled servers
// and the master switch remove them.
func TestMCPAgentWiring(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 不可用，跳过 MCP 接线测试")
	}
	a := newTestApp(t)
	if err := saveMCPServers(a.dataDir, map[string]MCPServerConfig{
		"demo": {Command: "python3", Args: []string{"-c", fixturePy}},
	}); err != nil {
		t.Fatal(err)
	}

	ses, err := a.store.New(a.cfg.WorkDir, "标准", "s")
	if err != nil {
		t.Fatal(err)
	}
	ag := a.newAgentFor(ses)
	if !hasSpec(ag.PackageSpecs, "mcp_demo_echo") {
		t.Fatalf("mcp_demo_echo not mounted: %+v", ag.PackageSpecs)
	}
	for _, sp := range ag.PackageSpecs {
		if sp.Function.Name == "mcp_demo_echo" && !strings.Contains(sp.Function.Description, "[MCP demo]") {
			t.Errorf("description prefix missing: %q", sp.Function.Description)
		}
	}
	if !ag.MutatingTools["mcp_demo_echo"] {
		t.Error("MCP tools must default to mutating")
	}

	// 服务器级停用
	a.setEnabled("mcp/demo", false)
	if ag2 := a.newAgentFor(ses); hasSpec(ag2.PackageSpecs, "mcp_demo_echo") {
		t.Error("disabled server still mounted")
	}
	a.setEnabled("mcp/demo", true)
	// 总开关
	a.setEnabled("mcp", false)
	if ag3 := a.newAgentFor(ses); hasSpec(ag3.PackageSpecs, "mcp_demo_echo") {
		t.Error("master switch ignored")
	}
}

func hasSpec(specs []llm.ToolSpec, name string) bool {
	for _, sp := range specs {
		if sp.Function.Name == name {
			return true
		}
	}
	return false
}

// REQ-005 AC-5: add/remove servers persist to mcp.json.
func TestAddRemoveMCPServer(t *testing.T) {
	a := newTestApp(t)

	a.mcpNameInput = "sqlite"
	a.mcpCmdInput = "uvx mcp-server-sqlite --db test.db"
	a.addMCPServer()
	if a.errText != "" {
		t.Fatalf("add failed: %s", a.errText)
	}
	servers := loadMCPServers(a.dataDir)
	if len(servers) != 1 || servers["sqlite"].Command != "uvx" || len(servers["sqlite"].Args) != 3 {
		t.Fatalf("servers: %+v", servers)
	}
	// 重复
	a.mcpNameInput = "sqlite"
	a.mcpCmdInput = "x"
	a.addMCPServer()
	if !strings.Contains(a.errText, "已存在") {
		t.Fatalf("duplicate: %q", a.errText)
	}
	// 非法名
	a.errText = ""
	a.mcpNameInput = "Bad Name"
	a.mcpCmdInput = "x"
	a.addMCPServer()
	if !strings.Contains(a.errText, "名称") {
		t.Fatalf("bad name: %q", a.errText)
	}
	// 两步删除
	a.removeMCPServerUI("sqlite")
	if _, ok := loadMCPServers(a.dataDir)["sqlite"]; !ok {
		t.Fatal("removed before confirmation")
	}
	a.removeMCPServerUI("sqlite")
	if _, ok := loadMCPServers(a.dataDir)["sqlite"]; ok {
		t.Fatal("not removed after confirmation")
	}
}
