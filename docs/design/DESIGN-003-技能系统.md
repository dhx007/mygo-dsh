# DESIGN-003：技能系统 设计

- 状态：已落地
- 关联：REQ-003 · TEST-003
- 日期：2026-10-30

## 1. 现状分析

- 无任何技能概念。REQ-002 已建立可复用的模式：agent 回调工具（PluginTool）+ 动态开关
  （PluginEnabled）+ 注册表（builtinPlugins）+ 设置分区渲染。技能照此模式接入即可，
  新增逻辑集中在「SKILL.md 发现与解析」。

## 2. 方案对比

| 方案 | 思路 | 优点 | 缺点 | 结论 |
|---|---|---|---|---|
| A：文件系统技能包（每技能一个目录 + SKILL.md） | 扫描全局/工作区两处 skills 目录，frontmatter+正文 | 与 DSH `skill-filesystem` 语义一致；用户手写即可装；无安装器 | 路径为假设（§6） | ✅ 采用 |
| B：单文件 skills.json 集中登记 | 一个 JSON 列全部技能 | 解析简单 | 与 DSH 技能包形态不符；无正文分离；不可单技能分享 | ❌ |
| C：技能=普通插件（进注册表列表） | 每个技能渲染进插件页 | 「一切皆插件」字面一致 | 动态内容混进静态注册表，UI 语义混乱 | ❌（折中：注册表收一个「技能」开关插件） |

## 3. 选定方案（A）

```
skills.go（main 包）
  Skill{ID, Name, Description, Body, Source("全局"|"工作区"), Path}
  discoverSkills(dataDir, workspace) []Skill
      全局:  <dataDir>/skills/*/SKILL.md
      工作区: <workspace>/.harness/skills/*/SKILL.md   （同名覆盖全局）
  parseSkillMD(dir) (Skill, error)   // frontmatter name/description + 正文（64KB 上限）

agent.Agent
  SkillTool func(action, name string) (string, bool)  // list | run
  skillSpec{name: string, action: "list"|"run"}
  toolSpecs(): a.SkillTool != nil && PluginEnabled("skill") 时追加（depth 0）

app
  skillControl(action, name) → 读 discoverSkills 缓存（每条消息现扫，两目录通常 <50 项）
  secSkills(c) → 设置·技能分区
```

## 4. 数据结构与接口

- Skill 字段见上；`ID` = frontmatter name（回退目录名）。
- frontmatter 解析：首行 `---` 起到下一个 `---` 止，逐行 `key: value`，仅识别
  name/description；无 frontmatter 视为整体正文、name=目录名。
- `skill` 工具输出：list = 每行 `- 名称（来源）— 描述`；run = `【技能：name】` + 正文；
  未知 name → `error: 未知技能`。
- 注册表新增第 13 项：`{ID:"skill", Name:"技能", Category:"系统", Tools:["skill"], Kind:"tool"}`。
- presets.go：standardTools 与 creatorTools 追加 `"skill"`。

## 5. 改动文件清单

| 文件 | 改动 | 对应 FR |
|---|---|---|
| `skills.go` | 新增：Skill、discoverSkills、parseSkillMD | FR-1/2 |
| `plugins.go` | 注册表追加「技能」插件 | FR-6 |
| `internal/agent/agent.go` | SkillTool 字段、skillSpec、execTool 分发、toolSpecs 挂载 | FR-3/4 |
| `presets.go` | standardTools/creatorTools 追加 skill | FR-4/7 |
| `app.go` | skillControl、newAgentFor 接线、settingsSections 增「技能」、secSkills | FR-3/4/5 |
| `skills_test.go` | 发现/覆盖/解析/工具行为 | AC-1/2/3 |
| `app_test.go` | 设置·技能分区 UI | AC-4 |

## 6. 与 DSH 原版的差异声明

- 技能目录（`<dataDir>/skills`、`<workspace>/.harness/skills`）与 frontmatter 字段为
  合理假设（REQ-003 §6）；发现数量为 0 时无任何行为变化。
- 技能徽标（badge）UI 不在本轮；`skill run` 的结果直接以工具结果进入对话，无独立
  展示卡片——与现有 todo 卡片机制不同，属简化。

## 7. 测试计划

| FR/AC | 用例 | 层级 | 断言要点 |
|---|---|---|---|
| AC-1 | TestDiscoverSkills | 单测 | 全局+工作区发现、同名覆盖、缺 name 回退、坏文件跳过 |
| AC-2 | TestSkillControlActions | app 测试 | list/run/未知技能 |
| AC-3 | TestSkillToolPresetWiring | app 测试 | 标准/创造挂载、停用即失、PTC/极简无 |
| AC-4 | TestSettingsSkillsSection | ui.Tester | 技能行/来源徽标/空态指引 |
| AC-5 | TestBuiltinPluginRegistry（改 13） | 单测 | 注册表一致性 |
| AC-6 | 全量 | `go test ./...` | 无回归 |

## 8. 风险与回退

- **风险**：技能正文注入提示（恶意 SKILL.md）→ 与 bash 审批同级风险：技能只是指令文本，
  变更类动作仍走审批门；文档明示只装可信来源的技能。
- **风险**：工作区 .harness 目录扫描误伤大目录 → 只扫一层子目录的 SKILL.md，O(n) 目录数。
- **回退**：预设 Tools 去掉 `"skill"` 即整体关闭；skills.go 独立可移除。

## 9. 变更记录

- 2026-10-30 定稿；实现与设计一致，无偏离。
