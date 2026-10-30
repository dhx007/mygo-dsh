# DESIGN-012：快捷键体系 设计

- 状态：已落地
- 关联：REQ-012 · TEST-012
- 日期：2026-10-30

## 1. 现状分析

main.go 只建窗口；无任何菜单/快捷键。MyGo 提供 `NewMenu`（MenuItem.Accelerator +
Click）与 `mygo.App.SetMenu`（macOS 菜单栏），Click 在主线程回调。

## 2. 方案对比

| 方案 | 思路 | 优点 | 缺点 | 结论 |
|---|---|---|---|---|
| A：应用菜单栏加速器 | SetMenu + Accelerator + Click→现有动作 | 原生惯例；编辑角色白送；Click 可直测 | 仅应用聚焦时生效 | ✅ 采用 |
| B：GlobalShortcut.Register | 系统级热键 | 全局可用 | 劫持系统快捷键，与原版「应用内快捷键」语义不符 | ❌ |
| C：窗口 KeyDown 分发 | ui 层按键钩子 | 灵活 | 需逐控件处理焦点冲突，MyGo 无窗口级 KeyDown API | ❌ |

## 3. 选定方案（A）

```go
// shortcuts.go（新增）
func buildAppMenu(a *app) *mygo.Menu {
    App 角色 | 文件：新会话 ⌥⌘N（ID menu-new-session）→ a.newSession；RoleClose
    编辑角色 | 显示：对话 ⌘1 / 轨迹 ⌘2 / 插件 ⌘3（menu-chat/trajectory/plugins）→
              a.selectedID = "page:<x>"；设置 ⌥⌘,（menu-settings）→ a.openSettings
}
// main.go：WhenReady 内窗口创建后 mygo.App.SetMenu(buildAppMenu(a))
```

导航动作包 `a.onMain(...)` 冗余（Click 已在主线程），直接调用；点击后让输入框
保持焦点的细节交给现有 focusComposer 机制不动。

## 4. 数据结构

无新存储。

## 5. 改动文件清单

| 文件 | 改动 | 对应 FR |
|---|---|---|
| `shortcuts.go` | 新增：buildAppMenu | FR-1~3 |
| `main.go` | WhenReady 内 SetMenu | FR-1/4 |
| `shortcuts_test.go` | 菜单构建与动作触发 | AC-1/2 |

## 6. 与 DSH 原版的差异声明

- ⌘1/2/3 导航为本实现自定；其余加速器对齐活体证据。菜单栏文案中文（原版英文）。

## 7. 测试计划

| AC | 用例 | 层级 | 断言 |
|---|---|---|---|
| AC-1 | TestAppMenuAccelerators | 单测 | 菜单项/加速器/角色齐全 |
| AC-2 | TestAppMenuActions | 单测 | item.Click 触发 newSession/openSettings/导航 |
| AC-3 | 全量 | `go test ./...` | 无回归 |

## 8. 风险与回退

- SetMenu 调用失败仅 log；移除 SetMenu 即回到 MyGo 默认菜单。

## 9. 变更记录

- 2026-10-30 定稿；实现与设计一致，无偏离。
