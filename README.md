# DeepSeek Harness (Go 原生 UI · 复刻初代)

用 [MyGo](https://github.com/egoist/mygo) 的**原生 UI 工具包**（Go 自绘 GPU 渲染，无 webview、无 HTML、无 cgo）复刻初代 [DeepSeek Harness](https://github.com/egoist/mygo) v0.1 的桌面端：会话按工作区组织、四种模式、工具调用摊开在时间线上、Trajectory 轨迹可复盘。

## 与初代对齐的功能

| 初代功能 | 本实现 |
|---|---|
| 左侧会话列表，按工作区（本地项目目录）组织 | ✅ 品牌区 + 新会话大按钮 + 插件/设置导航 + 工作区分组（含搜索），底部状态/用户行 |
| 对话区时间线：每一步工具调用摊开 | ✅ 顶栏标题+模式+对话/轨迹标签页（橙色下划线），工具行内折叠，Markdown 流式渲染 |
| 四种模式：标准 / PTC / 极简 / 创造 | ✅ 标准=全套工具；PTC=goja 跑脚本合并工具调用；极简=持久 bash（env/cwd 跨调用保留）+编辑器；创造=可读写 presets.json 造新模式 |
| Trajectory 轨迹（只增不改的会话日志） | ✅ 每会话 events.jsonl，轨迹页按来源逐条检查（原文 JSON 折叠） |
| 会话恢复 / 分叉 / 搜索 | ✅ history.json 续聊；一键分叉；侧栏全文搜索 |
| 工具权限审批（允许一次 / 总是允许 / 拒绝） | ✅ 变更类工具（bash/写/编辑/PTC）先审批，可「总是允许」，设置里可全自动 |
| `/compact` 自动历史压缩 | ✅ `/compact` 用模型生成摘要替换历史，轨迹留痕 |
| 子代理 / 规划 | ✅ `task` 工具（一层深子代理）；`todo` 工具渲染成任务清单卡片 |
| 一切皆插件 | ✅ 插件页三段式对齐原版：内置插件清单卡（搜索 + 展开详情）· 实验插件开关（持久终端）· 已装插件包（plugin.json 包模型 + 子插件开关 + 本地/git 安装 + 卸载）；模型可用 `plugins` 工具管理（创造模式）；包工具在 goja 沙箱执行（复用 PTC 桥，30s 超时） |
| MCP 客户端 | ✅ stdio JSONRPC（纯标准库）：mcp.json 配置服务器，插件页「MCP 服务器」段添加/删除/启停/看错误，工具以 `mcp_<服务器>_<工具>` 挂载并默认走审批门 |
| 技能（skills） | ✅ 双目录 SKILL.md 发现（数据目录 `skills/` 与工作区 `.harness/skills/`，工作区覆盖全局），模型 `skill` 工具 list/run 按需加载指令，设置·技能分区展示 |
| 设置 → 模型（DeepSeek，DEEPSEEK_API_KEY） | ✅ 模态设置子窗口（打开期间主窗口锁定）：账号与余额（/user/balance 真实查询）/ 通用设置 / 模型 / 内置插件 / Agent 预设 / 已归档会话 / Web 插件 / 皮肤（暖黑·深海·森绿）/ 宠物 / 创意工坊 / 使用统计；入口在左下角用户菜单 |

斜杠命令：`/help` `/new` `/fork` `/compact` `/mode <标准|PTC|极简|创造|…>` `/cwd <目录>` `/trajectory`

## 运行

```sh
go run .
# 或单文件二进制（纯 Go，无 cgo）
go build -trimpath -o build/mygo-harness .
./build/mygo-harness
```

首次启动 → 「设置」填 [DeepSeek API Key](https://platform.deepseek.com)（或 `DEEPSEEK_API_KEY` 环境变量）→ 新建会话。

## 界面

暗色主题对齐桌面版截图：暖黑画布 `#131110`、侧栏 `#1e1b19`、发丝边框、动作橙 `#e85d2a`（发送按钮/活动标签页）。输入框为 DSH 式内嵌圆角容器（输入行 + ＋ / 模型 / ↑ 操作行），底部状态栏显示状态与轨迹条数。用户消息右侧气泡，助手消息 Markdown，工具调用为带状态点的折叠卡片，任务清单渲染为 checklist 卡。

## 结构

```
main.go            入口：窗口（1200×780，记忆位置）
theme.go           DSH 暗色设计令牌 → MyGo Theme
store.go           会话仓库：meta / events.jsonl（轨迹）/ history.json，分叉与搜索
presets.go         四种模式 + 自定义预设（presets.json）
plugins.go         插件注册表：13 个内置插件（分类/版本/来源），插件页·设置·模型工具同源
packages.go        本地插件包：plugin.json 包模型、本地/git 安装、子插件开关、卸载
skills.go          技能：双目录 SKILL.md 发现与解析（全局 + 工作区）
config.go          配置（Key/模型/工作区/插件开关/自动审批）
app.go             全部 UI：侧栏、时间线、审批卡、轨迹页、插件页、设置页、/命令
app_test.go        ui.Tester 无头 UI 流程测试
internal/llm/      DeepSeek SSE 客户端（流式 + tool_calls 增量组装）
internal/agent/    Agent 循环：审批门、todo、task 子代理、PTC 执行器、插件包运行时
internal/mcp/      MCP stdio JSONRPC 客户端（initialize/tools.list/tools.call）
internal/tools/    bash / 持久 shell / read_file / write_file / edit_file / glob / grep / web_fetch / web_search
mcp.go             MCP 管理器：mcp.json 配置、懒连接、工具 spec 组装、错误态
```

## 文档

功能复刻的需求、设计与测试留痕在 `docs/`：流程见 `docs/PROCESS.md`，功能总台账见
`docs/inventory/FEATURE-MATRIX.md`，已交付特性见 `docs/requirements|design|testing/`。

## 测试

```sh
go test ./...
```

覆盖三层：工具行为（glob `**`、grep、bash 退出码、写文件）、SSE 组装（tool_calls 增量、401 错误、reasoning 不回传）、PTC（goja 桥接 bash/glob/读写文件、循环求值），以及 **`ui.Tester` 无头 UI 测试**——不开窗口跑完整用户路径：新建会话 → 输入 → 发送（mock SSE 服务器）→ 回复与轨迹落盘、设置保存 key、侧栏切换会话。

## 与初代的已知差距

- 工作流编排、终端面板未包含（进度台账见 `docs/inventory/FEATURE-MATRIX.md`；技能已复刻）
- PTC 是 goja（纯 Go JS 解释器）而非 Node SDK，超时 120s
- 自定义模式经 UI 创建；改名/删除直接编辑 presets.json

## 打包

```sh
go run github.com/egoist/mygo/cmd/mygo@latest build .
```
