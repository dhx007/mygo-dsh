# DESIGN-014：LLM 自动会话标题 设计

- 状态：已落地
- 关联：REQ-014 · TEST-014
- 日期：2026-10-30

## 1. 选定方案

```go
internal/llm/client.go: Complete(ctx, model, system, user, maxTokens)
    // 非流式 POST /chat/completions，choices[0].message.content
app.go: autoTitleAsync(first)
    // send() 的 Retitle 兜底分支触发；goroutine + Complete → 清洗
    // （去引号/空白/截断 24 rune）→ onMain 覆盖 ses.Meta.Title + Touch（落盘）
    // + refreshSessions；API Key 未配置直接跳过
```

## 2. 方案对比

| 方案 | 思路 | 结论 |
|---|---|---|
| A：首条消息非流式补全（first-prompt-llm） | 一次请求、异步覆盖兜底 | ✅ 采用 |
| B：随每条消息刷新（Conversation-following） | 标题始终新鲜 | ❌ 原版也归实验插件，每条消息一次请求成本高，留后续 |

## 3. 改动文件清单

| 文件 | 改动 | 对应 FR |
|---|---|---|
| `internal/llm/client.go` | Complete | FR-2 |
| `app.go` | autoTitleAsync + send() 钩子 | FR-1/3/4 |
| `complete_test.go`、`app_test.go` | 用例 | AC-1/2 |

## 4. 与原版差异声明

仅 first-prompt 策略；无重试/退避；标题语言固定中文指令（原版策略细节未公开）。

## 5. 测试计划

AC-1→TestCompleteParsing（mock 非流式 + 401 + 断言请求非 stream）；AC-2→
TestAutoSessionTitle（mockChatAndTitle 双模服务器：stream→SSE 回复、非 stream→
标题 JSON；断言覆盖兜底、meta.json 落盘、后续消息不重命名）。结果见 TEST-014。

## 6. 风险与回退

- 标题请求与主对话并发 → 独立请求无共享状态；失败静默回退兜底标题，无行为风险。
- 回退：移除 send() 钩子即回到纯兜底标题。

## 7. 变更记录

- 2026-10-30 定稿并落地，无偏离。
