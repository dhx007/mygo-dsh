# DESIGN-007：Shell 输出上限 + 落盘 设计

- 状态：已落地
- 关联：REQ-007 · TEST-007
- 日期：2026-10-30

## 1. 现状分析

- `internal/tools/tools.go:37`：`maxOutput = 16KB` 常量，bash 与持久 shell 结果
  `truncate()` 后丢弃超出部分；read_file 也用它做缓冲护栏。
- 持久 shell `shell.go` Run 出口 `truncate(out)`。
- config 无 shell 相关字段。

## 2. 方案对比

| 方案 | 思路 | 优点 | 缺点 | 结论 |
|---|---|---|---|---|
| A：tools 包可注入变量 + 系统临时目录落盘 | `OutputCap`/`DefaultTimeout` 变量，app 启动同步；超限写 os.CreateTemp | 工具签名不变；实现集中 | 包级变量（可测，测试可覆写） | ✅ 采用 |
| B：Tool 增加 config 参数 | 显式传参 | 无全局态 | Tool.Run 签名与全部调用点（agent/PTC 桥/持久 shell）连锁改动 | ❌ |

## 3. 选定方案（A）

```go
// tools 包
var (
    OutputCapBytes = 64000              // bash/持久 shell 输出上限
    DefaultTimeout = 60 * time.Second   // 未显式传 timeout_seconds 时的默认值
)
func truncateWithSpill(s string, cap int) string
    // s ≤ cap → 原样（超 cap 截断并落盘）
    // f, err := os.CreateTemp("", "dsh-shell-*.log"); f.WriteString(s); 失败 → 降级普通截断
    // 返回 s[:cap] + "\n…[输出超过 <cap> 字节上限，完整输出已落盘：<path>（可用 read_file 读取）]"
```

- 一发式 bash：`CombinedOutput` → `truncateWithSpill`；默认超时用 `DefaultTimeout`。
- 持久 shell：Run 出口（成功/失败路径统一）→ `truncateWithSpill`；默认超时同上。
- app 启动（newApp）从 config 同步两个变量；config 默认值 0 视为内置默认。

```go
// config.go
ShellTimeoutMS int `json:"shell_timeout_ms,omitempty"` // 0 → 60000
ShellOutputCap int `json:"shell_output_cap,omitempty"` // 0 → 64000
```

## 4. UI（Shell 设置卡）

`builtinPluginCard` 中 `p.ID == "bash"` 的展开详情追加两行：

```
命令超时 (ms)  [TextInput ↔ app.fShellTimeout]
输出上限 (bytes) [TextInput ↔ app.fShellCap]
```

渲染后解析并比对 cfg（与插件开关同一同步模式）：可解析且不同 → saveConfig；
不可解析 → 显示红色提示。newApp 时从 cfg 初始化字符串字段。

## 5. 改动文件清单

| 文件 | 改动 | 对应 FR |
|---|---|---|
| `internal/tools/tools.go` | OutputCapBytes/DefaultTimeout 变量、truncateWithSpill、bash 接入 | FR-1/2 |
| `internal/tools/shell.go` | 持久 shell 出口接 truncateWithSpill、默认超时 | FR-1/2 |
| `config.go` | ShellTimeoutMS/ShellOutputCap 字段 + 取值兜底 | FR-2 |
| `app.go` | fShellTimeout/fShellCap 字段、Shell 卡设置 UI、启动同步 | FR-3 |
| `internal/tools/*_test.go`、`app_test.go` | 用例 | AC-1~5 |

## 6. 与 DSH 原版的差异声明

- 落盘文件在系统临时目录（命名 `dsh-shell-*.log`），无 DSH 的 spill 策略包
  （dsh-spill-local 的保留/清理策略）。
- 超时硬上限保持 600s（原版可配任意 ms）。

## 7. 测试计划

| AC | 用例 | 层级 | 断言 |
|---|---|---|---|
| AC-1/2 | TestBashOutputSpill（cap=1KB，输出 4KB → 提示+文件完整；cap 内不落盘） | 单测 | 一发式 |
| AC-1 | TestPersistentShellSpill | 单测 | 持久 shell |
| AC-3 | TestShellTimeoutConfig（默认 1s→sleep 3 失败；显式参数覆盖） | 单测 | 超时 |
| AC-4 | TestShellCardSettings（展开卡可见输入框，改值落盘） | ui.Tester | UI |
| AC-5 | 全量 | `go test ./...` | 无回归（旧 truncate 断言适配） |

## 8. 风险与回退

- **风险**：spill 文件残留（模型没读）→ 系统临时目录由系统清理；文件名前缀
  `dsh-shell-` 可辨识。
- **回退**：OutputCapBytes 恢复 64000 默认即回到原版等价行为；落盘失败自动降级截断。

## 9. 变更记录

- 2026-10-30 定稿；实现与设计一致，无偏离（旧 truncate() 由 truncateWithSpill 取代后
  无其余调用方，已删除）。
