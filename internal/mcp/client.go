// Package mcp implements a minimal Model Context Protocol client over the
// stdio transport: newline-delimited JSON-RPC 2.0 with the initialize
// handshake and the tools domain (tools/list, tools/call). Only the
// standard library is used; servers run as child processes and exit when
// our pipes close.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Tool is one tool advertised by a server.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"inputSchema,omitempty"`
}

const protocolVersion = "2024-11-05"

// requestTimeout bounds one JSON-RPC round trip; tools/call may run long
// host commands, so it is generous.
var requestTimeout = 120 * time.Second

// Client is one connected stdio MCP server.
type Client struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	out    *bufio.Scanner
	nextID int
	dead   bool
	name   string
}

// Start spawns the server process and performs the initialize handshake.
func Start(server string, command []string, extraEnv map[string]string) (*Client, error) {
	if len(command) == 0 || strings.TrimSpace(command[0]) == "" {
		return nil, fmt.Errorf("命令为空")
	}
	cmd := exec.Command(command[0], command[1:]...) //nolint:gosec
	cmd.Env = os.Environ()
	for k, v := range extraEnv {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		stdin.Close()
		return nil, fmt.Errorf("启动 %s: %w", server, err)
	}
	c := &Client{
		cmd:   cmd,
		stdin: stdin,
		out:   newScanner(stdout),
		name:  server,
	}
	if err := c.initialize(); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

func newScanner(r io.Reader) *bufio.Scanner {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 8*1024*1024)
	return sc
}

// rpc performs one request/response round trip. Responses are matched by
// id; notification lines and stale messages are skipped.
func (c *Client) rpc(method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.dead {
		return nil, fmt.Errorf("MCP 服务器 %s 已断开", c.name)
	}
	c.nextID++
	id := c.nextID
	req, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": id, "method": method, "params": params,
	})
	if err != nil {
		return nil, err
	}
	if _, err := fmt.Fprintf(c.stdin, "%s\n", req); err != nil {
		c.dead = true
		return nil, fmt.Errorf("写入 %s: %w", c.name, err)
	}

	type result struct {
		raw json.RawMessage
		err error
	}
	ch := make(chan result, 1)
	go func() {
		for c.out.Scan() {
			var resp struct {
				ID    *int `json:"id"`
				Error *struct {
					Message string `json:"message"`
				} `json:"error"`
				Result json.RawMessage `json:"result"`
			}
			if json.Unmarshal(c.out.Bytes(), &resp) != nil {
				continue // 非法行跳过
			}
			if resp.ID == nil || *resp.ID != id {
				continue // 通知或过期消息
			}
			if resp.Error != nil {
				ch <- result{nil, fmt.Errorf("%s: %s", method, resp.Error.Message)}
			} else {
				ch <- result{resp.Result, nil}
			}
			return
		}
		ch <- result{nil, fmt.Errorf("MCP 服务器 %s 连接中断", c.name)}
	}()

	select {
	case r := <-ch:
		if r.err != nil {
			if strings.Contains(r.err.Error(), "连接中断") {
				c.dead = true
			}
			return nil, r.err
		}
		return r.raw, nil
	case <-time.After(requestTimeout):
		c.dead = true
		return nil, fmt.Errorf("%s 超时（%s）", method, requestTimeout)
	}
}

func (c *Client) initialize() error {
	raw, err := c.rpc("initialize", map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "mygo-harness", "version": "0.1.0"},
	})
	if err != nil {
		return err
	}
	var info struct {
		ServerInfo struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	if json.Unmarshal(raw, &info) == nil && info.ServerInfo.Name != "" {
		c.name = info.ServerInfo.Name
	}
	return c.notify("notifications/initialized")
}

// notify sends a JSON-RPC notification (no id, no response expected).
func (c *Client) notify(method string) error {
	req, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": method})
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(c.stdin, "%s\n", req); err != nil {
		c.dead = true
		return err
	}
	return nil
}

// ListTools returns the server's advertised tools.
func (c *Client) ListTools() ([]Tool, error) {
	raw, err := c.rpc("tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	var res struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, err
	}
	return res.Tools, nil
}

// CallTool invokes one tool and joins its text content blocks.
func (c *Client) CallTool(ctx context.Context, name, argsJSON string) (string, bool, error) {
	args := map[string]any{}
	if strings.TrimSpace(argsJSON) != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", false, fmt.Errorf("参数不是合法 JSON: %w", err)
		}
	}
	raw, err := c.rpc("tools/call", map[string]any{"name": name, "arguments": args})
	if err != nil {
		return "", true, err
	}
	var res struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return "", true, err
	}
	var b strings.Builder
	for _, part := range res.Content {
		if part.Text != "" {
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(part.Text)
		}
	}
	if b.Len() == 0 {
		b.WriteString("(工具无文本输出)")
	}
	return b.String(), res.IsError, nil
}

// Close kills the server process; the child exits when its stdin closes.
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.dead {
		return
	}
	c.dead = true
	_ = c.stdin.Close()
	if c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
}
