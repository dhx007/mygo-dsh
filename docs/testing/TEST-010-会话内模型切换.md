# TEST-010：会话内模型切换 测试报告

- 状态：通过
- 关联：REQ-010 · DESIGN-010
- 执行日期 / 执行环境：2026-10-30 · macOS 24.1.0 arm64 · Go（标准库）

## 1. 自动化用例执行记录

全量回归（`go test ./...`，原样输出）：

```
ok  	mygo-harness	2.689s
ok  	mygo-harness/internal/agent	(cached)
ok  	mygo-harness/internal/llm	(cached)
ok  	mygo-harness/internal/mcp	(cached)
ok  	mygo-harness/internal/tools	(cached)
```

新增用例（verbose，原样输出）：

```
--- PASS: TestSessionModelSwitch (0.01s)
--- PASS: TestComposerModelMenu (0.04s)
--- PASS: TestListModels (0.00s)
```

另：`go vet ./...` 无告警；`go build -trimpath` 构建成功。

## 2. 用例 ↔ 验收标准对照

| FR | 验收标准 | 用例 | 结果 |
|---|---|---|---|
| FR-2 | AC-1 /models 返回排序 id（空 id 过滤）；401 报错带状态码 | TestListModels | ✅ |
| FR-1/5 | AC-2 覆盖生效（effectiveModel/agent.Model）；meta.json 落盘；切换丢 agent；/model 命令与无参用法提示；无会话报错 | TestSessionModelSwitch | ✅ |
| FR-3 | AC-4 composer 控件显示生效模型；Popover 清单打开（预置选项免网络）；点选切换写入会话 | TestComposerModelMenu | ✅ |
| FR-2 | 回退清单 + 当前模型并入（ensureModels 逻辑，headless 内联回调） | 代码审查 + TestComposerModelMenu（modelFetched 短路） | ✅ |
| AC-5 | 全量无回归 | `go test ./...` 五包全 ok | ✅ |

## 3. 手工 / 活体核对

控件形态对齐活体证据（`docs/evidence/LIVE-UI-2026-10-30.md` §4）：composer 内
"Select model, current <模型>" → 图标+模型名+下箭头，点击弹清单。清单来源对齐
DSH 启动日志的 /models 拉取（"model listing from https://api.deepseek.com/models
loaded 2 model(s)"）。原版控件视觉细节未逐像素核对。

## 4. 已知问题与后续项

- /models 拉取失败静默回退内置清单（deepseek-chat/deepseek-reasoner）+ 当前模型，
  不打扰用户；如需错误可见可后续加提示。
- 模型清单每次应用启动后首次打开选择器时拉取一次并缓存（不随会话刷新）。
- 会话内修改模型后，进行中的一轮不变（下一轮生效），与插件切换语义一致。

## 5. 结论

3/3 新用例通过，AC-1~AC-5 全部满足，无回归。FEATURE-MATRIX：
`client-ui-model-selection` 🟡→✅。建议 REQ-010 置「已验证」。
