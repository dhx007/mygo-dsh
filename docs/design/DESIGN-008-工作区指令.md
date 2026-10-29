# DESIGN-008：agent-instructions 设计

- 状态：已落地
- 关联：REQ-008 · TEST-008
- 日期：2026-10-30

## 1. 现状分析

- `newAgentFor`（app.go）构建 `prompt := applyCwdTemplate(preset.SystemPrompt, …)`，
  是唯一系统提示词出口；子代理经 `child := base` 继承。
- 注册表 `plugins.go` 14 项；`builtinPluginCard` 详情渲染「挂载工具：…」假定有工具。

## 2. 方案对比

| 方案 | 思路 | 优点 | 缺点 | 结论 |
|---|---|---|---|---|
| A：newAgentFor 末尾追加指令段 | 读工作区文件 → prompt += 段落 | 注入点唯一、子代理天然继承 | 无 | ✅ 采用 |
| B：作为首条 user 消息注入 | 不污染系统提示 | — | 与 DSH "system prompt loader" 语义不符；压缩后丢失 | ❌ |

## 3. 选定方案（A）

```go
// instructions.go（新增）
const instructionsLimit = 32 * 1024
func loadWorkspaceInstructions(workspace string) string
    // AGENTS.md → CLAUDE.md 顺序，取第一个非空文件；32KB 截断；
    // 返回 "[工作区指令：<名>] 请遵循以下来自工作区的项目指令：\n\n<正文>"，无则 ""

// newAgentFor（app.go）
if a.enabledTool("instructions") {
    if instr := loadWorkspaceInstructions(ses.Meta.Workspace); instr != "" {
        prompt += "\n\n" + instr
    }
}
```

注册表第 15 项：`{ID:"instructions", Name:"工作区指令", Kind:"context", Tools:[]}`。
卡片与 info 对无工具插件（Kind=="context"）显示「贡献：系统提示词注入」。

## 4. 数据结构

无新存储；文件格式即 Markdown 原文。

## 5. 改动文件清单

| 文件 | 改动 | 对应 FR |
|---|---|---|
| `instructions.go` | 新增：发现与注入文本构建 | FR-1/2 |
| `plugins.go` | 注册表第 15 项 | FR-3 |
| `app.go` | newAgentFor 接线；卡片/info 对 context 类插件的展示 | FR-3/4 |
| `instructions_test.go`、`plugins_test.go`、`app_test.go` | 用例 | AC-1~5 |

## 6. 与 DSH 原版的差异声明

- 优先级（AGENTS.md 先于 CLAUDE.md）为合理假设（REQ §7）；嵌套/全局指令不做。

## 7. 测试计划

| AC | 用例 | 层级 | 断言 |
|---|---|---|---|
| AC-1/2 | TestLoadWorkspaceInstructions（优先级/回退/空文件/截断/缺失） | 单测 | 发现逻辑 |
| AC-3 | TestAgentInstructionsWiring（注入、停用即失） | app 测试 | 门控 |
| AC-4 | TestBuiltinPluginRegistry（15）· TestPluginsPageSections（15） | 单测/UI | 注册表 |
| AC-5 | 全量 | `go test ./...` | 无回归 |

## 8. 风险与回退

- **风险**：项目指令提示注入 → 与技能同级风险：指令只是文本，变更类动作仍走审批门；
  只加载工作区根目录文件（用户自己的项目）。
- **回退**：停用「工作区指令」插件即可整体关闭。

## 9. 变更记录

- 2026-10-30 定稿；实现与设计一致，无偏离。
