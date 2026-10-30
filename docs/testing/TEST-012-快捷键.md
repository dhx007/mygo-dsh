# TEST-012：快捷键体系 测试报告

- 状态：通过
- 关联：REQ-012 · DESIGN-012
- 执行日期 / 执行环境：2026-10-30 · macOS 24.1.0 arm64 · Go（标准库 + MyGo 菜单模块）

## 1. 自动化用例执行记录

全量回归（`go test ./...`，原样输出）：

```
ok  	mygo-harness	0.798s
ok  	mygo-harness/internal/agent	(cached)
ok  	mygo-harness/internal/llm	(cached)
ok  	mygo-harness/internal/mcp	(cached)
ok  	mygo-harness/internal/tools	(cached)
```

新增用例（verbose，原样输出）：

```
--- PASS: TestAppMenuAccelerators (0.01s)
--- PASS: TestAppMenuActions (0.00s)
--- PASS: TestMountAppMenuHeadlessNoop (0.00s)
```

另：`go vet ./...` 无告警；`go build -trimpath` 构建成功。

## 2. 用例 ↔ 验收标准对照

| FR | 验收标准 | 用例 | 结果 |
|---|---|---|---|
| FR-1/3 | AC-1 菜单含全部加速器（⌥⌘N / ⌥⌘, / ⌘1/2/3）且各有 Click；编辑/关闭角色存在 | TestAppMenuAccelerators | ✅ |
| FR-2 | AC-2 item.Click 触发既有动作：新会话创建会话、轨迹导航切 selectedID、设置走无头回退打开 | TestAppMenuActions | ✅ |
| FR-4 | AC-3 无头模式 mountAppMenu 是 no-op 不 panic | TestMountAppMenuHeadlessNoop | ✅ |
| AC-3 | 全量无回归 | `go test ./...` 五包全 ok | ✅ |

## 3. 手工 / 活体核对

加速器取自活体证据（`docs/evidence/LIVE-UI-2026-10-30.md`）：⌥⌘N（侧栏 New session
角标）、⌥⌘,（Settings 角标）；⌘1/2/3 导航为本实现自定（DESIGN §6 声明）。
菜单栏在真实窗口的行为（加速器触发、编辑角色剪贴板）需窗口环境人工验证——
`go run .` 后按 ⌥⌘N / ⌥⌘, 核对即可，自动化仅覆盖数据与动作层。

## 4. 已知问题与后续项

- 菜单栏为中文文案（原版英文）；如需跟随系统语言，走 MyGo 菜单角色本地化，后续。
- 应用内非菜单快捷键（Esc 关弹层由 Popover 自带；composer ⌘↑ 发送等）未做。
- 设置以 ⌥⌘, 打开的是模态子窗口；关闭它需点击 ✕ 或 RoleClose（窗口级）。

## 5. 结论

3/3 新用例通过，AC-1~AC-3 全部满足，无回归。FEATURE-MATRIX：
`dsh-client-shortcuts` ❌→🟡（应用菜单加速器体系就位；应用内非菜单快捷键未做）。
建议 REQ-012 置「已验证」。
