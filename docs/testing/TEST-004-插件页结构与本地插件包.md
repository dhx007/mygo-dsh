# TEST-004：插件页结构修订 + 本地插件包运行时 测试报告

- 状态：通过
- 关联：REQ-004 · DESIGN-004
- 执行日期 / 执行环境：2026-10-30 · macOS 24.1.0 arm64 · Go（go.mod toolchain，goja 无 cgo）

## 1. 自动化用例执行记录

全量回归（`go test ./...`，原样输出）：

```
ok  	mygo-harness	4.647s
ok  	mygo-harness/internal/agent	2.442s
ok  	mygo-harness/internal/llm	(cached)
ok  	mygo-harness/internal/tools	(cached)
```

新增用例（verbose，原样输出）：

```
--- PASS: TestPluginsPageSections (0.05s)
--- PASS: TestPluginsPageInstalledPackage (0.01s)
--- PASS: TestPersistentTerminalsFlag (0.00s)
--- PASS: TestPackageToolAgentWiring (0.00s)
--- PASS: TestSettingsPluginsSectionGrouped (0.01s)
--- PASS: TestManifestValidation (0.00s)
--- PASS: TestInstallUninstallPackage (0.00s)
--- PASS: TestInstallFromGit (0.09s)
--- PASS: TestPackageState (0.00s)
--- PASS: TestRunPackageToolBridge (0.00s)
--- PASS: TestRunPackageToolErrors (0.00s)
--- PASS: TestRunPackageToolTimeout (0.20s)
--- PASS: TestPackageToolDispatch (0.00s)
```

另：`go vet ./...` 无告警；`go build -trimpath` 构建成功。

## 2. 用例 ↔ 验收标准对照

| FR | 验收标准 | 用例 | 结果 |
|---|---|---|---|
| FR-1 | AC-1 三段式清单卡 + 搜索过滤 + 展开详情（含开关） | TestPluginsPageSections · TestSettingsPluginsSectionGrouped | ✅ |
| FR-2 | AC-2 持久终端开关：开→所有 bash 预设挂 Shell，关→恢复；极简恒开 | TestPersistentTerminalsFlag | ✅ |
| FR-3 | AC-3/FR-3 包状态 On/Partially on/Off；子插件开关 | TestPackageState · TestPluginsPageInstalledPackage | ✅ |
| FR-6 | AC-3 本地安装复制目录；非法 manifest/id 冲突报错 | TestInstallUninstallPackage · TestManifestValidation | ✅ |
| FR-6 | AC-4 git clone 安装（本地仓库）且无半成品残留 | TestInstallFromGit | ✅ |
| FR-7 | AC-4 两步确认卸载后目录消失 | TestPluginsPageInstalledPackage | ✅ |
| FR-4 | AC-5 脚本桥接（write_file）、args 传递、缺 run/抛错/非法 JSON/超时中断 | TestRunPackageToolBridge · TestRunPackageToolErrors · TestRunPackageToolTimeout | ✅ |
| FR-5 | AC-5 spec 挂载 + 包/子插件门控 + mutating 审批 | TestPackageToolAgentWiring · TestPackageToolDispatch | ✅ |
| AC-6 | 全量无回归 | `go test ./...` 四包全 ok | ✅ |

## 3. 手工 / 活体核对

本轮实现即以 2026-10-30 活体核对（`docs/evidence/LIVE-UI-2026-10-30.md`）为验收基准：
三段式结构、实验卡片样式（标题+Experimental 徽标+描述+开关）、包卡（名/vX/来源/状态）、
Add plugin 入口均已对齐截图语义。残留差异：Plugin manager 标签的更新检查/安装冲突/
启动失败修复未实现（REQ 范围外）；无 Discover 标签（REQ-006）。

## 4. 已知问题与后续项

- npm registry 安装不支持（source 字段仅显示）；git 安装依赖系统 git，未装 git 时报错提示。
- URL 识别为启发式：目录存在→本地导入；`git@`/`.git`/`http(s)://`→git clone；普通不存在
  路径报错并提示 git URL 写法。
- 包工具 spec 为开放 object（无 JSON Schema 校验）；脚本改动需重启会话（dropAgent）后生效。
- 桥接 bash 时若挂了持久 shell 则走持久 shell（与 REQ-001 一致），测试覆盖桥接 write_file。
- 后续：REQ-005 MCP 客户端；REQ-006 插件市场（Discover 标签 + market index）；
  Plugin manager 标签（更新检查）候选。

## 5. 结论

13/13 新用例通过，AC-1~AC-6 全部满足，无回归。FEATURE-MATRIX：`plugin-manager` 保持
🟡（安装/启停/配置已齐，更新检查缺）、`config-editor` ❌→🟡（清单以 plugin.json 等价承载）。
建议 REQ-004 置「已验证」。
