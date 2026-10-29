# REQ-NNN：<特性名>

- 状态：草稿 | 评审通过 | 已实现 | 已验证
- 关联：DESIGN-NNN · TEST-NNN · FEATURE-MATRIX 行 `<功能域>/<包>`
- 日期：YYYY-MM-DD
- 提出人 / 负责人：

## 1. 背景与证据

DSH 里的对应功能是什么、长什么样、为什么值得复刻。**每条结论都要带证据引用**：

> 证据：`.dsh-ref/dsh-package.json` 含 `@deepseek-ai/dsh-tool-bash-persistent`；
> `.dsh-ref/webapp-readme-zh.md` §配置「`minimal` agent preset 使用持久 Shell…」；
> 活体 UI（127.0.0.1:3080）截图 `docs/evidence/REQ-NNN-*.png`（如有）。

## 2. 用户故事

作为 <谁>，我希望 <能力>，以便 <价值>。

## 3. 功能需求（FR）

每条必须**可独立验收**，编号 FR-1、FR-2…：

- **FR-1** <一句话需求>。
- **FR-2** …

## 4. 非功能需求（NFR）

性能、安全（审批门、沙箱）、兼容（macOS 优先）、资源（进程泄漏）等约束。

## 5. 验收标准

与 FR 一一对应，写成「给定…当…则…」或检查清单；测试阶段逐条打勾。

## 6. 范围外

明确不做什么、留到哪个 REQ。防止范围蔓延，也给后续需求留锚点。

## 7. 决策记录

评审中的关键取舍（如：作用域选会话级而非进程级，理由…），一句话一条。
