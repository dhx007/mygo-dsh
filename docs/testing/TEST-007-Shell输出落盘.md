# TEST-007：Shell 输出上限 + 落盘 测试报告

- 状态：通过
- 关联：REQ-007 · DESIGN-007
- 执行日期 / 执行环境：2026-10-30 · macOS 24.1.0 arm64 · Go（标准库）

## 1. 自动化用例执行记录

全量回归（`go test ./...`，原样输出）：

```
ok  	mygo-harness	(cached)
ok  	mygo-harness/internal/agent	(cached)
ok  	mygo-harness/internal/llm	(cached)
ok  	mygo-harness/internal/mcp	(cached)
ok  	mygo-harness/internal/tools	6.373s
```

新增用例（verbose，原样输出）：

```
--- PASS: TestBashOutputSpill (0.02s)
--- PASS: TestPersistentShellSpill (0.01s)
--- PASS: TestShellTimeoutConfig (3.03s)
--- PASS: TestShellCardSettings (0.06s)
```

另：`go vet ./...` 无告警；`go build -trimpath` 构建成功。

## 2. 用例 ↔ 验收标准对照

| FR | 验收标准 | 用例 | 结果 |
|---|---|---|---|
| FR-1 | AC-1 一发式 bash：cap=1KB 输出 4KB → 结果含落盘提示、临时文件存在且**完整 4096 字节** | TestBashOutputSpill | ✅ |
| FR-1 | AC-1 持久 shell 同样落盘（≥4096 字节完整保留） | TestPersistentShellSpill | ✅ |
| FR-1 | AC-2 上限内输出不落盘不提示 | TestBashOutputSpill（cap=64000 段） | ✅ |
| FR-2 | AC-3 默认超时 1s → sleep 3 失败（<2.5s 处杀掉）；显式 timeout_seconds 覆盖默认值；持久 shell 同样受约束 | TestShellTimeoutConfig | ✅ |
| FR-3 | AC-4 Shell 卡展开可见「命令超时 (ms) / 输出上限 (bytes)」；改 30000/1024 → config.json 落盘 + tools 包变量同步；非法输入出红色提示 | TestShellCardSettings | ✅ |
| AC-5 | 全量无回归 | `go test ./...` 五包全 ok | ✅ |

## 3. 手工 / 活体核对

参数与语义直接取自活体核对（`docs/evidence/LIVE-UI-2026-10-30.md` §1）：默认值
60000ms/64000B 与原版 Shell 设置卡一致；"spills to a temporary file rather than
being lost" 的行为由 TestBashOutputSpill/TestPersistentShellSpill 以真实文件断言。

## 4. 已知问题与后续项

- 落盘文件在系统临时目录（`dsh-shell-*.log`），无主动清理（依赖系统临时目录策略）；
  结果文本已给模型路径，通常即读即用。
- 超时硬上限保持 600s（REQ §6 声明的安全姿态）。
- PTC run_code（32KB）与包工具（32KB）输出上限未联动此配置（FR-4 明确不变）。

## 5. 结论

4/4 新用例通过，AC-1~AC-5 全部满足，无回归。FEATURE-MATRIX：`dsh-output-retention`
🟡（落盘保留已复刻，保留/清理策略策略包仍归 ⚪ 框架层）；Shell 设置卡对齐原版
View Shell。建议 REQ-007 置「已验证」。
