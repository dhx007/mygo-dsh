# DESIGN-005：MCP 客户端 设计

- 状态：已落地
- 关联：REQ-005 · TEST-005
- 日期：2026-10-30

## 1. 现状分析

- REQ-004 已建立「外部工具 → PackageSpecs/PackageTools/MutatingTools → PluginEnabled
  门控 → 审批门」的完整通道，MCP 工具可直接复用。
- 无 JSON-RPC 客户端；无外部进程工具管理。

## 2. 方案对比

| 方案 | 思路 | 优点 | 缺点 | 结论 |
|---|---|---|---|---|
| A：stdio JSONRPC 自实现（标准库） | 子进程 + 按行 JSON-RPC，initialize/list/call 三方法 | 零依赖；MCP stdio 传输就是按行 JSON-RPC；覆盖 tools 域 | 不含 resources/prompts | ✅ 采用（REQ 范围） |
| B：第三方 MCP SDK | 引官方 Go SDK | 协议全 | 新依赖，违背 NFR-1；tools 域用不到全量 | ❌ |
| C：经包脚本桥接调 CLI | 用 goja 脚本 spawn？ | 无 | goja 无进程能力（沙箱），方向错误 | ❌ |

## 3. 选定方案（A）

```
internal/mcp/client.go
  Tool{Name,Description,InputSchema map[string]any}
  Client{cmd, stdin, scanner, mu, nextID, dead}
  Start(server string, command []string, env []string) (*Client, error)
  rpc(method, params) (json.RawMessage, error)   // 按行写读、id 匹配、通知跳过、120s 超时
  Initialize() error          // initialize + notifications/initialized
  ListTools() ([]Tool, error)
  CallTool(name, argsJSON) (text string, isError bool, err error)

mcp.go（main 包）
  MCPServerConfig{Command string, Args []string, Env map[string]string}
  loadMCPServers(dataDir) map[string]MCPServerConfig   // <dataDir>/mcp.json
  saveMCPServers(...)
  MCPManager{dataDir, mu, clients map[string]*mcp.Client, errs map[string]string}
    ClientFor(name) (*mcp.Client, error)   // 懒启动；dead 则重建
    Specs(a *app, workdir) ([]llm.ToolSpec, runners, mutating)  // 组装进包工具通道
    Call(server, tool, argsJSON) (string, bool)

app
  mcp *MCPManager；插件页第四段；add/remove server 动作；状态字段
```

协议时序：`Start → initialize(req) → result → notifications/initialized（无 id）→
tools/list / tools/call…`。响应按 id 匹配；收到无 id 行（服务器通知）跳过。

## 4. 数据结构

```jsonc
// <dataDir>/mcp.json
{ "servers": { "sqlite": { "command": "uvx", "args": ["mcp-server-sqlite"], "env": {} } } }
```

- 工具 spec：name=`mcp_<srv>_<tool>`；description=`[MCP <srv>] <原描述>`；
  parameters=原样透传 inputSchema（空则 `{"type":"object"}`）。
- 门控键：`mcp`（注册表插件）与 `mcp/<srv>`（服务器级），经现有 enabledTool。
- 失败语义：isError=true 或 err → 工具结果 failed；服务器错误态存 errs（UI 显示）。

## 5. 改动文件清单

| 文件 | 改动 | 对应 FR |
|---|---|---|
| `internal/mcp/client.go` | 新增：stdio JSONRPC 客户端 | FR-3 |
| `internal/mcp/client_test.go` | python3 夹具服务器全链路 | AC-1/4 |
| `mcp.go` | 新增：配置读写 + MCPManager | FR-1/5 |
| `plugins.go` | 注册表第 14 项「MCP」 | FR-2 |
| `app.go` | mcp 字段、newAgentFor 接线、插件页第四段 + add/remove | FR-4/6 |
| `mcp_test.go`、`app_test.go`、`plugins_test.go` | 用例 | AC-2/3/5/6 |

## 6. 与 DSH 原版的差异声明

- 仅 stdio 传输；无 resources/prompts；无 DSH 的权限预设粒度（以审批门兜底）。
- 命令行在 UI 用空格分词（不支持引号转义）；复杂 args 直接编辑 mcp.json。

## 7. 测试计划

| AC | 用例 | 层级 | 断言 |
|---|---|---|---|
| AC-1/4 | TestMCPClientLifecycle（python3 夹具：initialize/list/call、id 匹配） | 单测 | 协议全链路 |
| AC-3 | TestMCPAgentWiring（启用/停用服务器、总开关） | app 测试 | spec 门控 |
| AC-2 | TestBuiltinPluginRegistry（14）· TestPluginControlActions（14） | 单测 | 注册表 |
| AC-5 | TestPluginsPageMCP（添加/展开/删除 + mcp.json 落盘） | ui.Tester | UI |
| AC-6 | 全量 | `go test ./...` | 无回归 |

## 8. 风险与回退

- **风险**：恶意/劣质 MCP 服务器 → 默认 mutating 审批 + 超时中断 + 错误态隔离（单服务器
  故障不影响其他工具）。
- **风险**：python3 不可用的测试环境 → 夹具用 `python3` 前置检查跳过（CI 无 python 时
  标记 t.Skip）。
- **回退**：注册表 MCP 项开关关掉即整体下线；mcp.json 删除即无 MCP；代码独立可移除。

## 9. 变更记录

- 2026-10-30 定稿；实现与设计一致，无偏离。插件页「加载工具清单」按钮为显式触发
  （避免展开卡片即同步启动服务器进程），工具清单缓存于会话内。
