# 特性交付流程（需求 → 设计 → 开发 → 测试 → 验收）

> 目标：对齐 DeepSeek Harness（DSH）的全部功能。每加一个功能都留下可追溯的
> 需求、设计、测试产物；将来对照原版逐项验收时，证据链完整。

## 0. 需求来源与取证方式

写需求文档必须给出**证据引用**（来源 + 具体位置），不允许凭记忆描述 DSH 行为：

| 来源 | 用法 |
|---|---|
| `.dsh-ref/dsh-package.json` | DSH 桌面版全部子包清单（每个包 ≈ 一个功能子系统），功能存在性与命名以它为准 |
| `.dsh-ref/webapp-readme-zh.md` / `-en.md` | Web 表层行为说明（预设、压缩、调度、监听器等） |
| `.dsh-ref/gui-index.js` / `.css` / `theme-client.js` | 已构建前端（英文文案、样式 token；中文文案走 locale，未随包发布） |
| **活体 UI** `http://127.0.0.1:3080` | 本机运行的 `dsh --profile web` 完整界面。**需要认证**：直连返回 401，必须用 `dsh web` 启动时打印的带 token URL 打开。核对界面时：由人工提供带 token 的 URL 供浏览器自动化检查，或贴截图存入 `docs/evidence/` |
| DSH 官方仓库/文档（如有） | 语义与协议细节 |

## 1. 流程总览

```
[选需求]──►[REQ 需求文档]──►[DESIGN 设计文档]──►[开发实现]──►[测试 + TEST 报告]──►[回写矩阵/README]──►[验收归档]
  FEATURE-     docs/req...      docs/design...     代码+commit    go test + docs/testing...    FEATURE-MATRIX
  MATRIX 领号
```

每个箭头都是**门禁**：上一阶段产物未完成，不进入下一阶段。设计定稿前不写业务代码。

## 2. 各阶段定义

### 阶段 1 · 需求（REQ）

- 产物：`docs/requirements/REQ-NNN-<slug>.md`，按 `templates/REQ-模板.md`。
- 必须包含：背景与证据引用、用户故事、功能需求（FR-1、FR-2…每条可验收）、
  非功能需求、验收标准、范围外（明确不做什么）。
- 完成标志（DoD）：每条 FR 都能对应到验收标准；证据引用可复查；范围外不空着。

### 阶段 2 · 设计（DESIGN）

- 产物：`docs/design/DESIGN-NNN-<slug>.md`，按 `templates/DESIGN-模板.md`。
- 必须包含：现状分析（引用现有代码 `文件:行号`）、方案对比（至少两案，给出取舍理由）、
  数据结构与接口、改动文件清单、与 DSH 原版的差异声明、风险、**测试计划**（对应每条 FR）。
- 完成标志（DoD）：改动文件清单完整（含测试文件）；测试计划覆盖全部 FR。

### 阶段 3 · 开发

- 严格按 DESIGN 的文件清单实施；清单外的新增改动要**回写**设计文档（变更记录节）。
- commit 规范：`REQ-NNN <一句话>`，一次特性一个或少量 commit。
- 代码风格与现有一致：注释解释约束，不叙述过程。

### 阶段 4 · 测试（TEST）

- 先补自动化用例（`go test ./...` 必须全绿；UI 路径用 `ui.Tester` 无头测试）。
- 产物：`docs/testing/TEST-NNN-<slug>.md`，按 `templates/TEST-模板.md`，粘贴**真实执行输出**。
- 覆盖要求：每条 FR 至少一个用例；边界与失败路径（超时、死亡重启、拒绝审批）必须有用例。

### 阶段 5 · 验收与回写

- FEATURE-MATRIX：该项状态改 ✅/🟡，登记 REQ/DESIGN/TEST 编号。
- README「与初代对齐的功能」表与「已知差距」同步增删。
- 对照活体 UI（3080）做一次人工/截图核对，结论记入 TEST 报告的「验收」节。

## 3. 编号与状态

- 编号：`REQ/DESIGN/TEST-NNN`，同一特性共用 NNN；`docs/README.md` 的水位表管理下一个号。
- slug：短横线小写或中文短语，与特性名一致。
- 文档头部状态字段：
  - REQ：`草稿 → 评审通过 → 已实现 → 已验证`
  - DESIGN：`草稿 → 定稿 → 已落地`（实现中偏离要记变更记录）
  - TEST：`未执行 → 通过 / 失败（附阻断项）`

## 4. 优先级口径（供 FEATURE-MATRIX 排期）

- **P0**：插件体系本体（cordis 插件管理器/插件市场/内置插件清单）、MCP 客户端、技能（skills）
  ——用户点名的「插件上的功能」。
- **P1**：日常体验缺口感：持久 bash、终端面板、工作流编排、计划模式、附件、快捷键、
  自动标题、凭据安全存储、权限预设、目录选择器。
- **P2**：锦上添花与平台层：语音、转 PDF、遥测、多代理团队、worktree、headless 等。
- **⚪ 不复刻**：DSH 框架内部件（存储域、投影缓存、otel、typert、Web 表层传输栈等），
  桌面原生版有等价实现即可，不逐包对齐。

## 5. 当前快照

- 基线：初代复刻（README「与初代对齐的功能」全绿），差距见 README「已知差距」与 FEATURE-MATRIX。
- 已完成：REQ-001 持久 bash 会话；REQ-002 插件体系骨架（注册表/plugins 工具）；
  REQ-003 技能系统（SKILL.md/skill 工具/设置·技能分区）；REQ-004 插件页三段式修订 +
  本地插件包运行时（plugin.json 包模型/goja 沙箱/Add plugin/卸载/持久终端实验开关）；
  REQ-005 MCP 客户端（internal/mcp stdio JSONRPC、mcp.json 配置、插件页 MCP 段、
  工具经包工具通道挂载并默认走审批门）；REQ-006 插件市场（实验开关「自定义插件市场」
  门控、markets.json 索引、浏览/一键安装）——**P0 插件集群全部交付**；
  REQ-007 Shell 输出上限 64KB + 落盘 spill + 可配置超时（Shell 设置卡对齐原版 View Shell）；
  REQ-008 工作区指令（AGENTS.md→CLAUDE.md 发现并注入系统提示词，注册表第 15 项门控）。
- 活体 UI 核对已完成一轮（`docs/evidence/LIVE-UI-2026-10-30.md`），结构类假设已对齐。
- 下一批（待立 REQ，按 FEATURE-MATRIX P1 排）：agent-instructions（AGENTS.md/CLAUDE.md）、
  tool-jobs、会话内模型切换、附件、
  快捷键、权限预设、目录选择器、自动标题；候选：time-context、tool-ask-user、
  侧栏用量卡、Plugin manager 更新检查、MCP resources/SSE、工作流编排、终端面板。
