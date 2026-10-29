# DESIGN-004：插件页结构修订 + 本地插件包运行时 设计

- 状态：已落地
- 关联：REQ-004 · TEST-004
- 日期：2026-10-30

## 1. 现状分析

- `app.go` pluginsView/secPlugins 用 `pluginRenderPlan()` 分组渲染开关行；注册表
  `plugins.go` 含 13 内置插件；`cfg.EnabledTools` 键 = 插件 id。
- `internal/agent/ptc.go` 已有 goja 工具桥（`runPTCCode` + wrapJS 包装函数），可直接
  抽出复用于包工具脚本。
- `cfg` 无实验开关字段；无插件包概念。

## 2. 方案对比

| 方案 | 思路 | 优点 | 缺点 | 结论 |
|---|---|---|---|---|
| A：目录包 + goja 沙箱（复用 PTC 桥） | 包 = 目录 + plugin.json；工具脚本 goja 执行 | 零新依赖；与 PTC 同一安全模型；"一切皆插件"实体化 | 脚本能力限于桥接函数 | ✅ 采用 |
| B：外部进程插件（stdin/stdout JSONRPC） | 每包一个子进程 | 隔离最强、语言无关 | 进程生命周期管理 + 协议栈，量级失控 | ❌ 留给 MCP（REQ-005 天然解决） |
| C：仅 UI 修订，不做运行时 | 只改三段式 | 最小 | "安装"无意义，包模型无实体 | ❌ |

## 3. 选定方案（A）

```
packages.go（main 包）
  PackageTool{Name,Description,Mutating,Script}
  PluginPackage{ID,Name,Version,Source,Origin,Description,Tools}
  pluginsRoot() = <dataDir>/plugins
  installedPackages() []PluginPackage        // 扫描 + manifest 校验
  loadManifest(dir) (PluginPackage, error)   // id^=[a-z0-9-]+$、name、version、tools.name 唯一
  installFromPath(src) (PluginPackage, error)   // 校验 → CopyFS 到 plugins/<id>
  installFromGit(url) (PluginPackage, error)    // git clone --depth 1 → 校验 → 失败清理
  uninstallPackage(id) error                    // RemoveAll

internal/agent/packagetool.go
  RunPackageTool(ctx, script, argsJSON, cwd, call) (string, bool)
    // 新 goja VM + PTC 同款 wrapJS 桥 + 脚本定义 run(args)；30s Interrupt；32KB 截断

internal/agent/agent.go
  Agent.PackageTools map[string]func(argsJSON string) (string, bool)  // name→runner
  execTool: default 前查 PackageTools（mutating 由 app 侧注册时决定审批——见 app 接线）
  toolSpecs(): depth0 时追加包工具 spec（PluginEnabled(name) 门控）

app
  cfg.Experimental map[string]bool           // "persistent_terminals"
  pkgCache []PluginPackage + refreshPackages()
  newAgentFor(): 实验开关 → 所有含 bash 预设挂 Shell；包工具 spec + runner + 审批注册
  pluginsView(): 三段式；secPlugins 复用清单卡渲染
```

### 审批门接线细节

包工具的 mutating 标志在 manifest；agent.execTool 的审批只看 `tools.ByName` 的
`Mutating`。做法：app 把包工具 runner 包一层，注册进 `Agent.PackageTools` 的同时把
工具名登记进 `Agent.MutatingTools map[string]bool`；execTool 统一查
`t.Mutating || a.MutatingTools[name]` 后走 `a.approve(...)`。

### 门控键

- 包级：`cfg.EnabledTools["<pkgid>"]`（缺省开）；子级：`["<pkgid>/<tool>"]`（缺省开）。
- `enabledTool` 现语义（nil map = 全开）不变，包键直接复用。

## 4. 数据结构

```jsonc
// <dataDir>/plugins/<id>/plugin.json
{
  "id": "demo-pack", "name": "示例插件包", "version": "1.0.0",
  "source": "local",              // local | git | npm（显示用）
  "origin": "/path or git url",   // 安装来源记录
  "description": "…",
  "tools": [
    {"name": "demo_tool", "description": "…", "mutating": false, "script": "demo_tool.js"}
  ]
}
```

工具脚本约定：定义 `function run(args) { … return "结果字符串"; }`；`args` 为模型传参
对象；桥接函数与 PTC 相同；抛错即失败。

## 5. 改动文件清单

| 文件 | 改动 | 对应 FR |
|---|---|---|
| `packages.go` | 新增：包模型/安装/卸载/扫描 | FR-3/6/7 |
| `internal/agent/packagetool.go` | 新增：RunPackageTool | FR-4 |
| `internal/agent/agent.go` | PackageTools/MutatingTools 字段、execTool 分发、toolSpecs 追加 | FR-4/5 |
| `config.go` | Experimental 字段 + experimentalOn | FR-2 |
| `app.go` | pkgCache、newAgentFor 接线、pluginsView 三段式、secPlugins 复用、安装/卸载动作 | FR-1/2/6/7 |
| `packages_test.go`、`app_test.go`、`internal/agent/packagetool_test.go` | 用例 | AC-1~6 |

## 6. 与 DSH 原版的差异声明

- npm registry 安装不支持（source 字段仅显示）；git clone 用系统 git。
- 内置插件启用开关保留在详情内（原版在 Agent 预设编辑）。
- 包工具 spec 无 JSON Schema 校验（开放 object）；脚本是 goja 子集 JS 而非 Node。
- 无启动失败修复/更新检查（Plugin manager 标签职责）→ 后续 REQ。

## 7. 测试计划

| AC | 用例 | 层级 | 断言 |
|---|---|---|---|
| AC-3/4 | TestInstallUninstallPackage（本地 + 本地 git 仓库 clone + 非法 manifest + id 冲突） | 单测 | 目录复制/清理/报错 |
| AC-5 | TestRunPackageTool（写文件桥接、run 缺失报错、超时中断、抛错失败） | 单测 | 输出与失败标记 |
| AC-5 | TestPackageToolAgentWiring（spec 挂载、审批触发、停用即失） | app+agent 测试 | 门控 |
| AC-2 | TestPersistentTerminalsFlag | app 测试 | 开/关挂 Shell |
| AC-1 | TestPluginsPageSections | ui.Tester | 三段/搜索/展开详情 |
| AC-6 | 全量 | `go test ./...` | 无回归 |

## 8. 风险与回退

- **风险**：恶意包脚本滥用桥接（bash 任意命令）→ 与内置 bash 同一审批门（mutating）+
  沙箱限制（无直连能力）；文档明示只装可信来源。
- **风险**：goja 死循环 → 30s Interrupt（PTC 已验证该机制）。
- **回退**：Experimental 关闭；包目录删除即回到纯内置；UI 分段为独立渲染函数可单独还原。

## 9. 变更记录

- 2026-10-30 定稿。
- 2026-10-30 落地，两处实现细节与定稿不同（均已在测试覆盖）：
  1. 包工具桥接回调直接收到**干净工具名**（定稿写的是 raw 名 + TrimSuffix）：裁剪收敛在
     RunPackageTool 内部，调用方（app 桥接、测试）无需感知 `_raw` 后缀。
  2. 安装输入的 git 识别为启发式：目录存在→本地导入；`git@`/`.git`/`http(s)://` 前缀→
     git clone；其余报错并提示写法。
