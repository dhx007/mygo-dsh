# DESIGN-010：会话内模型切换 设计

- 状态：已落地
- 关联：REQ-010 · TEST-010
- 日期：2026-10-30

## 1. 现状分析

- composer 操作行 `app.go:783` 静态显示 `a.cfg.Model`；`newAgentFor` 用 `a.cfg.Model`。
- `SessionMeta`（store.go）无 model 字段；弹层模式 `ui.Popover(c, anchor, &open, body)`
  已在用户菜单验证。

## 2. 方案对比

| 方案 | 思路 | 优点 | 缺点 | 结论 |
|---|---|---|---|---|
| A：会话覆盖 + Popover 选择器 + /model | meta.model 覆盖 cfg.Model；控件对齐原版 | 语义清晰、多会话互不影响 | 两层来源需 effectiveModel 收口 | ✅ 采用 |
| B：直接改全局 cfg.Model | 最简单 | 多会话互相干扰，丢"会话级"语义 | ❌ |

## 3. 选定方案（A）

```go
// internal/llm/client.go
func (c *Client) ListModels(ctx context.Context) ([]string, error)   // GET /models，排序

// store.go
SessionMeta.Model string `json:"model,omitempty"`   // 空 = 跟随全局

// app.go
func (a *app) effectiveModel() string          // ses.Meta.Model 优先，否则 cfg.Model
func (a *app) ensureModels()                   // 首次异步拉取 → modelOptions（失败回退内置 + 当前模型）
func (a *app) switchSessionModel(m string)     // 写 meta + Touch + dropAgent + note
composer：模型名 → anchor Row（icoBot + 名 + icoDown）→ Clicked 切 modelMenu →
          ui.Popover 列出 a.modelOptions（当前项 ● 标记）+ 提示行
newAgentFor：Model: a.effectiveModel()
/model 命令：switchSessionModel（无参 → 用法 + 当前生效模型）
```

## 4. 数据结构

- meta.model（string，omitted）；无新配置字段；modelOptions 为内存缓存。

## 5. 改动文件清单

| 文件 | 改动 | 对应 FR |
|---|---|---|
| `internal/llm/client.go` | ListModels | FR-2 |
| `store.go` | SessionMeta.Model | FR-1/5 |
| `app.go` | effectiveModel/ensureModels/switchSessionModel、composer 控件+Popover、/model、newAgentFor | FR-3/4/5 |
| `internal/llm/client_test.go`、`app_test.go` | 用例 | AC-1~5 |

## 6. 与 DSH 原版的差异声明

- 选择器为 Popover 下拉（原版控件细节未逐像素核对）；无按模型计价显示。

## 7. 测试计划

| AC | 用例 | 层级 | 断言 |
|---|---|---|---|
| AC-1 | TestListModels（mock 200 排序 / 401 报错） | 单测 | 端点 |
| AC-2/3 | TestSessionModelSwitch（meta 落盘、agent 重建用新模型、回退全局、/model 命令） | app 测试 | 切换语义 |
| AC-4 | TestComposerModelMenu（控件文本、Popover 清单、点选切换） | ui.Tester | UI |
| AC-5 | 全量 | `go test ./...` | 无回归 |

## 8. 风险与回退

- **风险**：/models 端点不可用 → 回退内置清单 + 当前模型，功能不失效。
- **回退**：控件退回静态文本即回到现状；meta.model 字段向后兼容（空 = 全局）。

## 9. 变更记录

- 2026-10-30 定稿；实现与设计一致，无偏离。
