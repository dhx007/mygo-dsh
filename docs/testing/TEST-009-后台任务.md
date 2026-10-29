# TEST-009：后台任务（tool-jobs） 测试报告

- 状态：通过
- 关联：REQ-009 · DESIGN-009
- 执行日期 / 执行环境：2026-10-30 · macOS 24.1.0 arm64 · Go（标准库）

## 1. 自动化用例执行记录

全量回归（`go test ./...`，原样输出）：

```
ok  	mygo-harness	2.696s
ok  	mygo-harness/internal/agent	(cached)
ok  	mygo-harness/internal/llm	(cached)
ok  	mygo-harness/internal/mcp	(cached)
ok  	mygo-harness/internal/tools	(cached)
```

新增/受影响用例（verbose，原样输出）：

```
--- PASS: TestJobLifecycle (0.33s)
--- PASS: TestJobOutputPaging (0.02s)
--- PASS: TestJobKill (0.02s)
--- PASS: TestBashBackgroundParam (隐含于上方运行，PASS)
--- PASS: TestBuiltinPluginRegistry (0.00s)   ← 16 项（新增 后台任务）
--- PASS: TestPluginControlActions (0.01s)
```

另：`go vet ./...` 无告警；`go build -trimpath` 构建成功。

## 2. 用例 ↔ 验收标准对照

| FR | 验收标准 | 用例 | 结果 |
|---|---|---|---|
| FR-1/6 | AC-1 启动即返（running）→ 输出可达 → finished/exit 0；job_list 反映 | TestJobLifecycle | ✅ |
| FR-2 | AC-2 4KB 输出 + cap=1KB → offset 翻页读全量 4096 字节 | TestJobOutputPaging | ✅ |
| FR-4 | AC-3 job_kill 杀 sleep 30 → finished/killed；重复杀已结束任务报错 | TestJobKill | ✅ |
| FR-1 | AC-1 bash `background:true` <1s 返回 job id、job_list 可见 running、未知 job_output 报错 | TestBashBackgroundParam | ✅ |
| FR-5 | AC-4 注册表 16 项；标准/创造挂载三工具（preset 列表 + SpecsFor 通道，一致性测试覆盖映射）；停用经 enabledTool | TestBuiltinPluginRegistry · TestPluginControlActions | ✅ |
| AC-5 | 全量无回归（UI 计数断言同步 16） | `go test ./...` 五包全 ok | ✅ |

## 3. 手工 / 活体核对

三工具命名与职责取自活体核对（`docs/evidence/LIVE-UI-2026-10-30.md` §2.2）：
"Model-facing background job control tools (job_output, job_list, job_kill) over the
ctx.jobs registry"；后台启动入口对齐 tool-bash 描述 "optional generic background-job
support"（`background` 参数）。Task Board 面板属社区插件形态，不在本轮范围（REQ §6）。

## 4. 已知问题与后续项

- 任务无持久化：应用退出时后台进程随宿主终止（子进程不持宿主 stdin，写管道 EPIPE
  自然退出；个别守护化进程可能残留）。
- 输出环形 1MB 丢头部（原版保留策略未知，已在 DESIGN 声明）。
- job_output 首次读取可能遇输出未刷出（running 且缓冲为空返回「没有更多输出」）——
  模型可稍后重试；与真实终端行为一致。
- Task Board 面板、定时任务（dsh-schedule）→ 后续 REQ。

## 5. 结论

5/5 新用例通过，AC-1~AC-5 全部满足，无回归。FEATURE-MATRIX：`dsh-tool-jobs`
❌→🟡（三工具全链路可用；Task Board UI 缺）。建议 REQ-009 置「已验证」。
