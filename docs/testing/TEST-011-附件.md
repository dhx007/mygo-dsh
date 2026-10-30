# TEST-011：附件 测试报告

- 状态：通过
- 关联：REQ-011 · DESIGN-011
- 执行日期 / 执行环境：2026-10-30 · macOS 24.1.0 arm64 · Go（标准库）

## 1. 自动化用例执行记录

全量回归（`go test ./...`，原样输出；附件相关用例连跑 2 遍确认稳定）：

```
ok  	mygo-harness	5.840s
ok  	mygo-harness/internal/agent	(cached)
ok  	mygo-harness/internal/llm	2.202s
ok  	mygo-harness/internal/mcp	(cached)
ok  	mygo-harness/internal/tools	(cached)
```

新增用例（verbose，原样输出）：

```
--- PASS: TestAttachmentTextFlow (0.06s)
--- PASS: TestAttachmentImageFlow (0.02s)
--- PASS: TestComposerAttachUI (0.01s)
--- PASS: TestMessageMultimodalMarshal (0.00s)
```

另：`go vet ./...` 无告警；`go build -trimpath` 构建成功。

## 2. 用例 ↔ 验收标准对照

| FR | 验收标准 | 用例 | 结果 |
|---|---|---|---|
| FR-4/6 | AC-4 多模态序列化：content 数组 + 嵌套 image_url、Unmarshal 还原 Parts、纯文本/工具消息线格式逐字节不变 | TestMessageMultimodalMarshal | ✅ |
| FR-1/2/3/5 | AC-1 文本流：相对路径添加 → chip → 发送注入 `[附件：a.txt]` 围栏 → 轨迹 attachments 名单 → pendingFiles 清空；不存在路径报错 | TestAttachmentTextFlow | ✅ |
| FR-2/4 | AC-2 图片流：png → history content 数组（text part + data URL image part）；气泡 chip；超 8MB 拒绝 | TestAttachmentImageFlow | ✅ |
| FR-1/3 | AC-1 ＋ Popover（添加按钮/上限提示）→ chip → 点 chip 移除 | TestComposerAttachUI | ✅ |
| AC-5 | 全量无回归 | `go test ./...` 五包全 ok | ✅ |

## 3. 手工 / 活体核对

入口形态对齐活体证据的 ＋（"Add files or run commands"）；「选择文件…」按钮仅窗口
模式渲染（调 mygo.Dialog 原生对话框），无头测试走路径输入——两条路径汇入同一
staging 逻辑。OpenAI 多模态 content 数组为通用规范格式，DSH 同构。

## 4. 已知问题与后续项

- 测试期间发现并修复两个用例自身问题：发送后断言缺 tt.Frame()（检查了旧帧）、
  占位符文本不进 ui.Tester.Texts()——产品代码无改动。
- 纯文本模型收到图片 part 时 API 显式报错（REQ §7 声明的可见失败）。
- 文本附件按原文注入，二进制非图片文件会是乱码（后续可加类型检测拒绝）。
- 图片 base64 存于 history.json（8MB/张上限）；附件外置存储留后续。
- read_image 工具、粘贴板图片 → 后续 REQ。

## 5. 结论

4/4 新用例通过，AC-1~AC-5 全部满足，无回归。FEATURE-MATRIX：`client-ui-attachment` /
`dsh-attachment` ❌→🟡（文本+图片附件全链路；附件存储服务/图片工具缺）。
建议 REQ-011 置「已验证」。
