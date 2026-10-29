# DESIGN-001：持久 bash 会话 设计

- 状态：已落地
- 关联：REQ-001 · TEST-001
- 日期：2026-10-30

## 1. 现状分析

- `internal/tools/tools.go:63-109`：bash 工具为闭包实现，`exec.CommandContext("bash","-c",cmd)`，
  每次全新进程，`cmd.Env = os.Environ()`，`CombinedOutput` 收结果；无任何跨调用状态。
- `internal/agent/agent.go:215-259`：`execTool` 按 `tools.ByName(name)` 分发，工具本身无状态；
  Agent（`agent.go:55-72`）持有会话级字段（History/WorkDir），且 app 层一个会话复用同一个
  Agent（`app.go:1797-1799`：`a.ag == nil` 才重建；会话切换/重建时置 nil）。
- `presets.go:12-19`：Preset 结构只有工具集与提示词，无 shell 形态开关。
- 结论：最小侵入点是「Agent 持有一个可选的 `*tools.PersistentShell`，bash 分发时优先走它」，
  不需要改动 Tool 结构与一发式实现。

## 2. 方案对比

| 方案 | 思路 | 优点 | 缺点 | 结论 |
|---|---|---|---|---|
| A：常驻进程 + stdin 定界符协议 | 每个 Agent 一个 `bash --noprofile --norc` 长进程，命令写进 stdin，输出用唯一 nonce 定界行收取 | 纯标准库；env/cwd/函数天然跨调用保留；与 REQ 的 NFR-1 吻合 | 需自管超时杀树、死亡重启、输出粘包 | ✅ 采用 |
| B：PTY + 逐条喂命令（creack/pty） | 真终端语义 | 行为最接近人用终端 | 引入 cgo/第三方依赖，违背 NFR-1；TTY 回显/控制字符要清洗 | ❌ 依赖成本高 |
| C：每条命令落临时脚本执行 | 状态（cwd/env）由 harness 捕获后重放 | 无常驻进程 | 只能近似 env/cwd（函数、alias、shopt 丢失）；多一次落盘 | ❌ 语义不全 |

## 3. 选定方案（A）

```
Agent(会话级) ──持有──► tools.PersistentShell ──懒启动──► bash --noprofile --norc（独立进程组）
                            │  mutex 串行
                            ▼
        stdin 写入：  <command>\n__dsh_rc=$?\nprintf '%s%d\n' '<END-nonce>' "$__dsh_rc"\n
        stdout 读出：逐行收，直到 <END-nonce>:<rc> 行；stderr 由 cmd.Stderr 并到同一管道
```

生命周期：
1. **懒启动**：Agent 带 Shell 时，第一次 bash 调用才 `Start`（`cmd.Dir` = 会话工作区，`cmd.Env` = os.Environ()）。
2. **执行**：整把互斥锁内「写命令 → 定界读 → 解析 rc」。rc≠0 视为失败；读满定界行即成功。
3. **超时**：读之前对管道 `SetReadDeadline(now+timeout)`；到点即 `kill(-pgid, SIGKILL)` 杀整棵树，重置进程句柄，返回失败并注明「已重置」。
4. **死亡自愈**：读到 EOF（模型执行了 `exit` 等）→ 杀残留、重置句柄、本次调用如实报「会话已退出」；**不**自动重试同一命令（避免重复执行有副作用的命令）；下次调用自动重启新 shell（状态清零）。
5. **回收**：`Close()` 杀进程组并关管道；app 在丢弃 Agent 的每条路径上调用。

### stdin 协议的关键取舍

- 命令**不加包装**直接写入（不用 `{ …; }` 分组）：模型命令可能含不平衡的 `}`，分组会引入解析错误；
  未完成命令（悬挂的 for/heredoc）导致定界行迟迟不来，由超时兜底（杀树重启），行为可预期。
- rc 通过单独一行 `__dsh_rc=$?` 捕获，再 `printf` 出定界行；nonce 含递增计数 + 时间戳，杜绝旧输出/模型回显撞车。
- stderr 在进程级并流（`cmd.Stdout` 与 `cmd.Stderr` 指向同一 `*os.Pipe`），等价一发式的 `CombinedOutput`。

## 4. 数据结构与接口

```go
// internal/tools/shell.go（新增）
func NewPersistentShell(dir string) *PersistentShell
func (s *PersistentShell) Run(ctx context.Context, argsJSON string) (string, bool)
    // argsJSON 与一发式 bash 相同：{"command": string, "timeout_seconds"?: int}
    // 返回（结果文本, 是否失败）
func (s *PersistentShell) Close()   // 杀进程组、关管道；幂等
```

```go
// internal/agent/agent.go（修改）
type Agent struct { …
    Shell *tools.PersistentShell  // 非 nil 时 bash（含 PTC 桥接）走持久 shell
}
```

```go
// presets.go（修改）
type Preset struct { …
    PersistentShell bool `json:"persistent_shell"` // 极简=true；自定义可开启
}
```

presets.json 兼容性：新字段缺省 false，旧文件无需迁移。bash 工具的 API schema 不变
（模型看到的参数完全一致，仅描述追加一句「状态跨调用保留」）。

## 5. 改动文件清单

| 文件 | 改动 | 对应 FR |
|---|---|---|
| `internal/tools/shell.go` | 新增 PersistentShell（协议/超时/自愈/Close） | FR-1~7,10 |
| `internal/tools/shell_test.go` | 新增 6 个单测 | AC-1~4 |
| `internal/agent/agent.go` | Agent.Shell 字段；execTool 分发；Run 内 spec 描述追加持久说明；runCode 桥接 | FR-1,8,9 |
| `presets.go` | Preset.PersistentShell 字段；极简=true；minimal 提示词补持久语义 | FR-1,8 |
| `app.go` | newAgentFor 按 preset 挂 Shell；dropAgent() 统一回收（替换 5 处 `a.ag = nil`） | FR-8,10 |
| `app_test.go` | 新增 preset 接线断言 | AC-5/6 |

## 6. 与 DSH 原版的差异声明

- 原版持久 shell 的进程管理/协议细节未随包发布，**按可观察行为对齐**（env/cwd/选项跨调用保留、超时、自愈）。
- 原版是否连「标准」preset 也用持久 shell 证据不足，本实现仅在极简（及显式开启的自定义）preset 启用。
- PTC 的 `run_code` 桥接 bash 时同样走持久 shell（若 preset 配置了），原版行为未知，属合理推断。

## 7. 测试计划

| FR/AC | 用例 | 层级 | 断言要点 |
|---|---|---|---|
| AC-1 | TestPersistentShellStatePersists | 单测 | export/cd 跨 Run 保留 |
| AC-2 | TestPersistentShellExitCodeAndStderr | 单测 | rc≠0→failed；stderr 并入 |
| AC-3 | TestPersistentShellTimeout | 单测 | 1s 超时杀树；下次调用新 shell 可用 |
| AC-4 | TestPersistentShellRestartAfterExit | 单测 | `exit 7` 报会话退出；重启后状态清零 |
| AC-1 | TestPersistentShellHeredocAndMultiline | 单测 | 多行命令/heredoc 经 stdin 正常执行 |
| AC-5 | TestPersistentShellPresetWiring | app 测试 | 极简挂 Shell、标准不挂 |
| AC-6 | TestDropAgentClosesShell | app 测试 | dropAgent 后进程回收 |
| AC-7 | 全量 | `go test ./...` | 无回归 |

## 8. 风险与回退

- **风险**：模型命令读 stdin（如裸 `cat`）会吞掉定界行 → 命中超时路径，杀树重启，结果如实报超时。
  缓解：超时默认 60s 且可参数化；TEST-001 记录该行为。
- **风险**：模型执行 `exit` 使会话 shell 死亡。缓解：FR-6 自愈 + 结果文本明示，模型可自行重试。
- **回退**：Preset.PersistentShell=false 即回到一发式行为；shell.go 为独立新文件，可整体移除。

## 9. 变更记录

- 2026-10-30 定稿：无实现期偏离（本节随实现结果回填）。
- 2026-10-30 落地：`internal/tools/shell.go` 额外新增 `Running() bool`（测试断言进程
  回收用，后续可复用于状态显示），超出第 5 节清单，特此记录；其余与设计一致。
