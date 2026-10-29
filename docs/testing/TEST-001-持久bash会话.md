# TEST-001：持久 bash 会话 测试报告

- 状态：通过
- 关联：REQ-001 · DESIGN-001
- 执行日期 / 执行环境：2026-10-30 · macOS 24.1.0 arm64 · Go（go.mod toolchain，纯 Go 无 cgo）

## 1. 自动化用例执行记录

全量回归（`go test ./...`，原样输出）：

```
ok  	mygo-harness	3.618s
ok  	mygo-harness/internal/agent	3.252s
ok  	mygo-harness/internal/llm	3.063s
ok  	mygo-harness/internal/tools	4.423s
```

新增用例（verbose，原样输出）：

```
=== RUN   TestPersistentShellStatePersists
--- PASS: TestPersistentShellStatePersists (0.00s)
=== RUN   TestPersistentShellExitCodeAndStderr
--- PASS: TestPersistentShellExitCodeAndStderr (0.00s)
=== RUN   TestPersistentShellTimeout
--- PASS: TestPersistentShellTimeout (1.01s)
=== RUN   TestPersistentShellRestartAfterExit
--- PASS: TestPersistentShellRestartAfterExit (0.01s)
=== RUN   TestPersistentShellHeredocAndMultiline
--- PASS: TestPersistentShellHeredocAndMultiline (0.01s)
PASS
ok  	mygo-harness/internal/tools	1.186s
=== RUN   TestBashRoutesThroughPersistentShell
--- PASS: TestBashRoutesThroughPersistentShell (0.01s)
PASS
ok  	mygo-harness/internal/agent	0.240s
```

另：`go vet ./...` 无告警；`go build -trimpath -o /tmp/mygo-harness-check .` 构建成功。

## 2. 用例 ↔ 验收标准对照

| FR | 验收标准 | 用例 | 结果 |
|---|---|---|---|
| FR-1/2 | AC-1 env/cwd 跨调用保留 | TestPersistentShellStatePersists（export→echo；cd→pwd） | ✅ |
| FR-3/4 | AC-2 退出码 + stderr 合并 | TestPersistentShellExitCodeAndStderr（false→rc=1 失败；1>&2 并入） | ✅ |
| FR-5 | AC-3 超时杀树并重置 | TestPersistentShellTimeout（sleep 5 @1s 于 1.01s 处失败，随后新 shell 可用） | ✅ |
| FR-6 | AC-4 死亡自愈 + 状态清零 | TestPersistentShellRestartAfterExit（exit 7 → 如实报告；GONE 变量不复存在） | ✅ |
| FR-7 | AC-5 截断复用 | 代码审查：collectLocked 出口经 `truncate()`（16KB 上限，与一发式共用） | ✅ |
| FR-8 | AC-5 标准 preset 不变 | TestPersistentShellPresetWiring（极简挂 Shell、标准不挂） | ✅ |
| FR-9 | AC-5 审批不绕过 | TestBashRoutesThroughPersistentShell（两次调用 approval 均触发，计数=2） | ✅ |
| FR-10 | AC-6 生命周期回收 | TestDropAgentClosesShell（dropAgent 后 Running()=false；Close 幂等） | ✅ |
| — | AC-7 全量无回归 | `go test ./...` 四包全 ok | ✅ |
| — | 附加 | TestPersistentShellHeredocAndMultiline（多行/heredoc 经 stdin 正常执行） | ✅ |

## 3. 手工 / 活体核对

**未做**：活体 UI（127.0.0.1:3080）需 token 认证，本轮无法直接对照原版持久 shell 的
行为细节（尤其：原版超时后是否重启、是否回显提示语）。已按 REQ-001 §6 的假设实现。
→ 遗留：拿到带 token 的启动 URL 后，跑一次极简模式对照核对，结论回填本节。

## 4. 已知问题与后续项

- **长驻子进程的输出粘包**：若命令派生了持有 stdout 的后台进程（如 `nohup server &`），
  该进程在定界行之后的输出会残留管道，混入下一次调用的输出头部（不悬挂、不丢命令）。
  一发式 bash 无此问题；DSH 原版行为未知。跟进：如活体核对发现差异，立 REQ。
- **模型命令读 stdin**（裸 `cat` 等）会吞掉定界行 → 走超时路径（60s 默认），杀树重启，
  结果如实报超时。属设计内行为（DESIGN-001 §8）。
- 终端 UI 面板（人用的持久终端）不在本 REQ 范围，见 FEATURE-MATRIX `tool-terminal`。

## 5. 结论

10/10 用例通过，AC-1~AC-7 全部满足，无回归。FEATURE-MATRIX `dsh-tool-bash-persistent`
已置 ✅；README「已知差距」第 1 条已消除。建议 REQ-001 置「已验证」（活体 UI 核对
作为追加项，不阻塞）。
