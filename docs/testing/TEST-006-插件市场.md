# TEST-006：插件市场 测试报告

- 状态：通过
- 关联：REQ-006 · DESIGN-006
- 执行日期 / 执行环境：2026-10-30 · macOS 24.1.0 arm64 · Go（标准库）· 安装源用本地 git 仓库（无网络）

## 1. 自动化用例执行记录

全量回归（`go test ./...`，原样输出）：

```
ok  	mygo-harness	2.699s
ok  	mygo-harness/internal/agent	(cached)
ok  	mygo-harness/internal/llm	(cached)
ok  	mygo-harness/internal/mcp	(cached)
ok  	mygo-harness/internal/tools	(cached)
```

新增用例（verbose，原样输出）：

```
--- PASS: TestPluginsPageMarket (0.17s)
--- PASS: TestFetchMarketIndex (0.00s)
--- PASS: TestMarketsConfigRoundtrip (0.00s)
--- PASS: TestInstallFromMarket (0.10s)
```

另：`go vet ./...` 无告警；`go build -trimpath` 构建成功。

## 2. 用例 ↔ 验收标准对照

| FR | 验收标准 | 用例 | 结果 |
|---|---|---|---|
| FR-1 | AC-1 实验开关开→市场段出现、关→消失（开关卡「自定义插件市场」可见） | TestPluginsPageMarket | ✅ |
| FR-2 | AC-2 markets.json 读写；添加/两步删除索引（UI 内验证删除复用 MCP 段同一模式） | TestMarketsConfigRoundtrip · TestPluginsPageMarket | ✅ |
| FR-3 | AC-3 本地索引解析（缺 origin 条目跳过、缺字段回退）、坏 JSON 报错、多索引合并去重 + 单索引失败不影响其他、全失败报错 | TestFetchMarketIndex | ✅ |
| FR-4 | AC-4 刷新出条目（异步拉取 waitFor）、名称/版本/来源徽标可见 | TestPluginsPageMarket | ✅ |
| FR-5 | AC-4 git 来源安装→包进已装段、条目卡变「已安装」；npm 来源拒绝；重复安装拒绝 | TestPluginsPageMarket · TestInstallFromMarket | ✅ |
| FR-6 | 安全：安装走 REQ-004 全部校验（manifest/id 冲突在 TestInstallUninstallPackage 覆盖） | 复用 | ✅ |
| AC-5 | 全量无回归 | `go test ./...` 五包全 ok | ✅ |

## 3. 手工 / 活体核对

形态差异（分区 vs Discover 标签页）已在 REQ §6 声明；开关卡描述与原版
"Custom plugin markets" 对应。索引格式为自定义 JSON，无官方索引可对照——
TEST 以本地索引文件验证全流程，网络 URL 拉取逻辑与本地读取共用同一解析路径。

## 4. 已知问题与后续项

- 索引无签名/防篡改；索引内容不可信，安装动作显式触发且复用包校验与审批门（REQ §6）。
- npm 来源仅显示；无官方内置索引（用户添加起步）。
- 无包更新检查（Plugin manager 标签职责，候选 REQ）。

## 5. 结论

4/4 新用例通过，AC-1~AC-5 全部满足，无回归。FEATURE-MATRIX：`dsh-plugin-market`
❌→🟡（自定义索引市场全流程可用；官方源/npm registry/Discover 标签形态差异见 REQ §6）。
建议 REQ-006 置「已验证」。至此 P0 插件集群（REQ-002~006）全部交付。
