# TEST-005：MCP 客户端 测试报告

- 状态：通过
- 关联：REQ-005 · DESIGN-005
- 执行日期 / 执行环境：2026-10-30 · macOS 24.1.0 arm64 · Go（标准库 JSONRPC，无新依赖）· 协议夹具 python3

## 1. 自动化用例执行记录

全量回归（`go test ./...`，原样输出——五包首次全绿）：

```
ok  	mygo-harness	(cached)
ok  	mygo-harness/internal/agent	(cached)
ok  	mygo-harness/internal/llm	(cached)
ok  	mygo-harness/internal/mcp	2.557s
ok  	mygo-harness/internal/tools	(cached)
```

新增用例（verbose，原样输出）：

```
--- PASS: TestMCPClientLifecycle (0.03s)
--- PASS: TestMCPClientCallError (0.02s)
--- PASS: TestMCPClientTimeout (0.32s)
ok  	mygo-harness/internal/mcp	2.527s
--- PASS: TestPluginsPageMCP (0.05s)
--- PASS: TestMCPServerConfigRoundtrip (0.00s)
--- PASS: TestMCPAgentWiring (0.02s)
--- PASS: TestAddRemoveMCPServer (0.00s)
--- PASS: TestBuiltinPluginRegistry (0.00s)   ← 14 项（新增 MCP）
--- PASS: TestPluginControlActions (0.01s)
```

另：`go vet ./...` 无告警；`go build -trimpath` 构建成功。python3 不可用的环境自动
跳过协议/接线用例（t.Skip），其余用例不受影响。

## 2. 用例 ↔ 验收标准对照

| FR | 验收标准 | 用例 | 结果 |
|---|---|---|---|
| FR-3 | AC-1 initialize 握手 + tools/list + tools/call 全链路（id 匹配、通知跳过、连续调用） | TestMCPClientLifecycle | ✅ |
| FR-3 | AC-4 isError 映射失败标记 | TestMCPClientCallError | ✅ |
| FR-5 | AC-1 未应答方法 120s 超时（测试 300ms 注入）→ 死亡态拒绝再请求 | TestMCPClientTimeout | ✅ |
| FR-4 | AC-3 `mcp_demo_echo` 挂载（[MCP demo] 前缀 + 默认 mutating）；服务器级/总开关停用即失 | TestMCPAgentWiring | ✅ |
| FR-2 | AC-2 注册表 14 项；plugins list 计数 | TestBuiltinPluginRegistry · TestPluginControlActions | ✅ |
| FR-1/6 | AC-5 mcp.json 读写、添加（含重复/非法名校验）、两步删除、插件页 MCP 段渲染 | TestMCPServerConfigRoundtrip · TestAddRemoveMCPServer · TestPluginsPageMCP | ✅ |
| AC-6 | 全量无回归 | `go test ./...` 五包全 ok | ✅ |

## 3. 手工 / 活体核对

DSH 0.2 活体 UI 未见 MCP 独立界面（插件面板无 MCP 段），本实现把 MCP 归入插件页第四段
属自有设计（与 REQ-003 技能分区同类扩展，已在 REQ §7 声明）。协议行为以 MCP 通用规范
与夹具服务器验证为准；未对照原版 `dsh-mcp-client` 的传输细节（其未随包发布）。

## 4. 已知问题与后续项

- 仅 stdio 传输；SSE/HTTP、resources/prompts/sampling 未做（REQ §6）。
- collectSpecs 在每条消息构建时对每个启用服务器做 ClientFor+ListTools（已连接则复用；
  首次连接为同步握手，通常 <100ms）；服务器未装命令时错误态记录并在 UI 显示。
- UI 命令行空格分词，不支持引号；复杂参数直接编辑 mcp.json。
- 服务器进程随宿主退出（stdin 关闭）；无显式 shutdown 通知（JSON-RPC 未实现
  notifications/shutdown → 可后续补）。
- 插件市场（REQ-006）可在 Discover 标签推荐 MCP 服务器条目。

## 5. 结论

7/7 新用例通过，AC-1~AC-6 全部满足，无回归（注册表断言已同步 14 项）。
FEATURE-MATRIX：`dsh-mcp-client` ❌→🟡（tools 域全链路可用；resources/SSE 传输缺）。
建议 REQ-005 置「已验证」。
