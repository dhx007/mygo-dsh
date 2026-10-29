# TEST-002：插件体系骨架 测试报告

- 状态：通过
- 关联：REQ-002 · DESIGN-002
- 执行日期 / 执行环境：2026-10-30 · macOS 24.1.0 arm64 · Go（go.mod toolchain）

## 1. 自动化用例执行记录

全量回归（`go test ./...`，原样输出）：

```
ok  	mygo-harness	2.353s
ok  	mygo-harness/internal/agent	(cached)
ok  	mygo-harness/internal/llm	(cached)
ok  	mygo-harness/internal/tools	(cached)
```

新增用例（verbose，原样输出）：

```
--- PASS: TestToolSpecsFilterAndPlugins (0.00s)
--- PASS: TestPluginsToolDispatch (0.00s)
PASS
ok  	mygo-harness/internal/agent	2.324s
--- PASS: TestPersistentShellPresetWiring (0.01s)
--- PASS: TestDropAgentClosesShell (0.01s)
--- PASS: TestPluginsPageGroups (0.04s)
--- PASS: TestSettingsPluginsSectionGrouped (0.00s)
--- PASS: TestBuiltinPluginRegistry (0.00s)
--- PASS: TestPluginControlActions (0.00s)
--- PASS: TestPluginToolPresetWiring (0.00s)
PASS
ok  	mygo-harness	2.247s
```

另：`go vet ./...` 无告警；`go build -trimpath` 构建成功。

## 2. 用例 ↔ 验收标准对照

| FR | 验收标准 | 用例 | 结果 |
|---|---|---|---|
| FR-1 | AC-1 注册表 12 项、字段齐全、预设工具全覆盖、分组无损 | TestBuiltinPluginRegistry | ✅ |
| FR-5 | AC-3 list/info/enable/disable/非法输入 | TestPluginControlActions（含 config.json 落盘断言） | ✅ |
| FR-5 | — 模型调用分发与未挂载报错 | TestPluginsToolDispatch | ✅ |
| FR-6/7 | AC-4 创造挂载、停用后不挂载、标准不挂载 | TestPluginToolPresetWiring | ✅ |
| FR-6 | — 开关状态动态过滤工具集 | TestToolSpecsFilterAndPlugins | ✅ |
| FR-2/3 | AC-2 插件页分组卡片 | TestPluginsPageGroups（组头/显示名/版本徽标） | ✅ |
| FR-4 | AC-2 设置·内置插件同源渲染 | TestSettingsPluginsSectionGrouped（直接驱动生产 settingsWindowView） | ✅ |
| AC-5 | 全量无回归 | `go test ./...` 四包全 ok | ✅ |

## 3. 手工 / 活体核对

**已完成（2026-10-30）**：详见 `docs/evidence/LIVE-UI-2026-10-30.md`（含 2 张截图）。
结论：✅ 插件管理工具按预设停用（standard 下 plugin-manager/tools、plugin-market/tools
均 Disabled）与 mygo 创造模式限定一致；❌ 插件页结构假设不成立——DSH 为「内置插件
只读清单卡 + 实验插件开关 + 已装 npm/git 包」三段式（侧栏 Plugins 全局面板），无分类
分组；修订方向记录在证据文档 §5（REQ-002 修订 + REQ-004 包模型升级）。

## 4. 已知问题与后续项

- 插件开关对**当前运行中**的 agent 不即时生效，从下一条用户消息开始（DESIGN-002 §9
  变更记录中的动态过滤已覆盖「下一轮」语义；进行中的一轮不变）。
- 模型停用 `plugins` 插件自身的保护只有提示语（返回文本警告），无硬约束——与 DSH
  「用户可在 UI 恢复」一致，可接受。
- 本地/第三方插件（JS 插件运行时、安装/卸载）未含 → REQ-003；技能 → REQ-004；
  MCP → REQ-005；插件市场 → REQ-006。

## 5. 结论

8/8 新用例通过，AC-1~AC-5 全部满足，无回归。FEATURE-MATRIX：
`dsh-tool-cordis` 置 ✅，`plugin-manager` / `plugin-inventory` 保持 🟡（骨架完成，
安装/第三方待 REQ-003+）。建议 REQ-002 置「已验证」（活体核对为追加项）。
