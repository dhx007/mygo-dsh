# DESIGN-015：侧栏用量卡 设计

- 状态：已落地
- 关联：REQ-015 · TEST-015
- 日期：2026-10-30

## 1. 选定方案

```go
internal/llm/client.go:
  Usage{PromptTokens,CompletionTokens,TotalTokens}
  Request += StreamOptions（流式时 include_usage）
  streamEvent += Usage；Chat 收尾取最后一个 usage → c.OnUsage
  Complete 解析响应 usage → c.OnUsage
  Client += OnUsage func(Usage)

usage.go（main 包）:
  usageDay{Calls,PromptTokens,CompletionTokens}; usageFile{Days map}
  loadUsage/saveUsage（dataDir/usage.json）; todayKey()
app:
  usageAll map[string]*usageDay（newApp 加载）; usageUpdated time.Time
  recordUsage(u)：onMain → 当日累计 → 落盘
  fmtTokens(n)：人性化
  newAgentFor：base.Client.OnUsage = recordUsage（子代理共享 Client 指针，同样计入）
侧栏 footer：
  Today 行（row.Clicked 折叠/展开，cfg.UsageCollapsed 持久化）
  展开时：stats 行（tokens · calls，点击 → setSection=使用统计 + fetchStats +
  openSettings）、cost 行（DeepSeek ¥x.xx · Updated HH:MM）
openSettings：不再强制重置 setSection（newApp 初始化默认账号与余额）
```

## 2. 改动文件清单

| 文件 | 改动 | 对应 FR |
|---|---|---|
| `internal/llm/client.go` | Usage/OnUsage/StreamOptions/两处捕获 | FR-1 |
| `usage.go` | 新增：按天统计与持久化、fmtTokens | FR-2/4 |
| `app.go` | recordUsage 接线、侧栏卡片、openSettings 调整、config.UsageCollapsed | FR-3 |
| `internal/llm/*_test.go`、`usage_test.go`、`app_test.go` | 用例 | AC-1~4 |

## 3. 与原版差异声明

- 价格常量假设（¥2/M 入、¥8/M 出，deepseek-chat 牌价）且不按模型/缓存区分；
- 折叠交互为 Today 行点击（原版独立折叠钮）；点击统计行进入 mygo 的设置·使用统计
  （原版跳独立统计页）。

## 4. 测试计划

AC-1→TestUsageCaptureStreaming / TestUsageCaptureComplete；AC-2→TestRecordUsage；
AC-3→TestUsageCardUI；AC-4→全量。结果见 TEST-015。

## 5. 风险与回退

- 流式尾块无 usage（提供方不支持）→ 当次不计，不影响其他；回退：移除 OnUsage
  接线即停止统计。

## 6. 变更记录

- 2026-10-30 定稿并落地，无偏离。
