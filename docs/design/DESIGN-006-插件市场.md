# DESIGN-006：插件市场 设计

- 状态：已落地
- 关联：REQ-006 · TEST-006
- 日期：2026-10-30

## 1. 现状分析

REQ-004 的 `installFromPath` / `installFromGit` 已覆盖安装动作与校验；市场只需补
「索引拉取/解析 + 条目浏览 + 分派安装」。实验开关机制（config.Experimental +
experimentalCard）REQ-004 已就绪。

## 2. 方案对比

| 方案 | 思路 | 优点 | 缺点 | 结论 |
|---|---|---|---|---|
| A：自定义 JSON 索引（URL/本地） | markets.json 存索引地址；fetch+解析；按 source 分派安装 | 最小实现、本地路径可测、语义清晰 | 非官方 npm 协议 | ✅ 采用 |
| B：npm registry 适配 | 查询 registry API | 与 DSH 包生态对齐 | 强网络/协议复杂，起步不需要 | ❌ 后续 |
| C：内置静态市场 | 官方索引硬编码 | 开箱即用 | 无官方源，硬编码很快过期 | ❌ |

## 3. 选定方案（A）

```
market.go（main 包）
  MarketEntry{ID,Name,Version,Description,Source,Origin}
  fetchMarketIndex(src) ([]MarketEntry, error)   // http(s) GET 10s / 本地 ReadFile
  fetchAllIndexes(srcs) ([]MarketEntry, error)   // 合并去重（先到先得），错误聚合
  loadMarketIndexes(dataDir) []string            // <dataDir>/markets.json {"indexes":[]}
  saveMarketIndexes(...)

app
  状态：marketsInput、marketLoading/Loaded/Err、marketEntries、marketFrom map[id]src、
       confirmRemoveMarket string
  refreshMarket()：goroutine 拉取 → onMain 回写（win==nil 时内联，测试友好）
  installFromMarketEntry(e)：source=git → installFromGit；else → installFromPath
  插件页：实验开关「自定义插件市场」开启时，在「已安装插件」与「MCP 服务器」之间
  插入市场段（索引添加/两步删除 + 刷新 + 条目卡）
```

## 4. 数据结构

```jsonc
// <dataDir>/markets.json
{ "indexes": ["https://example.com/mygo-market.json", "/path/to/index.json"] }

// 索引文件
{ "name": "mygo 社区索引", "plugins": [
  { "id": "demo-pack", "name": "示例包", "version": "1.0.0",
    "description": "…", "source": "git", "origin": "https://github.com/x/demo-pack.git" }
] }
```

条目校验：id（pkgIDRe）与 origin 必填，非法条目跳过；name/version 缺省回退 id/"0.0.0"。

## 5. 改动文件清单

| 文件 | 改动 | 对应 FR |
|---|---|---|
| `market.go` | 新增：索引拉取/解析/合并 + markets.json 读写 | FR-2/3 |
| `app.go` | 实验卡第二项、市场段渲染（含异步刷新）、安装/索引增删动作 | FR-1/4/5 |
| `market_test.go`、`app_test.go` | 用例 | AC-1~5 |

## 6. 与 DSH 原版的差异声明

- 市场为插件页分段而非 Discover 标签页（UI 形态差异，REQ §6 已声明）。
- 索引格式为自定义 JSON；无签名校验；npm registry 不支持。

## 7. 测试计划

| AC | 用例 | 层级 | 断言 |
|---|---|---|---|
| AC-3 | TestFetchMarketIndex（本地文件解析、坏 JSON、缺 origin 跳过、多索引去重） | 单测 | 解析与合并 |
| AC-2 | TestMarketsConfigRoundtrip | 单测 | markets.json |
| AC-4 | TestInstallFromMarket（本地 git 仓库 origin） | 单测 | 安装链路 |
| AC-1/4 | TestPluginsPageMarket（开关出现/消失、添加索引、刷新出条目、安装进已装段） | ui.Tester | UI 全流程 |
| AC-5 | 全量 | `go test ./...` | 无回归 |

## 8. 风险与回退

- **风险**：恶意索引诱导安装 → 索引仅展示；安装显式触发且复用全部包校验与审批门。
- **风险**：索引大/慢 → 10s 超时 + 异步刷新，UI 永不阻塞。
- **回退**：实验开关关闭即整体隐藏；markets.json 删除即无索引；代码独立可移除。

## 9. 变更记录

- 2026-10-30 定稿；实现与设计一致，无偏离（marketLoaded 状态位简化为按条目数判断，
  属实现细节）。
