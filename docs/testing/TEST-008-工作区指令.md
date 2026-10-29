# TEST-008：工作区指令（agent-instructions） 测试报告

- 状态：通过
- 关联：REQ-008 · DESIGN-008
- 执行日期 / 执行环境：2026-10-30 · macOS 24.1.0 arm64 · Go（标准库）

## 1. 自动化用例执行记录

全量回归（`go test ./...`，原样输出）：

```
ok  	mygo-harness	2.740s
ok  	mygo-harness/internal/agent	(cached)
ok  	mygo-harness/internal/llm	(cached)
ok  	mygo-harness/internal/mcp	(cached)
ok  	mygo-harness/internal/tools	(cached)
```

新增/受影响用例（verbose，原样输出）：

```
--- PASS: TestLoadWorkspaceInstructions (0.00s)
--- PASS: TestAgentInstructionsWiring (0.01s)
--- PASS: TestBuiltinPluginRegistry (0.00s)   ← 15 项（新增 工作区指令）
--- PASS: TestPluginControlActions (0.01s)
```

另：`go vet ./...` 无告警；`go build -trimpath` 构建成功。

## 2. 用例 ↔ 验收标准对照

| FR | 验收标准 | 用例 | 结果 |
|---|---|---|---|
| FR-1 | AC-1 AGENTS.md 优先；仅 CLAUDE.md 回退；AGENTS.md 空文件回退；都无不注入；空工作区安全 | TestLoadWorkspaceInstructions | ✅ |
| FR-2 | AC-2 >32KB 截断并注明（总长受控） | TestLoadWorkspaceInstructions | ✅ |
| FR-3 | AC-3 注入系统提示词末尾（`[工作区指令：AGENTS.md]` + 正文）；停用「工作区指令」即失；无文件不注入 | TestAgentInstructionsWiring | ✅ |
| FR-3 | AC-4 注册表 15 项（context 类允许无工具）+ plugins list 计数 | TestBuiltinPluginRegistry · TestPluginControlActions | ✅ |
| FR-4 | 生效范围：注入在 newAgentFor，子代理经 base 继承（代码路径同一 prompt，无新逻辑） | 代码审查 | ✅ |
| AC-5 | 全量无回归（UI 计数断言同步 15） | `go test ./...` 五包全 ok | ✅ |

## 3. 手工 / 活体核对

行为语义取自活体核对（`docs/evidence/LIVE-UI-2026-10-30.md` §2.2）：standard 预设
agent-instructions Enabled，描述 "Workspace context loader for AGENTS.md/CLAUDE.md
instruction files"。优先级（AGENTS.md 先）与 32KB 上限为已声明假设（REQ §7）；
文件发现属后端行为，活体 UI 无对应界面可对照。

## 4. 已知问题与后续项

- 子目录嵌套 AGENTS.md（就近发现）不做（REQ §6）；全局用户级指令（~/.mygo/）候选后续。
- 会话内修改指令文件：下一次构建 agent（新消息于 a.ag 为空时，或切换会话/模式）时生效；
  同一 agent 连续对话不重读（与系统提示词的生命周期一致）。

## 5. 结论

4/4 用例通过，AC-1~AC-5 全部满足，无回归。FEATURE-MATRIX：`dsh-agent-instructions`
❌→✅。建议 REQ-008 置「已验证」。
