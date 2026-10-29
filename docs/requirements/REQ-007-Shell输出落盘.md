# REQ-007：Shell 输出上限 64KB + 落盘（spill）+ 可配置超时

- 状态：已验证（TEST-007 全部通过）
- 关联：DESIGN-007 · TEST-007 · FEATURE-MATRIX 行 `工具/dsh-output-retention`、`工具/dsh-tool-bash`
- 日期：2026-10-30
- 证据：活体核对 `docs/evidence/LIVE-UI-2026-10-30.md` §1——Shell 内置插件设置卡：
  Command timeout (ms)=**60000**；Output cap per stream (bytes)=**64000**，
  "Output beyond this **spills to a temporary file** rather than being lost."

## 1. 背景

mygo 的 bash（一发式与持久）输出超 16KB 即**硬截断丢弃**，与原版行为不符：原版
上限 64KB 且超出部分落临时文件保留，模型可再读取。超时默认值也应可配置（原版
60000ms，与 mygo 现默认一致，但不可配置）。

## 2. 用户故事

作为用户，我希望模型跑构建/测试时的大量输出不丢失——超出上限的部分落临时文件，
模型能按提示用 read_file 读到完整输出；我希望能在插件页的 Shell 卡上调整
超时与输出上限（对齐原版 View Shell 设置卡）。

## 3. 功能需求（FR）

- **FR-1（落盘）**：bash（一发式与持久 shell）输出超过上限时，完整输出写入临时文件
  （系统临时目录，进程退出后由系统清理），结果文本保留前 N 字节并追加提示
  `…[输出超过 N 字节上限，完整输出已落盘：<路径>（可用 read_file 读取）]`。
- **FR-2（可配置）**：输出上限（字节，默认 64000）与命令超时默认值（毫秒，默认 60000）
  持久化到 config.json；上限 1KB~16MB 可调，超时默认值 1s~600s 可调（模型显式传
  `timeout_seconds` 时仍以其为准，硬上限 600s 不变）。
- **FR-3（设置卡 UI）**：插件页「Shell 终端」卡展开详情内提供两个输入框
  （命令超时 ms / 输出上限 bytes），对齐原版 View Shell 设置卡；修改即持久化。
- **FR-4（一致性）**：落盘逻辑对一发式 bash 与持久 shell 行为一致；read_file、PTC
  run_code（32KB）、包工具（32KB）的现有上限不变。

## 4. 非功能需求

- **NFR-1** 落盘文件命名含随机后缀，不冲突；写入失败降级为普通截断（不因 spill 失败
  而丢结果）。
- **NFR-2** 纯标准库。

## 5. 验收标准

- AC-1：上限设为 1KB 时执行输出 4KB 的命令 → 结果含落盘提示、临时文件存在且含完整
  输出；一发式与持久 shell 均如此。
- AC-2：上限内输出不落盘、不提示。
- AC-3：超时默认值改为 1（s）时 `sleep 3` 失败并提示超时；模型显式传 timeout_seconds
  时覆盖默认值。
- AC-4：Shell 卡展开可见两个输入框，修改后 config.json 落盘。
- AC-5：`go test ./...` 全绿无回归。

## 6. 范围外

- 图片/二进制输出的独立处理（dsh-compaction-image-offload 语义）。
- 大文件输出的分页读取增强（read_file 已有 offset/limit）。
- 超时硬上限放开（保持 600s 安全姿态）。

## 7. 决策记录

- 落盘位置用系统临时目录而非数据目录：原版语义即 "temporary file"，系统清理兜底，
  不膨胀应用数据。
- 配置经 tools 包可注入变量传入（工具签名无状态保持不变），app 启动时从 config 同步。
