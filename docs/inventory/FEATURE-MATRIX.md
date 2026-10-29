# FEATURE-MATRIX：DSH ↔ mygo 功能对照总表

> 需求总台账。行来源：`.dsh-ref/dsh-package.json`（v0.2.1-rc.2 全部子包，每个包 ≈ 一个
> 功能子系统）+ `.dsh-ref/webapp-readme-zh.md` + README「已知差距」+
> **活体 UI 核对（`docs/evidence/LIVE-UI-2026-10-30.md`，含截图）**。
> 每复刻一项：状态列更新，并登记 REQ/DESIGN/TEST 编号，形成证据链。
>
> 状态：✅ 已复刻 · 🟡 部分复刻 · ❌ 未复刻 · ⚪ 不逐包复刻（框架内部/平台层，桌面原生版有等价物即可）
> 优先级：P0 插件体系（用户点名）· P1 日常体验 · P2 锦上添花/平台层

## 1. 会话与存储

| DSH 包/功能 | 说明 | mygo 现状 | 状态 | 优先级 | 文档 |
|---|---|---|---|---|---|
| `dsh-session` / `session-persistence(-jsonl)` | 会话持久化（JSONL 事件流） | `store.go`：meta.json + events.jsonl + history.json | 🟡 自有格式，仅本应用可读 | P2 | — |
| `dsh-session-format`(+catalog, v0→v4) | 会话格式版本化与迁移 | 无版本概念 | ❌ | P2 | — |
| `dsh-session-projection`(+cache) | 会话投影缓存（快速列表/装载） | 直接扫 meta.json | 🟡 | P2 | — |
| `dsh-session-query`(+sqlite, tool-session-query) | 会话 SQL 查询（模型可查历史） | 无 | ❌ | P2 | — |
| `dsh-session-title`(+first-prompt/all-prompts/llm, experimental-session-titles-bundle) | LLM 自动起标题 | 首行截断 24 字 | ❌ | P1 | — |
| `dsh-session-turn-outline` | 轮次大纲 | 无 | ❌ | P2 | — |
| `dsh-session-checkpoint-policy` | 会话检查点/回滚 | 无（有 fork） | ❌ | P2 | — |
| `dsh-session-log-deepseek` / `session-log-export` | 会话日志导出 | 无 | ❌ | P2 | — |
| `dsh-session-stats` | 用量统计 | 设置·使用统计（会话/事件计数） | 🟡 | P2 | — |
| `dsh-session-reference` / `dsh-file-reference(-local)` | 消息内会话/文件引用（@） | 无 | ❌ | P1 | — |
| `dsh-session-telemetry`(-otel) / `dsh-otel` / `host-product-telemetry-otel` | 遥测 | 无（也不该有） | ⚪ | — | — |
| `dsh-workspace`(+changes, client-ui-workspace) | 多工作区管理与变更视图 | 单工作区（/cwd 切换，侧栏分组） | 🟡 | P1 | — |
| `dsh-working-directory` / `tool-working-directory` | 工作目录管理 | ✅ `/cwd` | ✅ | — | — |
| `dsh-storage`(-domain/json) / `dsh-spill`(-local/policy) / `dsh-deque` / `dsh-atomic-write` / `dsh-home-paths` | 存储基建 | Go 等价实现，不逐包 | ⚪ | — | — |

## 2. Agent 核心

| DSH 包/功能 | 说明 | mygo 现状 | 状态 | 优先级 | 文档 |
|---|---|---|---|---|---|
| `dsh-agent-loop` / `dsh-agent` | 工具调用循环 | ✅ `internal/agent`（流式 + tool_calls，30 轮上限） | ✅ | — | — |
| `dsh-agent-instructions` | 工作区指令加载（AGENTS.md/CLAUDE.md 注入系统提示词） | ✅ `instructions.go`：AGENTS.md→CLAUDE.md 发现、32KB 截断、注册表门控（REQ-008） | ✅ | P1 | REQ-008 |
| `dsh-agent-preset`(+registry) / `client-ui-agent-preset` | Agent 预设（多模式） | ✅ 标准/PTC/极简/创造 + 自定义；改名/删除缺 UI | 🟡 | P1 | — |
| `dsh-agent-default-model` | 默认模型 | ✅ deepseek-chat | ✅ | — | — |
| `dsh-persona` / `dsh-system-prompt` | 人设/系统提示词管理 | 固定模板 | 🟡 | P1 | — |
| `dsh-plan-mode` / `client-ui-plan` | 计划模式（只读规划→批准→执行） | 无 | ❌ | P1 | — |
| `dsh-goal`(+round-driver, tool-goal, client-ui-goal, command-goal) | 目标驱动多轮循环 | 无 | ❌ | P2 | — |
| `dsh-user-approval` / `client-ui-approval` | 工具审批 | ✅ 允许一次/总是/拒绝 | ✅ | — | — |
| `dsh-permission-presets` / `client-ui-permission-presets` | 权限预设（按工具/规则批量授权） | 仅全局自动审批 | ❌ | P1 | — |
| `dsh-user-questions` / `tool-ask-user` / `client-ui-user-questions` | 模型向用户提问（选择题） | 无 | ❌ | P1 | — |
| `dsh-subagent`(+spawn/fork-in-process, tool-subagent, -control, client-ui-subagent) | 子代理（生成/分叉/控制） | ✅ `task` 一层深；fork 即会话分叉；控制（暂停/多代理）缺 | 🟡 | P1 | — |
| `dsh-compaction`(+basic/image-offload/tool-result-pruner) | 历史压缩 | ✅ `/compact` 简版（无图片卸载/工具结果裁剪） | 🟡 | P2 | — |
| `dsh-token-meter` | token 用量计量 | 无；活体证实 UI 形态：侧栏用量卡（tokens/calls/费用） | ❌ | P1 | — |
| `dsh-repeat-tool-reminder` | 重复工具调用提醒注入 | 无 | ❌ | P2 | — |
| `dsh-tool-call-timeout-policy` / `dsh-timeout` | 工具超时策略 | 固定默认 60s/上限 600s | 🟡 | P2 | — |
| `dsh-time-context` | 系统提示注入当前时间 | 无 | ❌ | P2 | — |
| `dsh-output-retention` | 大输出落盘保留策略 | ✅ Shell 输出 64KB 上限 + 落盘 spill + 可配置超时（REQ-007，对齐原版 View Shell 卡）；spill 策略包归 ⚪ 框架层 | 🟡 | P2 | REQ-007 |

## 3. 工具

| DSH 包/功能 | 说明 | mygo 现状 | 状态 | 优先级 | 文档 |
|---|---|---|---|---|---|
| `dsh-tool-bash` / `bash-local` | bash 工具（一发式） | ✅ `internal/tools` | ✅ | — | — |
| `dsh-tool-bash-persistent` | **持久 bash**（env/cwd 跨调用保留，minimal 预设标配） | `internal/tools/shell.go`：极简/自定义 preset 挂持久 shell，超时杀树 + 死亡自愈；活体验证通过（DSH "Persistent terminals" 实验开关语义一致） | ✅ | P1 | REQ-001 · DESIGN-001 · TEST-001 |
| `dsh-tool-pwsh`(-persistent, -local, -sandbox) | PowerShell（Windows） | 无（macOS 优先） | ❌ | P2 | — |
| `dsh-sandbox`(-local, -policy, -windows-acl, bash-sandbox, fs-sandbox) | 命令/文件沙箱隔离 | 无（审批门替代） | ❌ | P1 | — |
| `dsh-tool-fs` / `tool-fs-search` / `tool-str-replace-editor` | 文件读写/搜索/编辑器 | ✅ read/write/edit/glob/grep | ✅ | — | — |
| `dsh-tool-web` / `web-fetch-http` / `web-search-deepseek` | 网页抓取/搜索 | 🟡 web_fetch + DDG 搜索（非官方搜索后端） | 🟡 | P2 | — |
| `dsh-tool-todo` | 任务清单 | ✅ checklist 卡片 | ✅ | — | — |
| `dsh-tool-skill` / `dsh-skill`(-filesystem, -office, -badge, experimental-badge-skill-bundle) / `client-ui-skill` | **技能系统**（SKILL.md 发现/调用/徽标） | ✅ 核心（REQ-003）：双目录 SKILL.md 发现 + `skill` 工具 + 设置·技能分区；徽标/office 技能缺 | 🟡 | **P0** | REQ-003 |
| `dsh-tool-subagent-control` | 子代理运行控制 | 无 | ❌ | P1 | — |
| `dsh-tool-terminal` / `dsh-terminal`(-bash) / `experimental-terminal-bundle` / `client-ui-sidebar-terminal` / `api-terminal-controller` | **内嵌终端**（持久 shell 面板） | 无（README 差距） | ❌ | P1 | — |
| `dsh-tool-jobs` / `dsh-jobs`(-local) / `client-ui-jobs` / `api-job-controller` | 后台任务（长命令后台运行/列表） | 无 | ❌ | P1 | — |
| `dsh-tool-schedule` / `dsh-schedule` / `client-ui-schedule` | **定时任务与提醒**（4 个提醒工具 + 时钟）；活体证实侧栏有 Automation tasks 全局面板 | 无 | ❌ | P1 | — |
| `dsh-tool-workflow` / `dsh-workflow`(-ptc) / `client-ui-workflow-run` | **工作流编排** | 无（README 差距） | ❌ | P1 | — |
| `dsh-tool-present` / `client-ui-deliverables` | 交付物展示面板 | 无 | ❌ | P2 | — |
| `dsh-tool-ralph` / `experimental-ralph-bundle` | Ralph 自动循环 | 无 | ❌ | P2 | — |
| `dsh-tool-workspace-dependencies` | 工作区依赖安装 | 无 | ❌ | P2 | — |
| `dsh-mcp-client` / `dsh-mcp-resources` | **MCP 客户端**（外部工具接入） | 🟡 tools 域全链路（REQ-005）：stdio JSONRPC 客户端 + mcp.json 配置 + 插件页 MCP 段 + 审批门；resources/SSE 缺 | 🟡 | **P0** | REQ-005 |

## 4. 插件体系（cordis）— P0 主战场

| DSH 包/功能 | 说明 | mygo 现状 | 状态 | 优先级 | 文档 |
|---|---|---|---|---|---|
| `@deepseek-ai/cordis` + `cordis-plugin-loader/-group/-include/-timer` | 插件运行时：加载器/分组/包含/定时 | 无（插件=内置工具开关） | ❌ | **P0** | — |
| `dsh-tool-cordis` / `client-ui-cordis` | 模型可用的插件管理工具（Creator 启用） | ✅ `plugins` 工具（list/info/enable/disable），按预设授权 + 开关门控 | ✅ | **P0** | REQ-002 |
| `dsh-plugin-manager` / `client-ui-plugin-manager` | **插件管理器**（安装/启停/配置） | ✅ 三段式对齐活体结构（REQ-004）：内置清单卡+搜索+展开详情 / 实验插件开关（持久终端）/ 已装包（plugin.json 模型、子插件开关、Add plugin 本地+git、两步卸载）；更新检查缺 | 🟡 | **P0** | REQ-002 · REQ-004 |
| `dsh-plugin-market`(+index, experimental-plugin-market-bundle) / `client-ui-plugin-market` | **插件市场**（浏览/安装第三方插件） | 🟡 自定义索引市场全流程（REQ-006）：实验开关门控 + markets.json 索引 + 浏览/一键安装；官方源/npm registry 缺 | 🟡 | **P0** | REQ-006 |
| `dsh-plugin-package-inventory-deepseek` / `host-plugin-inventory` / `client-ui-settings-plugin-inventory` | 插件清单（盘点全部已装/内置） | ✅ 内置插件清单（12 项注册表，插件页/设置/模型工具同源）；第三方清单待 REQ-003+ | 🟡 | P1 | REQ-002 |
| `dsh-config-editor` / `dsh-package-manifest` | 插件 patch/清单编辑（cordis.patch.yml 语义） | 🟡 清单以 plugin.json 等价承载（REQ-004）；patch 叠加语义无 | 🟡 | P1 | REQ-004 |
| Web 插件（设置·Web 插件页） | 浏览器侧插件入口 | 设置有占位分区 | 🟡 | **P0** | — |
| `experimental-agent-team`(+profile, tool-, client-ui-) | 多代理团队协作 | 无 | ❌ | P2 | — |
| `experimental-worktree` / `tool-worktree` | git worktree 隔离执行 | 无 | ❌ | P2 | — |
| `experimental-auto-review` | 自动代码评审 | 无 | ❌ | P2 | — |
| `experimental-inspector`(-profile) / `experimental-session-inspector` | 会话检查器 | 轨迹页（只读逐条） | 🟡 | P2 | — |
| `experimental-session-search` | 会话搜索 | ✅ 侧栏全文搜索 | ✅ | — | — |
| `experimental-speech-to-text`(-sensevoice) / `-voice-input-bundle` / `client-ui-voice-input` / `api-speech-to-text` | 语音输入 | 无 | ❌ | P2 | — |
| `experimental-cot-translation`(+bundle, client-ui-) / `experimental-translator` | 思维链翻译/翻译器 | 无 | ❌ | P2 | — |

## 5. 界面（client-ui-*）

| DSH 包/功能 | 说明 | mygo 现状 | 状态 | 优先级 | 文档 |
|---|---|---|---|---|---|
| `client-ui-layout` / `sidebar` / `sidebar-browser` | 整体布局/会话侧栏/侧栏浏览器 | ✅ 对齐桌面版 | ✅ | — | — |
| `client-ui-sidebar-files` | 工作区文件树侧栏 | 无 | ❌ | P1 | — |
| `client-ui-sidebar-documentpreview` | 文档预览侧栏（含 Office 查看器） | 无 | ❌ | P2 | — |
| `client-ui-sidebar-right` | 右侧栏（交付物/任务等挂载点） | 无 | ❌ | P2 | — |
| `client-ui-chat` / `conversation` / `renderer` / `tool` | 聊天时间线/工具卡/Markdown | ✅ 流式 + 工具折叠卡 | ✅ | — | — |
| `client-ui-attachment` / `dsh-attachment(-local)` / `client-file-upload` | 附件上传与读取（图片 read_image） | 无 | ❌ | P1 | — |
| `client-ui-reference` | 输入框 @ 引用 | 无 | ❌ | P1 | — |
| `dsh-commands` / `client-ui-commands` / `command-compact/-feedback/-goal` | 斜杠命令体系 | 🟡 /help /new /fork /compact /mode /cwd /trajectory | 🟡 | P1 | — |
| `client-ui-model-selection`(-deepseek-model-label) | 会话内模型快速切换 | 仅设置页改全局模型 | 🟡 | P1 | — |
| `dsh-message-feedback` / `client-ui-message-feedback` | 消息点赞/点踩 | 无 | ❌ | P2 | — |
| `client-ui-secret-mask` | 秘钥掩码显示 | 无 | ❌ | P2 | — |
| `dsh-client-shortcuts` / `client-ui-shortcuts` | 键盘快捷键体系 | 无（仅回车发送） | ❌ | P1 | — |
| `client-ui-theme` | 主题 | ✅ 暖黑/深海/森绿 | ✅ | — | — |
| `client-ui-trajectory` | 轨迹页 | ✅ 逐条检查 | ✅ | — | — |
| `client-ui-settings(-account/-general/-models/-plugins/-plugin-inventory/-session-log/-shell/-subagent/-web-search/-agent-loop)` | 设置分区 | 🟡 账号/通用/模型/内置插件/预设/归档/Web插件/皮肤/宠物/工坊/统计；shell、子代理、网页搜索、会话日志、agent-loop 分区缺 | 🟡 | P1 | — |
| `client-ui-session` / 用户菜单 | 会话/用户菜单 | ✅ | ✅ | — | — |
| `client-ui-open-in-app` / `host-open-in-app` | 在应用中打开 | 无 | ❌ | P2 | — |
| `dsh-host-directory-picker`(-auto/-browse/-native) | 目录选择器 | 🟡 /cwd 手输路径 | 🟡 | P1 | — |
| `client-ui-brand-official` / `dsh-brand` | 品牌 | ✅ | ✅ | — | — |
| `dsh-anonymous-user-id` | 匿名用户标识 | 无 | ⚪ | — | — |

## 6. LLM 与传输

| DSH 包/功能 | 说明 | mygo 现状 | 状态 | 优先级 | 文档 |
|---|---|---|---|---|---|
| `dsh-llm` / `llm-deepseek` / `deepseek-llm-api-extensions` | DeepSeek Messages API | ✅ SSE 流式 + tool_calls 增量组装 | ✅ | — | — |
| `dsh-llm-retry` | 请求重试 | 无 | ❌ | P2 | — |
| `dsh-llm-pi-ai` | 其他 LLM 提供方（pi-ai） | 无 | ❌ | P2 | — |
| `dsh-llm-deepseek-account` / `deepseek-account(-platform)` / `llm-deepseek-api-key` | 账号/余额/API Key 管理 | ✅ /user/balance 余额查询 | ✅ | — | — |
| `dsh-credentials`(-local) | 凭据安全存储（钥匙串） | 明文 config.json（0600） | 🟡 | P1 | — |
| `dsh-web-app` / `host-webserver` / `frontend-static` / `client-connection` / `api-gateway` / `api-*` / `api-remote-access` / `dsh-acp`(+app, sdk) | Web 表层（监听器/认证/远程访问/ACP） | ⚪ 桌面原生等价；活体 UI 仅作对照 | ⚪ | — | — |
| `dsh-client-hmr` / `client-modules` / `client-locale` / `client-product-analytics` / `dsh-http-proxy` | Web 前端基建 | ⚪ | ⚪ | — | — |
| `dsh-headless` | 无头运行（脚本化调用） | 无 | ❌ | P2 | — |
| `dsh-office-to-pdf` | Office→PDF 转换 | 无 | ❌ | P2 | — |

## 7. 桌面宿主与基建

| DSH 包/功能 | 说明 | mygo 现状 | 状态 | 优先级 | 文档 |
|---|---|---|---|---|---|
| `dsh-desktop-host` / `app-boot` / `cmdline` / `launch-environment` | 桌面壳/启动 | ✅ mygo 原生窗口（1200×780 记忆位置） | ✅ | — | — |
| `dsh-win32-process` / `node-addon-system` | Windows 进程/原生插件 | ⚪ 平台层 | ⚪ | — | — |
| `dsh-fs`(-local, -observation-policy) / `dsh-subprocess`(-local) / `dsh-native-command` / `dsh-tmux-context` | 文件系统/子进程基建 | Go 标准库等价 | ⚪ | — | — |
| `dsh-shell` / `shell-env` / `dsh-bash-local` | 受管 shell 环境 | 🟡 os.Environ() 注入 | 🟡 | P2 | — |
| `dsh-typert-*` / `dsh-util-*` / `dsh-sdk-*` / `@agentclientprotocol/sdk` | 内部协议/工具库 | ⚪ | ⚪ | — | — |
| `dsh-harness-dev-context` | DSH 自身的开发上下文 | ⚪ 不适用 | ⚪ | — | — |

## 统计快照

| 状态 | 数量（截至 2026-10-30） |
|---|---|
| ✅ 已复刻 | 18 |
| 🟡 部分复刻 | 23 |
| ❌ 未复刻 | 26 |
| ⚪ 不逐包复刻 | 14 |

> 排期建议：~~P0 插件集群（REQ-002~006）~~、~~REQ-007 Shell 输出落盘~~、
> ~~REQ-008 工作区指令~~（已完成）→ P1 继续：tool-jobs、会话内模型切换、附件、
> 快捷键、权限预设、自动标题（逐项立 REQ）。
> 活体核对新增候选：tool-jobs、time-context、tool-ask-user、侧栏用量卡（token/calls/费用）。
