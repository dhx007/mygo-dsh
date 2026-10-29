package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"mygo-harness/internal/llm"
	"mygo-harness/internal/mcp"
)

// MCP server config lives in <dataDir>/mcp.json; each enabled server's
// tools mount through the same channel as plugin-package tools (REQ-004).
// Gating: the "mcp" registry plugin is the master switch, "mcp/<name>"
// toggles one server; MCP tools default to mutating (approval gate).

type MCPServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

type mcpConfigFile struct {
	Servers map[string]MCPServerConfig `json:"servers"`
}

func mcpConfigPath(dataDir string) string { return filepath.Join(dataDir, "mcp.json") }

func loadMCPServers(dataDir string) map[string]MCPServerConfig {
	b, err := os.ReadFile(mcpConfigPath(dataDir)) //nolint:gosec
	if err != nil {
		return map[string]MCPServerConfig{}
	}
	var f mcpConfigFile
	if json.Unmarshal(b, &f) != nil || f.Servers == nil {
		return map[string]MCPServerConfig{}
	}
	return f.Servers
}

func saveMCPServers(dataDir string, servers map[string]MCPServerConfig) error {
	if servers == nil {
		servers = map[string]MCPServerConfig{}
	}
	b, err := json.MarshalIndent(&mcpConfigFile{Servers: servers}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(mcpConfigPath(dataDir), b, 0o644)
}

// MCPManager owns the connected server clients; a server that died or hit
// a protocol error is respawned on next use.
type MCPManager struct {
	dataDir string

	mu      sync.Mutex
	clients map[string]*mcp.Client
	errs    map[string]string
}

func newMCPManager(dataDir string) *MCPManager {
	return &MCPManager{dataDir: dataDir, clients: map[string]*mcp.Client{}, errs: map[string]string{}}
}

// ClientFor returns a live client for the server, (re)spawning as needed.
func (m *MCPManager) ClientFor(name string) (*mcp.Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.clients[name]; ok && c != nil {
		return c, nil
	}
	servers := loadMCPServers(m.dataDir)
	cfg, ok := servers[name]
	if !ok {
		return nil, fmt.Errorf("未知 MCP 服务器 %s", name)
	}
	command := append([]string{cfg.Command}, cfg.Args...)
	c, err := mcp.Start(name, command, cfg.Env)
	if err != nil {
		m.errs[name] = err.Error()
		return nil, err
	}
	delete(m.errs, name)
	m.clients[name] = c
	return c, nil
}

// fail marks a server broken so the next call respawns it.
func (m *MCPManager) fail(name, msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errs[name] = msg
	if c := m.clients[name]; c != nil {
		c.Close()
	}
	delete(m.clients, name)
}

// Call invokes one tool on one server.
func (m *MCPManager) Call(server, tool, argsJSON string) (string, bool) {
	c, err := m.ClientFor(server)
	if err != nil {
		return "error: " + err.Error(), true
	}
	text, isErr, err := c.CallTool(context.Background(), tool, argsJSON)
	if err != nil {
		m.fail(server, err.Error())
		return "error: " + err.Error(), true
	}
	if isErr {
		return text, true
	}
	return text, false
}

// serverErr exposes the last error of a server for the plugins page.
func (m *MCPManager) serverErr(name string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.errs[name]
}

// forget drops a server entirely (config removed): closes its client and
// clears state.
func (m *MCPManager) forget(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c := m.clients[name]; c != nil {
		c.Close()
	}
	delete(m.clients, name)
	delete(m.errs, name)
}

// specName is the model-facing tool name of one MCP tool.
func mcpSpecName(server, tool string) string {
	return "mcp_" + server + "_" + tool
}

// collectSpecs assembles tool specs and runners for every enabled server.
// Server-level errors are recorded and shown on the plugins page; a broken
// server never blocks the others.
func (m *MCPManager) collectSpecs(a *app) (specs []llm.ToolSpec, runners map[string]func(ctx context.Context, argsJSON string) (string, bool), mutating map[string]bool) {
	runners = map[string]func(ctx context.Context, argsJSON string) (string, bool){}
	mutating = map[string]bool{}
	for name := range loadMCPServers(m.dataDir) {
		if !a.enabledTool("mcp/" + name) {
			continue
		}
		c, err := m.ClientFor(name)
		if err != nil {
			continue
		}
		tools, err := c.ListTools()
		if err != nil {
			m.fail(name, err.Error())
			continue
		}
		for _, tl := range tools {
			tool := tl
			specName := mcpSpecName(name, tool.Name)
			desc := tool.Description
			if desc == "" {
				desc = "(MCP 工具，无描述)"
			}
			params := tool.InputSchema
			if params == nil {
				params = map[string]any{"type": "object"}
			}
			specs = append(specs, llm.ToolSpec{
				Type: "function",
				Function: llm.ToolDef{
					Name:        specName,
					Description: fmt.Sprintf("[MCP %s] %s", name, desc),
					Parameters:  params,
				},
			})
			runners[specName] = func(ctx context.Context, argsJSON string) (string, bool) {
				return m.Call(name, tool.Name, argsJSON)
			}
			mutating[specName] = true // MCP 工具默认走审批门
		}
	}
	return specs, runners, mutating
}
