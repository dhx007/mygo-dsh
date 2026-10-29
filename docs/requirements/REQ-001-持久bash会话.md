# REQ-001：持久 bash 会话（Persistent Shell）

- 状态：已验证（TEST-001 全部通过；活体 UI 对照为追加项，见 TEST-001 §3）
- 关联：DESIGN-001 · TEST-001 · FEATURE-MATRIX 行 `工具/dsh-tool-bash-persistent`
- 日期：2026-10-30
- 提出人：dhx007 / 负责人：ZCode

## 1. 背景与证据

初代 DSH 的「极简」preset 使用**持久 Shell**：同一个 bash 进程跨工具调用存活，
环境变量（export）、当前目录（cd）、shell 选项（set -e 等）在多次调用之间保留。
本项目的 bash 工具是 `exec.CommandContext` 一发式进程，每次调用都是全新环境，
模型每跑一步带状态的操作（`export FOO=1` 后 `make`、`cd` 后相对路径构建）都要
把状态重放进命令里，与原版行为不一致。

> 证据：
> - `.dsh-ref/dsh-package.json`：`"@deepseek-ai/dsh-tool-bash-persistent": "0.2.1-rc.2"`（独立于 `dsh-tool-bash` 的持久实现）。
> - `.dsh-ref/webapp-readme-zh.md` §配置："`minimal` agent preset 使用持久 Shell、`read_image` 工具和固定 persona"。
> - 本仓库 README「与初代的已知差距」第 1 条："极简模式的「持久 bash」（跨命令保留 env/cwd）未做，每次调用是独立 shell"。
> - 活体 UI（127.0.0.1:3080）：需 token 认证，本轮未逐界面核对；行为语义以 readme 为准（已在 TEST-001 记为遗留核对项）。

## 2. 用户故事

作为使用「极简」模式的用户，我希望模型连续执行的多条命令共享同一个 shell
（export、cd、函数定义跨调用保留），以便像人操作终端一样完成构建、测试等多步任务，
而不是每条命令都重设环境。

## 3. 功能需求（FR）

- **FR-1** 「极简」preset 的 `bash` 工具运行在持久 shell 上：同会话内多次调用复用同一 bash 进程。
- **FR-2** 跨调用保留状态：上一条命令 `export` 的变量、`cd` 后的工作目录、`alias`/函数/`set` 选项，在下一条命令中可见。
- **FR-3** 退出码语义不变：命令失败（rc≠0）时工具结果标记为失败，结果文本含退出码。
- **FR-4** stderr 并入结果（与一发式 bash 的 CombinedOutput 行为一致）。
- **FR-5** 超时保护：单条命令超时（默认 60s，上限 600s，参数与一发式一致 `timeout_seconds`）；超时时杀死该命令整棵进程树，重置 shell，下次调用自动重启。
- **FR-6** 自愈：shell 进程意外退出（模型执行 `exit`、`kill $$` 等）不崩溃，本次调用如实报告「会话已退出」，下次调用自动重启全新 shell（状态清零）。
- **FR-7** 输出上限与一发式一致（16KB 截断）。
- **FR-8** 「标准」「创造」「PTC」preset 行为不变（仍为一发式 bash）；自定义 preset 可通过 `presets.json` 的 `persistent_shell` 字段启用。
- **FR-9** 审批门不变：持久 bash 仍是变更类工具，每次调用照常走「允许一次/总是/拒绝」。
- **FR-10** 会话生命周期：切换/新建/删除会话或切换模式时，旧持久 shell 进程被回收，不泄漏。

## 4. 非功能需求（NFR）

- **NFR-1** 纯 Go、无 cgo，跨平台不引入新依赖（`os/exec` + 标准库）。
- **NFR-2** 单命令超时用进程组杀死（`Setpgid` + `kill -pgid`），不留孤儿进程。
- **NFR-3** 同一 shell 的调用串行化（互斥锁），避免两个命令的输出交叉。
- **NFR-4** 输出定界符使用每次调用唯一的 nonce，模型输出内容即使包含相似文本也不会截错。

## 5. 验收标准

- AC-1（FR-1/2）：单测——`export MYGO_PERSIST=hello` 后新调用 `echo $MYGO_PERSIST` 输出 `hello`；`cd /tmp` 后新调用 `pwd` 输出 `/tmp`。
- AC-2（FR-3/4）：单测——`false` 返回失败标记与 rc=1；`echo err 1>&2` 的 stderr 出现在结果里。
- AC-3（FR-5）：单测——`sleep 3` 配 `timeout_seconds:1` 在约 1s 处失败并提示重置；随后 `echo ok` 正常（新 shell）。
- AC-4（FR-6）：单测——执行 `exit 7` 后调用如实失败；再调用 `echo alive` 成功，且此前 export 的变量不复存在。
- AC-5（FR-7/8/9）：单测/代码审查——截断逻辑复用；标准 preset 的 agent 无持久 shell；审批路径不绕过。
- AC-6（FR-10）：单测/审查——app 丢弃 agent 的每条路径都调用 shell 关闭。
- AC-7（整体）：`go test ./...` 全绿，现有测试不回归。

## 6. 范围外

- 内嵌终端 UI 面板（人用的持久终端）→ FEATURE-MATRIX `工具/tool-terminal`，另行立 REQ。
- 沙箱隔离（sandbox）→ 独立 REQ。
- 「标准」preset 是否切持久 shell：原版证据不足（readme 只点名 minimal），保持一发式，待活体核对后再定。
- PTC `run_code` 桥接的 bash 也走持久 shell：顺带实现（同一 Agent 持有同一 shell 时），不单独验收。

## 7. 决策记录

- 作用域选**会话级**（挂在 Agent 上）而非进程级全局：原版按 agent 运行时隔离，且全局共享会让两个会话互相污染 cwd/env。
- 定界符协议而非 PTY：PTY 在纯 Go 无 cgo 约束下要 CGO/第三方库（creack/pty），违背 NFR-1；DSH 原版实现细节未知，按可观察行为（状态保留）对齐即可。
