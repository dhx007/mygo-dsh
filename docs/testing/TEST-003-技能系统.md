# TEST-003：技能系统 测试报告

- 状态：通过
- 关联：REQ-003 · DESIGN-003
- 执行日期 / 执行环境：2026-10-30 · macOS 24.1.0 arm64 · Go（go.mod toolchain）

## 1. 自动化用例执行记录

全量回归（`go test ./...`，原样输出）：

```
ok  	mygo-harness	2.334s
ok  	mygo-harness/internal/agent	4.399s
ok  	mygo-harness/internal/llm	(cached)
ok  	mygo-harness/internal/tools	(cached)
```

新增/受影响用例（verbose，原样输出）：

```
--- PASS: TestSettingsSkillsSection (0.03s)
--- PASS: TestDiscoverSkills (0.00s)
--- PASS: TestSkillControlActions (0.00s)
--- PASS: TestSkillToolPresetWiring (0.00s)
ok  	mygo-harness	2.279s
--- PASS: TestSkillSpecMounting (0.00s)
ok  	mygo-harness/internal/agent	4.411s
```

另：`go vet ./...` 无告警；`go build -trimpath` 构建成功。

## 2. 用例 ↔ 验收标准对照

| FR | 验收标准 | 用例 | 结果 |
|---|---|---|---|
| FR-1/2 | AC-1 双目录发现、工作区覆盖、name 回退、无 SKILL.md 目录跳过 | TestDiscoverSkills | ✅ |
| FR-3 | AC-2 list / run / 未知技能 / 非法 action | TestSkillControlActions | ✅ |
| FR-4 | AC-3 标准与创造挂载；PTC/极简不挂；停用「技能」插件即失 | TestSkillToolPresetWiring | ✅ |
| FR-4 | — spec 挂载条件（SkillTool + PluginEnabled） | TestSkillSpecMounting（agent 包） | ✅ |
| FR-5 | AC-4 设置·技能分区（技能行/来源徽标/空态指引） | TestSettingsSkillsSection | ✅ |
| FR-6 | AC-5 注册表 13 项一致性 | TestBuiltinPluginRegistry（更新至 13） | ✅ |
| AC-6 | 全量无回归 | `go test ./...` 四包全 ok | ✅ |

## 3. 手工 / 活体核对

**已完成（2026-10-30）**：详见 `docs/evidence/LIVE-UI-2026-10-30.md`。结论：✅ Standard
预设同时启用 skill-filesystem + tool-skill，mygo 的挂载决策与原版一致；❌ DSH 0.2 设置
导航没有「技能」分区，mygo 的技能分区为超出原版的自有扩展（保留并标注差异）；技能徽标
是独立实验插件 "DSH badge skill"。SKILL.md 格式与目录位置属后端行为，维持合理假设声明。

## 4. 已知问题与后续项

- 技能正文直接以工具结果进入对话（无独立展示卡片/徽标）→ 徽标 UI 列入后续 UI REQ。
- 恶意 SKILL.md 提示注入的防护与 bash 一致（变更类动作仍走审批门）；只应安装可信来源技能。
- 每 Run 现扫技能目录（通常 <50 项，开销可忽略）；若未来技能量级变大再加缓存。
- JS 插件运行时 → REQ-004；MCP → REQ-005；插件市场/在线安装 → REQ-006。

## 5. 结论

5/5 新用例通过，AC-1~AC-6 全部满足，无回归。FEATURE-MATRIX：`dsh-skill`（工具+文件系统）
置 ✅、`client-ui-skill` 置 🟡（分区列表已有，徽标缺）。README「已知差距」技能条目消除。
建议 REQ-003 置「已验证」（活体核对为追加项）。
