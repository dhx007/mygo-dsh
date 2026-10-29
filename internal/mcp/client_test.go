package mcp

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// fixturePy is a minimal MCP stdio server: initialize, tools/list with one
// echo tool, tools/call echoing its arguments (isError when fail=true), and
// a "tools/never" method that never answers (timeout tests).
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
        continue  # 通知（notifications/initialized 等）不应答
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
        continue  # tools/never：不应答，用于超时测试
    print(json.dumps({"jsonrpc": "2.0", "id": rid, "result": result}, ensure_ascii=False), flush=True)
`

func startFixture(t *testing.T) *Client {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 不可用，跳过 MCP 协议测试")
	}
	c, err := Start("fixture", []string{"python3", "-c", fixturePy}, nil)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(c.Close)
	return c
}

// REQ-005 AC-1: initialize handshake, tools/list, tools/call with id
// matching and notification skipping.
func TestMCPClientLifecycle(t *testing.T) {
	c := startFixture(t)

	tools, err := c.ListTools()
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "echo" || !strings.Contains(tools[0].Description, "回声") {
		t.Fatalf("tools: %+v", tools)
	}

	text, isErr, err := c.CallTool(context.Background(), "echo", `{"who":"zz"}`)
	if err != nil || isErr {
		t.Fatalf("call: text=%q isErr=%v err=%v", text, isErr, err)
	}
	if !strings.Contains(text, `"who"`) || !strings.Contains(text, "zz") {
		t.Fatalf("echo content: %q", text)
	}

	// 连续调用（id 递增、无粘包）
	text, _, err = c.CallTool(context.Background(), "echo", `{"n":2}`)
	if err != nil || !strings.Contains(text, `"n": 2`) {
		t.Fatalf("second call: text=%q err=%v", text, err)
	}
}

// REQ-005 AC-4: isError maps to the failure marker.
func TestMCPClientCallError(t *testing.T) {
	c := startFixture(t)
	text, isErr, err := c.CallTool(context.Background(), "echo", `{"fail":true}`)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !isErr || !strings.Contains(text, "echo") {
		t.Fatalf("isError: text=%q isErr=%v", text, isErr)
	}
}

// REQ-005 AC-1: an unanswered method hits the request timeout.
func TestMCPClientTimeout(t *testing.T) {
	old := requestTimeout
	requestTimeout = 300 * time.Millisecond
	t.Cleanup(func() { requestTimeout = old })

	c := startFixture(t)
	start := time.Now()
	if _, err := c.rpc("tools/never", map[string]any{}); err == nil || !strings.Contains(err.Error(), "超时") {
		t.Fatalf("timeout err: %v", err)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatalf("timeout too slow: %s", time.Since(start))
	}
	// 死亡后拒绝再请求，而不是挂起
	if _, err := c.rpc("tools/list", map[string]any{}); err == nil || !strings.Contains(err.Error(), "已断开") {
		t.Fatalf("dead client: %v", err)
	}
}
