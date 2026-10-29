# DESIGN-002：插件体系骨架 设计

- 状态：定稿
- 关联：REQ-002 · TEST-002
- 日期：2026-10-30

## 1. 现状分析

- `app.go:1065`（插件页）与 `app.go:1433`（设置·内置插件）各自硬编码同一名单
  `bash, read_file, …, run_code`；`toolDescExtra`（`app.go:1425`）第三处补描述。
- 开关状态：`pluginToggles` 全局 map 缓存 `*bool`（`app.go:1045`），落盘走
  `setEnabled` → `cfg.EnabledTools`（键 = 工具名）。
- `internal/agent/agent.go:55-72`：Agent 已有动态 spec 机制（todoSpec 恒挂、taskSpec
  按 `Subagent != nil` 挂载），`execTool` switch 分发 —— `plugins` 工具照此接入。
- 缺：插件实体（名/版/类/源）、分组 UI、模型侧管理工具。

## 2. 方案对比

| 方案 | 思路 | 优点 | 缺点 | 结论 |
|---|---|---|---|---|
| A：主包插件注册表 + agent 回调工具 | 新增 `plugins.go` 集中注册表；agent 加 `PluginTool` 回调字段，app 注入实现 | 一处事实源；与 task/todo 同构；不污染 tools 包 | 注册表在 main 包，agent 经回调解耦（接口窄） | ✅ 采用 |
| B：注册表下沉 internal/plugins，tools 注册时自描述 | 工具自带插件元数据 | 包结构更"正规" | tools 包不知 todo/task/run_code（在 agent/主包），元数据仍要第二处 | ❌ 事实源依旧分裂 |
| C：直接在两个 view 里各自加分组 | 不动数据结构 | 最小改动 | 三处名单变五处，违背 NFR-1 | ❌ 加重债务 |

## 3. 选定方案（A）

```
plugins.go（main 包）
  Plugin{ID,Name,Description,Version,Category,Source,Tools,Kind}
  builtinPlugins() []Plugin          // 12 项，Category 有序
  pluginGroups() []pluginGroup       // 分类分组（组名 + 插件切片）
  byID(all, id) *Plugin

app
  pluginOn(id)  → 读 cfg（沿用 pluginToggles 缓存）
  setEnabled(id) → cfg.EnabledTools + saveConfig（沿用）
  pluginControl(action, id) → 供 agent 回调：list/info/enable/disable
  pluginsView / secPlugins → pluginGroups 渲染（组头 + 卡片行）

agent.Agent
  PluginTool func(action, id string) (string, bool)   // nil = 不挂载
  pluginSpec{plugins(action, plugin?)}                // Run() 时按需追加
  execTool case "plugins" → a.PluginTool(...)
```

数据流：UI 开关与模型 `plugins enable/disable` 都收敛到 `setEnabled`，落盘与生效
（下次 `newAgentFor` 按 enabledTool 过滤）路径唯一。

## 4. 数据结构与接口

```go
// plugins.go（新增，main 包）
type Plugin struct {
    ID          string   `json:"id"`          // = enabled_tools 键 / 工具名
    Name        string   `json:"name"`        // 中文显示名
    Description string   `json:"description"`
    Version     string   `json:"version"`     // 内置统一 0.1.0
    Category    string   `json:"category"`    // 终端|文件|搜索|子代理|循环|系统
    Source      string   `json:"source"`      // 内置
    Tools       []string `json:"tools"`       // 挂载的工具名
    Kind        string   `json:"kind"`        // tool
}
func builtinPlugins() []Plugin
func pluginGroups(all []Plugin) []pluginGroup   // 按固定分类序分组

// agent.go（修改）
type Agent struct { …
    PluginTool func(action, plugin string) (string, bool)
}
var pluginSpec = llm.ToolSpec{…} // action: list|info|enable|disable; plugin: 插件 id

// presets.go（修改）
type Preset struct { …
    CanManagePlugins bool `json:"can_manage_plugins"` // 创造=true
}
```

12 个内置插件：bash→「Shell 终端」；read_file/write_file/edit_file→「读/写/编辑文件」；
glob/grep→「文件匹配/内容搜索」；web_fetch/web_search→「网页抓取/网页搜索」；todo→
「任务清单」；task→「子代理」；run_code→「PTC 脚本」；plugins→「插件管理」（Tools:
["plugins"]，挂载 models 侧管理工具）。

`plugins` 工具输出（供模型读）：list=每行 `id  [开/关] 名称 — 描述`；info=完整字段；
enable/disable=回显新状态；未知 action/id → `error: …`。

## 5. 改动文件清单

| 文件 | 改动 | 对应 FR |
|---|---|---|
| `plugins.go` | 新增：Plugin 结构、builtinPlugins、pluginGroups、byID | FR-1 |
| `app.go` | pluginsView/secPlugins 改为分组渲染（读注册表）；pluginControl 实现；newAgentFor 挂 PluginTool（创造 + enabledTool("plugins")） | FR-2/3/4/5/6 |
| `internal/agent/agent.go` | PluginTool 字段、pluginSpec、execTool 分发 | FR-5/6 |
| `presets.go` | CanManagePlugins 字段；创造=true；creatorPrompt 提及 plugins 工具 | FR-6/8 |
| `plugins_test.go` | 注册表一致性、pluginControl 行为 | AC-1/3 |
| `app_test.go` | 插件页分组 UI 断言、预设接线断言 | AC-2/4 |

## 6. 与 DSH 原版的差异声明

- 分类名「文件 / 系统」为**合理假设**：DSH 素材只给出 Terminal/Subagent/Search/Loop/
  Default 五类，文件工具需归位、插件管理归「系统」；Loop 对应 循环（todo/run_code）。
- DSH 插件卡片字段未知，卡片行按截图对齐过的现有行式布局升级（名+徽标+描述+开关），
  不做弹窗详情页（info 详情由模型工具与轨迹承载）。
- cordis 运行时（loader/group/include/patch）不复刻：内置插件编译在二进制里，
  enabled_tools 承载开关；本地插件运行时留给 REQ-003。

## 7. 测试计划

| FR/AC | 用例 | 层级 | 断言要点 |
|---|---|---|---|
| AC-1 | TestBuiltinPluginRegistry | 单测 | 12 项、无重复 id、四预设工具名全覆盖 |
| AC-3 | TestPluginControlActions | app 测试 | list/info/enable/disable/非法 action |
| AC-4 | TestPluginToolPresetWiring | app 测试 | 创造挂载且开关生效；标准不挂载 |
| AC-2 | TestPluginsPageGroups | ui.Tester | 六组头 + 显示名可见 |
| AC-5 | 全量 | `go test ./...` | 无回归（含既有 TestNewSessionAndSend 等） |

## 8. 风险与回退

- **风险**：`plugins` 工具被模型滥用（关掉 bash 自残）→ 工具描述声明「停用工具插件会
  让对应工具从后续轮次消失，慎重」；且 UI 随时可恢复，风险可逆。
- **风险**：pluginToggles 全局缓存跨测试实例串味 → 既有模式，测试内先读 pluginOn 再
  断言，不新增全局可变状态。
- **回退**：CanManagePlugins 全部置 false 即回到无模型工具状态；plugins.go 可整体移除，
  两个 view 恢复名单渲染即可。

## 9. 变更记录

- 2026-10-30 定稿。
- 2026-10-30 落地，两处超出定稿范围的改进（目的：让 FR-5 的「下一轮生效」语义为真）：
  1. Agent 新增 `PluginEnabled func(id) bool` 回调：每次 Run 前按开关过滤 `a.Specs`
     （新增 `toolSpecs()` 方法承载组装逻辑），UI 或模型停用的插件从下一条消息起从
     工具集消失——定稿时只有 newAgentFor 的静态过滤，会话中途切换不生效。
  2. 第 5 节清单外新增测试文件 `internal/agent/agent_shell_test.go` 的
     TestToolSpecsFilterAndPlugins / TestPluginsToolDispatch（覆盖上述过滤）。
