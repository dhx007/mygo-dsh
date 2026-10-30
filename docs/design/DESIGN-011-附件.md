# DESIGN-011：附件 设计

- 状态：已落地
- 关联：REQ-011 · TEST-011
- 日期：2026-10-30

## 1. 现状分析

- `llm.Message.Content` 是纯 string，线格式 content 总是字符串；`agent.Run(ctx,
  userText, cb)` 只收文本；composer ＋ 按钮当前把输入置为 "/"。

## 2. 方案对比

| 方案 | 思路 | 优点 | 缺点 | 结论 |
|---|---|---|---|---|
| A：Message.Parts 本地字段 + 自定义序列化 | 纯文本走原格式；含 Parts 序列化为 content 数组 | 线格式零回归（测试锁定）；history 兼容 | Message 需自定义 Marshal/Unmarshal | ✅ 采用 |
| B：全部消息改数组格式 | 统一 | — | 破坏现有请求格式与历史文件 | ❌ |

## 3. 选定方案（A）

```go
// internal/llm/client.go
type ContentPart struct { Type, Text, ImageURL string }        // image_url 序列化为嵌套 {"image_url":{"url"}}
Message += Parts []ContentPart `json:"-"`                       // 本地形态
MarshalJSON：Parts 非空 → content = parts 数组；否则 content = 字符串
UnmarshalJSON：content 为数组 → 还原 Parts；为字符串 → Content

// internal/agent/agent.go
func (a *Agent) RunMessage(ctx, user llm.Message, cb) error     // 原 Run 主体
func (a *Agent) Run(ctx, userText, cb)                          // 委托 RunMessage（签名不变）

// app.go
type pendingAttachment struct { Name, Text string; IsImage bool; Data []byte }
app.pendingFiles []pendingAttachment; attachMenu bool; attachInput string
addAttachmentPath(p)：相对工作区解析 → 图片（png/jpg/jpeg/gif/webp，≤8MB）读字节 /
  文本（≤64KB 截断）读内容 → 入列；错误走 errText
send()：文本附件拼进 text；图片 → Parts（data URL）；RunMessage 发送；
  轨迹 user 事件带 attachments；uiMsg 带附件名（气泡 chip）
composer：＋ → attachMenu Popover（路径输入 + 添加 + 窗口模式「选择文件…」调
  mygo.Dialog.Open）；chips 行渲染于输入行上方
```

## 4. 数据结构

线格式（含图片的消息）：
```json
{"role":"user","content":[
  {"type":"text","text":"看这张图"},
  {"type":"image_url","image_url":{"url":"data:image/png;base64,…"}}]}
```

## 5. 改动文件清单

| 文件 | 改动 | 对应 FR |
|---|---|---|
| `internal/llm/client.go` | ContentPart + Message.Parts + 自定义序列化 | FR-4/6 |
| `internal/agent/agent.go` | RunMessage 拆分 | FR-4 |
| `app.go` | pendingAttachment 管理、发送注入、composer Popover/chips、气泡 chip、轨迹字段 | FR-1~3/5 |
| `internal/llm/*_test.go`、`internal/agent/*_test.go`、`app_test.go` | 用例 | AC-1~5 |

## 6. 与 DSH 原版的差异声明

- 无附件存储服务（原版 dsh-attachment-local 跨会话附件库）；入口为路径/对话框
  （原版另有目录选择器体系）。

## 7. 测试计划

| AC | 用例 | 层级 | 断言 |
|---|---|---|---|
| AC-4 | TestMessageMultimodalMarshal（数组格式/嵌套 image_url/往返还原/纯文本不变） | 单测 | 序列化 |
| AC-1/3 | TestAttachmentTextFlow（相对路径添加、chip、发送注入、轨迹、清空、不存在/超限） | app 测试（mock SSE） | 文本流 |
| AC-2 | TestAttachmentImageFlow（png → content 数组 + data URL 进 history） | app 测试 | 图片流 |
| AC-1 | TestComposerAttachUI（Popover 添加 → chip → 移除） | ui.Tester | UI |
| AC-5 | 全量 | `go test ./...` | 无回归 |

## 8. 风险与回退

- **风险**：纯文本模型收图片 part → API 报错（可见失败，REQ §7 声明）。
- **回退**：不添加附件行为与现状一致；Parts 零影响由 TestMessageMultimodalMarshal 锁定。

## 9. 变更记录

- 2026-10-30 定稿；实现与设计一致，无偏离（＋ 按钮原为"/"快捷键入口，其作用由
  旁边常驻的 "/help 命令" 文本承接）。
