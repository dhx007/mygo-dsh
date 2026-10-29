# REQ-005：MCP 客户端（stdio JSONRPC · 插件页服务器管理 · 工具接入）

- 状态：已验证（TEST-005 全部通过）
- 关联：DESIGN-005 · TEST-005 · FEATURE-MATRIX 行 `工具/dsh-mcp-client`
- 日期：2026-10-30
- 证据：`.dsh-ref/dsh-package.json` 含 `dsh-mcp-client`、`dsh-mcp-resources`；
  通用 MCP 规范（stdio 传输 = 按行分隔的 JSON-RPC 2.0；initialize 握手 →
  notifications/initialized → tools/list → tools/call）。

## 1. 背景

DSH 通过 MCP 客户端接入外部工具服务器（`dsh-mcp-client`），是「一切皆插件」里
外部扩展的标准通道。mygo 已有插件包机制（REQ-004，进程内 goja），但外部进程型
工具（数据库、浏览器、各类领域服务）需要 MCP。本 REQ 实现最小可用的 MCP 客户端：
管理 stdio 服务器、把其 tools 挂进现有门控体系。

## 2. 用户故事

作为用户，我在插件页添加一个 MCP 服务器（名称 + 启动命令），它的工具就出现在
标准/创造等模式的工具集里，模型可直接调用；服务器出问题时我能看到错误并可
停用或删除它。

## 3. 功能需求（FR）

- **FR-1（配置）**：服务器配置存 `<数据目录>/mcp.json`：`{"servers":{"<名>":{"command","args","env"}}}`；
  插件页可添加（名称 + 命令行，空格分词）、删除（两步确认）；高级字段（args 数组、env）
  可直接编辑 mcp.json。
- **FR-2（门控）**：注册表新增「MCP」插件（系统分类，第 14 项）——总开关停用则全部
  MCP 工具不挂载；每服务器开关（enabled_tools 键 `mcp/<名>`）可单独停用。
- **FR-3（协议）**：stdio 传输、按行分隔 JSON-RPC 2.0；连接时完成 initialize 握手 +
  notifications/initialized；`tools/list` 取工具清单；`tools/call` 执行并取文本内容。
- **FR-4（工具挂载）**：启用服务器的工具以 `mcp_<服务器>_<工具>` 命名挂载（描述前缀
  `[MCP <服务器>]`），spec 复用 REQ-004 的包工具机制；MCP 工具默认视为变更类
  （mutating，走审批门）——外部行为不可信，安全默认。
- **FR-5（健壮性）**：服务器进程懒启动；单请求超时 120s；进程死亡或协议错误标记该
  服务器错误态（插件页可见）并在下次调用时重启；配置更改后下一条消息生效。
- **FR-6（可见性）**：插件页第四段「MCP 服务器」列出全部配置（名称/命令/工具数/状态
  或最近错误），展开可看工具清单与开关。

## 4. 非功能需求

- **NFR-1** 纯 Go 标准库实现 JSON-RPC（不引第三方 MCP SDK）。
- **NFR-2** 客户端互斥串行请求；响应按 id 匹配，通知行跳过。
- **NFR-3** 服务器进程随宿主退出（stdin 关闭即终止），不额外守护。

## 5. 验收标准

- AC-1：夹具服务器（python3 脚本）上完成 initialize → tools/list → tools/call 全链路，
  id 匹配、通知跳过。
- AC-2：注册表 14 项；`plugins list` 输出含 MCP；总数断言更新。
- AC-3：启用服务器 → agent 工具集含 `mcp_<srv>_<tool>` spec 且 mutating；停用服务器或
  总开关后 spec 消失。
- AC-4：工具调用经夹具返回文本；isError/协议错误映射失败标记。
- AC-5：插件页 MCP 段渲染（添加/展开/删除）；mcp.json 落盘。
- AC-6：`go test ./...` 全绿无回归。

## 6. 范围外

- MCP **resources/prompts/sampling** 等其余能力（`dsh-mcp-resources`）→ 后续 REQ。
- SSE/HTTP 传输（仅 stdio）；OAuth；服务器市场集成（REQ-006 的 Discover 可后续推荐
  MCP 服务器）。
- 工具 schema 深度校验：inputSchema 原样透传到 spec.parameters（模型自行遵守）。

## 7. 决策记录

- 工具命名 `mcp_<服务器>_<工具>` 而非原版可能的别名机制：防内置/包工具撞名，一眼可溯。
- 默认 mutating：DSH 有完整权限体系，mygo 以审批门兜底最安全；用户可全局自动审批。
- 复用 REQ-004 的 PackageTools/PackageSpecs 挂载机制：MCP 工具本质上就是另一类
  「外部插件」，不新开通道。
