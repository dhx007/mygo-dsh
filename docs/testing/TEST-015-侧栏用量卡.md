# TEST-015：侧栏用量卡 测试报告

- 状态：通过
- 关联：REQ-015 · DESIGN-015
- 执行日期 / 执行环境：2026-10-30 · macOS 24.1.0 arm64 · Go（标准库）

## 1. 自动化用例执行记录

全量回归（`go test ./...`，原样输出）：

```
ok  	mygo-harness	2.855s
ok  	mygo-harness/internal/agent	(cached)
ok  	mygo-harness/internal/llm	(cached)
ok  	mygo-harness/internal/mcp	(cached)
ok  	mygo-harness/internal/tools	(cached)
```

新增用例（verbose，原样输出）：

```
--- PASS: TestUsageCaptureStreaming (0.00s)
--- PASS: TestUsageCaptureComplete (0.00s)
--- PASS: TestRecordUsage (0.00s)
--- PASS: TestUsageCardUI (0.05s)
```

另：`go vet ./...` 无告警；`go build -trimpath` 构建成功。

## 2. 用例 ↔ 验收标准对照

| FR | 验收标准 | 用例 | 结果 |
|---|---|---|---|
| FR-1 | AC-1 流式：请求带 `stream_options.include_usage`（mock 断言）、尾块 usage（空 choices）捕获、OnUsage 恰一次回调 | TestUsageCaptureStreaming | ✅ |
| FR-1 | AC-1 非流式 Complete 同样上报 usage | TestUsageCaptureComplete | ✅ |
| FR-2/4 | AC-2 两次 recordUsage → calls=2、tokens 累加（1100/550）、usage.json 落盘含当日键；费用估算 >0；fmtTokens 三档（1.6M/1.5K/876） | TestRecordUsage | ✅ |
| FR-3 | AC-3 卡片显示 Today / `1.6K tokens · 2 calls` / `DeepSeek ¥0.01（估算）`；点统计行 → 打开设置且 setSection=使用统计；Today 行折叠/展开且 `usage_collapsed` 落盘 | TestUsageCardUI | ✅ |
| AC-4 | 全量无回归 | `go test ./...` 五包全 ok | ✅ |

## 3. 手工 / 活体核对

布局与字段对齐活体证据（`docs/evidence/LIVE-UI-2026-10-30.md` §4）：Today /
tokens / calls / DeepSeek ¥ / Updated；折叠交互简化为 Today 行点击（DESIGN §3
声明）；费用为估算（牌价常量假设 ¥2/M 入、¥8/M 出，REQ §7）。真实计费以
platform.deepseek.com 账单为准——卡片文案已标注「估算」。

## 4. 已知问题与后续项

- 单一价格对，不区分 deepseek-reasoner 牌价与缓存命中计价（REQ §6）。
- usage.json 明文累计，无按小时/按模型明细；历史图表未做（设置·使用统计的
  会话/事件计数保持原状）。
- 流式提供方不支持尾块 usage 时当次不计（OpenAI 兼容端点均支持 include_usage）。

## 5. 结论

4/4 新用例通过，AC-1~AC-4 全部满足，无回归。FEATURE-MATRIX：`dsh-token-meter`
❌→🟡（当日捕获/累计/展示全链路；按模型计价与历史图表缺）。建议 REQ-015 置「已验证」。
