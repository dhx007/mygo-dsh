# DESIGN-009：后台任务 设计

- 状态：已落地
- 关联：REQ-009 · TEST-009
- 日期：2026-10-30

## 1. 现状分析

- bash 工具一发式（`CombinedOutput`）+ 持久 shell（REQ-001）都没有后台形态；
  tools 包已有包级可注入变量先例（REQ-007 OutputCapBytes）。
- 工具经 `tools.all` 注册 → preset 工具列表 + `SpecsFor` 挂载；门控经注册表 +
  `enabledTool`。job 三工具完全复用该通道，无需 agent 层改动。

## 2. 方案对比

| 方案 | 思路 | 优点 | 缺点 | 结论 |
|---|---|---|---|---|
| A：bash `background` 参数 + 包级 JobRegistry | 启动入口在 bash；三工具读注册表 | 与原版描述一致；零 agent 层接线 | 注册表为全局 | ✅ 采用（对齐 host 级语义） |
| B：独立 job_start 工具 | 工具面更对称 | 多一个工具；bash 双入口 | 与原版 tool-bash 描述不符 | ❌ |

## 3. 选定方案（A）

```go
// internal/tools/jobs.go（新增）
type Job struct { ID, Command string; Started, Finished time.Time; ExitErr error; buf *jobBuffer; cmd *exec.Cmd }
type jobBuffer struct { mu; buf bytes.Buffer }        // 子进程写 / 工具读，环形上限 1MB（丢头部）
var Jobs = &JobRegistry{jobs: map[string]*Job{}}
func (r *JobRegistry) Start(command, cwd string) (string, error)
    // bash -c <command>；Setpgid（kill 杀树）；cmd.Start；go cmd.Wait → 记录 Finish
func (r *JobRegistry) List() []JobInfo                // id/命令/状态/退出码/启动时间/输出字节
func (r *JobRegistry) Output(id string, offset int) (text string, state string, next int, err)
    // 输出[offset:offset+OutputCapBytes]，next=下次 offset；状态与总长随文返回
func (r *JobRegistry) Kill(id string) error           // kill(-pgid, SIGKILL)

// bash 工具参数 +{"background": bool}：true → Jobs.Start，返回
// "后台任务已启动：<id>（job_output 查看输出 / job_list 列表 / job_kill 终止）"

// 三个新工具注册进 tools.all：job_output（只读）、job_list（只读）、job_kill（mutating）
```

状态机：`running → finished`（cmd.Wait 返回，ExitErr 记录）；Kill 即 finished
（ExitErr = signal: killed）。

## 4. 数据结构

无持久化（应用退出任务终止）；JobInfo 为只读视图。

## 5. 改动文件清单

| 文件 | 改动 | 对应 FR |
|---|---|---|
| `internal/tools/jobs.go` | 新增：JobRegistry/Start/List/Output/Kill | FR-1~4/6 |
| `internal/tools/tools.go` | bash background 参数；job_output/job_list/job_kill 三工具注册 | FR-1~4 |
| `presets.go` | standardTools/creatorTools += 三工具 | FR-5 |
| `plugins.go` | 注册表第 16 项「后台任务」 | FR-5 |
| `internal/tools/jobs_test.go`、`plugins_test.go`、`app_test.go` | 用例 | AC-1~5 |

## 6. 与 DSH 原版的差异声明

- 无 Task Board UI（社区插件形态）；无任务持久化与跨进程注册表（host 内存级等价）。
- 输出环形 1MB（原版保留策略未知）。

## 7. 测试计划

| AC | 用例 | 层级 | 断言 |
|---|---|---|---|
| AC-1 | TestJobLifecycle（启动即返、输出可达、状态 finished/exit 0） | 单测 | 生命周期 |
| AC-2 | TestJobOutputPaging（4KB 输出 cap=1KB 翻页） | 单测 | 分页 |
| AC-3 | TestJobKill（sleep 30 杀树、状态非零） | 单测 | 终止 |
| AC-1 | TestBashBackgroundParam（<1s 返回 job id；job_list 可见） | 单测 | bash 参数 |
| AC-4 | TestBuiltinPluginRegistry（16）· TestPluginsPageSections（16）· TestPluginControlActions（16） | 单测/UI | 注册表与门控 |
| AC-5 | 全量 | `go test ./...` | 无回归 |

## 8. 风险与回退

- **风险**：任务失控堆积 → job_list 可见 + job_kill 终止；环形缓冲防内存失控；
  应用退出进程随宿主终止。
- **风险**：模型用后台绕过审批 → 启动走 bash 审批门（不变），job_kill 亦 mutating。
- **回退**：preset 工具列表移除三工具即整体下线。

## 9. 变更记录

- 2026-10-30 定稿；实现与设计一致，无偏离（get 辅助并入 Output/Kill 的就地查询）。
