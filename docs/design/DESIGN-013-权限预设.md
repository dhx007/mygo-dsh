# DESIGN-013：权限预设 设计

- 状态：已落地
- 关联：REQ-013 · TEST-013
- 日期：2026-10-30

## 1. 现状分析

OnApproval 闭包内联在 send()：`AutoApprove` 直接放行 → `alwaysAllow` 记忆 →
pending 审批卡。config 只有 AutoApprove 布尔，无预设概念。

## 2. 方案对比

| 方案 | 思路 | 优点 | 缺点 | 结论 |
|---|---|---|---|---|
| A：三档预设 + decideApproval 抽取 | 模式进 config；决策逻辑成方法可直测 | 语义清晰；测试无需跑完整 send | — | ✅ 采用 |
| B：保留布尔再加只读开关 | 两个独立开关 | 改动最小 | 4 种组合语义混乱 | ❌ |

## 3. 选定方案

```go
config.go:  AccessMode string + accessMode() 归一化（未知→工作区写入）+ loadConfig 迁移
            （auto_approve:true → 完全信任）；configPath 尊重 dataDirOverride（测试缝）
app.go:     decideApproval(ses, tool, args, ask)：只读→Deny（留痕 denied-readonly）；
            完全信任→AllowOnce；工作区写入→alwaysAllow / ask() 弹卡，AllowAlways 记忆
composer:   盾牌 icoShield + 模式名 anchor → accessPopover 三选一（● 当前 + 一句描述）
设置卡:     复选框 ↔ 完全信任 双向联动
```

## 4. 改动文件清单

| 文件 | 改动 | 对应 FR |
|---|---|---|
| `config.go` | AccessMode + 归一化 + 迁移 + configPath 测试缝 | FR-3 |
| `app.go` | decideApproval / setAccessMode / accessPopover / composer 控件 / 设置卡联动 | FR-1/2/4 |
| `permission_test.go` | 迁移/三档决策/UI 用例 | AC-1~3 |

## 5. 与原版差异声明

「工作区写入」未实现路径级沙箱（仅审批门语义），台账单列 dsh-sandbox 差距。

## 6. 测试计划

AC-1→TestAccessModeMigration；AC-2→TestDecideApproval（含轨迹留痕与 AllowAlways）；
AC-3→TestAccessModeUI；AC-4→全量。结果见 TEST-013。

## 7. 风险与回退

- 只读档模型可能反复尝试被拒 → 拒绝文案已声明尊重决定；回退：切回工作区写入。

## 8. 变更记录

- 2026-10-30 定稿并落地，无偏离。
