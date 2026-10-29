# MyGo Harness 文档中心

本目录是「复刻 DeepSeek Harness」的**需求、设计与验证留痕**。每加一个功能，都按
[PROCESS.md](PROCESS.md) 的流程走完五个阶段，中间产物全部落在这里，编号可追溯。

## 目录结构

```
docs/
├── README.md            ← 本文件：导航
├── PROCESS.md           ← 流程定义：五阶段、产物、状态机、DoD
├── templates/           ← 文档模板（复制后填写）
│   ├── REQ-模板.md       需求文档模板
│   ├── DESIGN-模板.md    设计文档模板
│   └── TEST-模板.md      测试报告模板
├── inventory/
│   └── FEATURE-MATRIX.md ← 功能对照总表：DSH 全功能 ↔ mygo 现状 ↔ 优先级（需求总台账）
├── requirements/        ← REQ-NNN-<特性>.md 需求文档
├── design/              ← DESIGN-NNN-<特性>.md 设计文档
└── testing/             ← TEST-NNN-<特性>.md 测试报告（含真实执行输出）
```

## 快速上手

1. **立需求**：从 [FEATURE-MATRIX.md](inventory/FEATURE-MATRIX.md) 挑一个未复刻项，
   复制 `templates/REQ-模板.md` 到 `requirements/REQ-NNN-<特性>.md` 填写，编号顺延。
2. **做设计**：`templates/DESIGN-模板.md` → `design/DESIGN-NNN-<特性>.md`，方案定稿后才动代码。
3. **开发**：commit 信息引用编号，如 `REQ-001 持久 bash：进程组超时杀树`。
4. **测试**：按设计文档的测试计划补用例，跑 `go test ./...`，把真实输出写进
   `testing/TEST-NNN-<特性>.md`。
5. **回写**：更新 FEATURE-MATRIX 状态列与 README 对齐表，闭环。

## 当前编号水位

| 类型 | 已用编号 | 下一个 |
|---|---|---|
| REQ | 001 · 002 · 003 · 004 · 005 · 006 · 007 · 008 | **REQ-009** |
| DESIGN | 001 · 002 · 003 · 004 · 005 · 006 · 007 · 008 | **DESIGN-009** |
| TEST | 001 · 002 · 003 · 004 · 005 · 006 · 007 · 008 | **TEST-009** |

> 领取编号时先看本表，做完后把「下一个」推进一位。
