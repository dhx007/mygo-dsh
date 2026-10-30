# TEST-013：权限预设 测试报告

- 状态：通过
- 关联：REQ-013 · DESIGN-013
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
--- PASS: TestAccessModeMigration (0.00s)
--- PASS: TestDecideApproval (0.00s)
--- PASS: TestAccessModeUI (0.01s)
```

另：`go vet ./...` 无告警；`go build -trimpath` 构建成功。

## 2. 用例 ↔ 验收标准对照

| FR | 验收标准 | 用例 | 结果 |
|---|---|---|---|
| FR-3 | AC-1 旧 `auto_approve:true` → 完全信任；未知值归一化默认 | TestAccessModeMigration | ✅ |
| FR-1/4 | AC-2 三档决策：只读不询问直接 Deny（无 ask 调用）、完全信任不询问放行、工作区写入走 ask 且 AllowAlways 记忆 + approval 轨迹留痕 | TestDecideApproval | ✅ |
| FR-2 | AC-3 composer 显示当前模式 → Popover 三选一（含描述）→ 切「只读」持久化 config.json | TestAccessModeUI | ✅ |
| AC-4 | 全量无回归（设置卡联动在同一渲染同步路径，TestAccessModeUI 覆盖 popover 侧） | `go test ./...` 五包全 ok | ✅ |

## 3. 手工 / 活体核对

模式语义对齐活体证据的 "Access mode, current: Workspace Write" 控件位置与命名
概念；三档的精确名称/描述为中文自定（原版菜单内容未逐项核对）。沙箱级「工作区
写入」路径限制不在范围（REQ §6，台账单列 dsh-sandbox）。

## 4. 已知问题与后续项

- 只读档模型仍会看到变更类工具并收到拒绝文案（工具不隐藏）——与原版「权限模式」
  的隐藏策略可能不同，留活体核对。
- 「总是允许」记忆跨模式共享（只读档仍拒绝，不受记忆影响）。

## 5. 结论

3/3 新用例通过，AC-1~AC-4 全部满足，无回归。FEATURE-MATRIX：
`dsh-permission-presets` ❌→🟡（三档预设全链路；按路径沙箱与工具隐藏缺）。
建议 REQ-013 置「已验证」。
