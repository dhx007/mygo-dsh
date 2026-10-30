# TEST-014：LLM 自动会话标题 测试报告

- 状态：通过
- 关联：REQ-014 · DESIGN-014
- 执行日期 / 执行环境：2026-10-30 · macOS 24.1.0 arm64 · Go（标准库）

## 1. 自动化用例执行记录

全量回归（`go test ./...`，原样输出）：

```
ok  	mygo-harness	2.835s
ok  	mygo-harness/internal/agent	6.850s
ok  	mygo-harness/internal/llm	4.409s
ok  	mygo-harness/internal/mcp	(cached)
ok  	mygo-harness/internal/tools	(cached)
```

新增用例（verbose，原样输出）：

```
--- PASS: TestCompleteParsing (0.00s)
--- PASS: TestAutoSessionTitle (0.07s)
```

另：`go vet ./...` 无告警；`go build -trimpath` 构建成功。

## 2. 用例 ↔ 验收标准对照

| FR | 验收标准 | 用例 | 结果 |
|---|---|---|---|
| FR-2 | AC-1 Complete 解析非流式补全；401 报错；请求体非 stream（mock 断言） | TestCompleteParsing | ✅ |
| FR-1/3 | AC-2 首条消息 → 标题异步覆盖兜底（「登录功能实现」）并落盘 meta.json；第二条消息不重命名 | TestAutoSessionTitle（mockChatAndTitle 双模服务器） | ✅ |
| FR-4 | Key 未配置跳过（autoTitleAsync 前置检查）；请求不带历史（Complete 只发 system+user） | 代码审查 + TestCompleteParsing | ✅ |
| — | 全量无回归（既有 mockDeepSeek 为纯 SSE 服务器，标题请求失败静默回退兜底——回退路径被既有测试隐式覆盖） | `go test ./...` 五包全 ok | ✅ |

## 3. 手工 / 活体核对

first-prompt-llm 策略对齐 DSH 随发行版交付的 `session-title-first-prompt-llm`；
system 指令（≤12 字中文标题）为本实现自定（原版提示词未公开）。跟随式刷新
（Conversation-following titles）明确不做（原版亦为实验插件）。

## 4. 已知问题与后续项

- 标题质量依赖模型；生成失败静默保留首行截断兜底。
- 会话切换后后台标题仍写入原会话（按 ses 引用，行为正确）；删除会话后写 meta
  静默失败（目录已删）。

## 5. 结论

2/2 新用例通过，AC-1~AC-2 全部满足，无回归。FEATURE-MATRIX：
`dsh-session-title` ❌→🟡（first-prompt-llm 策略就位；跟随式刷新与 all-prompts 策略缺）。
建议 REQ-014 置「已验证」。
