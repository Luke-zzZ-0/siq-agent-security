# M115 原文仓签名启用状态与管理 API

日期：2026-09-13；任务：UX-013；规格：§3.12.24、ADR-048。

本批新增 `local-raw-task-content-activation/v1`。它以本机 Ed25519 身份签名 `enabled=true`、操作者摘要、启用时间、保留秒数、磁盘预算和独立 AES 密钥指纹，是服务重启后恢复原文仓限制的唯一事实源。只创建了密钥或目录的中断状态保持 disabled；Activation 字段、签名、文件权限或密钥绑定异常均进入 error，既不自动覆盖，也不影响默认脱敏决策路径。相同限制的启用重试返回同一记录，不同限制返回冲突；Activation 不授予任何任务采集权限。

本地管理服务新增：

- `GET /v1/raw-task-content/status`：返回 `local-raw-task-content-status/v1`，明确 disabled/ready/error、`default_capture=false`，且只有 ready 输出限制和启用时间。
- `GET /v1/raw-task-content/activation`：返回已复验的完整签名 Activation；未启用为 404，损坏为 503。
- `POST /v1/raw-task-content/activation`：接收 `local-raw-task-content-activate/v1` 严格正文；首次成功为 201，幂等重试为 200，限制冲突为 409。

三个入口只允许配对后的管理会话，决策凭据返回 403；响应均为 `Cache-Control: no-store`。每次 GET 都重新读取并验签磁盘记录及密钥指纹。正文缺失、null、重复字段、未知字段、尾随 JSON、非整数、越界和无效操作者在初始化密钥前拒绝。

验证通过：

- `go test ./internal/rawcontent ./internal/server -run 'Test(Activation|RawTaskContent)' -count=1`。
- `go test ./...` 与 `go vet ./...`（`apps/agentshield`）。
- `go test -race ./internal/rawcontent ./internal/server -run 'Test(Activation|RawTaskContent)' -count=1`。
- Activation、Activate 请求和 Status 固定 Go 样例通过 Python Draft 7 合同；Activation 与 M114 Grant/Revocation 使用固定种子由 Python 独立验签。合同测试共 181 项，Ruff 通过。
- `CGO_ENABLED=0` 下 `linux/amd64`、`linux/arm64`、`darwin/arm64`、`windows/amd64` 四目标编译通过。
- `gofmt -l` 与 `git diff --check` 无输出。

边界：管理服务目前只接入启用和状态；逐任务 Grant/Revoke、运行时采集凭据、密文清单/读取/删除、持续前端提示和诊断包排除检查仍待后续批次。四目标编译不是 macOS/Windows 原生运行证据。本批未提交、推送或合并。
